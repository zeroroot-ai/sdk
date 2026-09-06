// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"path/filepath"
	"runtime"
	"testing"

	astchecks "github.com/zeroroot-ai/ast-checks"
)

// TestNoGibsonImport asserts that the SDK does not import anything from
// `github.com/zeroroot-ai/gibson`. The SDK is customer-releasable OSS; it
// MUST NOT pull in daemon-internal types, helpers, or infrastructure.
//
// Per workspace CLAUDE.md / org AGENTS.md: "SDK is customer-facing OSS;
// no infrastructure clients permitted." The ImportBoundary primitive
// from ast-checks (slice 3.1) enforces the same rule via typed AST
// inspection — catches aliased imports + transitive references that the
// previous shell-script grep would miss.
//
// Implements slice 3.4 of the production-readiness epic
// (zeroroot-ai/.github#51 → zeroroot-ai/gibson#173).
//
// The shell-script grep version (the previous enforcement) is preserved
// at `scripts/check-no-gibson.sh` (forthcoming follow-up) and as the
// `check-no-gibson` Makefile target. The shell version remains valid as
// an ad-hoc verification tool; this test is the canonical CI gate.
func TestNoGibsonImport(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(thisFile)

	matchers := []astchecks.Matcher{
		astchecks.NewImportBoundary(
			"SDK must not import github.com/zeroroot-ai/gibson — SDK is customer-OSS, gibson is daemon-internal",
			"github.com/zeroroot-ai/gibson",
		),
	}

	// Walk the entire SDK module. ast-checks recurses; vendor/, testdata/,
	// .worktrees/ are skipped by the harness; non-.go files are filtered.
	// Generated bindings (`.pb.go`, `zz_generated*.go`) are skipped via
	// SkipGenerated.
	opts := astchecks.WalkOpts{
		ScopeDirs:     []string{repoRoot},
		RepoRoot:      repoRoot,
		Matchers:      matchers,
		SkipTestFiles: false, // boundary applies to test files too
		SkipGenerated: true,
	}

	findings, err := astchecks.Walk(opts)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(findings) > 0 {
		t.Errorf("SDK imports github.com/zeroroot-ai/gibson (forbidden by workspace policy):\n%s\n\n"+
			"The SDK is customer-releasable OSS. It MUST NOT depend on daemon-internal\n"+
			"packages. If you need a shared type, promote it to the SDK; never reach\n"+
			"into gibson from here.\n\n"+
			"This test replaces the legacy `make check-no-gibson` shell grep with a\n"+
			"typed AST inspection that catches aliased imports the grep would miss.\n",
			astchecks.RenderFindings(findings))
	}
}
