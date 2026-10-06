// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createIncidentReq is a representative Go-first request struct.
type createIncidentReq struct {
	Title    string   `json:"title"`
	Severity int      `json:"severity"`
	Tags     []string `json:"tags,omitempty"`
}

type createIncidentResp struct {
	ID string `json:"id"`
}

// applyOptions builds a config from options exactly as Serve does, so the test
// exercises the real WithHandler wiring.
func applyOptions(opts ...Option) *config {
	c := &config{}
	for _, o := range opts {
		o(c)
	}
	c.defaults()
	return c
}

// TestWithHandler_UnderivableTypeRecordsError asserts that a handler whose
// request type cannot be expressed as a JSON schema (an interface/union field)
// records a startup error rather than silently passing.
func TestWithHandler_UnderivableTypeRecordsError(t *testing.T) {
	type badReq struct {
		Payload any `json:"payload"`
	}
	handler := func(_ context.Context, _ badReq) (createIncidentResp, error) {
		return createIncidentResp{}, nil
	}
	c := applyOptions(WithHandler("Bad", "test handler for Bad", handler))
	require.NotEmpty(t, c.optionErrs)
	assert.Contains(t, c.optionErrs[0].Error(), "request schema")
}

// TestWithHandler_DuplicateRegistrationRecordsError asserts that registering the
// same method name twice is a startup error (ADR-0027 one code path).
func TestWithHandler_DuplicateRegistrationRecordsError(t *testing.T) {
	h := func(_ context.Context, req createIncidentReq) (createIncidentResp, error) {
		return createIncidentResp{}, nil
	}
	c := applyOptions(
		WithHandler("Dup", "first registration", h),
		WithHandler("Dup", "second registration, same name", h),
	)
	require.NotEmpty(t, c.optionErrs)
	assert.Contains(t, c.optionErrs[0].Error(), "registered more than once")
}

// TestWithHandler_EmptyRequestBody asserts the adapter tolerates an empty
// payload (a method whose request struct has no required content, or a nil Any),
// decoding it as the zero request.
func TestWithHandler_EmptyRequestBody(t *testing.T) {
	handler := func(_ context.Context, req createIncidentReq) (createIncidentResp, error) {
		assert.Empty(t, req.Title)
		return createIncidentResp{ID: "zero"}, nil
	}
	c := applyOptions(WithHandler("M", "test handler for M", handler))
	require.Empty(t, c.optionErrs)
	respJSON, err := c.handlers["M"](context.Background(), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"zero"}`, string(respJSON))
}
