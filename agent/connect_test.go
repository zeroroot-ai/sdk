// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
)

func mustInstall(t *testing.T, name, url string) capabilitygrant.RuntimeInstall {
	t.Helper()
	key, err := capabilitygrant.GenerateAgentKey()
	if err != nil {
		t.Fatal(err)
	}
	return capabilitygrant.RuntimeInstall{
		GibsonURL: url,
		Credential: capabilitygrant.RuntimeCredential{
			HostID:         "host-" + name,
			AgentID:        "agent-" + name,
			ComponentScope: "component:" + name,
			AgentKeySeed:   key.Seed(),
		},
	}
}

func saveAgent(t *testing.T, name, url string) {
	t.Helper()
	if _, err := capabilitygrant.SaveRuntimeInstall("agent", name, mustInstall(t, name, url)); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAgentInstall_AutoDetectSingle(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	saveAgent(t, "solo", "https://daemon.example")

	in, err := resolveAgentInstall("")
	if err != nil {
		t.Fatalf("resolveAgentInstall: %v", err)
	}
	if in.GibsonURL != "https://daemon.example" || in.Credential.AgentID != "agent-solo" {
		t.Fatalf("install = %+v", in)
	}
}

func TestResolveAgentInstall_Ambiguous(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	saveAgent(t, "a", "u")
	saveAgent(t, "b", "u")

	_, err := resolveAgentInstall("")
	if !errors.Is(err, ErrAmbiguousInstall) {
		t.Fatalf("err = %v, want ErrAmbiguousInstall", err)
	}
}

func TestResolveAgentInstall_ExplicitNameAmongMany(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	saveAgent(t, "a", "ua")
	saveAgent(t, "b", "ub")

	in, err := resolveAgentInstall("b")
	if err != nil {
		t.Fatalf("resolveAgentInstall(b): %v", err)
	}
	if in.GibsonURL != "ub" {
		t.Fatalf("install = %+v, want gibson_url=ub", in)
	}
}

func TestResolveAgentInstall_Missing(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	_, err := resolveAgentInstall("")
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("err = %v, want ErrMissingCredentials", err)
	}
}

func TestResolveAgentInstall_EnvOverride(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir()) // empty dir — env must win
	in := mustInstall(t, "env", "ignored")
	enc, err := in.Credential.Encode()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(capabilitygrant.EnvRuntimeCredential, base64.StdEncoding.EncodeToString(enc))
	t.Setenv(capabilitygrant.EnvGibsonURL, "https://env.example")

	got, err := resolveAgentInstall("")
	if err != nil {
		t.Fatalf("resolveAgentInstall (env): %v", err)
	}
	if got.GibsonURL != "https://env.example" || got.Credential.AgentID != "agent-env" {
		t.Fatalf("install = %+v", got)
	}
}

func TestConnect_BuildsClientFromInstall(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	saveAgent(t, "solo", "https://daemon.example:8443")

	// grpc.NewClient is lazy, so this succeeds without a live server. The
	// insecure transport (appended last) overrides the SPIFFE one for the test.
	client, err := connect(context.Background(), ConnectConfig{}, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if client == nil || client.Conn() == nil {
		t.Fatal("expected a non-nil client + conn")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestConnect_MissingCredentials(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	_, err := connect(context.Background(), ConnectConfig{}, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("err = %v, want ErrMissingCredentials", err)
	}
}

func TestConnect_NoURLAnywhere(t *testing.T) {
	t.Setenv("GIBSON_HOME", t.TempDir())
	t.Setenv("GIBSON_URL", "")
	saveAgent(t, "solo", "") // install has no URL, no override, no env

	_, err := connect(context.Background(), ConnectConfig{}, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if !errors.Is(err, ErrDial) {
		t.Fatalf("err = %v, want ErrDial (no URL)", err)
	}
}

func TestNormalizeTarget(t *testing.T) {
	cases := map[string]string{
		"https://api.zeroroot.ai":      "api.zeroroot.ai:443",
		"https://api.zeroroot.ai:8443": "api.zeroroot.ai:8443",
		"http://localhost:50051/":      "localhost:50051",
		"daemon:50051":                 "daemon:50051",
	}
	for in, want := range cases {
		if got := normalizeTarget(in); got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
