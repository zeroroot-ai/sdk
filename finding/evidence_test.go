// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"testing"
)

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
