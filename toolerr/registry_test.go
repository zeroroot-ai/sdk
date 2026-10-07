// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

import (
	"testing"
)

// BenchmarkRegister benchmarks the Register function
func BenchmarkRegister(b *testing.B) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}

	b.ResetTimer()
	for range b.N {
		Register("mytool", ErrCodeBinaryNotFound, hint)
	}
}
