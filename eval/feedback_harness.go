// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// FeedbackOptions configures real-time evaluation feedback behavior.
type FeedbackOptions struct {
	// Scorers are the streaming scorers to use for real-time evaluation.
	Scorers []StreamingScorer

	// WarningThreshold is the score below which a warning alert is triggered (0.0 to 1.0).
	// Default: 0.5
	WarningThreshold float64

	// CriticalThreshold is the score below which a critical alert is triggered (0.0 to 1.0).
	// Default: 0.2
	CriticalThreshold float64

	// Frequency controls when feedback evaluations are triggered.
	Frequency FeedbackFrequency

	// AutoInject controls whether feedback is automatically injected into LLM calls.
	// If true, the harness will prepend feedback messages to LLM message history.
	// Default: false (agent must explicitly call GetFeedback)
	AutoInject bool

	// ScorerWeights defines relative weights for each scorer when aggregating.
	// If nil or empty, all scorers are weighted equally.
	// Keys are scorer names, values are weights (will be normalized to sum to 1.0).
	ScorerWeights map[string]float64
}

// FeedbackFrequency controls when feedback evaluations are triggered.
type FeedbackFrequency struct {
	// EveryNSteps triggers evaluation every N trajectory steps.
	// If 0 or 1, evaluates after every step.
	// If > 1, evaluates after every Nth step.
	EveryNSteps int

	// Debounce is the minimum time between evaluations.
	// This prevents rapid-fire evaluations during fast operations.
	// If zero, no debouncing is applied.
	Debounce time.Duration

	// OnThreshold controls whether to always evaluate on threshold breaches.
	// If true, evaluation is triggered immediately when a score crosses a threshold,
	// regardless of EveryNSteps or Debounce settings.
	// This requires at least one previous evaluation to detect threshold crossings.
	OnThreshold bool
}

// FeedbackHarness wraps an agent.Harness with real-time evaluation capabilities.
// It records all operations using a RecordingHarness and triggers streaming
// scorers based on frequency settings to provide feedback during execution.
type FeedbackHarness struct {
	recording *RecordingHarness
	opts      FeedbackOptions

	// Feedback state
	mu                sync.RWMutex
	pendingFeedback   *Feedback
	feedbackHistory   []Feedback
	lastEvalTime      time.Time
	lastEvalStepIndex int
	stepsSinceEval    int

	// Background evaluation
	evalCtx    context.Context
	evalCancel context.CancelFunc
	// evalWg is kept as sync.WaitGroup (not errgroup) because it tracks a single
	// persistent background worker goroutine (evalWorker) that drains evalQueue
	// until Close() cancels evalCtx. This is a lifecycle coordination primitive,
	// not a fan-out: the goroutine runs indefinitely, making errgroup semantics
	// (first-error cancels siblings) inappropriate here.
	evalWg    sync.WaitGroup
	evalQueue chan evalRequest
}

// evalRequest represents a request to evaluate the current trajectory.
type evalRequest struct {
	ctx       context.Context
	stepIndex int
}

// NewFeedbackHarness creates a new feedback harness that wraps the given inner harness.
// The inner harness is wrapped with a RecordingHarness for trajectory capture.
// Feedback evaluation runs asynchronously and does not block harness operations.
func NewFeedbackHarness(inner agent.Harness, opts FeedbackOptions) *FeedbackHarness {
	// Apply defaults
	if opts.WarningThreshold == 0 {
		opts.WarningThreshold = 0.5
	}
	if opts.CriticalThreshold == 0 {
		opts.CriticalThreshold = 0.2
	}
	if opts.Frequency.EveryNSteps <= 0 {
		opts.Frequency.EveryNSteps = 1
	}

	// Create evaluation context
	evalCtx, evalCancel := context.WithCancel(context.Background())

	fh := &FeedbackHarness{
		recording:    NewRecordingHarness(inner),
		opts:         opts,
		evalCtx:      evalCtx,
		evalCancel:   evalCancel,
		evalQueue:    make(chan evalRequest, 10), // Buffer up to 10 eval requests
		lastEvalTime: time.Now(),
	}

	// Start background evaluation worker
	fh.evalWg.Add(1)
	go fh.evalWorker()

	return fh
}

// evalWorker processes evaluation requests asynchronously.
func (f *FeedbackHarness) evalWorker() {
	defer f.evalWg.Done()

	for {
		select {
		case <-f.evalCtx.Done():
			return
		case req := <-f.evalQueue:
			f.processEvaluation(req)
		}
	}
}

