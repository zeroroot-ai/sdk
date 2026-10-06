// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package events implements the plugin-side component callback subscriber.
//
// The Subscriber consumes an EventStream (backed in production by the daemon's
// existing component callback channel) and dispatches rotation and revocation
// events to the secrets client and lifecycle state machine.
//
// Event types handled:
//   - "secret_rotated":        cache invalidation, so the next Resolve fetches the new value.
//   - "secret_access_revoked": cache revocation + lifecycle degradation.
//   - all other types:         logged at debug level and silently dropped.
//
// Idempotency is maintained via a fixed-size ring buffer of recently-seen
// (Type, Name, Version, OccurredAt) tuples. Duplicate events are no-ops.
//
// Spec: plugin-runtime Requirement 10.
package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// EventTypeSecretRotated is the event type string for secret rotation events.
const EventTypeSecretRotated = "secret_rotated"

// EventTypeSecretAccessRevoked is the event type string for secret revocation
// events.
const EventTypeSecretAccessRevoked = "secret_access_revoked"

// dedupeRingSize is the number of recently-observed events kept for
// idempotency checking. Requirement 10.5.
const dedupeRingSize = 100

// Event is a single notification received from the component callback stream.
type Event struct {
	// Type identifies the event kind. Values handled by this package are
	// EventTypeSecretRotated and EventTypeSecretAccessRevoked. Any other
	// value is silently dropped after a debug-level log.
	Type string

	// Name is the secret name referenced by the event.
	Name string

	// Reason is an optional human-readable explanation, present on
	// secret_access_revoked events.
	Reason string

	// Version is the new secret version, present on secret_rotated events.
	// Zero means unknown.
	Version int

	// OccurredAt is the server-side timestamp of the event.
	OccurredAt time.Time
}

// EventStream is the abstract source of events. In production it is wired to
// the daemon's component callback gRPC stream. In tests a simple channel-based
// fake is used.
type EventStream interface {
	// Recv returns the next event or an error (e.g. context cancellation,
	// connection closed). Recv must block until an event is available.
	Recv(ctx context.Context) (Event, error)
}

// SecretsHook is the subset of plugin/secrets.Client needed by the Subscriber.
// Defining a minimal interface here avoids an import cycle and keeps coupling
// loose between the events and secrets packages.
type SecretsHook interface {
	// Invalidate drops the cached value for name, forcing the next Resolve
	// call to fetch a fresh value from the daemon.
	Invalidate(name string)

	// MarkRevoked sets a permanent denied flag for name. Subsequent Resolve
	// calls for this name return ErrPermissionDenied without RPC.
	MarkRevoked(name string)
}

// LifecycleHook is the subset of plugin/lifecycle.StateMachine the Subscriber
// needs. Keeping this as a minimal interface avoids importing the lifecycle
// package directly and makes testing straightforward.
type LifecycleHook interface {
	// MarkDegraded transitions the state machine to Degraded (if the
	// transition is legal) and calls the OnDegraded hook with reason.
	MarkDegraded(reason string) error
}

// dedupeKey is the comparable key used for idempotency checking.
type dedupeKey struct {
	eventType  string
	name       string
	version    int
	occurredAt time.Time
}

// RotationCallback is the optional callback invoked after a secret_rotated
// event has been processed (the cache entry invalidated). The metrics package
// wires this to record
// gibson_plugin_rotation_propagation_seconds.
//
// name is the secret name from the event. lag is the wall-clock time
// between event.OccurredAt (server-side timestamp) and the moment the
// subscriber finished processing it. lag may be negative under clock skew;
// the metric recorder clamps to zero in that case.
type RotationCallback func(name string, lag time.Duration)

// Subscriber consumes the component callback stream and dispatches
// rotation/revocation events to the secrets client and lifecycle hook.
type Subscriber struct {
	stream     EventStream
	secrets    SecretsHook
	lifecycle  LifecycleHook
	onRotation RotationCallback // may be nil

	// dedupeRing holds the last dedupeRingSize event keys for idempotency.
	dedupeRing [dedupeRingSize]dedupeKey
	ringHead   int // index of the slot to overwrite next (circular)
}

