// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package daemonclient

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
)

// pageMock records the page fields of each list request.
type pageMock struct {
	extendedMockClient
	listReq    *daemonpb.ListMissionsRequest
	historyReq *daemonpb.GetMissionHistoryRequest
	err        error
}

func (m *pageMock) ListMissions(_ context.Context, req *daemonpb.ListMissionsRequest, _ ...grpc.CallOption) (*daemonpb.ListMissionsResponse, error) {
	m.listReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &daemonpb.ListMissionsResponse{Total: 7, NextPageToken: "next-1"}, nil
}

func (m *pageMock) GetMissionHistory(_ context.Context, req *daemonpb.GetMissionHistoryRequest, _ ...grpc.CallOption) (*daemonpb.GetMissionHistoryResponse, error) {
	m.historyReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &daemonpb.GetMissionHistoryResponse{Total: 3, NextPageToken: "next-2"}, nil
}

// TestClient_ListCallsSendThePageAndReturnTheNextToken proves the one page
// shape of ADR-0028, rule 3 (sdk#186).
func TestClient_ListCallsSendThePageAndReturnTheNextToken(t *testing.T) {
	mock := &pageMock{}
	c := &Client{daemon: mock}

	_, page, err := c.ListMissions(context.Background(), false, "", "", PageRequest{Size: 25, Token: "tok-a"})
	require.NoError(t, err)
	assert.Equal(t, int32(25), mock.listReq.GetPageSize())
	assert.Equal(t, "tok-a", mock.listReq.GetPageToken())
	assert.Equal(t, PageResult{Total: 7, NextToken: "next-1"}, page)

	_, page, err = c.GetMissionHistory(context.Background(), "recon", PageRequest{Size: 5000, Token: "tok-b"})
	require.NoError(t, err)
	assert.Equal(t, int32(1000), mock.historyReq.GetPageSize(), "a size above the wire bound is clamped")
	assert.Equal(t, "tok-b", mock.historyReq.GetPageToken())
	assert.Equal(t, PageResult{Total: 3, NextToken: "next-2"}, page)
}

func TestPageRequest_WireSize(t *testing.T) {
	assert.Equal(t, int32(0), PageRequest{}.wireSize())
	assert.Equal(t, int32(0), PageRequest{Size: -4}.wireSize())
	assert.Equal(t, int32(50), PageRequest{Size: 50}.wireSize())
	assert.Equal(t, int32(1000), PageRequest{Size: 1001}.wireSize())
}

// TestClient_ListCallsReturnAnEmptyPageOnError proves that each error path
// of the two list calls names the call and returns no page.
func TestClient_ListCallsReturnAnEmptyPageOnError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"a status error", status.Error(codes.Internal, "boom"), "boom"},
		{"a plain error", errors.New("broken pipe"), "broken pipe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{daemon: &pageMock{err: tc.err}}

			_, page, err := c.ListMissions(context.Background(), false, "", "", PageRequest{})
			require.ErrorContains(t, err, "failed to list missions")
			require.ErrorContains(t, err, tc.want)
			assert.Equal(t, PageResult{}, page)

			_, page, err = c.GetMissionHistory(context.Background(), "recon", PageRequest{})
			require.ErrorContains(t, err, "failed to get mission history")
			require.ErrorContains(t, err, tc.want)
			assert.Equal(t, PageResult{}, page)
		})
	}
}
