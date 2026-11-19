// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"time"
)

// Evidence represents a piece of evidence supporting a security finding.
//
// It is the agent-side form of types.v1.Evidence. It has no tier and no tool
// call ID, so an agent cannot set either. The platform sets the tier of each
// item. An item that an agent types is tier 2. To get tier 1 evidence, name the
// tool calls with Finding.CiteToolCalls, and the platform builds the items from
// its own record.
type Evidence struct {
	// Type specifies the kind of evidence.
	Type EvidenceType `json:"type"`

	// Title is a brief description of the evidence.
	Title string `json:"title"`

	// Content contains the actual evidence data.
	Content string `json:"content"`

	// Timestamp indicates when the evidence was collected.
	Timestamp time.Time `json:"timestamp"`

	// Metadata contains additional context-specific information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EvidenceType represents the type of evidence collected.
type EvidenceType string

const (
	// EvidenceHTTPRequest represents an HTTP request capture.
	EvidenceHTTPRequest EvidenceType = "http_request"

	// EvidenceHTTPResponse represents an HTTP response capture.
	EvidenceHTTPResponse EvidenceType = "http_response"

	// EvidenceScreenshot represents a screenshot capture.
	EvidenceScreenshot EvidenceType = "screenshot"

	// EvidenceLog represents log output or traces.
	EvidenceLog EvidenceType = "log"

	// EvidencePayload represents an attack payload or test input.
	EvidencePayload EvidenceType = "payload"

	// EvidenceConversation represents a conversation transcript.
	EvidenceConversation EvidenceType = "conversation"

	// EvidenceCodeSnippet represents a piece of source code.
	EvidenceCodeSnippet EvidenceType = "code_snippet"

	// EvidenceNetworkTrace represents a network capture.
	EvidenceNetworkTrace EvidenceType = "network_trace"

	// EvidenceOther represents evidence that fits no other type.
	EvidenceOther EvidenceType = "other"
)

// String returns the string representation of the evidence type.
func (e EvidenceType) String() string {
	return string(e)
}
