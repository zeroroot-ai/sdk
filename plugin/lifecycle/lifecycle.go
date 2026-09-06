// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package lifecycle implements the plugin lifecycle state machine for the
// Gibson plugin SDK.
//
// A plugin progresses through a well-defined set of states from startup to
// shutdown. The [StateMachine] enforces legal transitions, fires registered
// observer callbacks on every transition, and runs the caller-supplied
// [LifecycleHooks] at the appropriate states.
//
// The state graph (only these transitions are legal):
//
//	Bootstrapping → Registering
//	Registering   → ResolvingSecrets
//	ResolvingSecrets → Starting
//	Starting      → Ready
//	Ready         → Draining
//	Ready         → Degraded
//	Degraded      → Ready (recovery)
//	Draining      → Stopped
//	Any state     → Stopped (forced stop)
//
// All operations are safe for concurrent use.
package lifecycle

import (
	"context"
	"fmt"
	"sync"
)

// State represents a single lifecycle phase of a plugin process.
type State int

const (
	// Bootstrapping is the initial state: the plugin is establishing its
	// daemon connection via the capability-grant bootstrap flow.
	Bootstrapping State = iota

	// Registering indicates the plugin is exchanging its host key for a
	// component registration with the daemon.
	Registering

	// ResolvingSecrets indicates the plugin is pre-resolving all
	// scope=startup, required=true secrets declared in its manifest.
	ResolvingSecrets

	// Starting indicates the plugin has resolved its startup secrets and is
	// running the OnStart lifecycle hook.
	Starting

	// Ready indicates the plugin is fully operational: registered, secrets
	// resolved, OnStart completed, and actively serving work from the
	// PollWork dispatch loop.
	Ready

	// Degraded indicates the plugin is in a reduced-capability state. The
	// plugin continues serving unaffected methods but one or more secrets
	// have been revoked or the daemon connection has been lost for longer
	// than the configured threshold (90 s). OnDegraded is called on entry.
	Degraded

	// Draining indicates the plugin has received a shutdown signal and is
	// waiting for in-flight handlers to complete before stopping. OnStop is
	// called on entry.
	Draining

	// Stopped is the terminal state: the plugin has shut down cleanly.
	Stopped
)

// stateNames maps State values to their human-readable names.
var stateNames = [...]string{
	"Bootstrapping",
	"Registering",
	"ResolvingSecrets",
	"Starting",
	"Ready",
	"Degraded",
	"Draining",
	"Stopped",
}

// String returns the name of the state, e.g. "Ready".
func (s State) String() string {
	if int(s) < len(stateNames) {
		return stateNames[s]
	}
	return fmt.Sprintf("State(%d)", int(s))
}

// allowedTransitions is the adjacency set for the state graph.
// Any transition not present here is illegal.
var allowedTransitions = map[State]map[State]bool{
	Bootstrapping: {
		Registering: true,
		Stopped:     true,
	},
	Registering: {
		ResolvingSecrets: true,
		Stopped:          true,
	},
	ResolvingSecrets: {
		Starting: true,
		Stopped:  true,
	},
	Starting: {
		Ready:   true,
		Stopped: true,
	},
	Ready: {
		Draining: true,
		Degraded: true,
		Stopped:  true,
	},
	Degraded: {
		Ready:    true,
		Draining: true,
		Stopped:  true,
	},
	Draining: {
		Stopped: true,
	},
	Stopped: {},
}

// TransitionError is returned by [StateMachine.Transition] when the requested
// state transition is not permitted by the state graph.
type TransitionError struct {
	// From is the current state at the time of the illegal transition attempt.
	From State
	// To is the target state that was rejected.
	To State
}

// Error implements the error interface.
func (e *TransitionError) Error() string {
	return fmt.Sprintf("lifecycle: illegal transition %s → %s", e.From, e.To)
}

