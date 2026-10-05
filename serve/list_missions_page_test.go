// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/mission"
)

// pagedMissionServer serves `total` missions in pages of at most `page`.
type pagedMissionServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	total, page int
	// ignoreSize makes the server return a full page whatever page_size says.
	ignoreSize bool
	// rpcErr and harnessErr make the server fail.
	rpcErr     error
	harnessErr string

	mu   sync.Mutex
	reqs []*harnesspb.ListMissionsRequest
}

func (s *pagedMissionServer) ListMissions(_ context.Context, req *harnesspb.ListMissionsRequest) (*harnesspb.ListMissionsResponse, error) {
	s.mu.Lock()
	s.reqs = append(s.reqs, req)
	s.mu.Unlock()

	if s.rpcErr != nil {
		return nil, s.rpcErr
	}
	if s.harnessErr != "" {
		return &harnesspb.ListMissionsResponse{Error: &harnesspb.HarnessError{Message: s.harnessErr}}, nil
	}
	start := 0
	if req.GetPageToken() != "" {
		if _, err := fmt.Sscanf(req.GetPageToken(), "at-%d", &start); err != nil {
			return nil, fmt.Errorf("bad token: %w", err)
		}
	}
	size := s.page
	if ps := int(req.GetPageSize()); !s.ignoreSize && ps > 0 && ps < size {
		size = ps
	}
	end := min(start+size, s.total)
	resp := &harnesspb.ListMissionsResponse{}
	for i := start; i < end; i++ {
		resp.Missions = append(resp.Missions, &harnesspb.MissionInfo{Id: fmt.Sprintf("m-%d", i)})
	}
	if end < s.total {
		resp.NextPageToken = fmt.Sprintf("at-%d", end)
	}
	return resp, nil
}

func pagedHarness(t *testing.T, srvImpl *pagedMissionServer) *CallbackHarness {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, srvImpl)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("paged mission server exited: %v", err)
		}
	}()
	//nolint:staticcheck // bufconn dialling mirrors the other serve tests
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	})
	return &CallbackHarness{
		client: &CallbackClient{
			conn:      conn,
			client:    harnesspb.NewHarnessCallbackServiceClient(conn),
			connected: true,
		},
		tracer: defaultNoopTracer(),
	}
}

// TestCallbackHarness_ListMissionsReadsEachPage proves that the harness
// follows next_page_token (ADR-0028, rule 3; sdk#186).
func TestCallbackHarness_ListMissionsReadsEachPage(t *testing.T) {
	t.Run("no limit reads every page", func(t *testing.T) {
		srv := &pagedMissionServer{total: 5, page: 2}
		got, err := pagedHarness(t, srv).ListMissions(context.Background(), &mission.MissionFilter{})
		require.NoError(t, err)
		assert.Len(t, got, 5)
		assert.Len(t, srv.reqs, 3)
		assert.Equal(t, "at-2", srv.reqs[1].GetPageToken())
	})

	t.Run("a limit stops the read", func(t *testing.T) {
		srv := &pagedMissionServer{total: 10, page: 2}
		got, err := pagedHarness(t, srv).ListMissions(context.Background(), &mission.MissionFilter{Limit: 3})
		require.NoError(t, err)
		assert.Len(t, got, 3)
		assert.Equal(t, int32(3), srv.reqs[0].GetPageSize())
		assert.Equal(t, int32(1), srv.reqs[1].GetPageSize())
	})

	t.Run("a server that returns too many is cut to the limit", func(t *testing.T) {
		srv := &pagedMissionServer{total: 10, page: 5, ignoreSize: true}
		got, err := pagedHarness(t, srv).ListMissions(context.Background(), &mission.MissionFilter{Limit: 3})
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})

	t.Run("an RPC error is returned", func(t *testing.T) {
		srv := &pagedMissionServer{total: 1, page: 1, rpcErr: status.Error(codes.Unavailable, "down")}
		_, err := pagedHarness(t, srv).ListMissions(context.Background(), nil)
		require.ErrorContains(t, err, "list missions callback failed")
	})

	t.Run("a harness error is returned", func(t *testing.T) {
		srv := &pagedMissionServer{total: 1, page: 1, harnessErr: "no tenant"}
		_, err := pagedHarness(t, srv).ListMissions(context.Background(), nil)
		require.ErrorContains(t, err, "list missions error: no tenant")
	})

	t.Run("a nil filter reads one page set", func(t *testing.T) {
		srv := &pagedMissionServer{total: 1, page: 2}
		got, err := pagedHarness(t, srv).ListMissions(context.Background(), nil)
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})
}
