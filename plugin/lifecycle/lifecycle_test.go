// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lifecycle_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
)

// TestStateString ensures every State constant has a human-readable name.
func TestStateString(t *testing.T) {
	cases := []struct {
		state lifecycle.State
		want  string
	}{
		{lifecycle.Bootstrapping, "Bootstrapping"},
		{lifecycle.Registering, "Registering"},
		{lifecycle.ResolvingSecrets, "ResolvingSecrets"},
		{lifecycle.Starting, "Starting"},
		{lifecycle.Ready, "Ready"},
		{lifecycle.Degraded, "Degraded"},
		{lifecycle.Draining, "Draining"},
		{lifecycle.Stopped, "Stopped"},
	}
	for _, tc := range cases {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("State(%d).String() = %q, want %q", int(tc.state), got, tc.want)
		}
	}
}

// TestLegalTransitions walks every edge in the allowed graph and verifies
// each one succeeds.
func TestLegalTransitions(t *testing.T) {
	edges := [][2]lifecycle.State{
		{lifecycle.Bootstrapping, lifecycle.Registering},
		{lifecycle.Registering, lifecycle.ResolvingSecrets},
		{lifecycle.ResolvingSecrets, lifecycle.Starting},
		{lifecycle.Starting, lifecycle.Ready},
		{lifecycle.Ready, lifecycle.Draining},
		{lifecycle.Ready, lifecycle.Degraded},
		{lifecycle.Degraded, lifecycle.Ready},
		{lifecycle.Draining, lifecycle.Stopped},
	}

	for _, edge := range edges {
		from, to := edge[0], edge[1]
		// Build a fresh machine that is already in the 'from' state by walking
		// through legal transitions from Bootstrapping.
		sm := advanceTo(t, from)
		if err := sm.Transition(to); err != nil {
			t.Errorf("legal transition %s → %s returned error: %v", from, to, err)
		}
		if got := sm.Current(); got != to {
			t.Errorf("after %s → %s, Current() = %s, want %s", from, to, got, to)
		}
	}
}

// TestForcedStopFromAnyState ensures every non-Stopped state can transition
// directly to Stopped.
func TestForcedStopFromAnyState(t *testing.T) {
	states := []lifecycle.State{
		lifecycle.Bootstrapping,
		lifecycle.Registering,
		lifecycle.ResolvingSecrets,
		lifecycle.Starting,
		lifecycle.Ready,
		lifecycle.Degraded,
		lifecycle.Draining,
	}
	for _, from := range states {
		sm := advanceTo(t, from)
		if err := sm.Transition(lifecycle.Stopped); err != nil {
			t.Errorf("forced stop from %s returned error: %v", from, err)
		}
	}
}

// TestIllegalTransitions verifies that invalid state graph edges are rejected.
func TestIllegalTransitions(t *testing.T) {
	illegal := [][2]lifecycle.State{
		// Going backwards
		{lifecycle.Registering, lifecycle.Bootstrapping},
		{lifecycle.Starting, lifecycle.Registering},
		{lifecycle.Ready, lifecycle.Starting},
		// Skipping states (Stopped is always legal, so excluded from this list)
		{lifecycle.Bootstrapping, lifecycle.Ready},
		{lifecycle.Bootstrapping, lifecycle.Degraded},
		{lifecycle.Bootstrapping, lifecycle.Draining},
		{lifecycle.Registering, lifecycle.Ready},
		{lifecycle.ResolvingSecrets, lifecycle.Ready},
		{lifecycle.Starting, lifecycle.Draining},
		// From terminal Stopped — no outgoing edges at all
		{lifecycle.Stopped, lifecycle.Bootstrapping},
		{lifecycle.Stopped, lifecycle.Ready},
		// From Draining — only Stopped is legal
		{lifecycle.Draining, lifecycle.Ready},
		{lifecycle.Draining, lifecycle.Degraded},
	}

	for _, edge := range illegal {
		from, to := edge[0], edge[1]
		sm := advanceTo(t, from)
		err := sm.Transition(to)
		if err == nil {
			t.Errorf("illegal transition %s → %s should have returned an error", from, to)
			continue
		}
		var te *lifecycle.TransitionError
		if !errors.As(err, &te) {
			t.Errorf("expected *TransitionError for %s → %s, got %T: %v", from, to, err, err)
			continue
		}
		if te.From != from || te.To != to {
			t.Errorf("TransitionError for %s → %s has From=%s To=%s", from, to, te.From, te.To)
		}
	}
}

