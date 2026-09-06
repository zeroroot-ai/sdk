// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"fmt"
	"strings"
	"time"
)

// Feedback represents aggregated evaluation feedback at a specific point
// during agent execution. It combines scores from multiple streaming scorers
// and generates alerts when thresholds are breached.
type Feedback struct {
	// Timestamp is when this feedback was generated.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`

	// StepIndex is the trajectory step index when this feedback was generated.
	StepIndex int `json:"step_index" yaml:"step_index"`

	// Scores contains individual partial scores from each scorer, keyed by scorer name.
	Scores map[string]PartialScore `json:"scores" yaml:"scores"`

	// Overall is the aggregated partial score across all scorers.
	Overall PartialScore `json:"overall" yaml:"overall"`

	// Alerts contains any threshold breach alerts generated.
	Alerts []Alert `json:"alerts,omitempty" yaml:"alerts,omitempty"`

	// Consumed indicates whether this feedback has been read by the agent.
	// This is set to true when GetFeedback() is called.
	// Internal use only - not serialized to JSON.
	Consumed bool `json:"-" yaml:"-"`
}

// Alert represents a threshold breach notification.
type Alert struct {
	// Level indicates the severity of this alert.
	Level AlertLevel `json:"level" yaml:"level"`

	// Scorer is the name of the scorer that triggered this alert.
	// Empty string indicates an overall score alert.
	Scorer string `json:"scorer" yaml:"scorer"`

	// Score is the score that triggered the alert.
	Score float64 `json:"score" yaml:"score"`

	// Threshold is the threshold that was breached.
	Threshold float64 `json:"threshold" yaml:"threshold"`

	// Message is a human-readable description of the alert.
	Message string `json:"message" yaml:"message"`

	// Action is the recommended action to take in response to this alert.
	Action RecommendedAction `json:"action" yaml:"action"`
}

// AlertLevel indicates the severity of a threshold breach.
type AlertLevel string

const (
	// AlertWarning indicates performance is below expected but not critical.
	AlertWarning AlertLevel = "warning"

	// AlertCritical indicates performance is critically low.
	AlertCritical AlertLevel = "critical"
)

// FormatForLLM formats this feedback as a clear text message suitable for
// injection into an LLM system prompt or message history. This provides
// the agent with actionable guidance based on the evaluation.
func (f *Feedback) FormatForLLM() string {
	var b strings.Builder

	b.WriteString("=== EVALUATION FEEDBACK ===\n\n")

	// Overall score and action
	b.WriteString(fmt.Sprintf("Overall Score: %.2f (confidence: %.2f)\n",
		f.Overall.Score, f.Overall.Confidence))
	b.WriteString(fmt.Sprintf("Recommended Action: %s\n\n", f.Overall.Action))

	// Overall feedback message if present
	if f.Overall.Feedback != "" {
		b.WriteString(f.Overall.Feedback + "\n\n")
	}

	// Alerts (if any)
	if len(f.Alerts) > 0 {
		b.WriteString("ALERTS:\n")
		for _, alert := range f.Alerts {
			b.WriteString(fmt.Sprintf("  [%s] %s\n", strings.ToUpper(string(alert.Level)), alert.Message))
		}
		b.WriteString("\n")
	}

	// Individual scorer feedback
	if len(f.Scores) > 0 {
		b.WriteString("Individual Scores:\n")
		for name, score := range f.Scores {
			b.WriteString(fmt.Sprintf("  - %s: %.2f", name, score.Score))
			if score.Feedback != "" {
				b.WriteString(" - " + score.Feedback)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Action guidance
	switch f.Overall.Action {
	case ActionContinue:
		b.WriteString("You are performing well. Continue with your current approach.\n")
	case ActionAdjust:
		b.WriteString("Consider making minor adjustments to your approach based on the feedback above.\n")
	case ActionReconsider:
		b.WriteString("Your current approach may not be optimal. Review the feedback and consider a different strategy.\n")
	case ActionAbort:
		b.WriteString("Critical issues detected. Consider stopping or significantly changing your approach.\n")
	}

	b.WriteString("\n=== END FEEDBACK ===")

	return b.String()
}
