// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package bumper

import (
	"context"
	"fmt"
)

// bumpDashboard performs the pnpm-based bump for the dashboard repo.
// The SDK version is not pinned in package.json directly; instead pnpm install
// refreshes the lock file and `npx buf generate` regenerates TS proto bindings
// against the SDK's published buf.gen.yaml artifacts.
//
// Note: actual SDK version pinning for the dashboard lives in package.json as
// a @zeroroot-ai/sdk-ts (or generated TS package) dependency. If such a dependency
// exists, the caller must update package.json before calling this function.
func bumpDashboard(ctx context.Context, runner CommandRunner, repoDir string) ([]byte, error) {
	// Check if pnpm is available.
	if _, err := runner(ctx, repoDir, "pnpm", "--version"); err != nil {
		return nil, fmt.Errorf("pnpm not found in PATH — install pnpm to bump dashboard: %w", err)
	}
	return nil, nil // actual PostBump commands handle pnpm install + buf generate
}
