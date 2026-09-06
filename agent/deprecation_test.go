// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// countingHandler is a minimal slog.Handler that counts the number of log
// records it receives. It is safe for concurrent use via an atomic counter.
type countingHandler struct {
	count atomic.Int64
}

func (h *countingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *countingHandler) Handle(_ context.Context, _ slog.Record) error {
	h.count.Add(1)
	return nil
}

func (h *countingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *countingHandler) WithGroup(_ string) slog.Handler      { return h }

// resetDeprecationWarnings clears the package-level warned map and registers a
// cleanup function to restore the original map when the test finishes.
func resetDeprecationWarnings(t *testing.T) {
	t.Helper()
	deprecatedMethodWarnings.mu.Lock()
	original := deprecatedMethodWarnings.warned
	deprecatedMethodWarnings.warned = make(map[string]bool)
	deprecatedMethodWarnings.mu.Unlock()

	t.Cleanup(func() {
		deprecatedMethodWarnings.mu.Lock()
		deprecatedMethodWarnings.warned = original
		deprecatedMethodWarnings.mu.Unlock()
	})
}

// TestLogDeprecationOnce_LogsOnceOnly verifies that calling logDeprecationOnce
// 100 times with the same method name results in exactly one log entry.
func TestLogDeprecationOnce_LogsOnceOnly(t *testing.T) {
	resetDeprecationWarnings(t)

	handler := &countingHandler{}
	logger := slog.New(handler)

	const method = "OldMethod"
	for range 100 {
		logDeprecationOnce(logger, method, "NewMethod", "v2.0")
	}

	assert.Equal(t, int64(1), handler.count.Load(),
		"logDeprecationOnce should emit exactly one log entry regardless of call count")
}

// TestLogDeprecationOnce_DifferentMethodsLogSeparately verifies that each
// distinct method name is logged independently, producing one entry per name.
func TestLogDeprecationOnce_DifferentMethodsLogSeparately(t *testing.T) {
	resetDeprecationWarnings(t)

	handler := &countingHandler{}
	logger := slog.New(handler)

	logDeprecationOnce(logger, "MethodA", "NewMethodA", "v2.0")
	logDeprecationOnce(logger, "MethodB", "NewMethodB", "v2.0")

	// Call both again to confirm the deduplication is per-method.
	logDeprecationOnce(logger, "MethodA", "NewMethodA", "v2.0")
	logDeprecationOnce(logger, "MethodB", "NewMethodB", "v2.0")

	assert.Equal(t, int64(2), handler.count.Load(),
		"logDeprecationOnce should emit one log entry per unique method name")
}