// processEvaluation performs the actual evaluation and stores feedback.
func (f *FeedbackHarness) processEvaluation(req evalRequest) {
	// Get current trajectory snapshot
	trajectory := f.recording.Trajectory()

	// Skip if trajectory is empty
	if len(trajectory.Steps) == 0 {
		return
	}

	// Evaluate with all scorers
	scores := make(map[string]PartialScore)
	var allScores []PartialScore

	for _, scorer := range f.opts.Scorers {
		if !scorer.SupportsStreaming() {
			continue
		}

		score, err := scorer.ScorePartial(req.ctx, trajectory)
		if err != nil {
			// Log error but continue with other scorers
			continue
		}

		scores[scorer.Name()] = score
		allScores = append(allScores, score)
	}

	// Skip if no scores were generated
	if len(scores) == 0 {
		return
	}

	// Aggregate scores
	overall := f.aggregateScores(allScores)

	// Generate alerts based on thresholds
	alerts := f.generateAlerts(overall, scores)

	// Create feedback
	feedback := Feedback{
		Timestamp: time.Now(),
		StepIndex: req.stepIndex,
		Scores:    scores,
		Overall:   overall,
		Alerts:    alerts,
		Consumed:  false,
	}

	// Store feedback
	f.mu.Lock()
	f.pendingFeedback = &feedback
	f.feedbackHistory = append(f.feedbackHistory, feedback)
	f.lastEvalStepIndex = req.stepIndex
	f.mu.Unlock()
}

// aggregateScores combines multiple partial scores into an overall score.
func (f *FeedbackHarness) aggregateScores(scores []PartialScore) PartialScore {
	if len(scores) == 0 {
		return PartialScore{
			Score:      0.0,
			Confidence: 0.0,
			Status:     ScoreStatusPending,
			Action:     ActionContinue,
		}
	}

	// Build score results for aggregation
	scoreResults := make(map[string]ScoreResult)
	var totalConfidence float64
	var lowestAction = ActionContinue

	for _, scorer := range f.opts.Scorers {
		if !scorer.SupportsStreaming() {
			continue
		}
		// Find matching score
		for scorerName, score := range scoreResults {
			if scorerName == scorer.Name() {
				totalConfidence += score.Score
				// Track the most severe action recommendation
				if isMoreSevere(score, lowestAction) {
					lowestAction = extractAction(score)
				}
			}
		}
	}

	// Convert PartialScores to map for aggregation
	scoreMap := make(map[string]ScoreResult)
	for i, scorer := range f.opts.Scorers {
		if i < len(scores) {
			scoreMap[scorer.Name()] = ScoreResult{
				Score:   scores[i].Score,
				Details: scores[i].Details,
			}
			totalConfidence += scores[i].Confidence
		}
	}

	// Aggregate using weights
	aggregatedScore := AggregateScoresWithNames(scoreMap, f.opts.ScorerWeights)
	avgConfidence := totalConfidence / float64(len(scores))

	// Determine overall action (use the most severe)
	overallAction := ActionContinue
	for _, score := range scores {
		if isActionMoreSevere(score.Action, overallAction) {
			overallAction = score.Action
		}
	}

	// Determine status
	status := ScoreStatusPartial
	allComplete := true
	for _, score := range scores {
		if score.Status != ScoreStatusComplete {
			allComplete = false
			break
		}
	}
	if allComplete {
		status = ScoreStatusComplete
	}

	return PartialScore{
		Score:      aggregatedScore,
		Confidence: avgConfidence,
		Status:     status,
		Action:     overallAction,
		Feedback:   f.buildOverallFeedback(scores),
	}
}

// isActionMoreSevere returns true if action a is more severe than action b.
func isActionMoreSevere(a, b RecommendedAction) bool {
	severity := map[RecommendedAction]int{
		ActionContinue:   0,
		ActionAdjust:     1,
		ActionReconsider: 2,
		ActionAbort:      3,
	}
	return severity[a] > severity[b]
}

// isMoreSevere helper function for comparing actions.
func isMoreSevere(score ScoreResult, current RecommendedAction) bool {
	// Extract action from score details if available
	if action, ok := score.Details["action"].(RecommendedAction); ok {
		return isActionMoreSevere(action, current)
	}
	return false
}

// extractAction extracts action from score result.
func extractAction(score ScoreResult) RecommendedAction {
	if action, ok := score.Details["action"].(RecommendedAction); ok {
		return action
	}
	return ActionContinue
}

