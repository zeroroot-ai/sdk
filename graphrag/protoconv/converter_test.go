// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
)

func TestToProperties(t *testing.T) {
	tests := []struct {
		name     string
		msg      func() *taxonomypb.Host
		expected map[string]any
	}{
		{
			name: "host with all fields set",
			msg: func() *taxonomypb.Host {
				ip := "192.168.1.1"
				hostname := "example.com"
				os := "Linux"
				osVersion := "5.15.0"
				macAddr := "00:11:22:33:44:55"
				state := "up"
				return &taxonomypb.Host{
					Id:         "host-123",
					Ip:         &ip,
					Hostname:   &hostname,
					Os:         &os,
					OsVersion:  &osVersion,
					MacAddress: &macAddr,
					State:      &state,
				}
			},
			expected: map[string]any{
				"ip":          "192.168.1.1",
				"hostname":    "example.com",
				"os":          "Linux",
				"os_version":  "5.15.0",
				"mac_address": "00:11:22:33:44:55",
				"state":       "up",
			},
		},
		{
			name: "host with only required fields",
			msg: func() *taxonomypb.Host {
				ip := "10.0.0.1"
				return &taxonomypb.Host{
					Id: "host-456",
					Ip: &ip,
				}
			},
			expected: map[string]any{
				"ip": "10.0.0.1",
			},
		},
		{
			name: "host with empty optional fields",
			msg: func() *taxonomypb.Host {
				ip := "172.16.0.1"
				hostname := ""
				return &taxonomypb.Host{
					Id:       "host-789",
					Ip:       &ip,
					Hostname: &hostname,
				}
			},
			expected: map[string]any{
				"ip": "172.16.0.1",
				// hostname should not be included (empty string)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props, err := ToProperties(tt.msg())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, props)
		})
	}
}

func TestToProperties_Port(t *testing.T) {
	state := "open"
	reason := "syn-ack"
	port := &taxonomypb.Port{
		Id:           "port-123",
		Number:       443,
		Protocol:     "tcp",
		State:        &state,
		Reason:       &reason,
		ParentHostId: "host-123",
	}

	props, err := ToProperties(port)
	require.NoError(t, err)

	expected := map[string]any{
		"number":   int32(443),
		"protocol": "tcp",
		"state":    "open",
		"reason":   "syn-ack",
	}

	assert.Equal(t, expected, props)
	// Verify framework fields are excluded
	assert.NotContains(t, props, "id")
	assert.NotContains(t, props, "parent_host_id")
}

func TestToProperties_Service(t *testing.T) {
	product := "nginx"
	version := "1.21.0"
	banner := "nginx/1.21.0"
	service := &taxonomypb.Service{
		Id:           "svc-123",
		Name:         "http",
		Product:      &product,
		Version:      &version,
		Banner:       &banner,
		ParentPortId: "port-123",
	}

	props, err := ToProperties(service)
	require.NoError(t, err)

	expected := map[string]any{
		"name":    "http",
		"product": "nginx",
		"version": "1.21.0",
		"banner":  "nginx/1.21.0",
	}

	assert.Equal(t, expected, props)
}

func TestToProperties_Endpoint(t *testing.T) {
	method := "GET"
	statusCode := int32(200)
	contentType := "text/html"
	contentLength := int64(1024)
	title := "Example Page"

	endpoint := &taxonomypb.Endpoint{
		Id:              "ep-123",
		Url:             "https://example.com/api/v1/users",
		Method:          &method,
		StatusCode:      &statusCode,
		ContentType:     &contentType,
		ContentLength:   &contentLength,
		Title:           &title,
		ParentServiceId: "svc-123",
	}

	props, err := ToProperties(endpoint)
	require.NoError(t, err)

	expected := map[string]any{
		"url":            "https://example.com/api/v1/users",
		"method":         "GET",
		"status_code":    int32(200),
		"content_type":   "text/html",
		"content_length": int64(1024),
		"title":          "Example Page",
	}

	assert.Equal(t, expected, props)
}

func TestToProperties_Finding(t *testing.T) {
	desc := "SQL injection vulnerability in login form"
	confidence := 0.95
	category := "injection"
	remediation := "Use parameterized queries"
	cvssScore := 9.8
	cveIds := "CVE-2024-1234"

	finding := &taxonomypb.Finding{
		Id:          "find-123",
		Title:       "SQL Injection",
		Description: &desc,
		Severity:    "critical",
		Confidence:  &confidence,
		Category:    &category,
		Remediation: &remediation,
		CvssScore:   &cvssScore,
		CveIds:      &cveIds,
	}

	props, err := ToProperties(finding)
	require.NoError(t, err)

	expected := map[string]any{
		"title":       "SQL Injection",
		"description": "SQL injection vulnerability in login form",
		"severity":    "critical",
		"confidence":  0.95,
		"category":    "injection",
		"remediation": "Use parameterized queries",
		"cvss_score":  9.8,
		"cve_ids":     "CVE-2024-1234",
	}

	assert.Equal(t, expected, props)
}

