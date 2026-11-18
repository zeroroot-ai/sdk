// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/zeroroot-ai/sdk/agent"
	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

// These converters serve DelegateToAgent only. The next change deletes that
// call and this file. They are not the A2A conversion: that is in the agent
// package (a2a.go).

func legacyTaskToProto(t agent.Task) *typespb.Task {
	return &typespb.Task{
		Id:       t.ID,
		Goal:     t.Goal,
		Context:  ToTypedMap(t.Context),
		Metadata: ToTypedMap(t.Metadata),
		Constraints: &typespb.TaskConstraints{
			MaxTurns:     int32(t.Constraints.MaxTurns),
			MaxTokens:    int32(t.Constraints.MaxTokens),
			AllowedTools: t.Constraints.AllowedTools,
			BlockedTools: t.Constraints.BlockedTools,
		},
	}
}

func legacyProtoToResult(pr *typespb.Result) agent.Result {
	if pr == nil {
		return agent.Result{}
	}
	r := agent.Result{
		Output:   FromTypedValue(pr.GetOutput()),
		Findings: pr.GetFindingIds(),
		Metadata: FromTypedMap(pr.GetMetadata()),
	}
	switch pr.GetStatus() {
	case typespb.ResultStatus_RESULT_STATUS_SUCCESS, typespb.ResultStatus_RESULT_STATUS_PARTIAL:
		r.Status = a2a.TaskStateCompleted
	case typespb.ResultStatus_RESULT_STATUS_CANCELLED:
		r.Status = a2a.TaskStateCanceled
	default:
		r.Status = a2a.TaskStateFailed
	}
	if e := pr.GetError(); e != nil {
		details := make(map[string]any)
		for k, v := range e.GetDetails() {
			details[k] = v
		}
		r.ErrorInfo = &agent.ResultError{
			Code:      commonpb.ErrorCode_name[int32(e.GetCode())],
			Message:   e.GetMessage(),
			Details:   details,
			Retryable: e.GetRetryable(),
		}
	}
	return r
}
