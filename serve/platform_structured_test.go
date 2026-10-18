// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/llm"
)

// structuredServer records the CompleteStructured request it receives.
type structuredServer struct {
	componentpb.UnimplementedComponentServiceServer
	got *componentpb.CompleteStructuredRequest
}

func (s *structuredServer) CompleteStructured(
	_ context.Context, req *componentpb.CompleteStructuredRequest,
) (*componentpb.CompleteStructuredResponse, error) {
	s.got = req
	return &componentpb.CompleteStructuredResponse{ResultJson: `{"verdict":"yes","score":3}`}, nil
}

type structuredVerdict struct {
	Verdict string `json:"verdict"`
	Score   int    `json:"score"`
}

// sdk#300: the platform harness sent json.Marshal of the struct value, the
// struct's zero values, where component.proto asks for a JSON Schema.
func TestPlatformHarness_CompleteStructured_SendsAJSONSchema(t *testing.T) {
	srv := &structuredServer{}
	harness, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, srv)
	})
	defer cleanup()

	_, err := harness.CompleteStructured(context.Background(), "primary",
		[]llm.Message{{Role: llm.RoleUser, Content: "judge it"}}, structuredVerdict{})
	require.NoError(t, err)
	require.NotNil(t, srv.got)

	var sent map[string]any
	require.NoError(t, json.Unmarshal([]byte(srv.got.SchemaJson), &sent))
	assert.Equal(t, "object", sent["type"], "schema_json must be a JSON Schema, got %s", srv.got.SchemaJson)
	props, ok := sent["properties"].(map[string]any)
	require.True(t, ok, "schema_json has no properties: %s", srv.got.SchemaJson)
	assert.Contains(t, props, "verdict")
	assert.Contains(t, props, "score")
}

// sdk#299: the result is the decoded JSON value, a map for an object schema,
// not a pointer to the caller's struct.
func TestPlatformHarness_CompleteStructured_ReturnsTheDecodedJSON(t *testing.T) {
	srv := &structuredServer{}
	harness, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, srv)
	})
	defer cleanup()

	got, err := harness.CompleteStructured(context.Background(), "primary",
		[]llm.Message{{Role: llm.RoleUser, Content: "judge it"}}, structuredVerdict{})
	require.NoError(t, err)

	m, ok := got.(map[string]any)
	require.True(t, ok, "result is %T, want map[string]any", got)
	assert.Equal(t, "yes", m["verdict"])
	assert.InDelta(t, 3.0, m["score"], 0)
}

// callbackStructuredServer answers LLMCompleteStructured with an object result.
type callbackStructuredServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
}

func (callbackStructuredServer) LLMCompleteStructured(
	_ context.Context, _ *harnesspb.LLMCompleteStructuredRequest,
) (*harnesspb.LLMCompleteStructuredResponse, error) {
	return &harnesspb.LLMCompleteStructuredResponse{
		Result: ToTypedValue(map[string]any{"verdict": "yes"}),
	}, nil
}

// sdk#299: the callback harness also returns the decoded value, a map for an
// object schema, not a pointer to the caller's struct.
func TestCallbackHarness_CompleteStructured_ReturnsTheDecodedJSON(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, callbackStructuredServer{})
	go func() { _ = srv.Serve(lis) }()
	//nolint:staticcheck // bufconn dialling mirrors the other serve tests
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	})
	h := &CallbackHarness{
		client: &CallbackClient{
			conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true,
		},
		tracer:       defaultNoopTracer(),
		tokenTracker: NewCallbackTokenTracker(),
	}

	got, err := h.CompleteStructured(context.Background(), "primary",
		[]llm.Message{{Role: llm.RoleUser, Content: "judge it"}}, structuredVerdict{})
	require.NoError(t, err)
	m, ok := got.(map[string]any)
	require.True(t, ok, "result is %T, want map[string]any", got)
	assert.Equal(t, "yes", m["verdict"])
}
