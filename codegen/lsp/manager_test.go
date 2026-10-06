// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp

import (
	"os/exec"
	"testing"
)

// skipIfNoLSPBinaries skips the test when none of the LSP server binaries
// (gopls, pyright-langserver, typescript-language-server) are present in PATH.
// These are integration tests that require external tooling. Pre-existing
// infrastructure requirement; not caused by zitadel-envoy-gateway-migration spec.
func skipIfNoLSPBinaries(t *testing.T) {
	t.Helper()
	binaries := []string{"gopls", "pyright-langserver", "typescript-language-server"}
	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err == nil {
			return // at least one binary is available
		}
	}
	t.Skip("skipping LSP integration test: gopls, pyright-langserver, and typescript-language-server not found in PATH")
}
