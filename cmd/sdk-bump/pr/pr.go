// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package pr wraps the gh CLI for PR creation and CI status checks.
package pr

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CommandRunner is the function signature for running external commands.
// This type is intentionally identical to bumper.CommandRunner; callers may
// use a simple conversion: pr.CommandRunner(myBumperRunner).
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

// CheckResult holds the outcome of a `gh pr checks` call.
type CheckResult struct {
	// PRNumber is the PR number checked.
	PRNumber string
	// Output is the raw output from gh pr checks.
	Output string
	// Success is true if gh pr checks exited 0.
	Success bool
	// Err is non-nil if the command failed.
	Err error
}

// CheckPR calls `gh pr checks <prRef>` after an optional delay and returns the result.
// prRef can be a PR number (e.g. "42") or a URL.
// delay of 0 means no sleep before checking.
//
// Note: gh pr checks may return non-zero if checks are still pending. Callers should
// treat a non-zero exit with "pending" in the output as a warning, not a hard failure.
func CheckPR(ctx context.Context, runner CommandRunner, repoDir string, prRef string, delay time.Duration) CheckResult {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return CheckResult{PRNumber: prRef, Err: ctx.Err()}
		case <-time.After(delay):
		}
	}

	out, err := runner(ctx, repoDir, "gh", "pr", "checks", prRef)
	return CheckResult{
		PRNumber: prRef,
		Output:   string(out),
		Success:  err == nil,
		Err:      err,
	}
}

// DefaultCheckDelay is the default time to wait after opening a PR before
// polling CI status.
const DefaultCheckDelay = 30 * time.Second

// FormatCheckResult returns a human-readable summary of a CheckResult.
func FormatCheckResult(cr CheckResult) string {
	var sb strings.Builder
	if cr.Err != nil {
		fmt.Fprintf(&sb, "  CI check FAILED (pr %s): %v\n", cr.PRNumber, cr.Err)
	} else {
		fmt.Fprintf(&sb, "  CI check OK (pr %s)\n", cr.PRNumber)
	}
	if cr.Output != "" {
		for _, line := range strings.Split(strings.TrimRight(cr.Output, "\n"), "\n") {
			fmt.Fprintf(&sb, "    %s\n", line)
		}
	}
	return sb.String()
}
