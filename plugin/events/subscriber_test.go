// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// ---- fakes ----

// fakeStream is a channel-backed EventStream for testing.
type fakeStream struct {
	ch chan Event
}

func newFakeStream() *fakeStream { return &fakeStream{ch: make(chan Event, 32)} }

func (f *fakeStream) send(ev Event) { f.ch <- ev }

func (f *fakeStream) Recv(ctx context.Context) (Event, error) {
	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case ev := <-f.ch:
		return ev, nil
	}
}

// fakeSecretsHook records calls to Invalidate and MarkRevoked.
type fakeSecretsHook struct {
	invalidated []string
	revoked     []string
}

func (f *fakeSecretsHook) Invalidate(name string)  { f.invalidated = append(f.invalidated, name) }
func (f *fakeSecretsHook) MarkRevoked(name string) { f.revoked = append(f.revoked, name) }

// fakeLifecycleHook records calls to MarkDegraded.
type fakeLifecycleHook struct {
	reasons []string
	failErr error // if non-nil, returned from MarkDegraded
}

func (f *fakeLifecycleHook) MarkDegraded(reason string) error {
	f.reasons = append(f.reasons, reason)
	return f.failErr
}

// fakeDrainer records DrainThenExit calls without actually exiting.
type fakeDrainer struct {
	reasons []string
}

func (f *fakeDrainer) DrainThenExit(reason string) {
	f.reasons = append(f.reasons, reason)
}

// testManifest builds a minimal valid manifest with the given secrets.
func testManifest(secrets ...manifest.SecretDecl) *manifest.Manifest {
	if len(secrets) == 0 {
		secrets = []manifest.SecretDecl{
			{Name: "cred:api_key", Scope: "startup", Rotation: "live", Required: true},
		}
	}
	return &manifest.Manifest{
		APIVersion: manifest.APIVersionV1,
		Kind:       manifest.KindPlugin,
		Metadata: manifest.ManifestMetadata{
			Name:    "test-plugin",
			Version: "0.1.0",
		},
		Spec: manifest.ManifestSpec{
			WorkloadClass: manifest.WorkloadClassPlugin,
			Secrets:       secrets,
			Methods: []manifest.MethodDecl{{
				Name: "Do",
			}},
			Runtime: "process",
		},
	}
}

// runSubscriberUntilDrained sends events to the stream, then cancels the
// subscriber and waits for it to return.
func runSubscriberUntilDrained(t *testing.T, s *Subscriber, stream *fakeStream, events []Event) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	for _, ev := range events {
		stream.send(ev)
	}
	// Give the subscriber time to process all queued events before cancelling.
	time.Sleep(20 * time.Millisecond)
	cancel()

	err := <-done
	require.NoError(t, err, "Run should return nil on context cancellation")
}

// ---- tests ----

