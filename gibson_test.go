// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"context"
	"reflect"
	"testing"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
)

func TestNewAgent(t *testing.T) {
	exec := func(_ context.Context, _ agent.Harness, _ agent.Task) (agent.Result, error) {
		return agent.Result{}, nil
	}
	a, err := NewAgent(
		WithName("scanner"),
		WithVersion("1.2.3"),
		WithDescription("finds things"),
		WithTargetTypes("http", "dns"),
		WithLLMSlot("primary", llm.SlotRequirements{MinContextWindow: 8000}),
		WithExecuteFunc(exec),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if a.Name() != "scanner" || a.Version() != "1.2.3" || a.Description() != "finds things" {
		t.Fatalf("identity = %q %q %q", a.Name(), a.Version(), a.Description())
	}
	if !reflect.DeepEqual(a.TargetTypes(), []string{"http", "dns"}) {
		t.Fatalf("TargetTypes = %v", a.TargetTypes())
	}
}

func TestNewAgentNeedsAName(t *testing.T) {
	if _, err := NewAgent(WithVersion("1")); err == nil {
		t.Fatal("NewAgent without a name must fail")
	}
}

func TestServeAgentNeedsAPlatformURL(t *testing.T) {
	t.Setenv("GIBSON_URL", "")
	a, err := NewAgent(
		WithName("a"), WithVersion("1"), WithDescription("d"),
		WithExecuteFunc(func(context.Context, agent.Harness, agent.Task) (agent.Result, error) { return agent.Result{}, nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ServeAgent(a); err == nil {
		t.Fatal("ServeAgent without GIBSON_URL must fail")
	}
}
