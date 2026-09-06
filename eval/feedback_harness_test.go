// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpb "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	"github.com/zeroroot-ai/sdk/llm"
)

// TestFeedbackHarnessBasics tests basic feedback harness initialization and cleanup.
func TestFeedbackHarnessBasics(t *testing.T) {
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers:           []StreamingScorer{scorer},
		WarningThreshold:  0.5,
		CriticalThreshold: 0.2,
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Verify initialization
	assert.NotNil(t, fh)
	assert.NotNil(t, fh.recording)
	assert.Equal(t, 0.5, fh.opts.WarningThreshold)
	assert.Equal(t, 0.2, fh.opts.CriticalThreshold)

	// Verify no feedback initially
	feedback := fh.GetFeedback()
	assert.Nil(t, feedback)

	// Verify empty history
	history := fh.FeedbackHistory()
	assert.Empty(t, history)
}

// TestFeedbackHarnessDefaultThresholds tests default threshold values.
func TestFeedbackHarnessDefaultThresholds(t *testing.T) {
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score: 0.8,
		},
		supportsStream: true,
	}

	// Create with no thresholds specified
	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Verify defaults were applied
	assert.Equal(t, 0.5, fh.opts.WarningThreshold)
	assert.Equal(t, 0.2, fh.opts.CriticalThreshold)
	assert.Equal(t, 1, fh.opts.Frequency.EveryNSteps)
}

// TestFeedbackHarnessTriggerEvaluation tests feedback evaluation triggering.
func TestFeedbackHarnessTriggerEvaluation(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Looking good!",
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1, // Evaluate after every step
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Perform an operation to trigger evaluation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, err := fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	// Wait for async evaluation to complete
	time.Sleep(100 * time.Millisecond)

	// Check for feedback
	feedback := fh.GetFeedback()
	assert.NotNil(t, feedback)
	if feedback != nil {
		assert.Equal(t, 0.8, feedback.Overall.Score)
		assert.Contains(t, feedback.Scores, "test-scorer")
		assert.True(t, feedback.Consumed)
	}
}

// TestFeedbackHarnessStepFrequency tests step-based evaluation frequency.
func TestFeedbackHarnessStepFrequency(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 3, // Evaluate every 3 steps
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Perform 2 operations (should not trigger evaluation)
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait briefly
	time.Sleep(50 * time.Millisecond)

	// Should have no feedback yet
	feedback := fh.PeekFeedback()
	assert.Nil(t, feedback)

	// Perform 3rd operation (should trigger evaluation)
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for async evaluation
	time.Sleep(100 * time.Millisecond)

	// Should now have feedback
	feedback = fh.PeekFeedback()
	assert.NotNil(t, feedback)
}

// TestFeedbackHarnessDebounce tests debounce timing.
func TestFeedbackHarnessDebounce(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score: 0.8,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
			Debounce:    200 * time.Millisecond,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Perform first operation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Perform second operation immediately (within debounce window)
	_, _ = fh.Complete(ctx, "primary", messages)

	// Should not trigger new evaluation due to debounce
	// Wait and check history length
	time.Sleep(100 * time.Millisecond)
	history := fh.FeedbackHistory()

	// Should have at most 1 evaluation (debounce prevented second)
	assert.LessOrEqual(t, len(history), 1)
}

// TestFeedbackHarnessWarningAlert tests warning threshold alerts.
func TestFeedbackHarnessWarningAlert(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.4, // Below warning threshold
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionAdjust,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers:           []StreamingScorer{scorer},
		WarningThreshold:  0.5,
		CriticalThreshold: 0.2,
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Perform operation to trigger evaluation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Check for warning alert
	feedback := fh.GetFeedback()
	assert.NotNil(t, feedback)
	if feedback != nil {
		assert.NotEmpty(t, feedback.Alerts)
		hasWarning := false
		for _, alert := range feedback.Alerts {
			if alert.Level == AlertWarning {
				hasWarning = true
				break
			}
		}
		assert.True(t, hasWarning, "Expected warning alert to be generated")
	}
}

// TestFeedbackHarnessCriticalAlert tests critical threshold alerts.
func TestFeedbackHarnessCriticalAlert(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.1, // Below critical threshold
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionAbort,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers:           []StreamingScorer{scorer},
		WarningThreshold:  0.5,
		CriticalThreshold: 0.2,
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Perform operation to trigger evaluation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Check for critical alert
	feedback := fh.GetFeedback()
	assert.NotNil(t, feedback)
	if feedback != nil {
		assert.NotEmpty(t, feedback.Alerts)
		hasCritical := false
		for _, alert := range feedback.Alerts {
			if alert.Level == AlertCritical {
				hasCritical = true
				break
			}
		}
		assert.True(t, hasCritical, "Expected critical alert to be generated")
	}
}