// buildOverallFeedback constructs a combined feedback message from individual scores.
func (f *FeedbackHarness) buildOverallFeedback(scores []PartialScore) string {
	if len(scores) == 0 {
		return ""
	}

	// Collect non-empty feedback messages
	var messages []string
	for _, score := range scores {
		if score.Feedback != "" {
			messages = append(messages, score.Feedback)
		}
	}

	if len(messages) == 0 {
		return ""
	}

	// Join with newlines
	var result string
	for i, msg := range messages {
		if i > 0 {
			result += "\n"
		}
		result += msg
	}

	return result
}

// generateAlerts creates alerts for threshold breaches.
func (f *FeedbackHarness) generateAlerts(overall PartialScore, scores map[string]PartialScore) []Alert {
	var alerts []Alert

	// Check overall score against thresholds
	if overall.Score < f.opts.CriticalThreshold {
		alerts = append(alerts, Alert{
			Level:     AlertCritical,
			Scorer:    "",
			Score:     overall.Score,
			Threshold: f.opts.CriticalThreshold,
			Message:   "Overall performance is critically low",
			Action:    ActionAbort,
		})
	} else if overall.Score < f.opts.WarningThreshold {
		alerts = append(alerts, Alert{
			Level:     AlertWarning,
			Scorer:    "",
			Score:     overall.Score,
			Threshold: f.opts.WarningThreshold,
			Message:   "Overall performance is below expected threshold",
			Action:    ActionReconsider,
		})
	}

	// Check individual scorer thresholds
	for name, score := range scores {
		if score.Score < f.opts.CriticalThreshold {
			alerts = append(alerts, Alert{
				Level:     AlertCritical,
				Scorer:    name,
				Score:     score.Score,
				Threshold: f.opts.CriticalThreshold,
				Message:   name + " performance is critically low",
				Action:    ActionReconsider,
			})
		} else if score.Score < f.opts.WarningThreshold {
			alerts = append(alerts, Alert{
				Level:     AlertWarning,
				Scorer:    name,
				Score:     score.Score,
				Threshold: f.opts.WarningThreshold,
				Message:   name + " performance is below expected threshold",
				Action:    ActionAdjust,
			})
		}
	}

	return alerts
}

// shouldEvaluate determines whether to trigger an evaluation based on frequency settings.
func (f *FeedbackHarness) shouldEvaluate() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check step-based frequency
	f.stepsSinceEval++
	if f.stepsSinceEval < f.opts.Frequency.EveryNSteps {
		// Not enough steps yet
		if !f.opts.Frequency.OnThreshold {
			return false
		}
	}

	// Check debounce
	if f.opts.Frequency.Debounce > 0 {
		if time.Since(f.lastEvalTime) < f.opts.Frequency.Debounce {
			return false
		}
	}

	return true
}

// triggerEvaluation queues an evaluation request if frequency conditions are met.
func (f *FeedbackHarness) triggerEvaluation(ctx context.Context) {
	if !f.shouldEvaluate() {
		return
	}

	// Reset counter
	f.mu.Lock()
	f.stepsSinceEval = 0
	stepIndex := len(f.recording.Trajectory().Steps) - 1
	f.mu.Unlock()

	// Queue evaluation (non-blocking)
	select {
	case f.evalQueue <- evalRequest{ctx: ctx, stepIndex: stepIndex}:
		f.mu.Lock()
		f.lastEvalTime = time.Now()
		f.mu.Unlock()
	default:
		// Queue is full, skip this evaluation
	}
}

// GetFeedback returns the pending feedback and marks it as consumed.
// Returns nil if no feedback is available.
func (f *FeedbackHarness) GetFeedback() *Feedback {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.pendingFeedback == nil {
		return nil
	}

	feedback := f.pendingFeedback
	feedback.Consumed = true
	f.pendingFeedback = nil

	return feedback
}

// PeekFeedback returns the pending feedback without consuming it.
// Returns nil if no feedback is available.
func (f *FeedbackHarness) PeekFeedback() *Feedback {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.pendingFeedback
}

// FeedbackHistory returns all feedback generated during execution.
// Returns a copy to prevent external modification.
func (f *FeedbackHarness) FeedbackHistory() []Feedback {
	f.mu.RLock()
	defer f.mu.RUnlock()

	history := make([]Feedback, len(f.feedbackHistory))
	copy(history, f.feedbackHistory)
	return history
}

// RecordingHarness returns the underlying recording harness.
// This allows access to the full trajectory for post-execution analysis.
func (f *FeedbackHarness) RecordingHarness() *RecordingHarness {
	return f.recording
}

