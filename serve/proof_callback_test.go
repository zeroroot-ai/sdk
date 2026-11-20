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

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// proofServer is a HarnessCallbackService that answers only the two proof RPCs.
type proofServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer

	gotProof *harnesspb.SubmitProofRequest
	proof    *harnesspb.SubmitProofResponse
	gotAuth  *harnesspb.RequestDestructiveAuthorizationRequest
	auth     *harnesspb.RequestDestructiveAuthorizationResponse
	err      error
}

func (s *proofServer) SubmitProof(_ context.Context, req *harnesspb.SubmitProofRequest) (*harnesspb.SubmitProofResponse, error) {
	s.gotProof = req
	return s.proof, s.err
}

func (s *proofServer) RequestDestructiveAuthorization(
	_ context.Context, req *harnesspb.RequestDestructiveAuthorizationRequest,
) (*harnesspb.RequestDestructiveAuthorizationResponse, error) {
	s.gotAuth = req
	return s.auth, s.err
}

// newProofClient returns a connected CallbackClient that talks to srvImpl.
func newProofClient(t *testing.T, srvImpl *proofServer) *CallbackClient {
	t.Helper()
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, srvImpl)
	conn := dialBufconn(t, srv)
	return &CallbackClient{
		conn:      conn,
		client:    harnesspb.NewHarnessCallbackServiceClient(conn),
		connected: true,
		missionID: "mission-1",
		agentName: "fixer",
		taskID:    "task-1",
	}
}

func TestCallbackClientSubmitProof(t *testing.T) {
	srv := &proofServer{proof: &harnesspb.SubmitProofResponse{
		HypothesisId: "h-1",
		Outcome:      harnesspb.SettlementOutcome_SETTLEMENT_OUTCOME_PENDING_AUTHORIZATION,
	}}
	client := newProofClient(t, srv)

	resp, err := client.SubmitProof(context.Background(), &harnesspb.SubmitProofRequest{
		HypothesisId:  "h-1",
		Technique:     "credential_access",
		PredicateName: "weak_password",
		ToolCallIds:   []string{"call-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, "h-1", resp.GetHypothesisId())
	assert.Equal(t, harnesspb.SettlementOutcome_SETTLEMENT_OUTCOME_PENDING_AUTHORIZATION, resp.GetOutcome())

	// The client stamps the context and keeps the fields of the caller.
	assert.Equal(t, "mission-1", srv.gotProof.GetContext().GetMissionId())
	assert.Equal(t, "weak_password", srv.gotProof.GetPredicateName())
	assert.Equal(t, []string{"call-1"}, srv.gotProof.GetToolCallIds())
}

func TestCallbackClientSubmitProof_TransportError(t *testing.T) {
	client := newProofClient(t, &proofServer{err: status.Error(codes.PermissionDenied, "no")})

	_, err := client.SubmitProof(context.Background(), &harnesspb.SubmitProofRequest{HypothesisId: "h-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SubmitProof:")
	assert.Equal(t, codes.PermissionDenied, status.Code(errors.Unwrap(err)))
}

func TestCallbackClientRequestDestructiveAuthorization(t *testing.T) {
	srv := &proofServer{auth: &harnesspb.RequestDestructiveAuthorizationResponse{}}
	client := newProofClient(t, srv)

	resp, err := client.RequestDestructiveAuthorization(context.Background(), &harnesspb.RequestDestructiveAuthorizationRequest{
		HypothesisId:      "h-1",
		PredicateName:     "weak_password",
		ActionDescription: "Reset the password of one account",
		Reversibility:     harnesspb.Reversibility_REVERSIBILITY_IRREVERSIBLE,
	})
	require.NoError(t, err)
	assert.Nil(t, resp.GetError())

	assert.Equal(t, "mission-1", srv.gotAuth.GetContext().GetMissionId())
	assert.Equal(t, "Reset the password of one account", srv.gotAuth.GetActionDescription())
	assert.Equal(t, harnesspb.Reversibility_REVERSIBILITY_IRREVERSIBLE, srv.gotAuth.GetReversibility())
}

func TestCallbackClientRequestDestructiveAuthorization_TransportError(t *testing.T) {
	client := newProofClient(t, &proofServer{err: status.Error(codes.Unavailable, "down")})

	_, err := client.RequestDestructiveAuthorization(context.Background(), &harnesspb.RequestDestructiveAuthorizationRequest{
		HypothesisId: "h-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestDestructiveAuthorization:")
	assert.Equal(t, codes.Unavailable, status.Code(errors.Unwrap(err)))
}

func TestCallbackClientProofCallsNotConnected(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)
	client.Close()

	_, err = client.SubmitProof(context.Background(), &harnesspb.SubmitProofRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SubmitProof")

	_, err = client.RequestDestructiveAuthorization(context.Background(), &harnesspb.RequestDestructiveAuthorizationRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestDestructiveAuthorization")
}
