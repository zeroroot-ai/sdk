// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// retiredSkewEnvVar is the variable that used to widen the replay window at
// process start. The doc comment asked operators not to raise it above 300
// seconds and nothing enforced that, so a hand-edited pod spec could set a
// day. These two tests are the bound's only remaining defence.
const retiredSkewEnvVar = "GIBSON_IDENTITY_FRESHNESS_SKEW_SEC"

// TestFreshnessSkewIgnoresTheEnvironment sets the retired variable to a day
// and shows a header well outside the 60-second window is still refused. It
// catches a re-introduction that reads the environment per call.
func TestFreshnessSkewIgnoresTheEnvironment(t *testing.T) {
	t.Setenv(retiredSkewEnvVar, "86400")

	md := validHeaders()
	stale := time.Now().Add(-10 * time.Minute).Unix()
	md.Set(HeaderIssuedAt, strconv.FormatInt(stale, 10))

	_, err := IdentityFromMetadata(md)
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("a 10-minute-old identity with %s=86400 set: err = %v, want ErrInvalidIdentity",
			retiredSkewEnvVar, err)
	}
	// ErrInvalidIdentity also covers a bad subject or tenant, so assert the
	// reason as well. Otherwise this passes on any rejection at all.
	if !strings.Contains(err.Error(), "outside freshness window") {
		t.Fatalf("rejected for the wrong reason: %v", err)
	}
	if !strings.Contains(err.Error(), "max=1m0s") {
		t.Fatalf("window was widened by the environment: %v", err)
	}
}

// TestFreshnessSkewIsAConstant shows the window is 60 seconds and that one
// second past it is refused, so the bound itself is asserted and not only its
// immunity to the environment.
func TestFreshnessSkewIsAConstant(t *testing.T) {
	if freshnessSkewSeconds != 60 {
		t.Errorf("freshnessSkewSeconds = %d, want 60", freshnessSkewSeconds)
	}
	if identityFreshnessSkew != 60*time.Second {
		t.Errorf("identityFreshnessSkew = %v, want 1m0s", identityFreshnessSkew)
	}

	md := validHeaders()
	stale := time.Now().Add(-(freshnessSkewSeconds + 2) * time.Second).Unix()
	md.Set(HeaderIssuedAt, strconv.FormatInt(stale, 10))
	if _, err := IdentityFromMetadata(md); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("an identity %ds old: err = %v, want ErrInvalidIdentity", freshnessSkewSeconds+2, err)
	}
}

// TestNoEnvTunableBoundsInThisPackage parses every non-test file in the
// package and fails on any os.Getenv call. A security bound is not an
// operator knob (owner decision 2026-10-02), and an initializer that reads
// the environment at package load is invisible to the behavioural test above:
// it runs before any t.Setenv can take effect.
func TestNoEnvTunableBoundsInThisPackage(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}

	fset := token.NewFileSet()
	scanned := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile %s: %v", name, err)
		}
		scanned++
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "os" {
				return true
			}
			switch sel.Sel.Name {
			case "Getenv", "LookupEnv":
				t.Errorf("os.%s at %s makes a bound in this package operator-tunable",
					sel.Sel.Name, fset.Position(call.Pos()))
			}
			return true
		})
	}

	// A glob that matched nothing, or a package that is all tests, would make
	// this test pass by measuring nothing.
	if scanned == 0 {
		t.Fatal("scanned no non-test files")
	}
	t.Logf("scanned %d non-test files for os.Getenv", scanned)
}