// Close stops the background evaluation worker and waits for pending evaluations.
// This should be called when the harness is no longer needed.
func (f *FeedbackHarness) Close() {
	f.evalCancel()
	close(f.evalQueue)
	f.evalWg.Wait()
}

// recordAndEvaluate is a helper that records a step and triggers evaluation.
func (f *FeedbackHarness) recordAndEvaluate(ctx context.Context) {
	f.triggerEvaluation(ctx)
}

// --- Harness Interface Implementation ---
// All methods delegate to the recording harness and trigger evaluation after recording.

// Complete performs a single LLM completion request.
func (f *FeedbackHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	// Auto-inject feedback if enabled
	if f.opts.AutoInject {
		if feedback := f.PeekFeedback(); feedback != nil {
			// Prepend feedback as a system message
			feedbackMsg := llm.Message{
				Role:    "system",
				Content: feedback.FormatForLLM(),
			}
			messages = append([]llm.Message{feedbackMsg}, messages...)
		}
	}

	resp, err := f.recording.Complete(ctx, slot, messages, opts...)
	f.recordAndEvaluate(ctx)
	return resp, err
}

// CompleteWithTools performs a completion with tool calling enabled.
func (f *FeedbackHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	// Auto-inject feedback if enabled
	if f.opts.AutoInject {
		if feedback := f.PeekFeedback(); feedback != nil {
			feedbackMsg := llm.Message{
				Role:    "system",
				Content: feedback.FormatForLLM(),
			}
			messages = append([]llm.Message{feedbackMsg}, messages...)
		}
	}

	resp, err := f.recording.CompleteWithTools(ctx, slot, messages, tools)
	f.recordAndEvaluate(ctx)
	return resp, err
}

// Stream performs a streaming completion request.
func (f *FeedbackHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	ch, err := f.recording.Stream(ctx, slot, messages)
	f.recordAndEvaluate(ctx)
	return ch, err
}

// CallToolProto invokes a tool with proto messages.
func (f *FeedbackHarness) CallToolProto(ctx context.Context, name string, request proto.Message, response proto.Message) error {
	err := f.recording.CallToolProto(ctx, name, request, response)
	f.recordAndEvaluate(ctx)
	return err
}

// ListTools returns descriptors for all available tools.
func (f *FeedbackHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	return f.recording.ListTools(ctx)
}

// QueryPlugin sends a query to a plugin.
func (f *FeedbackHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	result, err := f.recording.QueryPlugin(ctx, name, method, params)
	f.recordAndEvaluate(ctx)
	return result, err
}

// ListPlugins returns descriptors for all available plugins.
func (f *FeedbackHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	return f.recording.ListPlugins(ctx)
}

// DelegateToAgent assigns a task to another agent.
func (f *FeedbackHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	result, err := f.recording.DelegateToAgent(ctx, name, task)
	f.recordAndEvaluate(ctx)
	return result, err
}

// ListAgents returns descriptors for all available agents.
func (f *FeedbackHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	return f.recording.ListAgents(ctx)
}

// SubmitFinding records a new security finding.
func (f *FeedbackHarness) SubmitFinding(ctx context.Context, finding *finding.Finding) error {
	err := f.recording.SubmitFinding(ctx, finding)
	f.recordAndEvaluate(ctx)
	return err
}

// Mission returns the current mission context.
func (f *FeedbackHarness) Mission() types.MissionContext {
	return f.recording.Mission()
}

// Target returns information about the target being tested.
func (f *FeedbackHarness) Target() types.TargetInfo {
	return f.recording.Target()
}

// Tracer returns an OpenTelemetry tracer for distributed tracing.
func (f *FeedbackHarness) Tracer() trace.Tracer {
	return f.recording.Tracer()
}

// Logger returns a structured logger for the agent.
func (f *FeedbackHarness) Logger() *slog.Logger {
	return f.recording.Logger()
}

// TokenUsage returns the token usage tracker for this execution.
func (f *FeedbackHarness) TokenUsage() llm.TokenTracker {
	return f.recording.TokenUsage()
}

// WorldView reads the agent's slice of the World. It does not trigger an
// evaluation pass: reading changes nothing, so there is no new trajectory to score.
func (f *FeedbackHarness) WorldView(ctx context.Context, focus ...agent.Handle) (agent.WorldView, error) {
	return f.recording.WorldView(ctx, focus...)
}

// Observe emits a typed observation, recording and evaluating the trajectory.
func (f *FeedbackHarness) Observe(ctx context.Context, obs agent.Observation) error {
	err := f.recording.Observe(ctx, obs)
	f.recordAndEvaluate(ctx)
	return err
}