// TestFeedbackHarnessAutoInject tests automatic feedback injection into LLM calls.
func TestFeedbackHarnessAutoInject(t *testing.T) {
	ctx := context.Background()

	// Track if feedback was injected
	var injectedMessages []llm.Message
	mock := &mockHarness{
		completeFunc: func(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
			injectedMessages = messages
			return &llm.CompletionResponse{Content: "response"}, nil
		},
	}

	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Good progress",
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers:    []StreamingScorer{scorer},
		AutoInject: true,
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// First call - no feedback yet
	messages := []llm.Message{{Role: "user", Content: "test1"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Second call - should inject feedback
	messages = []llm.Message{{Role: "user", Content: "test2"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Verify feedback was injected
	assert.NotEmpty(t, injectedMessages)
	if len(injectedMessages) > 0 {
		// First message should be system message with feedback
		firstMsg := injectedMessages[0]
		assert.Equal(t, llm.Role("system"), firstMsg.Role)
		assert.Contains(t, firstMsg.Content, "EVALUATION FEEDBACK")
	}
}

// TestFeedbackHarnessPeekVsGet tests the difference between Peek and Get.
func TestFeedbackHarnessPeekVsGet(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Trigger evaluation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Peek should not consume
	feedback1 := fh.PeekFeedback()
	assert.NotNil(t, feedback1)
	assert.False(t, feedback1.Consumed)

	// Peek again should return same feedback
	feedback2 := fh.PeekFeedback()
	assert.NotNil(t, feedback2)
	assert.Equal(t, feedback1.Timestamp, feedback2.Timestamp)

	// Get should consume
	feedback3 := fh.GetFeedback()
	assert.NotNil(t, feedback3)
	assert.True(t, feedback3.Consumed)

	// Get again should return nil (consumed)
	feedback4 := fh.GetFeedback()
	assert.Nil(t, feedback4)
}

// TestFeedbackHarnessFeedbackHistory tests feedback history tracking.
func TestFeedbackHarnessFeedbackHistory(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Trigger multiple evaluations
	messages := []llm.Message{{Role: "user", Content: "test"}}
	for range 3 {
		_, _ = fh.Complete(ctx, "primary", messages)
		time.Sleep(100 * time.Millisecond)
	}

	// Check history
	history := fh.FeedbackHistory()
	assert.GreaterOrEqual(t, len(history), 1)

	// Verify history is a copy (modifications don't affect internal state)
	if len(history) > 0 {
		originalTimestamp := history[0].Timestamp
		history[0].Timestamp = time.Now()

		// Get history again
		history2 := fh.FeedbackHistory()
		assert.Equal(t, originalTimestamp, history2[0].Timestamp)
	}
}

// TestFeedbackHarnessMultipleScorers tests aggregation of multiple scorers.
func TestFeedbackHarnessMultipleScorers(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}

	scorer1 := &mockStreamingScorer{
		name: "scorer-1",
		score: PartialScore{
			Score:      0.9,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
		},
		supportsStream: true,
	}

	scorer2 := &mockStreamingScorer{
		name: "scorer-2",
		score: PartialScore{
			Score:      0.7,
			Confidence: 0.8,
			Status:     ScoreStatusPartial,
			Action:     ActionAdjust,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer1, scorer2},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Trigger evaluation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Check feedback
	feedback := fh.GetFeedback()
	assert.NotNil(t, feedback)
	if feedback != nil {
		// Should have scores from both scorers
		assert.Len(t, feedback.Scores, 2)
		assert.Contains(t, feedback.Scores, "scorer-1")
		assert.Contains(t, feedback.Scores, "scorer-2")

		// Overall score should be average (0.9 + 0.7) / 2 = 0.8
		assert.InDelta(t, 0.8, feedback.Overall.Score, 0.01)
	}
}

// TestFeedbackHarnessRecordingAccess tests access to underlying recording harness.
func TestFeedbackHarnessRecordingAccess(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score: 0.8,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Perform operations
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)
	_ = fh.CallToolProto(ctx, "generic-tool", &testpb.GenericRequest{Targets: []string{"test"}}, &testpb.GenericResponse{})

	// Access recording harness
	recording := fh.RecordingHarness()
	assert.NotNil(t, recording)

	// Verify trajectory was recorded
	traj := recording.Trajectory()
	assert.Len(t, traj.Steps, 2)
	assert.Equal(t, "llm", traj.Steps[0].Type)
	assert.Equal(t, "tool", traj.Steps[1].Type)
}

// TestFeedbackHarnessNonStreamingScorer tests handling of non-streaming scorers.
func TestFeedbackHarnessNonStreamingScorer(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}

	// Scorer that doesn't support streaming
	scorer := &mockStreamingScorer{
		name: "non-streaming",
		score: PartialScore{
			Score: 0.8,
		},
		supportsStream: false,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Trigger evaluation
	messages := []llm.Message{{Role: "user", Content: "test"}}
	_, _ = fh.Complete(ctx, "primary", messages)

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Should have no feedback (scorer doesn't support streaming)
	feedback := fh.GetFeedback()
	assert.Nil(t, feedback)
}

// TestFeedbackHarnessToolCallRecording tests that tool calls trigger evaluation.
func TestFeedbackHarnessToolCallRecording(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	scorer := &mockStreamingScorer{
		name: "test-scorer",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.9,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Call tool
	_ = fh.CallToolProto(ctx, "generic-tool", &testpb.GenericRequest{Targets: []string{"https://example.com"}}, &testpb.GenericResponse{})

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Should have feedback
	feedback := fh.GetFeedback()
	assert.NotNil(t, feedback)

	// Verify trajectory includes tool call
	traj := fh.RecordingHarness().Trajectory()
	assert.Len(t, traj.Steps, 1)
	assert.Equal(t, "tool", traj.Steps[0].Type)
}
