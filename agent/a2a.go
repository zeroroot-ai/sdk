// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"google.golang.org/protobuf/encoding/protojson"

	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
)

// This file is the one place where the Go types of an agent author, Task and
// Result, meet the A2A types. The work item of an agent holds an A2A Message
// and the result holds an A2A Task. The Task states are the a2a.TaskState
// values. There is no second list of states.

// ExtensionKey is the key in Message.Metadata and Task.Metadata under which
// Gibson carries what A2A has no field for. The value is a JSON object. Its
// shape is Envelope.
const ExtensionKey = "https://zeroroot.ai/a2a/ext/gibson-run/v1"

// RunContext names the run that the daemon started a task for.
type RunContext struct {
	MissionRunID string `json:"mission_run_id,omitempty"`
	AgentRunID   string `json:"agent_run_id,omitempty"`
	RunNumber    int    `json:"run_number,omitempty"`
	// JobID names the job that this run serves. Empty for a run with no job.
	JobID        string `json:"job_id,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

// EnvelopeConstraints are the limits of a task, as in TaskConstraints.
type EnvelopeConstraints struct {
	MaxTurns     int      `json:"max_turns,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	BlockedTools []string `json:"blocked_tools,omitempty"`
}

// Envelope is the Gibson context of a work item. The daemon puts it in
// Message.Metadata[ExtensionKey]. All fields are optional.
//
// The mission id, the mission name and phase, the target and the full mission
// execution context stay in the context map of the work item (PollWork), as
// before. They are not in the envelope.
type Envelope struct {
	RunContext
	// PinnedSkills are the Agent Skill ids (skill:<name>) that the mission
	// pins for this task. This is the SKILL.md format, not an A2A card skill.
	PinnedSkills []string             `json:"pinned_skills,omitempty"`
	Constraints  *EnvelopeConstraints `json:"constraints,omitempty"`
	// Context and Metadata are the free-form maps of agent.Task.
	Context  map[string]any `json:"context,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	// Job is a gibson.job.v1.JobSpec in protojson form.
	Job json.RawMessage `json:"job,omitempty"`
}

// ResultEnvelope is the Gibson part of the metadata of the Task that the SDK
// returns. The SDK puts it in Task.Metadata[ExtensionKey].
type ResultEnvelope struct {
	// FindingIDs are the ids of the findings that the run submitted.
	FindingIDs []string `json:"finding_ids,omitempty"`
	// Metadata is Result.Metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
	// Error is the structured error of a failed run.
	Error *ResultError `json:"error,omitempty"`
}

// TaskFromMessage builds the Task of an agent author from the A2A message of a
// work item. The message must name its task and its context.
func TaskFromMessage(msg *a2a.Message) (Task, error) {
	if msg == nil {
		return Task{}, errors.New("a2a message is nil")
	}
	if msg.TaskID == "" {
		return Task{}, errors.New("a2a message has no taskId")
	}
	if msg.ContextID == "" {
		return Task{}, errors.New("a2a message has no contextId")
	}
	t := Task{
		ID:        string(msg.TaskID),
		Goal:      messageText(msg),
		ContextID: msg.ContextID,
		Message:   msg,
	}
	raw, ok := msg.Metadata[ExtensionKey]
	if !ok {
		return t, nil
	}
	env, err := decodeEnvelope(raw)
	if err != nil {
		return Task{}, err
	}
	t.Run = env.RunContext
	t.PinnedSkills = env.PinnedSkills
	t.Context = env.Context
	t.Metadata = env.Metadata
	if c := env.Constraints; c != nil {
		t.Constraints = TaskConstraints{
			MaxTurns: c.MaxTurns, MaxTokens: c.MaxTokens, AllowedTools: c.AllowedTools, BlockedTools: c.BlockedTools,
		}
	}
	if len(env.Job) > 0 && string(env.Job) != "null" {
		var job jobpb.JobSpec
		if err := protojson.Unmarshal(env.Job, &job); err != nil {
			return Task{}, fmt.Errorf("decode job of the gibson envelope: %w", err)
		}
		t.Job = &job
	}
	return t, nil
}

// decodeEnvelope reads the value of the extension key. The value is a decoded
// JSON object when the message came from JSON.
func decodeEnvelope(v any) (*Envelope, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode gibson envelope: %w", err)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode gibson envelope: %w", err)
	}
	return &env, nil
}

// messageText joins the text parts of a message with a newline.
func messageText(msg *a2a.Message) string {
	var parts []string
	for _, p := range msg.Parts {
		if p == nil {
			continue
		}
		if txt := p.Text(); txt != "" {
			parts = append(parts, txt)
		}
	}
	return strings.Join(parts, "\n")
}

// ResultToTask builds the A2A Task that the SDK returns for a run. The Task
// state is the Status of the result. The history starts with the message that
// started the run.
func ResultToTask(t Task, r Result) (*a2a.Task, error) {
	state := r.Status
	failed := r.Error != nil || r.ErrorInfo != nil
	if state == a2a.TaskStateUnspecified {
		state = a2a.TaskStateCompleted
		if failed {
			state = a2a.TaskStateFailed
		}
	}
	now := time.Now().UTC()
	out := &a2a.Task{
		ID:        a2a.TaskID(t.ID),
		ContextID: t.ContextID,
		Status:    a2a.TaskStatus{State: state, Timestamp: &now},
		Artifacts: append([]*a2a.Artifact(nil), r.Artifacts...),
	}
	if t.Message != nil {
		out.History = append(out.History, t.Message)
	}
	out.History = append(out.History, r.History...)

	if failed {
		msg := a2a.NewMessageForTask(a2a.MessageRoleAgent, out, a2a.NewTextPart(failureText(r)))
		out.Status.Message = msg
	}
	if r.Output != nil {
		part, err := outputPart(r.Output)
		if err != nil {
			return nil, err
		}
		switch state {
		case a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired:
			out.Status.Message = a2a.NewMessageForTask(a2a.MessageRoleAgent, out, part)
		default:
			out.Artifacts = append(out.Artifacts, &a2a.Artifact{
				ID: a2a.NewArtifactID(), Name: "output", Parts: a2a.ContentParts{part},
			})
		}
	}
	if len(r.Findings) > 0 || len(r.Metadata) > 0 || r.ErrorInfo != nil {
		out.SetMeta(ExtensionKey, ResultEnvelope{FindingIDs: r.Findings, Metadata: r.Metadata, Error: r.ErrorInfo})
	}
	return out, nil
}

func failureText(r Result) string {
	switch {
	case r.ErrorInfo != nil:
		return r.ErrorInfo.Error()
	case r.Error != nil:
		return r.Error.Error()
	default:
		return "the run failed"
	}
}

// outputPart converts the output of a result to an A2A part.
func outputPart(v any) (*a2a.Part, error) {
	switch o := v.(type) {
	case string:
		return a2a.NewTextPart(o), nil
	case []byte:
		return a2a.NewRawPart(o), nil
	case *a2a.Part:
		return o, nil
	default:
		// Check that the value has a JSON form, so the Task can be sent.
		if _, err := json.Marshal(v); err != nil {
			return nil, fmt.Errorf("encode the output of the result: %w", err)
		}
		return a2a.NewDataPart(v), nil
	}
}
