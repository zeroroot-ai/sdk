// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"encoding/json"
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

// TestWithHandler_DerivesSchemaAndDispatches is the hermetic end-to-end check
// for the Go-first authoring path: WithHandler derives the request schema from
// the Go struct and installs an adapter that decodes JSON into the typed
// request, runs the handler, and encodes the typed response back to JSON.
func TestWithHandler_DerivesSchemaAndDispatches(t *testing.T) {
	called := false
	handler := func(_ context.Context, req createIncidentReq) (createIncidentResp, error) {
		called = true
		assert.Equal(t, "db is down", req.Title)
		assert.Equal(t, 1, req.Severity)
		assert.Equal(t, []string{"prod", "db"}, req.Tags)
		return createIncidentResp{ID: "INC-42"}, nil
	}

	c := applyOptions(WithHandler("CreateIncident", handler))
	require.Empty(t, c.optionErrs, "no derivation errors expected for a plain struct handler")

	// The derived request schema travels to the daemon as the tool-input contract.
	ms, ok := c.methodSchemas["CreateIncident"]
	require.True(t, ok, "method schema must be recorded")
	var inSchema map[string]any
	require.NoError(t, json.Unmarshal([]byte(ms.input), &inSchema))
	assert.Equal(t, "object", inSchema["type"])
	props := inSchema["properties"].(map[string]any)
	assert.Contains(t, props, "title")
	assert.Contains(t, props, "severity")
	assert.Contains(t, props, "tags")
	// title/severity are required; tags is omitempty.
	assert.ElementsMatch(t, []any{"title", "severity"}, inSchema["required"])

	var outSchema map[string]any
	require.NoError(t, json.Unmarshal([]byte(ms.output), &outSchema))
	assert.Equal(t, "object", outSchema["type"])

	// Dispatch a raw JSON request through the installed adapter.
	adapter, ok := c.handlers["CreateIncident"]
	require.True(t, ok, "handler adapter must be installed")
	respJSON, err := adapter(context.Background(), json.RawMessage(`{"title":"db is down","severity":1,"tags":["prod","db"]}`))
	require.NoError(t, err)
	assert.True(t, called, "typed handler must have been invoked")
	assert.JSONEq(t, `{"id":"INC-42"}`, string(respJSON))
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
	c := applyOptions(WithHandler("Bad", handler))
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
		WithHandler("Dup", h),
		WithHandler("Dup", h),
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
	c := applyOptions(WithHandler("M", handler))
	require.Empty(t, c.optionErrs)
	respJSON, err := c.handlers["M"](context.Background(), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"zero"}`, string(respJSON))
}