// SetOnRotation registers cb as the rotation observation callback.
// Subsequent secret_rotated events fire cb after Invalidate has returned. SetOnRotation is not safe for
// concurrent use with Run; call it before Run is invoked.
//
// Passing nil clears any previously set callback.
func (s *Subscriber) SetOnRotation(cb RotationCallback) {
	s.onRotation = cb
}

// New constructs a Subscriber.
func New(stream EventStream, secretsHook SecretsHook, lifecycleHook LifecycleHook) *Subscriber {
	return &Subscriber{
		stream:    stream,
		secrets:   secretsHook,
		lifecycle: lifecycleHook,
	}
}

// Run blocks, consuming events from the stream until ctx is cancelled or the
// stream returns an error.
//
// Reconnect logic is the responsibility of the EventStream implementation —
// this subscriber simply consumes whatever the stream provides.
//
// Run returns nil when ctx is cancelled. Any other stream error is returned
// directly.
func (s *Subscriber) Run(ctx context.Context) error {
	for {
		ev, err := s.stream.Recv(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled — clean exit.
				return nil
			}
			return fmt.Errorf("events: stream error: %w", err)
		}
		s.dispatch(ev)
	}
}

// dispatch routes a single event. All errors from hook calls are logged but
// do not terminate the subscriber loop.
func (s *Subscriber) dispatch(ev Event) {
	// Idempotency check.
	key := dedupeKey{
		eventType:  ev.Type,
		name:       ev.Name,
		version:    ev.Version,
		occurredAt: ev.OccurredAt,
	}
	if s.isDuplicate(key) {
		slog.Debug("events: duplicate event dropped",
			"type", ev.Type, "name", ev.Name, "version", ev.Version)
		return
	}
	s.recordSeen(key)

	switch ev.Type {
	case EventTypeSecretRotated:
		s.handleRotated(ev)
	case EventTypeSecretAccessRevoked:
		s.handleRevoked(ev)
	default:
		slog.Debug("events: unknown event type, dropping",
			"type", ev.Type, "name", ev.Name)
	}
}

// handleRotated handles secret_rotated events. It invalidates the cached
// value, so the next Resolve fetches the new version, and then fires the
// OnRotation callback when one is configured so the metrics recorder can
// observe end-to-end propagation lag. A secret this plugin never resolved has
// no cache entry, and the invalidation is a no-op.
func (s *Subscriber) handleRotated(ev Event) {
	s.secrets.Invalidate(ev.Name)
	slog.Info("events: secret rotated, cache invalidated",
		"name", ev.Name, "version", ev.Version)
	s.fireOnRotation(ev)
}

// fireOnRotation invokes the OnRotation callback with the propagation lag
// when one is configured. lag is zero (rather than negative) when
// ev.OccurredAt is unset.
func (s *Subscriber) fireOnRotation(ev Event) {
	if s.onRotation == nil {
		return
	}
	var lag time.Duration
	if !ev.OccurredAt.IsZero() {
		lag = time.Since(ev.OccurredAt)
	}
	s.onRotation(ev.Name, lag)
}

// handleRevoked handles secret_access_revoked events. The daemon sends one
// only for a grant this plugin held, so each one marks the secret revoked and
// degrades the plugin.
func (s *Subscriber) handleRevoked(ev Event) {
	s.secrets.MarkRevoked(ev.Name)
	slog.Info("events: secret access revoked, cache invalidated and flag set",
		"name", ev.Name, "reason", ev.Reason)

	if s.lifecycle != nil {
		reason := "secret_revoked: " + ev.Name
		if err := s.lifecycle.MarkDegraded(reason); err != nil {
			slog.Warn("events: MarkDegraded returned error",
				"name", ev.Name, "err", err)
		}
	}
}

// isDuplicate reports whether key has been seen within the ring buffer window.
// It performs a linear scan of the ring (O(dedupeRingSize) = O(100)).
func (s *Subscriber) isDuplicate(key dedupeKey) bool {
	for i := range dedupeRingSize {
		if s.dedupeRing[i] == key {
			return true
		}
	}
	return false
}

// recordSeen writes key into the ring buffer, overwriting the oldest slot.
func (s *Subscriber) recordSeen(key dedupeKey) {
	s.dedupeRing[s.ringHead] = key
	s.ringHead = (s.ringHead + 1) % dedupeRingSize
}
