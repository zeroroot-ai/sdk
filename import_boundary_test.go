// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// forbiddenModules is the one list of modules that no file in the sdk may
// import, keyed by module path (ADR-0058, sdk#188).
//
// The sdk holds what a developer needs to write, package, enroll and run a
// component. It holds nothing about how to operate the platform. So it imports
// neither the daemon nor a client of a platform back end: a component reaches
// secrets, identity and authorization through the daemon's RPCs, never by
// talking to the store behind them.
//
// A key is a module path. It matches that module and its packages, and never a
// module whose name only starts with the same text: github.com/zeroroot-ai/gibson
// does not match github.com/zeroroot-ai/gibson-executor.
var forbiddenModules = map[string]string{
	"github.com/zeroroot-ai/gibson": "the daemon. The daemon imports the sdk, never the reverse",
	"github.com/hashicorp/vault":    "a Vault client. Secrets reach a component through the daemon",
	"github.com/openbao/openbao":    "an OpenBao client. Secrets reach a component through the daemon",
	"github.com/zitadel/zitadel-go": "a Zitadel client. Identity is the daemon's job",
	"github.com/zitadel/zitadel":    "the Zitadel server module. Identity is the daemon's job",
	"github.com/openfga/go-sdk":     "an OpenFGA client. Authorization is decided by the daemon",
	"github.com/openfga/openfga":    "the OpenFGA server module. Authorization is decided by the daemon",
}

// forbiddenImport reports the forbidden module that an import path belongs to.
func forbiddenImport(importPath string) (module, reason string, forbidden bool) {
	for mod, why := range forbiddenModules {
		if importPath == mod || strings.HasPrefix(importPath, mod+"/") {
			return mod, why, true
		}
	}
	return "", "", false
}

// importViolation is one forbidden import.
type importViolation struct {
	file, importPath, module, reason string
}

// scanImports parses the import block of every .go file under root, test files
// included, and returns each forbidden import. It skips testdata, vendor and
// the worktree and git directories, as the Go tool does.
func scanImports(root string) (violations []importViolation, files int, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor", ".worktrees", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		files++
		for _, imp := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(imp.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if mod, why, bad := forbiddenImport(importPath); bad {
				rel, _ := filepath.Rel(root, path)
				violations = append(violations, importViolation{rel, importPath, mod, why})
			}
		}
		return nil
	})
	return violations, files, err
}

// TestImportBoundary walks the whole module and fails on an import of the
// daemon or of a platform back-end client.
func TestImportBoundary(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(thisFile)

	violations, files, err := scanImports(repoRoot)
	if err != nil {
		t.Fatalf("scan imports: %v", err)
	}
	// A floor. A walk that parsed almost nothing reads as a clean module.
	if files < 200 {
		t.Fatalf("parsed %d Go files under %s, want at least 200: the walk did not reach the module", files, repoRoot)
	}
	for _, v := range violations {
		t.Errorf("%s imports %q, which belongs to %s: %s (ADR-0058)", v.file, v.importPath, v.module, v.reason)
	}
}

// TestForbiddenImport is the failing fixture of the rule: each back-end client
// is refused under its module, and a module whose name only starts with a
// forbidden one is not.
func TestForbiddenImport(t *testing.T) {
	cases := []struct {
		importPath string
		wantModule string
	}{
		{"github.com/zeroroot-ai/gibson", "github.com/zeroroot-ai/gibson"},
		{"github.com/zeroroot-ai/gibson/internal/platform/authz", "github.com/zeroroot-ai/gibson"},
		{"github.com/hashicorp/vault/api", "github.com/hashicorp/vault"},
		{"github.com/hashicorp/vault/sdk/helper/consts", "github.com/hashicorp/vault"},
		{"github.com/openbao/openbao/api/v2", "github.com/openbao/openbao"},
		{"github.com/zitadel/zitadel-go/v3/pkg/client", "github.com/zitadel/zitadel-go"},
		{"github.com/zitadel/zitadel/pkg/grpc/management", "github.com/zitadel/zitadel"},
		{"github.com/openfga/go-sdk/client", "github.com/openfga/go-sdk"},
		{"github.com/openfga/openfga/pkg/tuple", "github.com/openfga/openfga"},
		// A name that only starts with a forbidden one is another module.
		{"github.com/zeroroot-ai/gibson-executor/pkg/runner", ""},
		{"github.com/hashicorp/vault-plugin-secrets-kv", ""},
		{"github.com/zitadel/oidc/v3/pkg/client", ""},
		{"github.com/zeroroot-ai/ast-checks", ""},
		{"context", ""},
	}
	for _, tc := range cases {
		module, _, forbidden := forbiddenImport(tc.importPath)
		if module != tc.wantModule || forbidden != (tc.wantModule != "") {
			t.Errorf("forbiddenImport(%q) = %q, %v; want %q", tc.importPath, module, forbidden, tc.wantModule)
		}
	}
}

// TestScanImports_AFileThatImportsABackEndClientIsReported writes a module
// tree with one such file and one clean file, and runs the same walk that
// TestImportBoundary runs.
func TestScanImports_AFileThatImportsABackEndClientIsReported(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("clean/clean.go", "package clean\n\nimport \"context\"\n\nvar _ = context.Background\n")
	write("secrets/vault.go",
		"package secrets\n\nimport vault \"github.com/hashicorp/vault/api\"\n\nvar _ = vault.DefaultConfig\n")
	// A test file is inside the boundary too.
	write("authz/fga_test.go", "package authz\n\nimport _ \"github.com/openfga/go-sdk/client\"\n")
	// A fixture under testdata is not part of the module.
	write("x/testdata/sample.go", "package sample\n\nimport _ \"github.com/zeroroot-ai/gibson/pkg/billing\"\n")

	violations, files, err := scanImports(root)
	if err != nil {
		t.Fatalf("scan imports: %v", err)
	}
	if files != 3 {
		t.Fatalf("parsed %d files, want 3 (testdata is skipped)", files)
	}
	got := map[string]string{}
	for _, v := range violations {
		got[filepath.ToSlash(v.file)] = v.module
	}
	want := map[string]string{
		"secrets/vault.go":  "github.com/hashicorp/vault",
		"authz/fga_test.go": "github.com/openfga/go-sdk",
	}
	if len(got) != len(want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
	for file, module := range want {
		if got[file] != module {
			t.Errorf("violation for %s = %q, want %q", file, got[file], module)
		}
	}
}
