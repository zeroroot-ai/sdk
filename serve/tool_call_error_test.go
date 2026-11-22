// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/zeroroot-ai/sdk/agent"
	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

type refusingServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	resp *harnesspb.CallToolProtoResponse
}

func (s *refusingServer) CallToolProto(context.Context, *harnesspb.CallToolProtoRequest) (*harnesspb.CallToolProtoResponse, error) {
	return s.resp, nil
}

// TestCallToolProto_RefusalCarriesTheCode: an agent tells a refusal that can
// pass from a final refusal by code, and the text stays as it was.
func TestCallToolProto_RefusalCarriesTheCode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  commonpb.ErrorCode
		retry bool
		want  string
	}{
		{"merge not granted", commonpb.ErrorCode_ERROR_CODE_UNAVAILABLE, true, agent.ToolErrUnavailable},
		{"not authorized", commonpb.ErrorCode_ERROR_CODE_PERMISSION_DENIED, false, agent.ToolErrPermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := grpc.NewServer()
			harnesspb.RegisterHarnessCallbackServiceServer(srv, &refusingServer{resp: &harnesspb.CallToolProtoResponse{
				Error: &harnesspb.HarnessError{Code: tc.code, Message: "the daemon said no"},
			}})
			conn := dialBufconn(t, srv)
			h := NewCallbackHarness(&CallbackClient{
				conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true, missionID: "m-1",
			})

			err := h.CallToolProto(context.Background(), "invoke_tool", &structpb.Struct{}, &structpb.Struct{})
			require.Error(t, err)
			var tce *agent.ToolCallError
			require.ErrorAs(t, err, &tce)
			assert.Equal(t, tc.want, tce.Code)
			assert.Equal(t, tc.retry, tce.Retryable())
			assert.Equal(t, "CallToolProto error: the daemon said no", err.Error())
		})
	}
}

type errCallback struct{ got error }

func (c *errCallback) OnProgress(int, string, string)    {}
func (c *errCallback) OnPartial(protolib.Message, bool)  {}
func (c *errCallback) OnWarning(string, string)          {}
func (c *errCallback) OnError(err error, _ bool)         { c.got = err }
func (c *errCallback) OnComplete(protolib.Message, bool) {}

// TestStreamError_CarriesTheCode: the streaming path returns the same typed
// error as the unary path.
func TestStreamError_CarriesTheCode(t *testing.T) {
	msg := &harnesspb.CallToolProtoStreamResponse{Payload: &harnesspb.CallToolProtoStreamResponse_Error{
		Error: &harnesspb.ToolErrorEvent{
			Error: &harnesspb.HarnessError{Code: commonpb.ErrorCode_ERROR_CODE_PERMISSION_DENIED, Message: "no"},
			Fatal: true,
		},
	}}
	cb := &errCallback{}
	err := dispatchToolStreamMessage(msg, &structpb.Struct{}, cb, protojson.UnmarshalOptions{})
	var tce *agent.ToolCallError
	require.ErrorAs(t, err, &tce)
	assert.Equal(t, agent.ToolErrPermissionDenied, tce.Code)
	assert.Equal(t, "no", tce.Message)
	assert.Same(t, err, cb.got)
}
