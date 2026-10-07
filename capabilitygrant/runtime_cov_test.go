// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func validRuntimeCredential() RuntimeCredential {
	return RuntimeCredential{
		HostID:         "host-1",
		AgentID:        "agent-1",
		ComponentScope: "component:agent/a",
		AgentKeySeed:   make([]byte, ed25519.SeedSize),
	}
}

func TestRuntimeCredentialValid(t *testing.T) {
	ok := validRuntimeCredential()
	if err := ok.Valid(); err != nil {
		t.Fatalf("Valid: %v", err)
	}
	for name, mut := range map[string]func(*RuntimeCredential){
		"host":  func(r *RuntimeCredential) { r.HostID = "" },
		"agent": func(r *RuntimeCredential) { r.AgentID = "" },
		"scope": func(r *RuntimeCredential) { r.ComponentScope = "" },
		"seed":  func(r *RuntimeCredential) { r.AgentKeySeed = nil },
	} {
		rc := validRuntimeCredential()
		mut(&rc)
		if rc.Valid() == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestDecodeRuntimeCredential(t *testing.T) {
	raw, err := json.Marshal(validRuntimeCredential())
	if err != nil {
		t.Fatal(err)
	}
	for _, enc := range []string{base64.StdEncoding.EncodeToString(raw), base64.RawStdEncoding.EncodeToString(raw)} {
		rc, err := DecodeRuntimeCredentialBase64(enc)
		if err != nil || rc.AgentID != "agent-1" {
			t.Fatalf("decode = %+v, %v", rc, err)
		}
	}
	if _, err := DecodeRuntimeCredentialBase64("!!not base64!!"); err == nil {
		t.Fatal("want a base64 error")
	}
	if _, err := DecodeRuntimeCredential([]byte("{")); err == nil {
		t.Fatal("want a JSON error")
	}
}

func TestRuntimeCredentialPerRPC(t *testing.T) {
	creds, err := validRuntimeCredential().PerRPCCredentials()
	if err != nil {
		t.Fatalf("PerRPCCredentials: %v", err)
	}
	if creds.RequireTransportSecurity() {
		t.Fatal("the transport carries security, not the credential")
	}
	if _, err := creds.GetRequestMetadata(context.Background()); err == nil {
		t.Fatal("a context without gRPC RequestInfo must fail")
	}
	bad := validRuntimeCredential()
	bad.AgentID = ""
	if _, err := bad.PerRPCCredentials(); err == nil {
		t.Fatal("an invalid credential must fail")
	}
	short := validRuntimeCredential()
	short.AgentKeySeed = []byte{1, 2, 3}
	if _, err := short.PerRPCCredentials(); err == nil {
		t.Fatal("a short seed must fail")
	}
}

func TestRuntimeInstallFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvGibsonHome, home)
	t.Setenv(EnvRuntimeCredential, "")

	dir, err := GibsonDir()
	if err != nil || dir != home {
		t.Fatalf("GibsonDir = %q, %v", dir, err)
	}
	path, err := RuntimeInstallPath("agent", "")
	if err != nil || path != filepath.Join(home, "agent", "default.runtime.json") {
		t.Fatalf("RuntimeInstallPath = %q, %v", path, err)
	}
	if _, err := ResolveRuntimeInstall("agent", ""); err == nil {
		t.Fatal("a missing install must fail")
	}

	install := RuntimeInstall{GibsonURL: "https://gibson.example", Credential: validRuntimeCredential()}
	raw, _ := json.Marshal(install)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "agent", "ignored.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	refs, err := ListInstalls()
	if err != nil || len(refs) != 1 || refs[0] != (InstallRef{Kind: "agent", Name: "default"}) {
		t.Fatalf("ListInstalls = %+v, %v", refs, err)
	}
	got, err := ResolveRuntimeInstall("agent", "")
	if err != nil || got.GibsonURL != "https://gibson.example" {
		t.Fatalf("ResolveRuntimeInstall = %+v, %v", got, err)
	}

	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRuntimeInstall("agent", ""); err == nil {
		t.Fatal("a corrupt install must fail")
	}
	incomplete, _ := json.Marshal(RuntimeInstall{GibsonURL: "u"})
	if err := os.WriteFile(path, incomplete, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRuntimeInstall("agent", ""); err == nil {
		t.Fatal("an incomplete install must fail")
	}
}

func TestResolveRuntimeInstallFromEnv(t *testing.T) {
	raw, _ := json.Marshal(validRuntimeCredential())
	t.Setenv(EnvRuntimeCredential, base64.StdEncoding.EncodeToString(raw))
	t.Setenv(EnvGibsonURL, "")
	if _, err := ResolveRuntimeInstall("agent", "x"); err == nil {
		t.Fatal("the credential without a URL must fail")
	}
	t.Setenv(EnvGibsonURL, "https://gibson.example")
	got, err := ResolveRuntimeInstall("agent", "x")
	if err != nil || got.Credential.AgentID != "agent-1" {
		t.Fatalf("ResolveRuntimeInstall = %+v, %v", got, err)
	}
	t.Setenv(EnvRuntimeCredential, "!!")
	if _, err := ResolveRuntimeInstall("agent", "x"); err == nil {
		t.Fatal("a bad env credential must fail")
	}
	bad, _ := json.Marshal(RuntimeCredential{})
	t.Setenv(EnvRuntimeCredential, base64.StdEncoding.EncodeToString(bad))
	if _, err := ResolveRuntimeInstall("agent", "x"); err == nil {
		t.Fatal("an invalid env credential must fail")
	}
}