func TestToProperties_Technology(t *testing.T) {
	version := "3.11.0"
	category := "programming-language"
	confidence := int32(90)

	tech := &taxonomypb.Technology{
		Id:         "tech-123",
		Name:       "Python",
		Version:    &version,
		Category:   &category,
		Confidence: &confidence,
	}

	props, err := ToProperties(tech)
	require.NoError(t, err)

	expected := map[string]any{
		"name":       "Python",
		"version":    "3.11.0",
		"category":   "programming-language",
		"confidence": int32(90),
	}

	assert.Equal(t, expected, props)
}

func TestToProperties_NilMessage(t *testing.T) {
	props, err := ToProperties(nil)
	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "proto message is nil")
}

func TestIdentifyingProperties(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		msg      func() *taxonomypb.Host
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "host identified by ip",
			nodeType: "host",
			msg: func() *taxonomypb.Host {
				ip := "192.168.1.100"
				hostname := "server.local"
				return &taxonomypb.Host{
					Id:       "host-123",
					Ip:       &ip,
					Hostname: &hostname,
				}
			},
			expected: map[string]any{
				"ip": "192.168.1.100",
			},
			wantErr: false,
		},
		{
			name:     "missing identifying property",
			nodeType: "host",
			msg: func() *taxonomypb.Host {
				hostname := "server.local"
				return &taxonomypb.Host{
					Id:       "host-123",
					Hostname: &hostname,
				}
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props, err := IdentifyingProperties(tt.nodeType, tt.msg())
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, props)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, props)
			}
		})
	}
}

func TestIdentifyingProperties_Port(t *testing.T) {
	state := "open"
	port := &taxonomypb.Port{
		Id:           "port-123",
		Number:       8080,
		Protocol:     "tcp",
		State:        &state,
		ParentHostId: "host-123",
	}

	props, err := IdentifyingProperties("port", port)
	require.NoError(t, err)

	expected := map[string]any{
		"number":   int32(8080),
		"protocol": "tcp",
	}

	assert.Equal(t, expected, props)
}

func TestIdentifyingProperties_Service(t *testing.T) {
	product := "Apache"
	version := "2.4.0"
	service := &taxonomypb.Service{
		Id:           "svc-123",
		Name:         "http",
		Product:      &product,
		Version:      &version,
		ParentPortId: "port-123",
	}

	props, err := IdentifyingProperties("service", service)
	require.NoError(t, err)

	expected := map[string]any{
		"name": "http",
	}

	assert.Equal(t, expected, props)
}

func TestIdentifyingProperties_Endpoint(t *testing.T) {
	method := "POST"
	statusCode := int32(201)
	endpoint := &taxonomypb.Endpoint{
		Id:              "ep-123",
		Url:             "https://api.example.com/v1/resource",
		Method:          &method,
		StatusCode:      &statusCode,
		ParentServiceId: "svc-123",
	}

	props, err := IdentifyingProperties("endpoint", endpoint)
	require.NoError(t, err)

	expected := map[string]any{
		"url":    "https://api.example.com/v1/resource",
		"method": "POST",
	}

	assert.Equal(t, expected, props)
}

func TestIdentifyingProperties_Technology(t *testing.T) {
	version := "16.0.0"
	category := "runtime"
	tech := &taxonomypb.Technology{
		Id:       "tech-123",
		Name:     "Node.js",
		Version:  &version,
		Category: &category,
	}

	props, err := IdentifyingProperties("technology", tech)
	require.NoError(t, err)

	expected := map[string]any{
		"name":    "Node.js",
		"version": "16.0.0",
	}

	assert.Equal(t, expected, props)
}

func TestIdentifyingProperties_UnknownNodeType(t *testing.T) {
	ip := "192.168.1.1"
	host := &taxonomypb.Host{
		Id: "host-123",
		Ip: &ip,
	}

	props, err := IdentifyingProperties("unknown_type", host)
	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "unknown node type")
}

func TestIdentifyingProperties_NilMessage(t *testing.T) {
	props, err := IdentifyingProperties("host", nil)
	assert.Error(t, err)
	assert.Nil(t, props)
	assert.Contains(t, err.Error(), "proto message is nil")
}

func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"zero int32", int32(0), true},
		{"non-zero int32", int32(42), false},
		{"zero int64", int64(0), true},
		{"non-zero int64", int64(100), false},
		{"zero float32", float32(0.0), true},
		{"non-zero float32", float32(3.14), false},
		{"zero float64", float64(0.0), true},
		{"non-zero float64", float64(2.718), false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"empty bytes", []byte{}, true},
		{"non-empty bytes", []byte{1, 2, 3}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isZeroValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFrameworkFieldFiltering(t *testing.T) {
	ip := "10.0.0.1"
	hostname := "test.local"
	host := &taxonomypb.Host{
		Id:       "host-123",
		Ip:       &ip,
		Hostname: &hostname,
	}

	props, err := ToProperties(host)
	require.NoError(t, err)

	// Verify framework fields are excluded
	frameworkFields := []string{
		"id",
		"parent_id",
		"parent_type",
		"parent_relationship",
		"mission_id",
		"mission_run_id",
		"agent_run_id",
		"discovered_by",
		"discovered_at",
		"created_at",
		"updated_at",
	}

	for _, field := range frameworkFields {
		assert.NotContains(t, props, field, "framework field %s should be excluded", field)
	}

	// Verify user fields are included
	assert.Contains(t, props, "ip")
	assert.Contains(t, props, "hostname")
}