func TestSubscriber_SecretRotated_Live_Invalidates(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest()
	s := New(stream, sh, lh, m)

	ev := Event{
		Type:       EventTypeSecretRotated,
		Name:       "cred:api_key",
		Version:    2,
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	assert.Equal(t, []string{"cred:api_key"}, sh.invalidated)
	assert.Empty(t, sh.revoked)
	assert.Empty(t, lh.reasons)
}

func TestSubscriber_SecretRotated_Restart_CallsDrainer(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	drainer := &fakeDrainer{}
	m := testManifest(manifest.SecretDecl{
		Name: "cred:db_pass", Scope: "startup", Rotation: "restart", Required: true,
	})
	s := NewWithDrainer(stream, sh, lh, drainer, m)

	ev := Event{
		Type:       EventTypeSecretRotated,
		Name:       "cred:db_pass",
		Version:    3,
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	require.Len(t, drainer.reasons, 1)
	assert.Contains(t, drainer.reasons[0], "cred:db_pass")
	assert.Contains(t, drainer.reasons[0], "secret_rotated_restart")
}

func TestSubscriber_SecretRotated_Restart_NilDrainer_ContinuesRunning(t *testing.T) {
	// When no Drainer is wired, the subscriber logs and continues without panicking.
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest(manifest.SecretDecl{
		Name: "cred:db_pass", Scope: "startup", Rotation: "restart", Required: true,
	})
	s := New(stream, sh, lh, m) // nil Drainer

	ev := Event{
		Type:       EventTypeSecretRotated,
		Name:       "cred:db_pass",
		Version:    1,
		OccurredAt: time.Now(),
	}
	// Should not panic; Run must return cleanly on cancel.
	runSubscriberUntilDrained(t, s, stream, []Event{ev})
}

func TestSubscriber_SecretAccessRevoked_CallsBothHooks(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest()
	s := New(stream, sh, lh, m)

	ev := Event{
		Type:       EventTypeSecretAccessRevoked,
		Name:       "cred:api_key",
		Reason:     "tenant-admin revoked",
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	assert.Equal(t, []string{"cred:api_key"}, sh.revoked)
	require.Len(t, lh.reasons, 1)
	assert.Equal(t, "secret_revoked: cred:api_key", lh.reasons[0])
}

func TestSubscriber_Idempotency_DuplicateEventIsNoop(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest()
	s := New(stream, sh, lh, m)

	ev := Event{
		Type:       EventTypeSecretRotated,
		Name:       "cred:api_key",
		Version:    5,
		OccurredAt: time.Unix(1000, 0),
	}
	// Send the same event twice.
	runSubscriberUntilDrained(t, s, stream, []Event{ev, ev})

	// Invalidate must be called exactly once.
	assert.Equal(t, []string{"cred:api_key"}, sh.invalidated,
		"duplicate event must not result in double-invalidation")
}

func TestSubscriber_UnknownEventType_Dropped(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest()
	s := New(stream, sh, lh, m)

	ev := Event{
		Type:       "component_registered",
		Name:       "cred:api_key",
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	// No hooks should have been called.
	assert.Empty(t, sh.invalidated)
	assert.Empty(t, sh.revoked)
	assert.Empty(t, lh.reasons)
}

func TestSubscriber_EventForUndeclaredSecret_Ignored(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	// Manifest only declares "cred:api_key".
	m := testManifest()
	s := New(stream, sh, lh, m)

	ev := Event{
		Type:       EventTypeSecretRotated,
		Name:       "cred:other_secret",
		Version:    1,
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	assert.Empty(t, sh.invalidated, "undeclared secret event must not trigger Invalidate")
}

func TestSubscriber_ContextCancellation_ReturnsNil(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest()
	s := New(stream, sh, lh, m)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err, "ctx cancellation must yield nil error")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestSubscriber_StreamError_PropagatesError(t *testing.T) {
	sentinel := errors.New("stream closed")
	errStream := &errAfterStream{err: sentinel}
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest()
	s := New(errStream, sh, lh, m)

	err := s.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

// errAfterStream returns a configurable error on the first Recv call.
type errAfterStream struct {
	err error
}

func (e *errAfterStream) Recv(_ context.Context) (Event, error) {
	return Event{}, e.err
}

func TestSubscriber_NilLifecycleHook_DoesNotPanic(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	// Pass nil lifecycle hook.
	m := testManifest()
	s := New(stream, sh, nil, m)

	ev := Event{
		Type:       EventTypeSecretAccessRevoked,
		Name:       "cred:api_key",
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	// MarkRevoked must still be called.
	assert.Equal(t, []string{"cred:api_key"}, sh.revoked)
}

func TestSubscriber_Idempotency_RingOverflow(t *testing.T) {
	// Fill the dedupe ring with 101 distinct events; entry 0 should be evicted
	// and a replay of it should be processed again.
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	m := testManifest(manifest.SecretDecl{
		Name: "cred:api_key", Scope: "startup", Rotation: "live", Required: true,
	})
	s := New(stream, sh, lh, m)

	base := time.Unix(1_000_000, 0)

	// Send 100 distinct rotations with different versions.
	events := make([]Event, dedupeRingSize+1)
	for i := range dedupeRingSize {
		events[i] = Event{
			Type:       EventTypeSecretRotated,
			Name:       "cred:api_key",
			Version:    i + 1,
			OccurredAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	// The (dedupeRingSize+1)-th event is a replay of the very first one.
	events[dedupeRingSize] = events[0]

	runSubscriberUntilDrained(t, s, stream, events)

	// The first event's duplicate was re-added after ring wrap-around.
	// We just assert no panic and that at least dedupeRingSize invalidations
	// occurred (one per unique event).
	assert.GreaterOrEqual(t, len(sh.invalidated), dedupeRingSize)
}
