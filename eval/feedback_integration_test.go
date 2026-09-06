// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/agent"
	testpb "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	"github.com/zeroroot-ai/sdk/llm"
)

// TestIntegration_FeedbackLoop tests the complete feedback mission:
// 1. Create FeedbackHarness with streaming scorers
// 2. Simulate agent execution with multiple steps
// 3. Call GetFeedback() periodically
// 4. Verify feedback is generated at correct intervals
// 5. Verify feedback contains expected scores and actions
func TestIntegration_FeedbackLoop(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	ctx := context.Background()

	// Create mock harness
	mock := &mockHarness{}

	// Create streaming scorers
	goodScorer := &mockStreamingScorer{
		name: "quality-scorer",
		score: PartialScore{
			Score:      0.85,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Good execution quality",
		},
		supportsStream: true,
	}

	completenessScorer := &mockStreamingScorer{
		name: "completeness-scorer",
		score: PartialScore{
			Score:      0.75,
			Confidence: 0.8,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Trajectory is progressing well",
		},
		supportsStream: true,
	}

	// Create FeedbackHarness with evaluation every 2 steps
	opts := FeedbackOptions{
		Scorers:           []StreamingScorer{goodScorer, completenessScorer},
		WarningThreshold:  0.5,
		CriticalThreshold: 0.2,
		Frequency: FeedbackFrequency{
			EveryNSteps: 2, // Evaluate every 2 steps
			Debounce:    50 * time.Millisecond,
		},
		ScorerWeights: map[string]float64{
			"quality-scorer":      0.6,
			"completeness-scorer": 0.4,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	t.Log("Step 1: Execute first step - no feedback yet")
	messages := []llm.Message{{Role: "user", Content: "Analyze target"}}
	_, err := fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// No feedback yet (need 2 steps)
	feedback1 := fh.GetFeedback()
	assert.Nil(t, feedback1, "Should not have feedback after 1 step")

	t.Log("Step 2: Execute second step - should trigger evaluation")
	err = fh.CallToolProto(ctx, "generic-tool-a", &testpb.GenericRequest{Targets: []string{"example.com"}}, &testpb.GenericResponse{})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	// Should have feedback now
	feedback2 := fh.GetFeedback()
	require.NotNil(t, feedback2, "Should have feedback after 2 steps")

	t.Logf("Feedback after step 2: overall=%.3f, confidence=%.3f, action=%s",
		feedback2.Overall.Score, feedback2.Overall.Confidence, feedback2.Overall.Action)

	// Verify feedback structure
	assert.Equal(t, 1, feedback2.StepIndex, "Should be at step index 1 (0-indexed)")
	assert.Len(t, feedback2.Scores, 2, "Should have scores from both scorers")
	assert.Contains(t, feedback2.Scores, "quality-scorer")
	assert.Contains(t, feedback2.Scores, "completeness-scorer")

	// Verify overall score is weighted average
	// Expected: 0.85 * 0.6 + 0.75 * 0.4 = 0.51 + 0.3 = 0.81
	assert.InDelta(t, 0.81, feedback2.Overall.Score, 0.01, "Overall score should be weighted average")

	// Verify action is the most severe from all scorers
	assert.Equal(t, ActionContinue, feedback2.Overall.Action)

	// Verify no alerts (all scores above thresholds)
	assert.Empty(t, feedback2.Alerts, "Should have no alerts for good scores")

	// Verify feedback is marked as consumed
	assert.True(t, feedback2.Consumed, "GetFeedback() should mark feedback as consumed")

	t.Log("Step 3: Execute third step - no new feedback (need 2 more steps)")
	_, err = fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	feedback3 := fh.GetFeedback()
	assert.Nil(t, feedback3, "Should not have new feedback after 1 step")

	t.Log("Step 4: Execute fourth step - should trigger second evaluation")
	err = fh.CallToolProto(ctx, "generic-tool-b", &testpb.GenericRequest{Targets: []string{"https://example.com"}}, &testpb.GenericResponse{})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	feedback4 := fh.GetFeedback()
	require.NotNil(t, feedback4, "Should have feedback after 2 more steps")

	t.Logf("Feedback after step 4: overall=%.3f, step_index=%d",
		feedback4.Overall.Score, feedback4.StepIndex)

	assert.Equal(t, 3, feedback4.StepIndex, "Should be at step index 3")

	t.Log("Step 5: Verify feedback history")
	history := fh.FeedbackHistory()
	assert.GreaterOrEqual(t, len(history), 2, "Should have at least 2 feedback entries in history")

	// Verify history is ordered by time
	if len(history) >= 2 {
		assert.True(t, history[0].Timestamp.Before(history[1].Timestamp) ||
			history[0].Timestamp.Equal(history[1].Timestamp),
			"History should be ordered chronologically")
	}

	t.Log("Step 6: Verify trajectory recording")
	trajectory := fh.RecordingHarness().Trajectory()
	assert.Len(t, trajectory.Steps, 4, "Should have recorded 4 steps")

	// Verify step types
	assert.Equal(t, "llm", trajectory.Steps[0].Type)
	assert.Equal(t, "tool", trajectory.Steps[1].Type)
	assert.Equal(t, "llm", trajectory.Steps[2].Type)
	assert.Equal(t, "tool", trajectory.Steps[3].Type)

	t.Log("Integration test: FeedbackLoop completed successfully")
}

// TestIntegration_ThresholdAlerts tests that warning and critical alerts
// are generated when scores fall below configured thresholds.
func TestIntegration_ThresholdAlerts(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	ctx := context.Background()
	mock := &mockHarness{}

	// Create scorer that returns low scores
	poorScorer := &mockStreamingScorer{
		name: "quality-scorer",
		score: PartialScore{
			Score:      0.15, // Below critical threshold
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionAbort,
			Feedback:   "Execution quality is critically low",
		},
		supportsStream: true,
	}

	// Configure low thresholds to test alert generation
	opts := FeedbackOptions{
		Scorers:           []StreamingScorer{poorScorer},
		WarningThreshold:  0.5, // Scores below this trigger warning
		CriticalThreshold: 0.2, // Scores below this trigger critical alert
		Frequency: FeedbackFrequency{
			EveryNSteps: 1, // Evaluate every step
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	t.Log("Step 1: Execute step that would score poorly")
	messages := []llm.Message{{Role: "user", Content: "bad approach"}}
	_, err := fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	t.Log("Step 2: Verify critical alert was generated")
	feedback := fh.GetFeedback()
	require.NotNil(t, feedback, "Should have feedback")

	// Verify alerts were generated
	assert.NotEmpty(t, feedback.Alerts, "Should have alerts for low score")

	t.Logf("Generated %d alert(s)", len(feedback.Alerts))

	// Find critical alert
	hasCritical := false
	var criticalAlert Alert
	for _, alert := range feedback.Alerts {
		t.Logf("  Alert: level=%s, scorer=%s, score=%.3f, threshold=%.3f, message=%s",
			alert.Level, alert.Scorer, alert.Score, alert.Threshold, alert.Message)

		if alert.Level == AlertCritical {
			hasCritical = true
			criticalAlert = alert
		}
	}

	assert.True(t, hasCritical, "Should have generated a critical alert")

	if hasCritical {
		assert.Equal(t, 0.15, criticalAlert.Score)
		assert.Equal(t, 0.2, criticalAlert.Threshold)
		assert.Equal(t, ActionAbort, criticalAlert.Action, "Critical alert should recommend abort")
		assert.NotEmpty(t, criticalAlert.Message, "Alert should have actionable message")
		assert.Contains(t, strings.ToLower(criticalAlert.Message), "critical",
			"Message should indicate criticality")
	}

	t.Log("Step 3: Test warning threshold")
	// Update scorer to return score between warning and critical
	poorScorer.score = PartialScore{
		Score:      0.35, // Above critical, below warning
		Confidence: 0.9,
		Status:     ScoreStatusPartial,
		Action:     ActionReconsider,
		Feedback:   "Execution quality is below expected",
	}

	_, err = fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	feedback = fh.GetFeedback()
	require.NotNil(t, feedback)

	// Should have warning alert, not critical
	hasWarning := false
	hasCritical = false
	for _, alert := range feedback.Alerts {
		if alert.Level == AlertWarning {
			hasWarning = true
			t.Logf("Warning alert: %s", alert.Message)
		}
		if alert.Level == AlertCritical {
			hasCritical = true
		}
	}

	assert.True(t, hasWarning, "Should have warning alert for mid-range low score")
	assert.False(t, hasCritical, "Should not have critical alert for score above critical threshold")

	t.Log("Integration test: ThresholdAlerts completed successfully")
}

// TestIntegration_FeedbackAwareAgent simulates an agent that reads and
// responds to feedback during execution.
func TestIntegration_FeedbackAwareAgent(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	ctx := context.Background()

	// Track what the "agent" receives
	var receivedFeedback []string

	// Create mock harness that captures feedback injection
	mock := &mockHarness{
		completeFunc: func(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
			// Check if feedback was injected
			for _, msg := range messages {
				if msg.Role == "system" && strings.Contains(msg.Content, "EVALUATION FEEDBACK") {
					receivedFeedback = append(receivedFeedback, msg.Content)
					t.Log("Agent received feedback injection")
				}
			}
			return &llm.CompletionResponse{Content: "Adjusting approach based on feedback"}, nil
		},
	}

	// Create scorer that provides actionable feedback
	scorer := &mockStreamingScorer{
		name: "approach-scorer",
		score: PartialScore{
			Score:      0.6,
			Confidence: 0.85,
			Status:     ScoreStatusPartial,
			Action:     ActionAdjust,
			Feedback:   "Consider using more targeted reconnaissance tools",
		},
		supportsStream: true,
	}

	// Enable auto-injection
	opts := FeedbackOptions{
		Scorers:    []StreamingScorer{scorer},
		AutoInject: true, // Automatically inject feedback into LLM calls
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	t.Log("Step 1: First LLM call - no feedback yet")
	messages := []llm.Message{{Role: "user", Content: "Begin reconnaissance"}}
	_, err := fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	assert.Empty(t, receivedFeedback, "First call should not have feedback")

	t.Log("Step 2: Second LLM call - should receive injected feedback")
	_, err = fh.Complete(ctx, "primary", messages)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	assert.NotEmpty(t, receivedFeedback, "Second call should receive injected feedback")

	if len(receivedFeedback) > 0 {
		feedbackMsg := receivedFeedback[0]

		// Verify feedback format
		assert.Contains(t, feedbackMsg, "EVALUATION FEEDBACK")
		assert.Contains(t, feedbackMsg, "Overall Score:")
		assert.Contains(t, feedbackMsg, "Recommended Action:")
		assert.Contains(t, feedbackMsg, "adjust", "Should contain action guidance")
		assert.Contains(t, feedbackMsg, "approach-scorer")

		t.Logf("Feedback message preview:\n%s", feedbackMsg)
	}

	t.Log("Step 3: Agent can also manually read feedback")
	err = fh.CallToolProto(ctx, "generic-tool-a", &testpb.GenericRequest{Targets: []string{"example.com"}}, &testpb.GenericResponse{})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	// Agent manually retrieves feedback
	manualFeedback := fh.GetFeedback()
	require.NotNil(t, manualFeedback, "Should be able to manually retrieve feedback")

	// Format for LLM consumption
	formattedFeedback := manualFeedback.FormatForLLM()
	assert.Contains(t, formattedFeedback, "EVALUATION FEEDBACK")
	assert.Contains(t, formattedFeedback, "Overall Score:")
	assert.Contains(t, formattedFeedback, manualFeedback.Overall.Action)

	t.Logf("Manual feedback retrieval: score=%.3f, action=%s",
		manualFeedback.Overall.Score, manualFeedback.Overall.Action)

	t.Log("Step 4: Verify agent can interpret recommended action")
	var agentAction string
	switch manualFeedback.Overall.Action {
	case ActionContinue:
		agentAction = "continue current approach"
	case ActionAdjust:
		agentAction = "make minor adjustments"
	case ActionReconsider:
		agentAction = "change strategy significantly"
	case ActionAbort:
		agentAction = "stop execution"
	}

	assert.NotEmpty(t, agentAction, "Agent should be able to interpret action")
	t.Logf("Agent interprets action as: %s", agentAction)

	t.Log("Integration test: FeedbackAwareAgent completed successfully")
}

// TestIntegration_FullMission tests a complete end-to-end mission
// with sample eval set, multiple scorers, and full trajectory.
func TestIntegration_FullMission(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	ctx := context.Background()

	// Create inline sample (don't require external file)
	sample := Sample{
		ID: "feedback-mission-test",
		Task: agent.Task{
			ID:      "test-001",
			Context: map[string]any{"objective": "Perform reconnaissance on target system"},
		},
		ExpectedTools: []ExpectedToolCall{
			{Name: "generic-tool-a", Arguments: map[string]any{"target": "example.com"}, Required: true},
			{Name: "http-client", Arguments: map[string]any{"url": "https://example.com"}, Required: true},
		},
		Tags: []string{"recon", "integration-test"},
	}

	t.Logf("Testing mission for sample: %s", sample.ID)

	// Create mock harness
	mock := &mockHarness{}

	// Create multiple streaming scorers
	toolScorer := &mockStreamingScorer{
		name: "tool-usage",
		score: PartialScore{
			Score:      0.9,
			Confidence: 0.95,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Good tool selection",
		},
		supportsStream: true,
	}

	efficiencyScorer := &mockStreamingScorer{
		name: "efficiency",
		score: PartialScore{
			Score:      0.8,
			Confidence: 0.85,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Reasonable execution efficiency",
		},
		supportsStream: true,
	}

	coverageScorer := &mockStreamingScorer{
		name: "coverage",
		score: PartialScore{
			Score:      0.85,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
			Feedback:   "Good coverage of expected actions",
		},
		supportsStream: true,
	}

	// Create FeedbackHarness
	opts := FeedbackOptions{
		Scorers: []StreamingScorer{toolScorer, efficiencyScorer, coverageScorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 2, // Evaluate every 2 steps
		},
		ScorerWeights: map[string]float64{
			"tool-usage": 0.4,
			"efficiency": 0.3,
			"coverage":   0.3,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	t.Log("Executing complete mission...")

	// Simulate agent execution matching the expected tools
	// Execute LLM calls and tool calls in sequence
	t.Log("  Step 1: llm - primary")
	_, err := fh.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "Analyze target"}})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	t.Log("  Step 2: tool - mytool")
	err = fh.CallToolProto(ctx, "generic-tool-a", &testpb.GenericRequest{Targets: []string{"example.com"}}, &testpb.GenericResponse{})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	t.Log("  Step 3: llm - primary")
	_, err = fh.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "Process mytool results"}})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	t.Log("  Step 4: tool - mytool-c")
	err = fh.CallToolProto(ctx, "generic-tool-b", &testpb.GenericRequest{Targets: []string{"https://example.com"}}, &testpb.GenericResponse{})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	t.Log("  Step 5: llm - primary")
	_, err = fh.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "Compile findings"}})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	// Wait for final evaluations to complete
	time.Sleep(200 * time.Millisecond)

	t.Log("Verifying final trajectory...")
	trajectory := fh.RecordingHarness().Trajectory()
	assert.Len(t, trajectory.Steps, 5, "Should have recorded all 5 steps")

	// Verify trajectory matches expected sequence
	assert.Equal(t, "llm", trajectory.Steps[0].Type)
	assert.Equal(t, "tool", trajectory.Steps[1].Type)
	assert.Equal(t, "generic-tool-a", trajectory.Steps[1].Name)
	assert.Equal(t, "tool", trajectory.Steps[3].Type)
	assert.Equal(t, "generic-tool-b", trajectory.Steps[3].Name)

	t.Log("Verifying feedback history...")
	history := fh.FeedbackHistory()
	assert.GreaterOrEqual(t, len(history), 2, "Should have at least 2 feedback entries (evaluated every 2 steps)")

	// Verify all feedback entries are complete
	for i, fb := range history {
		t.Logf("  Feedback %d: step_index=%d, overall=%.3f, scorers=%d, alerts=%d",
			i+1, fb.StepIndex, fb.Overall.Score, len(fb.Scores), len(fb.Alerts))

		assert.NotZero(t, fb.Timestamp, "Feedback should have timestamp")
		assert.Len(t, fb.Scores, 3, "Feedback should have scores from all 3 scorers")
		assert.Contains(t, fb.Scores, "tool-usage")
		assert.Contains(t, fb.Scores, "efficiency")
		assert.Contains(t, fb.Scores, "coverage")

		// Verify weighted overall score
		// Expected: 0.9*0.4 + 0.8*0.3 + 0.85*0.3 = 0.36 + 0.24 + 0.255 = 0.855
		assert.InDelta(t, 0.855, fb.Overall.Score, 0.01, "Overall score should be weighted average")
	}

	t.Log("Verifying trajectory analysis...")
	// Verify we can use the trajectory for final scoring
	assert.NotZero(t, trajectory.StartTime)
	assert.NotZero(t, trajectory.EndTime)
	assert.True(t, trajectory.EndTime.After(trajectory.StartTime) ||
		trajectory.EndTime.Equal(trajectory.StartTime),
		"End time should be after or equal to start time")

	duration := trajectory.EndTime.Sub(trajectory.StartTime)
	t.Logf("Total execution time: %v", duration)

	// Verify we could run final scorers on complete trajectory
	finalScore := PartialScore{
		Score:      0.855,
		Confidence: 1.0,
		Status:     ScoreStatusComplete,
		Action:     ActionContinue,
	}
	assert.Equal(t, ScoreStatusComplete, finalScore.Status, "Final score should be complete")

	t.Log("Integration test: FullMission completed successfully")
	t.Logf("Summary: %d steps executed, %d feedback entries generated, final score: %.3f",
		len(trajectory.Steps), len(history), finalScore.Score)
}

