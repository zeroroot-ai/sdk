// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package llm — errors.go
//
// Typed errors the SDK surfaces to agents when LLM calls are denied by
// platform policy. Agents should branch on these via errors.As rather
// than string-matching the wire-level gRPC status message, so fallback
// behavior (skip, switch slot, alert) stays robust across SDK versions.
//
// Spec: llm-user-attribution-governance (Requirement 6.4).

package llm

import (
	"errors"
	"fmt"
	"time"

	budgetstatuspb "github.com/zeroroot-ai/sdk/api/gen/gibson/budget_status/v1"
	"google.golang.org/grpc/status"
)

// BudgetExceededError is returned to an agent when an LLM call was denied
// because a token or spend budget would be exceeded.
//
// Populated from the gibson.budget_status.v1.BudgetExceeded status-detail
// the daemon attaches to its codes.ResourceExhausted response. Callers use
// IsBudgetExceeded(err) (or errors.As) to branch.
//
// Wire compat: BudgetExceeded was originally defined in gibson.budget.v1.
// As of sdk#106 it lives in the public gibson.budget_status.v1 package;
// field numbers and tag names are unchanged so on-wire bytes are identical
// (daemons that still emit gibson.budget.v1.BudgetExceeded will be re-routed
// by the daemon flip in gibson PR C, but the message-name string in the
// status-detail type URL changes — see test TestIsBudgetExceeded_*).
type BudgetExceededError struct {
	// Scope is "user", "team", or "tenant" — the scope that was exceeded.
	Scope string

	// Dimension is "tokens" or "spend" — which limit tripped.
	Dimension string

	// CurrentUsage is the current period-to-date usage on the limiting
	// dimension (tokens or USD cents).
	CurrentUsage int64

	// Limit is the configured ceiling on the limiting dimension.
	Limit int64

	// PeriodResetAt is the time the counter rolls over. Callers can use
	// this to decide whether to back off and retry.
	PeriodResetAt time.Time

	// SubjectID is the user or team ID that was over limit. Empty when
	// Scope is "tenant".
	SubjectID string
}

// Error formats a human-readable message. Never logged by the platform
// itself — that's the daemon's concern; this is for agent-side logging.
func (e *BudgetExceededError) Error() string {
	subject := e.SubjectID
	if subject == "" {
		subject = e.Scope
	}
	return fmt.Sprintf(
		"llm: %s budget exceeded for %s (%d / %d %s); resets at %s",
		e.Scope, subject, e.CurrentUsage, e.Limit, e.Dimension,
		e.PeriodResetAt.Format(time.RFC3339),
	)
}

// IsBudgetExceeded checks whether err is (or wraps) a BudgetExceededError,
// either as a typed error via errors.As or as a gRPC status with a
// gibson.budget_status.v1.BudgetExceeded detail attached. Returns the
// decoded struct and true when match; (nil, false) otherwise.
//
// Use this in agent code when you want to branch on budget denial:
//
//	if e, ok := llm.IsBudgetExceeded(err); ok {
//	    // Fallback: skip this step, log, alert the owner.
//	    return nil
//	}
func IsBudgetExceeded(err error) (*BudgetExceededError, bool) {
	if err == nil {
		return nil, false
	}

	// Fast path: already a typed error somewhere in the chain.
	var typed *BudgetExceededError
	if errors.As(err, &typed) {
		return typed, true
	}

	// Slow path: decode from gRPC status details.
	st, ok := status.FromError(err)
	if !ok {
		return nil, false
	}
	for _, d := range st.Details() {
		detail, ok := d.(*budgetstatuspb.BudgetExceeded)
		if !ok {
			continue
		}
		return &BudgetExceededError{
			Scope:         scopeString(detail.GetScope()),
			Dimension:     detail.GetDimension(),
			CurrentUsage:  detail.GetCurrentUsage(),
			Limit:         detail.GetLimit(),
			PeriodResetAt: time.Unix(detail.GetPeriodResetAtUnix(), 0),
			SubjectID:     detail.GetSubjectId(),
		}, true
	}
	return nil, false
}

// scopeString maps the proto enum to the human-friendly string used on
// BudgetExceededError.Scope.
func scopeString(s budgetstatuspb.BudgetScope) string {
	switch s {
	case budgetstatuspb.BudgetScope_BUDGET_SCOPE_USER:
		return "user"
	case budgetstatuspb.BudgetScope_BUDGET_SCOPE_TEAM:
		return "team"
	case budgetstatuspb.BudgetScope_BUDGET_SCOPE_TENANT:
		return "tenant"
	default:
		return "unspecified"
	}
}
