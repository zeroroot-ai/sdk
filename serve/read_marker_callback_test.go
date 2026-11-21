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
	"google.golang.org/protobuf/types/known/timestamppb"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// markerServer is a HarnessCallbackService that answers only the two read
// marker RPCs.
type markerServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer

	gotWrite *harnesspb.WriteReadMarkerRequest
	write    *harnesspb.WriteReadMarkerResponse
	gotLast  *harnesspb.LastReadMarkerRequest
	last     *harnesspb.LastReadMarkerResponse
	err      error
}

func (s *markerServer) WriteReadMarker(
	_ context.Context, req *harnesspb.WriteReadMarkerRequest,
) (*harnesspb.WriteReadMarkerResponse, error) {
	s.gotWrite = req
	return s.write, s.err
}

func (s *markerServer) LastReadMarker(
	_ context.Context, req *harnesspb.LastReadMarkerRequest,
) (*harnesspb.LastReadMarkerResponse, error) {
	s.gotLast = req
	return s.last, s.err
}

// newMarkerClient returns a connected CallbackClient that talks to srvImpl.
func newMarkerClient(t *testing.T, srvImpl *markerServer) *CallbackClient {
	t.Helper()
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, srvImpl)
	conn := dialBufconn(t, srv)
	return &CallbackClient{
		conn:      conn,
		client:    harnesspb.NewHarnessCallbackServiceClient(conn),
		connected: true,
		missionID: "mission-1",
		agentName: "intake",
		taskID:    "task-1",
	}
}

func TestCallbackClientWriteReadMarker(t *testing.T) {
	srv := &markerServer{write: &harnesspb.WriteReadMarkerResponse{}}
	client := newMarkerClient(t, srv)

	resp, err := client.WriteReadMarker(context.Background(), &harnesspb.WriteReadMarkerRequest{
		Connector: "splunk",
		Cursor:    "2026-11-20T10:00:00Z",
		ResultIds: []string{"r-1", "r-2"},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.GetError())

	// The client stamps the context and keeps the fields of the caller.
	assert.Equal(t, "mission-1", srv.gotWrite.GetContext().GetMissionId())
	assert.Equal(t, "intake", srv.gotWrite.GetContext().GetAgentName())
	assert.Equal(t, "splunk", srv.gotWrite.GetConnector())
	assert.Equal(t, "2026-11-20T10:00:00Z", srv.gotWrite.GetCursor())
	assert.Equal(t, []string{"r-1", "r-2"}, srv.gotWrite.GetResultIds())
}

func TestCallbackClientLastReadMarker(t *testing.T) {
	at := timestamppb.Now()
	srv := &markerServer{last: &harnesspb.LastReadMarkerResponse{
		Found: true,
		Marker: &harnesspb.ReadMarker{
			Connector: "splunk", Cursor: "c-9", ResultIds: []string{"r-9"}, At: at,
		},
	}}
	client := newMarkerClient(t, srv)

	resp, err := client.LastReadMarker(context.Background(), &harnesspb.LastReadMarkerRequest{Connector: "splunk"})
	require.NoError(t, err)
	assert.True(t, resp.GetFound())
	assert.Equal(t, "c-9", resp.GetMarker().GetCursor())
	assert.Equal(t, []string{"r-9"}, resp.GetMarker().GetResultIds())
	assert.True(t, at.AsTime().Equal(resp.GetMarker().GetAt().AsTime()))

	assert.Equal(t, "mission-1", srv.gotLast.GetContext().GetMissionId())
	assert.Equal(t, "splunk", srv.gotLast.GetConnector())
}

func TestCallbackClientLastReadMarker_NoMarker(t *testing.T) {
	client := newMarkerClient(t, &markerServer{last: &harnesspb.LastReadMarkerResponse{}})

	resp, err := client.LastReadMarker(context.Background(), &harnesspb.LastReadMarkerRequest{Connector: "splunk"})
	require.NoError(t, err)
	assert.False(t, resp.GetFound())
	assert.Nil(t, resp.GetMarker())
}

func TestCallbackClientReadMarker_TransportError(t *testing.T) {
	client := newMarkerClient(t, &markerServer{err: status.Error(codes.Unavailable, "down")})

	_, err := client.WriteReadMarker(context.Background(), &harnesspb.WriteReadMarkerRequest{Connector: "splunk"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WriteReadMarker:")
	assert.Equal(t, codes.Unavailable, status.Code(errors.Unwrap(err)))

	_, err = client.LastReadMarker(context.Background(), &harnesspb.LastReadMarkerRequest{Connector: "splunk"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LastReadMarker:")
	assert.Equal(t, codes.Unavailable, status.Code(errors.Unwrap(err)))
}

func TestCallbackClientReadMarkerCallsNotConnected(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)
	require.NoError(t, client.Close())

	_, err = client.WriteReadMarker(context.Background(), &harnesspb.WriteReadMarkerRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WriteReadMarker")

	_, err = client.LastReadMarker(context.Background(), &harnesspb.LastReadMarkerRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LastReadMarker")
}
