// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"testing"
	"time"
)

func TestEvidenceType_IsValid(t *testing.T) {
	tests := []struct {
		name         string
		evidenceType EvidenceType
		want         bool
	}{
		{"http_request is valid", EvidenceHTTPRequest, true},
		{"http_response is valid", EvidenceHTTPResponse, true},
		{"screenshot is valid", EvidenceScreenshot, true},
		{"log is valid", EvidenceLog, true},
		{"payload is valid", EvidencePayload, true},
		{"conversation is valid", EvidenceConversation, true},
		{"empty is invalid", EvidenceType(""), false},
		{"unknown is invalid", EvidenceType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.evidenceType.IsValid(); got != tt.want {
				t.Errorf("EvidenceType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvidenceType_String(t *testing.T) {
	tests := []struct {
		name         string
		evidenceType EvidenceType
		want         string
	}{
		{"http_request", EvidenceHTTPRequest, "http_request"},
		{"http_response", EvidenceHTTPResponse, "http_response"},
		{"screenshot", EvidenceScreenshot, "screenshot"},
		{"log", EvidenceLog, "log"},
		{"payload", EvidencePayload, "payload"},
		{"conversation", EvidenceConversation, "conversation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.evidenceType.String(); got != tt.want {
				t.Errorf("EvidenceType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvidenceType_DisplayName(t *testing.T) {
	tests := []struct {
		name         string
		evidenceType EvidenceType
		want         string
	}{
		{"http_request", EvidenceHTTPRequest, "HTTP Request"},
		{"http_response", EvidenceHTTPResponse, "HTTP Response"},
		{"screenshot", EvidenceScreenshot, "Screenshot"},
		{"log", EvidenceLog, "Log"},
		{"payload", EvidencePayload, "Payload"},
		{"conversation", EvidenceConversation, "Conversation"},
		{"unknown", EvidenceType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.evidenceType.DisplayName(); got != tt.want {
				t.Errorf("EvidenceType.DisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvidence_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		evidence Evidence
		wantErr  bool
	}{
		{
			name: "valid evidence",
			evidence: Evidence{
				Type:      EvidenceHTTPRequest,
				Title:     "Test Request",
				Content:   "POST /api/test",
				Timestamp: now,
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			evidence: Evidence{
				Type:      EvidenceType("invalid"),
				Title:     "Test",
				Content:   "Content",
				Timestamp: now,
			},
			wantErr: true,
		},
		{
			name: "missing title",
			evidence: Evidence{
				Type:      EvidenceLog,
				Title:     "",
				Content:   "Content",
				Timestamp: now,
			},
			wantErr: true,
		},
		{
			name: "missing content",
			evidence: Evidence{
				Type:      EvidenceLog,
				Title:     "Test",
				Content:   "",
				Timestamp: now,
			},
			wantErr: true,
		},
		{
			name: "missing timestamp",
			evidence: Evidence{
				Type:      EvidenceLog,
				Title:     "Test",
				Content:   "Content",
				Timestamp: time.Time{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.evidence.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Evidence.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewEvidence(t *testing.T) {
	before := time.Now()
	evidence := NewEvidence(EvidenceHTTPRequest, "Test Request", "POST /api/test")
	after := time.Now()

	if evidence.Type != EvidenceHTTPRequest {
		t.Errorf("NewEvidence() Type = %v, want %v", evidence.Type, EvidenceHTTPRequest)
	}
	if evidence.Title != "Test Request" {
		t.Errorf("NewEvidence() Title = %v, want %v", evidence.Title, "Test Request")
	}
	if evidence.Content != "POST /api/test" {
		t.Errorf("NewEvidence() Content = %v, want %v", evidence.Content, "POST /api/test")
	}
	if evidence.Timestamp.Before(before) || evidence.Timestamp.After(after) {
		t.Errorf("NewEvidence() Timestamp not in expected range")
	}
	if evidence.Metadata == nil {
		t.Error("NewEvidence() Metadata is nil, want initialized map")
	}
}

func TestEvidence_WithMetadata(t *testing.T) {
	evidence := NewEvidence(EvidenceLog, "Test", "Content")
	evidence.WithMetadata("key1", "value1")
	evidence.WithMetadata("key2", 123)

	if len(evidence.Metadata) != 2 {
		t.Errorf("WithMetadata() resulted in %d items, want 2", len(evidence.Metadata))
	}
	if evidence.Metadata["key1"] != "value1" {
		t.Errorf("WithMetadata() key1 = %v, want value1", evidence.Metadata["key1"])
	}
	if evidence.Metadata["key2"] != 123 {
		t.Errorf("WithMetadata() key2 = %v, want 123", evidence.Metadata["key2"])
	}
}

func TestParseEvidenceType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    EvidenceType
		wantErr bool
	}{
		{"parse http_request", "http_request", EvidenceHTTPRequest, false},
		{"parse http_response", "http_response", EvidenceHTTPResponse, false},
		{"parse screenshot", "screenshot", EvidenceScreenshot, false},
		{"parse log", "log", EvidenceLog, false},
		{"parse payload", "payload", EvidencePayload, false},
		{"parse conversation", "conversation", EvidenceConversation, false},
		{"invalid type", "invalid", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEvidenceType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEvidenceType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseEvidenceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllEvidenceTypes(t *testing.T) {
	evidenceTypes := AllEvidenceTypes()
	if len(evidenceTypes) != 6 {
		t.Errorf("AllEvidenceTypes() returned %d types, want 6", len(evidenceTypes))
	}

	expected := []EvidenceType{
		EvidenceHTTPRequest,
		EvidenceHTTPResponse,
		EvidenceScreenshot,
		EvidenceLog,
		EvidencePayload,
		EvidenceConversation,
	}

	for i, et := range expected {
		if evidenceTypes[i] != et {
			t.Errorf("AllEvidenceTypes()[%d] = %v, want %v", i, evidenceTypes[i], et)
		}
	}
}