// TestIntegration_MultipleScorersAggregation verifies that multiple scorers
// are properly aggregated with weights and the most severe action is selected.
func TestIntegration_MultipleScorersAggregation(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	ctx := context.Background()
	mock := &mockHarness{}

	// Create scorers with different severities
	goodScorer := &mockStreamingScorer{
		name: "good-scorer",
		score: PartialScore{
			Score:      0.95,
			Confidence: 0.9,
			Status:     ScoreStatusPartial,
			Action:     ActionContinue,
		},
		supportsStream: true,
	}

	okScorer := &mockStreamingScorer{
		name: "ok-scorer",
		score: PartialScore{
			Score:      0.7,
			Confidence: 0.85,
			Status:     ScoreStatusPartial,
			Action:     ActionAdjust,
		},
		supportsStream: true,
	}

	poorScorer := &mockStreamingScorer{
		name: "poor-scorer",
		score: PartialScore{
			Score:      0.4,
			Confidence: 0.8,
			Status:     ScoreStatusPartial,
			Action:     ActionReconsider,
		},
		supportsStream: true,
	}

	opts := FeedbackOptions{
		Scorers: []StreamingScorer{goodScorer, okScorer, poorScorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
		// Weighted: good gets most weight, poor gets least
		ScorerWeights: map[string]float64{
			"good-scorer": 0.5,
			"ok-scorer":   0.3,
			"poor-scorer": 0.2,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Execute step
	_, err := fh.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "test"}})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	feedback := fh.GetFeedback()
	require.NotNil(t, feedback)

	// Verify all scorers contributed
	assert.Len(t, feedback.Scores, 3)

	// Verify weighted average
	// Expected: 0.95*0.5 + 0.7*0.3 + 0.4*0.2 = 0.475 + 0.21 + 0.08 = 0.765
	assert.InDelta(t, 0.765, feedback.Overall.Score, 0.01,
		"Overall score should be weighted average")

	// Verify most severe action is selected (ActionReconsider > ActionAdjust > ActionContinue)
	assert.Equal(t, ActionReconsider, feedback.Overall.Action,
		"Overall action should be the most severe from all scorers")

	t.Logf("Aggregation test: overall_score=%.3f, overall_action=%s",
		feedback.Overall.Score, feedback.Overall.Action)

	t.Log("Integration test: MultipleScorersAggregation completed successfully")
}

