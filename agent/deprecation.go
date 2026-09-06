// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"log/slog"
	"sync"
)

var deprecatedMethodWarnings = struct {
	mu     sync.Mutex
	warned map[string]bool
}{warned: make(map[string]bool)}

// logDeprecationOnce logs a deprecation warning for a method exactly once per process lifetime.
// Subsequent calls with the same method name are silently ignored.
func logDeprecationOnce(logger *slog.Logger, method, replacement, removalVersion string) {
	deprecatedMethodWarnings.mu.Lock()
	defer deprecatedMethodWarnings.mu.Unlock()

	if deprecatedMethodWarnings.warned[method] {
		return
	}
	deprecatedMethodWarnings.warned[method] = true
	logger.Warn("deprecated method called",
		"method", method,
		"replacement", replacement,
		"removal_version", removalVersion,
	)
}
