// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

// TestProtoToTask_CarriesJob checks that the job of a fork claim survives the
// conversion, and that the spec is copied.
func TestProtoToTask_CarriesJob(t *testing.T) {
	spec := &jobpb.JobSpec{Goal: "g", CredentialNames: []string{"npm-token"}}
	pt := &typespb.Task{
		Id: "t-1", Goal: "ship it", Job: spec,
		Constraints: &typespb.TaskConstraints{MaxTurns: 20, AllowedTools: []string{"Bash"}},
	}
	got := ProtoToTask(pt)
	require.NotNil(t, got.Job)
	assert.True(t, proto.Equal(spec, got.Job))
	assert.NotSame(t, spec, got.Job, "the converter copies the spec")
	assert.Equal(t, 20, got.Constraints.MaxTurns)
	assert.Nil(t, ProtoToTask(&typespb.Task{Id: "t-2"}).Job)
}
