// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCallOptions(t *testing.T) {
	var id string
	o := ApplyToolCallOptions([]ToolCallOption{nil, ReceiveToolCallID(&id)})
	o.SetToolCallID("tc-1")
	assert.Equal(t, "tc-1", id)

	// With no destination, setting an id does nothing.
	ApplyToolCallOptions(nil).SetToolCallID("tc-2")
	assert.Equal(t, "tc-1", id)
}

func TestDelegateOptions(t *testing.T) {
	assert.Equal(t, DelegateOptions{}, ApplyDelegateOptions(nil))
	assert.Equal(t, "bank-1", ApplyDelegateOptions([]DelegateOption{nil, WithBank("bank-1")}).BankID)
}

func TestTaskFromMessageBadEnvelope(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("x"))
	msg.TaskID, msg.ContextID = "t", "c"
	msg.SetMeta(ExtensionKey, "not an object")
	_, err := TaskFromMessage(msg)
	require.ErrorContains(t, err, "decode gibson envelope")

	msg.SetMeta(ExtensionKey, map[string]any{"bad": make(chan int)})
	_, err = TaskFromMessage(msg)
	require.ErrorContains(t, err, "encode gibson envelope")

	msg.SetMeta(ExtensionKey, map[string]any{"constraints": map[string]any{"max_turns": 3}, "job": nil})
	task, err := TaskFromMessage(msg)
	require.NoError(t, err)
	assert.Equal(t, 3, task.Constraints.MaxTurns)
	assert.Nil(t, task.Job)
}

func TestMessageTextSkipsNilParts(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, nil, a2a.NewTextPart("a"), a2a.NewTextPart(""))
	assert.Equal(t, "a", messageText(msg))
}

func TestResultToTaskBadOutput(t *testing.T) {
	_, err := ResultToTask(Task{ID: "t", ContextID: "c"}, Result{Output: make(chan int)})
	require.Error(t, err)
	got, err := ResultToTask(Task{ID: "t", ContextID: "c"}, Result{Output: a2a.NewTextPart("p")})
	require.NoError(t, err)
	require.Len(t, got.Artifacts, 1)
}

func TestFailureText(t *testing.T) {
	assert.Equal(t, "the run failed", failureText(Result{}))
	assert.Equal(t, "boom", failureText(Result{Error: errString("boom")}))
}

type errString string

func (e errString) Error() string { return string(e) }

func TestCardOfWithNoCapabilities(t *testing.T) {
	skills, _, _ := CardOf(namedAgent{})
	require.Len(t, skills, 1)
	assert.Equal(t, "solo", skills[0].ID)
}

type namedAgent struct{ Agent }

func (namedAgent) Name() string             { return "solo" }
func (namedAgent) Description() string      { return "d" }
func (namedAgent) Capabilities() []string   { return nil }
func (namedAgent) TargetTypes() []string    { return nil }
func (namedAgent) TechniqueTypes() []string { return nil }