// TestOnTransitionObserver checks that registered observers fire in order
// with correct from/to values on each successful transition.
func TestOnTransitionObserver(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})

	type event struct{ from, to lifecycle.State }
	var mu sync.Mutex
	var events []event

	sm.OnTransition(func(from, to lifecycle.State) {
		mu.Lock()
		events = append(events, event{from, to})
		mu.Unlock()
	})

	steps := []lifecycle.State{
		lifecycle.Registering,
		lifecycle.ResolvingSecrets,
		lifecycle.Starting,
		lifecycle.Ready,
	}
	for _, target := range steps {
		if err := sm.Transition(target); err != nil {
			t.Fatalf("unexpected error on transition to %s: %v", target, err)
		}
	}

	mu.Lock()
	got := events
	mu.Unlock()

	if len(got) != len(steps) {
		t.Fatalf("expected %d observer events, got %d", len(steps), len(got))
	}
	want := []event{
		{lifecycle.Bootstrapping, lifecycle.Registering},
		{lifecycle.Registering, lifecycle.ResolvingSecrets},
		{lifecycle.ResolvingSecrets, lifecycle.Starting},
		{lifecycle.Starting, lifecycle.Ready},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("event[%d] = {%s,%s}, want {%s,%s}",
				i, got[i].from, got[i].to, w.from, w.to)
		}
	}
}

// TestObserverNotFiredOnIllegalTransition verifies that observers are not
// invoked when a transition is rejected.
func TestObserverNotFiredOnIllegalTransition(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	fired := false
	sm.OnTransition(func(from, to lifecycle.State) { fired = true })

	_ = sm.Transition(lifecycle.Ready) // illegal: Bootstrapping → Ready

	if fired {
		t.Error("observer should not fire on illegal transition")
	}
}

// TestRunOnStartTransitionsToReady verifies that RunOnStart moves the machine
// from Starting to Ready and calls OnStart.
func TestRunOnStartTransitionsToReady(t *testing.T) {
	called := false
	sm := lifecycle.New(lifecycle.LifecycleHooks{
		OnStart: func(ctx context.Context) error {
			called = true
			return nil
		},
	})

	advanceToViaFunc(t, sm, lifecycle.Starting)

	if err := sm.RunOnStart(context.Background()); err != nil {
		t.Fatalf("RunOnStart returned error: %v", err)
	}
	if !called {
		t.Error("OnStart hook was not called")
	}
	if got := sm.Current(); got != lifecycle.Ready {
		t.Errorf("after RunOnStart, state = %s, want Ready", got)
	}
}

// TestRunOnStartHookErrorLeavesStateUnchanged verifies that a hook error
// leaves the machine in Starting.
func TestRunOnStartHookErrorLeavesStateUnchanged(t *testing.T) {
	hookErr := errors.New("startup failed")
	sm := lifecycle.New(lifecycle.LifecycleHooks{
		OnStart: func(ctx context.Context) error { return hookErr },
	})

	advanceToViaFunc(t, sm, lifecycle.Starting)

	if err := sm.RunOnStart(context.Background()); !errors.Is(err, hookErr) {
		t.Fatalf("expected hookErr, got %v", err)
	}
	if got := sm.Current(); got != lifecycle.Starting {
		t.Errorf("state after failed hook = %s, want Starting", got)
	}
}

// TestRunOnStartFromWrongStateReturnsTransitionError verifies that calling
// RunOnStart from a non-Starting state returns a TransitionError.
func TestRunOnStartFromWrongStateReturnsTransitionError(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	// Machine starts in Bootstrapping, not Starting.
	err := sm.RunOnStart(context.Background())
	var te *lifecycle.TransitionError
	if !errors.As(err, &te) {
		t.Errorf("expected *TransitionError, got %T: %v", err, err)
	}
}

// TestRunOnStopTransitionsToDraining verifies that RunOnStop moves the machine
// to Draining and calls OnStop.
func TestRunOnStopTransitionsToDraining(t *testing.T) {
	called := false
	sm := lifecycle.New(lifecycle.LifecycleHooks{
		OnStop: func(ctx context.Context) error {
			called = true
			return nil
		},
	})

	advanceToViaFunc(t, sm, lifecycle.Ready)

	if err := sm.RunOnStop(context.Background()); err != nil {
		t.Fatalf("RunOnStop returned error: %v", err)
	}
	if !called {
		t.Error("OnStop hook was not called")
	}
	if got := sm.Current(); got != lifecycle.Draining {
		t.Errorf("after RunOnStop, state = %s, want Draining", got)
	}
}

// TestMarkDegradedCallsOnDegraded verifies that MarkDegraded transitions the
// machine to Degraded and calls OnDegraded with the supplied reason.
func TestMarkDegradedCallsOnDegraded(t *testing.T) {
	const reason = "secret_revoked: cred:api_key"
	var got string
	sm := lifecycle.New(lifecycle.LifecycleHooks{
		OnDegraded: func(r string) { got = r },
	})

	advanceToViaFunc(t, sm, lifecycle.Ready)

	if err := sm.MarkDegraded(reason); err != nil {
		t.Fatalf("MarkDegraded returned error: %v", err)
	}
	if sm.Current() != lifecycle.Degraded {
		t.Errorf("state = %s, want Degraded", sm.Current())
	}
	if got != reason {
		t.Errorf("OnDegraded reason = %q, want %q", got, reason)
	}
}

