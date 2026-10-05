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
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

// updateDefMock answers UpdateMissionDefinition with a fixed result.
type updateDefMock struct {
	extendedMockClient
	resp *daemonpb.UpdateMissionDefinitionResponse
	err  error
	got  *daemonpb.UpdateMissionDefinitionRequest
}

func (m *updateDefMock) UpdateMissionDefinition(
	_ context.Context, req *daemonpb.UpdateMissionDefinitionRequest, _ ...grpc.CallOption,
) (*daemonpb.UpdateMissionDefinitionResponse, error) {
	m.got = req
	return m.resp, m.err
}

func TestClient_UpdateMissionDefinition(t *testing.T) {
	def := &missionpb.MissionDefinition{Name: "recon"}

	t.Run("success returns the id and sends the definition", func(t *testing.T) {
		mock := &updateDefMock{resp: &daemonpb.UpdateMissionDefinitionResponse{MissionDefinitionId: "d-1"}}
		id, err := (&Client{daemon: mock}).UpdateMissionDefinition(context.Background(), def)
		require.NoError(t, err)
		assert.Equal(t, "d-1", id)
		assert.Equal(t, "recon", mock.got.GetDefinition().GetName())
	})

	t.Run("the input is checked before the call", func(t *testing.T) {
		c := &Client{daemon: &updateDefMock{}}
		_, err := c.UpdateMissionDefinition(context.Background(), nil)
		require.ErrorContains(t, err, "definition is required")
		_, err = c.UpdateMissionDefinition(context.Background(), &missionpb.MissionDefinition{})
		require.ErrorContains(t, err, "definition name is required")
	})

	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"not found", status.Error(codes.NotFound, "x"), `mission definition "recon" not found`},
		{"unavailable", status.Error(codes.Unavailable, "x"), "daemon not responding"},
		{"invalid", status.Error(codes.InvalidArgument, "bad node"), "invalid mission definition: bad node"},
		{"other status", status.Error(codes.Internal, "boom"), "failed to update mission definition: boom"},
		{"plain error", errors.New("broken pipe"), "failed to update mission definition: broken pipe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&Client{daemon: &updateDefMock{err: tc.err}}).UpdateMissionDefinition(context.Background(), def)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
