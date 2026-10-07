// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"time"
)

// Evidence represents a piece of evidence supporting a security finding.
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
)

// String returns the string representation of the evidence type.
func (e EvidenceType) String() string {
	return string(e)
}