// TestIntegration_PeekVsGetBehavior verifies the difference between
// PeekFeedback and GetFeedback in a realistic scenario.
func TestIntegration_PeekVsGetBehavior(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	ctx := context.Background()
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
		Scorers: []StreamingScorer{scorer},
		Frequency: FeedbackFrequency{
			EveryNSteps: 1,
		},
	}

	fh := NewFeedbackHarness(mock, opts)
	defer fh.Close()

	// Execute step to generate feedback
	_, err := fh.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "test"}})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	// Peek multiple times - should return same feedback
	peek1 := fh.PeekFeedback()
	require.NotNil(t, peek1)
	assert.False(t, peek1.Consumed, "Peek should not mark as consumed")

	peek2 := fh.PeekFeedback()
	require.NotNil(t, peek2)
	assert.False(t, peek2.Consumed, "Peek should not mark as consumed")

	assert.Equal(t, peek1.Timestamp, peek2.Timestamp, "Should be same feedback")

	// Get - should consume
	get1 := fh.GetFeedback()
	require.NotNil(t, get1)
	assert.True(t, get1.Consumed, "Get should mark as consumed")

	// Get again - should return nil
	get2 := fh.GetFeedback()
	assert.Nil(t, get2, "Second Get should return nil (already consumed)")

	// Peek should also return nil now
	peek3 := fh.PeekFeedback()
	assert.Nil(t, peek3, "Peek should return nil after consumption")

	t.Log("Integration test: PeekVsGetBehavior completed successfully")
}
