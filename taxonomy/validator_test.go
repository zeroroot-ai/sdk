// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCWEFormat(t *testing.T) {
	tests := []struct {
		cwe   string
		valid bool
	}{
		{"CWE-89", true},
		{"CWE-1234", true},
		{"CWE-1", true},
		{"cwe-89", true},
		{"CWE89", false},
		{"CWE-", false},
		{"89", false},
		{"", false},
		{"CWE-abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.cwe, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidCWEFormat(tt.cwe))
		})
	}
}

func TestCVEFormat(t *testing.T) {
	tests := []struct {
		cve   string
		valid bool
	}{
		{"CVE-2024-12345", true},
		{"CVE-2024-1234", true},
		{"CVE-2024-123456", true},
		{"cve-2024-12345", true},
		{"CVE-2024-123", false}, // too short
		{"CVE-24-12345", false}, // year too short
		{"CVE-2024", false},     // missing ID
		{"CVE2024-12345", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cve, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidCVEFormat(tt.cve))
		})
	}
}
