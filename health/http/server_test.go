// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package http

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/types"
)

// TestRunChecks_AllHealthy verifies that runChecks collects results from all checks.
func TestRunChecks_AllHealthy(t *testing.T) {
	s := NewServer(nil)

	checks := map[string]CheckFunc{
		"check_a": func(ctx context.Context) types.HealthStatus {
			return types.NewHealthyStatus("ok")
		},
		"check_b": func(ctx context.Context) types.HealthStatus {
			return types.NewHealthyStatus("ok")
		},
	}

	results := s.runChecks(context.Background(), checks)
	assert.Len(t, results, 2)
	for _, status := range results {
		assert.True(t, status.IsHealthy())
	}
}

// TestRunChecks_OneUnhealthy verifies that an unhealthy check result is captured.
func TestRunChecks_OneUnhealthy(t *testing.T) {
	s := NewServer(nil)

	checks := map[string]CheckFunc{
		"healthy": func(ctx context.Context) types.HealthStatus {
			return types.NewHealthyStatus("ok")
		},
		"unhealthy": func(ctx context.Context) types.HealthStatus {
			return types.NewUnhealthyStatus("broken", nil)
		},
	}

	results := s.runChecks(context.Background(), checks)
	require.Len(t, results, 2)
	assert.True(t, results["healthy"].IsHealthy())
	assert.True(t, results["unhealthy"].IsUnhealthy())
}

// TestRunChecks_PanicRecovery verifies that a panicking check is caught and
// recorded as unhealthy without crashing the process.
func TestRunChecks_PanicRecovery(t *testing.T) {
	s := NewServer(nil)

	checks := map[string]CheckFunc{
		"panicker": func(ctx context.Context) types.HealthStatus {
			panic("something terrible")
		},
	}

	results := s.runChecks(context.Background(), checks)
	require.Len(t, results, 1)
	assert.True(t, results["panicker"].IsUnhealthy())
	assert.Contains(t, results["panicker"].Message, "check panicked")
}

// TestRunChecks_ContextCancellationPropagates verifies that cancelling the
// parent context propagates into in-flight check functions via the group
// context (gctx). This is the key behavioral guarantee of the errgroup migration.
func TestRunChecks_ContextCancellationPropagates(t *testing.T) {
	s := NewServer(nil)

	// blockUntilCancelled is a CheckFunc that blocks until ctx is cancelled,
	// then records that it saw the cancellation.
	cancelled := make(chan struct{})
	checkStarted := make(chan struct{})

	checks := map[string]CheckFunc{
		"blocking_check": func(ctx context.Context) types.HealthStatus {
			close(checkStarted) // signal that the check has started
			select {
			case <-ctx.Done():
				close(cancelled) // prove we saw the cancellation
				return types.NewUnhealthyStatus("cancelled", nil)
			case <-time.After(5 * time.Second):
				return types.NewHealthyStatus("timed out unexpectedly")
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Run checks in a goroutine (runChecks blocks until all checks return).
	done := make(chan map[string]types.HealthStatus, 1)
	go func() {
		done <- s.runChecks(ctx, checks)
	}()

	// Wait for the check to start, then cancel the parent context.
	select {
	case <-checkStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("check did not start in time")
	}

	cancel()

	// The cancellation should propagate and unblock runChecks.
	select {
	case results := <-done:
		assert.Len(t, results, 1)
		// The check saw the cancellation and returned unhealthy.
		assert.True(t, results["blocking_check"].IsUnhealthy())
	case <-time.After(2 * time.Second):
		t.Fatal("runChecks did not return after context cancellation")
	}

	// Confirm the check's ctx.Done() channel was triggered.
	select {
	case <-cancelled:
		// passed
	case <-time.After(100 * time.Millisecond):
		t.Error("check did not observe context cancellation")
	}
}
