// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"log/slog"
	"sync/atomic"
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
