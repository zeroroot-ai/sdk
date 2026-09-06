// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
)

// TestToProperties_AllNumericTypes verifies all numeric types are handled correctly.
func TestToProperties_AllNumericTypes(t *testing.T) {
	statusCode := int32(200)
	contentLength := int64(2048)

	endpoint := &taxonomypb.Endpoint{
		Id:            "ep-123",
		Url:           "https://example.com",
		StatusCode:    &statusCode,
		ContentLength: &contentLength,
	}

	props, err := ToProperties(endpoint)
	require.NoError(t, err)

	assert.Equal(t, int32(200), props["status_code"])
	assert.Equal(t, int64(2048), props["content_length"])

	// Verify types are preserved
	assert.IsType(t, int32(0), props["status_code"])
	assert.IsType(t, int64(0), props["content_length"])
}

// TestToProperties_FloatTypes verifies float/double handling.
func TestToProperties_FloatTypes(t *testing.T) {
	confidence := 0.95
	cvssScore := 7.5

	finding := &taxonomypb.Finding{
		Id:         "find-123",
		Title:      "XSS Vulnerability",
		Severity:   "high",
		Confidence: &confidence,
		CvssScore:  &cvssScore,
	}

	props, err := ToProperties(finding)
	require.NoError(t, err)

	assert.Equal(t, 0.95, props["confidence"])
	assert.Equal(t, 7.5, props["cvss_score"])
	assert.IsType(t, float64(0), props["confidence"])
	assert.IsType(t, float64(0), props["cvss_score"])
}

// TestToProperties_EmptyMessage verifies handling of message with no set fields.
func TestToProperties_EmptyMessage(t *testing.T) {
	host := &taxonomypb.Host{
		Id: "host-empty",
		// No other fields set
	}

	props, err := ToProperties(host)
	require.NoError(t, err)

	// Should be empty since no optional fields are set and id is filtered
	assert.Empty(t, props)
}

// TestToProperties_ZeroValues verifies zero values are excluded.
func TestToProperties_ZeroValues(t *testing.T) {
	// Set fields to their zero values
	emptyString := ""
	zeroInt := int32(0)
	zeroFloat := 0.0

	finding := &taxonomypb.Finding{
		Id:          "find-123",
		Title:       "Test Finding",
		Severity:    "low",
		Description: &emptyString, // Empty string should be excluded
		Confidence:  &zeroFloat,   // Zero float should be excluded
	}

	props, err := ToProperties(finding)
	require.NoError(t, err)

	// Non-zero values included
	assert.Contains(t, props, "title")
	assert.Contains(t, props, "severity")

	// Zero values excluded
	assert.NotContains(t, props, "description")
	assert.NotContains(t, props, "confidence")

	// If we set a port with zero number (which shouldn't happen in practice)
	state := "closed"
	port := &taxonomypb.Port{
		Id:           "port-123",
		Number:       zeroInt,
		Protocol:     "tcp",
		State:        &state,
		ParentHostId: "host-123",
	}

	portProps, err := ToProperties(port)
	require.NoError(t, err)

	// Zero number should be excluded
	assert.NotContains(t, portProps, "number")
	assert.Contains(t, portProps, "protocol")
	assert.Contains(t, portProps, "state")
}

// TestToProperties_BooleanHandling verifies boolean fields are handled correctly.
func TestToProperties_BooleanHandling(t *testing.T) {
	// Note: taxonomypb doesn't have boolean fields in the current schema,
	// but we can verify the logic works for when they're added
	// This test documents expected behavior

	// For now, verify that false bools would be excluded (zero value)
	assert.True(t, isZeroValue(false))
	assert.False(t, isZeroValue(true))
}

// TestIdentifyingProperties_Domain verifies domain identification.
func TestIdentifyingProperties_Domain(t *testing.T) {
	domain := &taxonomypb.Domain{
		Id:   "dom-123",
		Name: "example.com",
	}

	idProps, err := IdentifyingProperties("domain", domain)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "example.com"}, idProps)
}

// TestIdentifyingProperties_Subdomain verifies subdomain identification.
func TestIdentifyingProperties_Subdomain(t *testing.T) {
	fullName := "api.example.com"
	subdomain := &taxonomypb.Subdomain{
		Id:             "sub-123",
		Name:           "api",
		FullName:       &fullName,
		ParentDomainId: "dom-123",
	}

	idProps, err := IdentifyingProperties("subdomain", subdomain)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "api"}, idProps)
}

