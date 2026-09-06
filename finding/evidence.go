// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"errors"
	"fmt"
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

// IsValid returns true if the evidence type is valid.
func (e EvidenceType) IsValid() bool {
	switch e {
	case EvidenceHTTPRequest,
		EvidenceHTTPResponse,
		EvidenceScreenshot,
		EvidenceLog,
		EvidencePayload,
		EvidenceConversation:
		return true
	default:
		return false
	}
}

// String returns the string representation of the evidence type.
func (e EvidenceType) String() string {
	return string(e)
}

// DisplayName returns a human-readable display name for the evidence type.
func (e EvidenceType) DisplayName() string {
	switch e {
	case EvidenceHTTPRequest:
		return "HTTP Request"
	case EvidenceHTTPResponse:
		return "HTTP Response"
	case EvidenceScreenshot:
		return "Screenshot"
	case EvidenceLog:
		return "Log"
	case EvidencePayload:
		return "Payload"
	case EvidenceConversation:
		return "Conversation"
	default:
		return string(e)
	}
}

// Validate checks if the evidence is valid.
func (e *Evidence) Validate() error {
	if !e.Type.IsValid() {
		return fmt.Errorf("invalid evidence type: %s", e.Type)
	}
	if e.Title == "" {
		return errors.New("evidence title is required")
	}
	if e.Content == "" {
		return errors.New("evidence content is required")
	}
	if e.Timestamp.IsZero() {
		return errors.New("evidence timestamp is required")
	}
	return nil
}

// NewEvidence creates a new Evidence with the current timestamp.
func NewEvidence(evidenceType EvidenceType, title, content string) *Evidence {
	return &Evidence{
		Type:      evidenceType,
		Title:     title,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// WithMetadata adds metadata to the evidence.
func (e *Evidence) WithMetadata(key string, value any) *Evidence {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// ParseEvidenceType parses a string into an EvidenceType value.
// Returns an error if the string is not a valid evidence type.
func ParseEvidenceType(s string) (EvidenceType, error) {
	evidenceType := EvidenceType(s)
	if !evidenceType.IsValid() {
		return "", fmt.Errorf("invalid evidence type: %s", s)
	}
	return evidenceType, nil
}

// AllEvidenceTypes returns all valid evidence types.
func AllEvidenceTypes() []EvidenceType {
	return []EvidenceType{
		EvidenceHTTPRequest,
		EvidenceHTTPResponse,
		EvidenceScreenshot,
		EvidenceLog,
		EvidencePayload,
		EvidenceConversation,
	}
}
