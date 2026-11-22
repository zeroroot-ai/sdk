// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// countermeasureServer is a HarnessCallbackService that answers only
// GetCountermeasures.
type countermeasureServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer

	got  *harnesspb.GetCountermeasuresRequest
	resp *harnesspb.GetCountermeasuresResponse
	err  error
}

func (s *countermeasureServer) GetCountermeasures(
	_ context.Context, req *harnesspb.GetCountermeasuresRequest,
) (*harnesspb.GetCountermeasuresResponse, error) {
	s.got = req
	return s.resp, s.err
}

func newCountermeasureClient(t *testing.T, impl *countermeasureServer) *CallbackClient {
	t.Helper()
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, impl)
	conn := dialBufconn(t, srv)
	return &CallbackClient{
		conn:      conn,
		client:    harnesspb.NewHarnessCallbackServiceClient(conn),
		connected: true,
		missionID: "mission-1",
		agentName: "fix-agent",
		taskID:    "task-1",
	}
}

func TestCallbackClientGetCountermeasures(t *testing.T) {
	srv := &countermeasureServer{resp: &harnesspb.GetCountermeasuresResponse{
		Countermeasures: []*harnesspb.Countermeasure{{Id: "D3-FA", Name: "File Analysis", Tactic: "Detect"}},
	}}
	client := newCountermeasureClient(t, srv)

	resp, err := client.GetCountermeasures(context.Background(), &harnesspb.GetCountermeasuresRequest{Technique: "T1190"})
	require.NoError(t, err)
	assert.Equal(t, "D3-FA", resp.GetCountermeasures()[0].GetId())
	assert.Equal(t, "T1190", srv.got.GetTechnique())
	assert.Equal(t, "mission-1", srv.got.GetContext().GetMissionId())
}

func TestCallbackClientGetCountermeasures_TransportError(t *testing.T) {
	client := newCountermeasureClient(t, &countermeasureServer{err: status.Error(codes.Unavailable, "down")})

	_, err := client.GetCountermeasures(context.Background(), &harnesspb.GetCountermeasuresRequest{Technique: "T1190"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetCountermeasures:")
	assert.Equal(t, codes.Unavailable, status.Code(errors.Unwrap(err)))
}

func TestCallbackClientGetCountermeasures_NotConnected(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)
	require.NoError(t, client.Close())

	_, err = client.GetCountermeasures(context.Background(), &harnesspb.GetCountermeasuresRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetCountermeasures")
}

func TestCallbackHarnessCountermeasures(t *testing.T) {
	srv := &countermeasureServer{resp: &harnesspb.GetCountermeasuresResponse{
		Countermeasures: []*harnesspb.Countermeasure{
			{Id: "D3-FA", Name: "File Analysis", Tactic: "Detect", NistControls: []string{"si-3"}},
		},
	}}
	h := &CallbackHarness{client: newCountermeasureClient(t, srv)}

	got, err := h.Countermeasures(context.Background(), "T1190")
	require.NoError(t, err)
	assert.Equal(t, []agent.Countermeasure{
		{ID: "D3-FA", Name: "File Analysis", Tactic: "Detect", NISTControls: []string{"si-3"}},
	}, got)
	assert.Equal(t, "T1190", srv.got.GetTechnique())
}

// TestCallbackHarnessCountermeasures_NoneIsEmptyAndNoError: a technique that no
// countermeasure counters is a fact about the technique, not a failed read.
func TestCallbackHarnessCountermeasures_NoneIsEmptyAndNoError(t *testing.T) {
	h := &CallbackHarness{client: newCountermeasureClient(t, &countermeasureServer{
		resp: &harnesspb.GetCountermeasuresResponse{},
	})}

	got, err := h.Countermeasures(context.Background(), "T9999")
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestCallbackHarnessCountermeasures_FailureIsAnErrorAndNeverEmpty: an empty
// slice with a nil error would read as "no countermeasure exists".
func TestCallbackHarnessCountermeasures_FailureIsAnErrorAndNeverEmpty(t *testing.T) {
	down := &CallbackHarness{client: newCountermeasureClient(t, &countermeasureServer{
		err: status.Error(codes.Unavailable, "down"),
	})}
	got, err := down.Countermeasures(context.Background(), "T1190")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "countermeasures callback failed")

	rejected := &CallbackHarness{client: newCountermeasureClient(t, &countermeasureServer{
		resp: &harnesspb.GetCountermeasuresResponse{Error: &harnesspb.HarnessError{Message: "catalog not loaded"}},
	})}
	got, err = rejected.Countermeasures(context.Background(), "T1190")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "catalog not loaded")
}

func TestPlatformHarnessCountermeasuresAreUnavailable(t *testing.T) {
	got, err := (&PlatformHarness{}).Countermeasures(context.Background(), "T1190")
	assert.Nil(t, got)
	assert.ErrorIs(t, err, agent.ErrKnowledgeUnavailable)
}

// TestCallbackHarnessCountermeasures_NotATechniqueIDMakesNoCall: a value that is
// not an ATT&CK technique id returns an empty slice and no error, and the daemon
// is not called.
func TestCallbackHarnessCountermeasures_NotATechniqueIDMakesNoCall(t *testing.T) {
	srv := &countermeasureServer{resp: &harnesspb.GetCountermeasuresResponse{
		Countermeasures: []*harnesspb.Countermeasure{{Id: "D3-FA"}},
	}}
	h := &CallbackHarness{client: newCountermeasureClient(t, srv)}

	for _, bad := range []string{"", "d3-fa", "T11", "T1190.1"} {
		got, err := h.Countermeasures(context.Background(), bad)
		require.NoError(t, err, bad)
		assert.Empty(t, got, bad)
	}
	assert.Nil(t, srv.got, "the daemon must not be called")
}
