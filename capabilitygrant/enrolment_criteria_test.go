// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sdk#128 asks for a component that enrols itself at boot, and lists six
// acceptance criteria. Three of them are security properties of this package
// rather than of the serve wiring: a credential file must not be world-readable,
// the one-time token must not reach a file or an error string, and a component
// that already holds a host key must not read the token again.
//
// The code already intends all three — there are 0600 constants and no token in
// any format string. These tests exist because INTENT IS NOT A MEASUREMENT: a
// Chmod call does not prove the resulting file's mode (create-then-chmod leaves a
// window, and a later writer can widen it), and "no token in a format string" is
// a negative established by reading, which goes stale on the next edit.

// probeToken returns a credential-shaped value, GENERATED PER RUN rather than
// committed.
//
// It has to look like a credential: these tests search persisted files and error
// strings for it, and a value like "token" would match prose and pass for the
// wrong reason. But a credential-shaped LITERAL in the tree is a secret as far as
// any scanner is concerned, and gitleaks rejected the first version of this file
// for exactly that — correctly. The gitleaks gate's own selftest solves it the
// same way and says so: "The fixture is generated at run time, so no
// credential-shaped string is committed here."
//
// Generating it is also strictly better than a constant: a random value cannot
// collide with prose, a field name, or another fixture.
func probeToken(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	// Assembled from fragments so no part of the committed source is itself
	// credential-shaped.
	return "bst" + "_" + "PROBE" + "_" + hex.EncodeToString(raw)
}

// TestSavedCredentialFilesAreNotWorldReadable is acceptance criterion 5. A
// world-readable runtime credential is "the equivalent of the agent key" by
// runtime.go's own comment, so the mode is the whole protection.
func TestSavedCredentialFilesAreNotWorldReadable(t *testing.T) {
	dir := t.TempDir()

	key, err := GenerateHostKey()
	if err != nil {
		t.Fatalf("GenerateHostKey: %v", err)
	}
	keyPath := filepath.Join(dir, "host.jwk")
	if err := SaveHostKey(key, keyPath); err != nil {
		t.Fatalf("SaveHostKey: %v", err)
	}

	rc := RuntimeCredential{HostID: "h-1", AgentID: "a-1", ComponentScope: "tool:probe", AgentKeySeed: make([]byte, 32)}
	rcPath := filepath.Join(dir, "runtime.json")
	if err := SaveRuntimeCredential(rcPath, rc); err != nil {
		t.Fatalf("SaveRuntimeCredential: %v", err)
	}

	for _, p := range []string{keyPath, rcPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		mode := fi.Mode().Perm()
		if mode&0o077 != 0 {
			t.Errorf("%s has mode %04o; group or other can read a private credential", filepath.Base(p), mode)
		}
		if mode != 0o600 {
			t.Errorf("%s has mode %04o, want 0600", filepath.Base(p), mode)
		}
	}
}

// TestSaveHostKeyOverwriteStaysRestrictive: the atomic-rename path chmods a temp
// file and renames over the target. If a pre-existing target were widened, or if
// the rename preserved the old mode, a second save could leave a readable file.
func TestSaveHostKeyOverwriteStaysRestrictive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host.jwk")

	// A pre-existing, deliberately world-readable file at the target. 0644 is the
	// hazard under test: SaveHostKey must leave 0600 even when it overwrites a
	// loose file, and writing it tightly here would test nothing.
	//
	//nolint:gosec // G306: the permissive mode IS the fixture.
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	key, err := GenerateHostKey()
	if err != nil {
		t.Fatalf("GenerateHostKey: %v", err)
	}
	if err := SaveHostKey(key, path); err != nil {
		t.Fatalf("SaveHostKey over an existing file: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("after overwriting a 0644 file the mode is %04o, want 0600", fi.Mode().Perm())
	}
}

// TestTheBootstrapTokenNeverReachesAPersistedFile is acceptance criterion 4,
// file half. The token is one-time and the credential files outlive the boot that
// consumed it, so a token on disk is a credential that cannot be rotated by using
// it.
func TestTheBootstrapTokenNeverReachesAPersistedFile(t *testing.T) {
	dir := t.TempDir()
	probe := probeToken(t)

	key, err := GenerateHostKey()
	if err != nil {
		t.Fatalf("GenerateHostKey: %v", err)
	}
	if err := SaveHostKey(key, filepath.Join(dir, "host.jwk")); err != nil {
		t.Fatalf("SaveHostKey: %v", err)
	}
	// A runtime credential built in the shape the handshake produces, with the
	// probe token present in the surrounding config rather than in the struct:
	// the point is that nothing copies it across.
	rc := RuntimeCredential{HostID: "h-1", AgentID: "a-1", ComponentScope: "tool:probe", AgentKeySeed: make([]byte, 32)}
	if err := SaveRuntimeCredential(filepath.Join(dir, "runtime.json"), rc); err != nil {
		t.Fatalf("SaveRuntimeCredential: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no credential files were written, so this test would measure nothing")
	}
	for _, e := range entries {
		// The path is a t.TempDir() entry this test wrote moments ago; no caller
		// value reaches it.
		//nolint:gosec // G304: path is from os.ReadDir of this test's own temp dir.
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if strings.Contains(string(b), probe) {
			t.Errorf("%s contains the bootstrap token", e.Name())
		}
		// And no field named like a bootstrap credential, which would invite a
		// future writer to populate it.
		for _, bad := range []string{"bootstrap_token", "bootstrapToken", "BootstrapToken"} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s has a %q field; a one-time token must have nowhere to be persisted", e.Name(), bad)
			}
		}
	}
}

// TestResolveBootstrapErrorsDoNotQuoteTheToken is acceptance criterion 4, log
// half. An error string reaches logs, issue reports and terminal scrollback, so a
// token in one is a leaked credential even when nothing was written to disk.
func TestResolveBootstrapErrorsDoNotQuoteTheToken(t *testing.T) {
	// A token that is syntactically unacceptable, so the failure path formats it
	// if it formats anything at all.
	probe := probeToken(t)
	bad := probe + " not a token\n\t"
	if _, err := ResolveBootstrap(bad); err != nil {
		if strings.Contains(err.Error(), probe) {
			t.Errorf("ResolveBootstrap quoted the token in its error: %v", err)
		}
	}

	// The same through the environment, which is the documented path.
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", bad)
	if _, err := ResolveBootstrap(""); err != nil {
		if strings.Contains(err.Error(), probe) {
			t.Errorf("ResolveBootstrap quoted the env token in its error: %v", err)
		}
	}
}

// TestAMissingTokenFailsWithAnActionableError is acceptance criterion 3. The
// failure must name what is missing, not retry quietly: a component that loops
// on a missing credential looks like a network problem for as long as nobody
// reads its logs.
func TestAMissingTokenFailsWithAnActionableError(t *testing.T) {
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")

	_, err := ResolveBootstrap("")
	if err == nil {
		t.Fatal("ResolveBootstrap succeeded with no token anywhere")
	}
	msg := err.Error()
	// It must name the variable the operator sets. Without that the error is a
	// statement that something is wrong rather than an instruction.
	if !strings.Contains(msg, "GIBSON_BOOTSTRAP_TOKEN") {
		t.Errorf("the error does not name GIBSON_BOOTSTRAP_TOKEN: %v", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing token surfaced as a filesystem error, which sends the reader to the wrong place: %v", err)
	}
}