// TestMarkDegradedFromIllegalStateReturnsError verifies rejection when
// MarkDegraded is called from a state that does not allow Degraded.
func TestMarkDegradedFromIllegalStateReturnsError(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	// Bootstrapping → Degraded is illegal.
	err := sm.MarkDegraded("test")
	var te *lifecycle.TransitionError
	if !errors.As(err, &te) {
		t.Errorf("expected *TransitionError, got %T: %v", err, err)
	}
}

// TestRecoveryFromDegraded verifies the Degraded → Ready transition (recovery).
func TestRecoveryFromDegraded(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	advanceToViaFunc(t, sm, lifecycle.Ready)
	if err := sm.Transition(lifecycle.Degraded); err != nil {
		t.Fatalf("transition to Degraded: %v", err)
	}
	if err := sm.Transition(lifecycle.Ready); err != nil {
		t.Fatalf("recovery transition Degraded → Ready: %v", err)
	}
	if got := sm.Current(); got != lifecycle.Ready {
		t.Errorf("state = %s, want Ready", got)
	}
}

// TestConcurrentTransitions hammers concurrent Transition calls to verify
// there are no data races (run with -race).
func TestConcurrentTransitions(t *testing.T) {
	// Use a machine that is sitting in Ready → Degraded → Ready cycling.
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	advanceToViaFunc(t, sm, lifecycle.Ready)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			// Each goroutine attempts the Degraded → Ready cycle; some will
			// succeed, some will get TransitionErrors — both are valid outcomes.
			// The important invariant is no race condition.
			_ = sm.Transition(lifecycle.Degraded)
			_ = sm.Transition(lifecycle.Ready)
			_ = sm.Current()
		}(i)
	}
	wg.Wait()
}

// TestMultipleObservers verifies that multiple registered observers all fire.
func TestMultipleObservers(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	count := 0
	sm.OnTransition(func(from, to lifecycle.State) { count++ })
	sm.OnTransition(func(from, to lifecycle.State) { count++ })
	sm.OnTransition(func(from, to lifecycle.State) { count++ })

	if err := sm.Transition(lifecycle.Registering); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 observer calls, got %d", count)
	}
}

// --- helpers ---

// advanceTo creates a new StateMachine and walks it through legal transitions
// until it reaches target state.
func advanceTo(t *testing.T, target lifecycle.State) *lifecycle.StateMachine {
	t.Helper()
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	advanceToViaFunc(t, sm, target)
	return sm
}

// advanceToViaFunc advances an existing machine to the target state using the
// linear happy path: Bootstrapping → Registering → ResolvingSecrets → Starting
// → Ready. Stops at the requested state.
func advanceToViaFunc(t *testing.T, sm *lifecycle.StateMachine, target lifecycle.State) {
	t.Helper()
	path := []lifecycle.State{
		lifecycle.Bootstrapping,
		lifecycle.Registering,
		lifecycle.ResolvingSecrets,
		lifecycle.Starting,
		lifecycle.Ready,
	}

	// Find where target sits on the path.
	idx := -1
	for i, s := range path {
		if s == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		// For states not on the main path (Degraded, Draining, Stopped),
		// advance to the last common ancestor and then do the extra hop.
		switch target {
		case lifecycle.Degraded:
			advanceToViaFunc(t, sm, lifecycle.Ready)
			if err := sm.Transition(lifecycle.Degraded); err != nil {
				t.Fatalf("advance to Degraded: %v", err)
			}
		case lifecycle.Draining:
			advanceToViaFunc(t, sm, lifecycle.Ready)
			if err := sm.Transition(lifecycle.Draining); err != nil {
				t.Fatalf("advance to Draining: %v", err)
			}
		case lifecycle.Stopped:
			if err := sm.Transition(lifecycle.Stopped); err != nil {
				t.Fatalf("force-stop: %v", err)
			}
		default:
			t.Fatalf("advanceToViaFunc: unknown off-path target %s", target)
		}
		return
	}

	// Walk from the machine's current state to target along the path.
	cur := sm.Current()
	curIdx := -1
	for i, s := range path {
		if s == cur {
			curIdx = i
			break
		}
	}
	if curIdx == -1 {
		t.Fatalf("advanceToViaFunc: machine is already at off-path state %s", cur)
	}
	for i := curIdx + 1; i <= idx; i++ {
		if err := sm.Transition(path[i]); err != nil {
			t.Fatalf("advance to %s via %s: %v", target, path[i], err)
		}
	}
}
