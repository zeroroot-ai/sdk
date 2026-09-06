// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	budgetstatuspb "github.com/zeroroot-ai/sdk/api/gen/gibson/budget_status/v1"
)

func TestBudgetExceededError_ErrorFormat(t *testing.T) {
	reset := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	e := &BudgetExceededError{
		Scope:         "user",
		Dimension:     "tokens",
		CurrentUsage:  210000,
		Limit:         200000,
		PeriodResetAt: reset,
		SubjectID:     "user-alice",
	}
	want := "llm: user budget exceeded for user-alice (210000 / 200000 tokens); resets at 2026-05-01T00:00:00Z"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestIsBudgetExceeded_NilErr(t *testing.T) {
	if _, ok := IsBudgetExceeded(nil); ok {
		t.Error("nil error must not match")
	}
}

func TestIsBudgetExceeded_UnrelatedErr(t *testing.T) {
	if _, ok := IsBudgetExceeded(errors.New("boom")); ok {
		t.Error("non-budget error must not match")
	}
	if _, ok := IsBudgetExceeded(status.Error(codes.NotFound, "nope")); ok {
		t.Error("gRPC status without detail must not match")
	}
}

func TestIsBudgetExceeded_TypedErr(t *testing.T) {
	inner := &BudgetExceededError{Scope: "team", Dimension: "spend"}
	wrapped := fmt.Errorf("outer: %w", inner)
	got, ok := IsBudgetExceeded(wrapped)
	if !ok {
		t.Fatal("typed error chain should match")
	}
	if got.Scope != "team" || got.Dimension != "spend" {
		t.Errorf("decoded wrong fields: %+v", got)
	}
}

func TestIsBudgetExceeded_GRPCStatusWithDetail(t *testing.T) {
	reset := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	detail := &budgetstatuspb.BudgetExceeded{
		Scope:             budgetstatuspb.BudgetScope_BUDGET_SCOPE_USER,
		Dimension:         "tokens",
		CurrentUsage:      250000,
		Limit:             200000,
		PeriodResetAtUnix: reset.Unix(),
		SubjectId:         "user-bob",
	}
	st, err := status.New(codes.ResourceExhausted, "user token budget exceeded").
		WithDetails(detail)
	if err != nil {
		t.Fatalf("status.WithDetails: %v", err)
	}

	got, ok := IsBudgetExceeded(st.Err())
	if !ok {
		t.Fatal("gRPC status with BudgetExceeded detail should match")
	}
	if got.Scope != "user" {
		t.Errorf("scope = %q, want user", got.Scope)
	}
	if got.Dimension != "tokens" {
		t.Errorf("dimension = %q, want tokens", got.Dimension)
	}
	if got.CurrentUsage != 250000 {
		t.Errorf("current = %d, want 250000", got.CurrentUsage)
	}
	if got.Limit != 200000 {
		t.Errorf("limit = %d, want 200000", got.Limit)
	}
	if !got.PeriodResetAt.Equal(reset) {
		t.Errorf("reset = %v, want %v", got.PeriodResetAt, reset)
	}
	if got.SubjectID != "user-bob" {
		t.Errorf("subject = %q, want user-bob", got.SubjectID)
	}
}

func TestIsBudgetExceeded_GRPCStatusWithUnrelatedDetail(t *testing.T) {
	// Older server that knows gRPC status details but doesn't attach
	// BudgetExceeded — still returns false without panicking.
	st, err := status.New(codes.ResourceExhausted, "quota exceeded").
		WithDetails(&errdetails.QuotaFailure{})
	if err != nil {
		t.Fatalf("status.WithDetails: %v", err)
	}
	if _, ok := IsBudgetExceeded(st.Err()); ok {
		t.Error("unrelated detail must not match")
	}
}

func TestIsBudgetExceeded_AllScopesDecode(t *testing.T) {
	cases := []struct {
		in       budgetstatuspb.BudgetScope
		wantName string
	}{
		{budgetstatuspb.BudgetScope_BUDGET_SCOPE_USER, "user"},
		{budgetstatuspb.BudgetScope_BUDGET_SCOPE_TEAM, "team"},
		{budgetstatuspb.BudgetScope_BUDGET_SCOPE_TENANT, "tenant"},
		{budgetstatuspb.BudgetScope_BUDGET_SCOPE_UNSPECIFIED, "unspecified"},
	}
	for _, tc := range cases {
		detail := &budgetstatuspb.BudgetExceeded{Scope: tc.in}
		st, _ := status.New(codes.ResourceExhausted, "x").WithDetails(detail)
		got, ok := IsBudgetExceeded(st.Err())
		if !ok {
			t.Errorf("scope %v did not decode", tc.in)
			continue
		}
		if got.Scope != tc.wantName {
			t.Errorf("scope = %q, want %q", got.Scope, tc.wantName)
		}
	}
}