// TestIdentifyingProperties_Certificate verifies certificate identification.
func TestIdentifyingProperties_Certificate(t *testing.T) {
	subject := "CN=example.com"
	fingerprint := "abc123def456"
	certificate := &taxonomypb.Certificate{
		Id:                "cert-123",
		Subject:           &subject,
		FingerprintSha256: &fingerprint,
	}

	idProps, err := IdentifyingProperties("certificate", certificate)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"fingerprint_sha256": "abc123def456"}, idProps)
}

// TestIdentifyingProperties_Finding verifies finding identification.
func TestIdentifyingProperties_Finding(t *testing.T) {
	desc := "Test vulnerability"
	finding := &taxonomypb.Finding{
		Id:          "find-123",
		Title:       "SQL Injection",
		Description: &desc,
		Severity:    "high",
	}

	idProps, err := IdentifyingProperties("finding", finding)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"title": "SQL Injection"}, idProps)
}

// TestConvertFieldValue_Coverage ensures all code paths are tested.
func TestConvertFieldValue_Coverage(t *testing.T) {
	// Test with various proto types
	version := "1.0.0"
	confidence := int32(100)

	tech := &taxonomypb.Technology{
		Id:         "tech-123",
		Name:       "TestTech",
		Version:    &version,
		Confidence: &confidence,
	}

	props, err := ToProperties(tech)
	require.NoError(t, err)

	// Verify all field types are converted
	assert.IsType(t, "", props["name"])             // string
	assert.IsType(t, "", props["version"])          // optional string
	assert.IsType(t, int32(0), props["confidence"]) // optional int32
}

// TestIsFrameworkField_AllPatterns verifies framework field detection.
func TestIsFrameworkField_AllPatterns(t *testing.T) {
	tests := []struct {
		field       string
		isFramework bool
	}{
		// Standard framework fields
		{"id", true},
		{"parent_id", true},
		{"parent_type", true},
		{"parent_relationship", true},
		{"mission_id", true},
		{"mission_run_id", true},
		{"agent_run_id", true},
		{"discovered_by", true},
		{"discovered_at", true},
		{"created_at", true},
		{"updated_at", true},

		// Parent reference fields (pattern: parent_*_id)
		{"parent_host_id", true},
		{"parent_port_id", true},
		{"parent_service_id", true},
		{"parent_finding_id", true},
		{"parent_domain_id", true},
		{"parent_mission_id", true},

		// User fields (should not be filtered)
		{"name", false},
		{"ip", false},
		{"hostname", false},
		{"protocol", false},
		{"number", false},
		{"url", false},
		{"title", false},
		{"severity", false},
		{"version", false},
		{"product", false},

		// Edge cases
		{"parent", false},           // Not a framework field
		{"parent_", false},          // Not matching pattern
		{"parent_x_id", true},       // Matches pattern
		{"my_parent_id", false},     // Doesn't start with parent_
		{"parent_id_custom", false}, // Doesn't end with _id
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := isFrameworkField(tt.field)
			assert.Equal(t, tt.isFramework, result, "field: %s", tt.field)
		})
	}
}

// TestIdentifyingProperties_UnknownType verifies error for unknown type.
func TestIdentifyingProperties_UnknownType(t *testing.T) {
	ip := "192.168.1.1"
	host := &taxonomypb.Host{Id: "h1", Ip: &ip}

	_, err := IdentifyingProperties("nonexistent_type", host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown node type")
}

// TestIdentifyingProperties_MissingProperty verifies error for missing property.
func TestIdentifyingProperties_MissingProperty(t *testing.T) {
	// No IP set
	hostname := "test.local"
	host := &taxonomypb.Host{Id: "h1", Hostname: &hostname}

	_, err := IdentifyingProperties("host", host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing identifying property")
}

// TestIdentifyingProperties_ZeroPortNumber verifies error for zero port number.
func TestIdentifyingProperties_ZeroPortNumber(t *testing.T) {
	// Number is zero (filtered out)
	port := &taxonomypb.Port{
		Id:           "p1",
		Number:       0, // Will be filtered
		Protocol:     "tcp",
		ParentHostId: "h1",
	}

	_, err := IdentifyingProperties("port", port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing identifying property")
}
