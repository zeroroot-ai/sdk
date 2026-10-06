// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/zeroroot-ai/sdk/llm"
)

func TestNewBuildsAnAgent(t *testing.T) {
	called := false
	cfg := NewConfig().
		SetName("n").
		SetVersion("v").
		SetDescription("d").
		SetTargetTypes([]string{"http"}).
		AddLLMSlot("primary", llm.SlotRequirements{MinContextWindow: 1}).
		SetExecuteFunc(func(context.Context, Harness, Task) (Result, error) {
			called = true
			return Result{Status: StatusSuccess}, nil
		})
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.Name() != "n" || a.Version() != "v" || a.Description() != "d" {
		t.Fatalf("identity = %q %q %q", a.Name(), a.Version(), a.Description())
	}
	if len(a.Capabilities()) != 0 || len(a.TechniqueTypes()) != 0 {
		t.Fatal("an agent built with New declares no capabilities and no techniques")
	}
	if got := a.TargetTypes(); len(got) != 1 || got[0] != "http" {
		t.Fatalf("TargetTypes = %v", got)
	}
	res, err := a.Execute(context.Background(), nil, Task{})
	if err != nil || !called || res.Status != StatusSuccess {
		t.Fatalf("Execute = %+v, %v (called %v)", res, err, called)
	}
	if s, ok := a.(interface{ Shutdown(context.Context) error }); !ok || s.Shutdown(context.Background()) != nil {
		t.Fatal("Shutdown must succeed")
	}
}

func TestNewRejectsAnIncompleteConfig(t *testing.T) {
	exec := func(context.Context, Harness, Task) (Result, error) { return Result{}, nil }
	cases := map[string]*Config{
		"no name":        NewConfig().SetVersion("v").SetDescription("d").SetExecuteFunc(exec),
		"no version":     NewConfig().SetName("n").SetDescription("d").SetExecuteFunc(exec),
		"no description": NewConfig().SetName("n").SetVersion("v").SetExecuteFunc(exec),
		"no execute":     NewConfig().SetName("n").SetVersion("v").SetDescription("d"),
	}
	for name, cfg := range cases {
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestResultErrorChainAndJSON(t *testing.T) {
	cause := &ResultError{Code: "TIMEOUT", Message: "slow"}
	err := &ResultError{Code: "FAILED", Message: "task failed", Component: "scanner", Cause: cause}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is must find the cause through Unwrap")
	}
	if (&ResultError{Code: "X"}).Unwrap() != nil {
		t.Fatal("Unwrap without a cause must be nil")
	}
	if got := err.Error(); got != "scanner [FAILED]: task failed: [TIMEOUT]: slow" {
		t.Fatalf("Error() = %q", got)
	}
	b, jerr := json.Marshal(err)
	if jerr != nil {
		t.Fatal(jerr)
	}
	var back ResultError
	if jerr := json.Unmarshal(b, &back); jerr != nil {
		t.Fatal(jerr)
	}
	if back.Code != "FAILED" || back.Cause == nil || back.Cause.Code != "TIMEOUT" {
		t.Fatalf("round trip = %+v", back)
	}
}

func TestStringers(t *testing.T) {
	for scope, want := range map[RunScope]string{
		RunScopePrevious: "previous", RunScopeAll: "all", RunScopeUnspecified: "unspecified", RunScope(99): "unspecified",
	} {
		if got := scope.String(); got != want {
			t.Errorf("RunScope(%d) = %q, want %q", scope, got, want)
		}
	}
	if StatusFailed.String() != "failed" {
		t.Errorf("StatusFailed = %q", StatusFailed.String())
	}
}

func TestObservationsAreSealed(_ *testing.T) {
	for _, o := range []Observation{
		AnalysisObservation{}, HostObservation{}, DomainObservation{}, SubdomainObservation{},
		CredentialObservation{}, AccountObservation{}, MemoryObservation{},
		LifecycleEntityObservation{}, HypothesisObservation{}, ReasoningStepObservation{},
	} {
		o.isObservation()
	}
}
