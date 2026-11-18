// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

type toolCallDaemon struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	resp *harnesspb.CallToolProtoResponse
}

func (d *toolCallDaemon) CallToolProto(context.Context, *harnesspb.CallToolProtoRequest) (*harnesspb.CallToolProtoResponse, error) {
	return d.resp, nil
}

func callbackHarnessFor(t *testing.T, srv harnesspb.HarnessCallbackServiceServer) *CallbackHarness {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	cc := &CallbackClient{conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true}
	return &CallbackHarness{client: cc, tokenTracker: NewCallbackTokenTracker(), tracer: defaultNoopTracer()}
}

func TestCallbackCallToolProtoGivesTheToolCallID(t *testing.T) {
	d := &toolCallDaemon{resp: &harnesspb.CallToolProtoResponse{OutputJson: []byte(`"pong"`), ToolCallId: "tc-7"}}
	h := callbackHarnessFor(t, d)

	var id string
	out := &wrapperspb.StringValue{}
	require.NoError(t, h.CallToolProto(context.Background(), "echo", wrapperspb.String("ping"), out, agent.ReceiveToolCallID(&id)))
	assert.Equal(t, "tc-7", id)
	assert.Equal(t, "pong", out.GetValue())

	// A failed call is recorded too: the id is set, and the call returns the error.
	d.resp = &harnesspb.CallToolProtoResponse{ToolCallId: "tc-8", Error: &harnesspb.HarnessError{Message: "boom"}}
	id = ""
	require.Error(t, h.CallToolProto(context.Background(), "echo", wrapperspb.String("ping"), out, agent.ReceiveToolCallID(&id)))
	assert.Equal(t, "tc-8", id)

	// An old daemon sends none: the id is the empty string, never a made-up value.
	d.resp = &harnesspb.CallToolProtoResponse{OutputJson: []byte(`"pong"`)}
	id = "stale"
	require.NoError(t, h.CallToolProto(context.Background(), "echo", wrapperspb.String("ping"), out, agent.ReceiveToolCallID(&id)))
	assert.Empty(t, id)

	// No option: nothing to set, and the call works.
	require.NoError(t, h.CallToolProto(context.Background(), "echo", wrapperspb.String("ping"), out))
}

type toolCallComponent struct {
	componentpb.UnimplementedComponentServiceServer
}

func (toolCallComponent) CallTool(context.Context, *componentpb.CallToolRequest) (*componentpb.CallToolResponse, error) {
	return &componentpb.CallToolResponse{OutputJson: `"pong"`, ToolCallId: "tc-p"}, nil
}

func (toolCallComponent) CallToolStream(
	_ *componentpb.CallToolStreamRequest, s grpc.ServerStreamingServer[componentpb.CallToolStreamResponse],
) error {
	if err := s.Send(&componentpb.CallToolStreamResponse{EventType: "progress", PayloadJson: `{"percent":1}`}); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	if err := s.Send(&componentpb.CallToolStreamResponse{
		EventType: "result", PayloadJson: `"done"`, Done: true, ToolCallId: "tc-s",
	}); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

func TestPlatformToolCallIDs(t *testing.T) {
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) { componentpb.RegisterComponentServiceServer(s, toolCallComponent{}) })
	defer cleanup()

	var id string
	out := &wrapperspb.StringValue{}
	require.NoError(t, h.CallToolProto(context.Background(), "echo", wrapperspb.String("x"), out, agent.ReceiveToolCallID(&id)))
	assert.Equal(t, "tc-p", id)

	id = ""
	require.NoError(t, h.CallToolProtoStream(context.Background(), "echo", wrapperspb.String("x"), out, nil, agent.ReceiveToolCallID(&id)))
	assert.Equal(t, "tc-s", id, "the id comes on the last event")
}
