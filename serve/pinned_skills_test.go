// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/zeroroot-ai/sdk/agent"
)

func TestTaskPinnedSkillsRoundTrip(t *testing.T) {
	in := agent.Task{ID: "t1", Goal: "g", PinnedSkills: []string{"skill:a", "skill:b"}}
	back := ProtoToTask(TaskToProto(in))
	assert.Equal(t, []string{"skill:a", "skill:b"}, back.PinnedSkills)
	assert.Empty(t, ProtoToTask(TaskToProto(agent.Task{ID: "t2"})).PinnedSkills)
}
