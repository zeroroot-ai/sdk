// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package egress

import (
	"context"
	"errors"
	"testing"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// ----------------------------------------------------------------------------
// Fake SetecClient
// ----------------------------------------------------------------------------

// fakeSetecClient records calls to ApplyNetworkPolicy and can be configured
// to return a preset error.
type fakeSetecClient struct {
	called bool
	decls  []manifest.EgressDecl
	err    error
}

func (f *fakeSetecClient) ApplyNetworkPolicy(_ context.Context, decls []manifest.EgressDecl) error {
	f.called = true
	f.decls = decls
	return f.err
}

// exampleDecls is a set of egress declarations used across tests.
var exampleDecls = []manifest.EgressDecl{
	{Host: "api.example.com", Protocol: "https", Port: 443, Purpose: "external API"},
	{Host: "db.internal.example.com", Protocol: "tcp", Port: 5432, Purpose: "database"},
}

// ----------------------------------------------------------------------------
// processEnforcer tests
// ----------------------------------------------------------------------------

// TestProcessEnforcer_Apply verifies that Apply in process mode returns nil
// and does not call any external system.
func TestProcessEnforcer_Apply(t *testing.T) {
	e := &processEnforcer{}
	if err := e.Apply(context.Background(), exampleDecls); err != nil {
		t.Fatalf("processEnforcer.Apply: unexpected error: %v", err)
	}
}

// TestProcessEnforcer_Apply_Empty verifies Apply with no declarations returns nil.
func TestProcessEnforcer_Apply_Empty(t *testing.T) {
	e := &processEnforcer{}
	if err := e.Apply(context.Background(), nil); err != nil {
		t.Fatalf("processEnforcer.Apply(nil): unexpected error: %v", err)
	}
}

// ----------------------------------------------------------------------------
// podEnforcer tests
// ----------------------------------------------------------------------------

// TestPodEnforcer_Apply_IsNoOp verifies that pod mode Apply is a no-op
// (returns nil without touching a SetecClient or any external system).
func TestPodEnforcer_Apply_IsNoOp(t *testing.T) {
	e := &podEnforcer{}
	if err := e.Apply(context.Background(), exampleDecls); err != nil {
		t.Fatalf("podEnforcer.Apply: unexpected error: %v", err)
	}
}

// TestPodEnforcer_Apply_NilDecls verifies Apply is safe with nil decls.
func TestPodEnforcer_Apply_NilDecls(t *testing.T) {
	e := &podEnforcer{}
	if err := e.Apply(context.Background(), nil); err != nil {
		t.Fatalf("podEnforcer.Apply(nil): unexpected error: %v", err)
	}
}

// ----------------------------------------------------------------------------
// setecEnforcer tests
// ----------------------------------------------------------------------------

// TestSetecEnforcer_Apply_ForwardsDecls verifies that Apply calls the Setec
// client with the supplied declarations.
func TestSetecEnforcer_Apply_ForwardsDecls(t *testing.T) {
	client := &fakeSetecClient{}
	e := &setecEnforcer{client: client}

	if err := e.Apply(context.Background(), exampleDecls); err != nil {
		t.Fatalf("setecEnforcer.Apply: unexpected error: %v", err)
	}
	if !client.called {
		t.Fatal("expected ApplyNetworkPolicy to be called")
	}
	if len(client.decls) != len(exampleDecls) {
		t.Fatalf("expected %d decls forwarded, got %d", len(exampleDecls), len(client.decls))
	}
}

// TestSetecEnforcer_Apply_NilClient verifies that a nil SetecClient returns
// a descriptive error rather than panicking.
func TestSetecEnforcer_Apply_NilClient(t *testing.T) {
	e := &setecEnforcer{client: nil}
	err := e.Apply(context.Background(), exampleDecls)
	if err == nil {
		t.Fatal("expected error for nil SetecClient, got nil")
	}
}

// TestSetecEnforcer_Apply_ClientError verifies that errors from the Setec
// client are wrapped and returned.
func TestSetecEnforcer_Apply_ClientError(t *testing.T) {
	want := errors.New("network policy rejected")
	client := &fakeSetecClient{err: want}
	e := &setecEnforcer{client: client}

	err := e.Apply(context.Background(), exampleDecls)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped error containing %v, got %v", want, err)
	}
}

// ----------------------------------------------------------------------------
// New() selection tests
// ----------------------------------------------------------------------------

// TestNew_DefaultIsProcess verifies that an absent GIBSON_PLUGIN_RUNTIME
// variable produces a processEnforcer.
func TestNew_DefaultIsProcess(t *testing.T) {
	t.Setenv(EnvRuntimeKey, "")
	e := New(nil)
	if _, ok := e.(*processEnforcer); !ok {
		t.Fatalf("expected *processEnforcer, got %T", e)
	}
}

// TestNew_ProcessMode verifies explicit "process" value produces processEnforcer.
func TestNew_ProcessMode(t *testing.T) {
	t.Setenv(EnvRuntimeKey, RuntimeProcess)
	e := New(nil)
	if _, ok := e.(*processEnforcer); !ok {
		t.Fatalf("expected *processEnforcer, got %T", e)
	}
}

// TestNew_PodMode verifies "pod" produces a podEnforcer.
func TestNew_PodMode(t *testing.T) {
	t.Setenv(EnvRuntimeKey, RuntimePod)
	e := New(nil)
	if _, ok := e.(*podEnforcer); !ok {
		t.Fatalf("expected *podEnforcer, got %T", e)
	}
}

// TestNew_SetecMode verifies "setec" produces a setecEnforcer.
func TestNew_SetecMode(t *testing.T) {
	t.Setenv(EnvRuntimeKey, RuntimeSetec)
	client := &fakeSetecClient{}
	e := New(client)
	se, ok := e.(*setecEnforcer)
	if !ok {
		t.Fatalf("expected *setecEnforcer, got %T", e)
	}
	if se.client != client {
		t.Fatal("setecEnforcer did not receive the provided SetecClient")
	}
}

// TestNew_UnknownModeDefaultsToProcess verifies an unrecognised env value
// falls back to process mode without panicking.
func TestNew_UnknownModeDefaultsToProcess(t *testing.T) {
	t.Setenv(EnvRuntimeKey, "hypervisor-v9")
	e := New(nil)
	if _, ok := e.(*processEnforcer); !ok {
		t.Fatalf("expected *processEnforcer for unknown mode, got %T", e)
	}
}

// TestSetecEnforcer_Apply_NilDecls verifies setecEnforcer handles empty decls.
func TestSetecEnforcer_Apply_NilDecls(t *testing.T) {
	client := &fakeSetecClient{}
	e := &setecEnforcer{client: client}
	if err := e.Apply(context.Background(), nil); err != nil {
		t.Fatalf("setecEnforcer.Apply(nil): unexpected error: %v", err)
	}
	if !client.called {
		t.Fatal("expected ApplyNetworkPolicy to be called even with nil decls")
	}
}