// LifecycleHooks are caller-supplied callbacks invoked at specific state
// transitions. Any nil hook is silently skipped.
type LifecycleHooks struct {
	// OnStart is called when the plugin transitions into the Ready state.
	// A non-nil error from OnStart is returned to the caller of [StateMachine.RunOnStart]
	// and leaves the machine in the Starting state; the caller is responsible
	// for deciding whether to retry or force-stop.
	OnStart func(ctx context.Context) error

	// OnStop is called when the plugin transitions into the Draining state.
	// A non-nil error from OnStop is logged but does not prevent the machine
	// from continuing to Stopped.
	OnStop func(ctx context.Context) error

	// OnDegraded is called when the plugin transitions into the Degraded state.
	// reason is a human-readable string identifying the cause (e.g.
	// "secret_revoked: cred:db_password", "daemon_disconnected").
	OnDegraded func(reason string)
}

// StateMachine is a concurrent-safe plugin lifecycle state machine.
// Create one via [New]; the zero value is not valid.
type StateMachine struct {
	mu        sync.Mutex
	state     State
	hooks     LifecycleHooks
	observers []func(from, to State)
}

// New constructs a new [StateMachine] starting in the [Bootstrapping] state.
// hooks may be a zero value if no lifecycle callbacks are needed.
func New(hooks LifecycleHooks) *StateMachine {
	return &StateMachine{
		state: Bootstrapping,
		hooks: hooks,
	}
}

// Current returns the machine's current state. Safe for concurrent use.
func (s *StateMachine) Current() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// OnTransition registers fn as an observer that is called synchronously after
// every successful state transition. Multiple observers are called in
// registration order. The observer must not call [Transition] or [Current]
// on the same machine (deadlock).
func (s *StateMachine) OnTransition(fn func(from, to State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, fn)
}

// Transition atomically advances the machine to target. It returns a
// [*TransitionError] if the transition is not permitted by the state graph.
//
// Observer callbacks registered with [OnTransition] are invoked inside the
// lock after the state update; they must not call back into the machine.
func (s *StateMachine) Transition(target State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	from := s.state
	allowed, exists := allowedTransitions[from]
	if !exists || !allowed[target] {
		return &TransitionError{From: from, To: target}
	}

	s.state = target

	// Fire observers while holding the lock so observers see a consistent state.
	for _, fn := range s.observers {
		fn(from, target)
	}

	return nil
}

// RunOnStart transitions the machine from Starting to Ready and runs the
// OnStart hook if one is configured.
//
// It returns a [*TransitionError] if the machine is not currently in the
// Starting state. If OnStart returns an error the machine remains in Starting
// and the error is returned to the caller.
func (s *StateMachine) RunOnStart(ctx context.Context) error {
	// Verify we are in Starting before running the hook; transition to Ready
	// only after the hook succeeds so the probe does not return 200 prematurely.
	s.mu.Lock()
	if s.state != Starting {
		from := s.state
		s.mu.Unlock()
		return &TransitionError{From: from, To: Ready}
	}
	hook := s.hooks.OnStart
	s.mu.Unlock()

	if hook != nil {
		if err := hook(ctx); err != nil {
			return err
		}
	}

	return s.Transition(Ready)
}

// RunOnStop transitions the machine from the current state to Draining and
// runs the OnStop hook. The hook error is returned to the caller but does not
// prevent the machine from reaching Draining.
//
// It returns a [*TransitionError] if transitioning to Draining is not legal
// from the current state (e.g. already Stopped).
func (s *StateMachine) RunOnStop(ctx context.Context) error {
	if err := s.Transition(Draining); err != nil {
		return err
	}

	s.mu.Lock()
	hook := s.hooks.OnStop
	s.mu.Unlock()

	if hook != nil {
		return hook(ctx)
	}
	return nil
}

// MarkDegraded transitions the machine to Degraded (if the current state
// permits) and calls OnDegraded with reason. If OnDegraded is not configured
// the call is a no-op beyond the state transition.
//
// It returns a [*TransitionError] if transitioning to Degraded is not legal
// from the current state.
func (s *StateMachine) MarkDegraded(reason string) error {
	if err := s.Transition(Degraded); err != nil {
		return err
	}

	s.mu.Lock()
	hook := s.hooks.OnDegraded
	s.mu.Unlock()

	if hook != nil {
		hook(reason)
	}
	return nil
}
