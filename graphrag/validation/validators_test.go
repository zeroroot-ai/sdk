// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package validation provides tests for generated validators.
package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
	"google.golang.org/protobuf/proto"
)

func TestIsCoreType(t *testing.T) {
	// v4.0: all 17 taxonomy node types are core types (including execution-tier types).
	coreTypes := []string{
		"mission", "mission_run", "agent_run", "tool_execution", "llm_call",
		"domain", "subdomain", "host", "port", "service", "endpoint",
		"technology", "certificate", "finding", "evidence", "technique",
		"compliance_signal",
	}

	for _, ct := range coreTypes {
		assert.True(t, IsCoreType(ct), "type %s should be a core type", ct)
	}

	// Non-taxonomy types should return false.
	customTypes := []string{
		"custom_type", "my_node", "unknown", "", "HOST", "Domain",
	}

	for _, ct := range customTypes {
		assert.False(t, IsCoreType(ct), "type %s should NOT be a core type", ct)
	}
}

func TestGetParentRequirement(t *testing.T) {
	tests := []struct {
		nodeType       string
		expectFound    bool
		expectParent   string
		expectRelation string
		expectRequired bool
	}{
		{"port", true, "host", "HAS_PORT", true},
		{"service", true, "port", "RUNS_SERVICE", true},
		{"subdomain", true, "domain", "HAS_SUBDOMAIN", true},
		{"endpoint", true, "service", "HAS_ENDPOINT", true},
		{"evidence", true, "finding", "HAS_EVIDENCE", true},
		// Types without parent requirements
		{"host", false, "", "", false},
		{"domain", false, "", "", false},
		{"finding", false, "", "", false},
		{"technology", false, "", "", false},
		{"certificate", false, "", "", false},
		// Custom type
		{"custom", false, "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			req, found := GetParentRequirement(tt.nodeType)
			assert.Equal(t, tt.expectFound, found)
			if found {
				assert.Equal(t, tt.expectParent, req.ParentType)
				assert.Equal(t, tt.expectRelation, req.Relationship)
				assert.Equal(t, tt.expectRequired, req.Required)
			}
		})
	}
}

func TestValidateNode(t *testing.T) {
	tests := []struct {
		name       string
		nodeType   string
		properties map[string]any
		hasParent  bool
		wantErr    bool
	}{
		{
			name:       "port with parent passes",
			nodeType:   "port",
			properties: map[string]any{"number": 443},
			hasParent:  true,
			wantErr:    false,
		},
		{
			name:       "port without parent fails",
			nodeType:   "port",
			properties: map[string]any{"number": 443},
			hasParent:  false,
			wantErr:    true,
		},
		{
			name:       "host without parent passes",
			nodeType:   "host",
			properties: map[string]any{"ip": "192.168.1.1"},
			hasParent:  false,
			wantErr:    false,
		},
		{
			name:       "custom type always passes",
			nodeType:   "custom_type",
			properties: map[string]any{},
			hasParent:  false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNode(tt.nodeType, tt.properties, tt.hasParent)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  *taxonomypb.Domain
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid domain",
			domain:  &taxonomypb.Domain{Name: "example.com"},
			wantErr: false,
		},
		{
			name:    "empty name fails",
			domain:  &taxonomypb.Domain{Name: ""},
			wantErr: true,
			errMsg:  "domain name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		host    *taxonomypb.Host
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid host with IP",
			host:    &taxonomypb.Host{Ip: proto.String("192.168.1.1")},
			wantErr: false,
		},
		{
			name:    "valid host with hostname",
			host:    &taxonomypb.Host{Hostname: proto.String("server.example.com")},
			wantErr: false,
		},
		{
			name:    "valid host with both IP and hostname",
			host:    &taxonomypb.Host{Ip: proto.String("192.168.1.1"), Hostname: proto.String("server.example.com")},
			wantErr: false,
		},
		{
			name:    "missing ip and hostname fails",
			host:    &taxonomypb.Host{},
			wantErr: true,
			errMsg:  "host requires either ip or hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHost(tt.host)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    *taxonomypb.Port
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid port 443",
			port:    &taxonomypb.Port{Number: 443, Protocol: "tcp"},
			wantErr: false,
		},
		{
			name:    "valid port 1",
			port:    &taxonomypb.Port{Number: 1, Protocol: "tcp"},
			wantErr: false,
		},
		{
			name:    "valid port 65535",
			port:    &taxonomypb.Port{Number: 65535, Protocol: "udp"},
			wantErr: false,
		},
		{
			name:    "invalid port 0",
			port:    &taxonomypb.Port{Number: 0, Protocol: "tcp"},
			wantErr: true,
			errMsg:  "port number must be between 1 and 65535",
		},
		{
			name:    "invalid port -1",
			port:    &taxonomypb.Port{Number: -1, Protocol: "tcp"},
			wantErr: true,
			errMsg:  "port number must be between 1 and 65535",
		},
		{
			name:    "invalid port 65536",
			port:    &taxonomypb.Port{Number: 65536, Protocol: "tcp"},
			wantErr: true,
			errMsg:  "port number must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *taxonomypb.Endpoint
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid endpoint",
			endpoint: &taxonomypb.Endpoint{Url: "https://example.com/api"},
			wantErr:  false,
		},
		{
			name:     "empty URL fails",
			endpoint: &taxonomypb.Endpoint{Url: ""},
			wantErr:  true,
			errMsg:   "endpoint URL cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpoint(tt.endpoint)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFinding(t *testing.T) {
	tests := []struct {
		name    string
		finding *taxonomypb.Finding
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid finding",
			finding: &taxonomypb.Finding{Title: "SQL Injection", Severity: "high"},
			wantErr: false,
		},
		{
			name:    "empty title fails",
			finding: &taxonomypb.Finding{Title: "", Severity: "high"},
			wantErr: true,
			errMsg:  "finding title cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFinding(tt.finding)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSubdomain(t *testing.T) {
	tests := []struct {
		name      string
		subdomain *taxonomypb.Subdomain
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid subdomain",
			subdomain: &taxonomypb.Subdomain{Name: "www", ParentDomainId: "example.com"},
			wantErr:   false,
		},
		{
			name:      "empty name fails",
			subdomain: &taxonomypb.Subdomain{Name: "", ParentDomainId: "example.com"},
			wantErr:   true,
			errMsg:    "subdomain name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubdomain(tt.subdomain)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidatorsWithNoRules tests validators that have no specific rules
func TestValidatorsWithNoRules(t *testing.T) {
	// These validators have no rules and should always pass
	t.Run("Service", func(t *testing.T) {
		err := ValidateService(&taxonomypb.Service{Name: "https"})
		assert.NoError(t, err)
	})

	t.Run("Technology", func(t *testing.T) {
		err := ValidateTechnology(&taxonomypb.Technology{Name: "nginx"})
		assert.NoError(t, err)
	})

	t.Run("Certificate", func(t *testing.T) {
		err := ValidateCertificate(&taxonomypb.Certificate{FingerprintSha256: proto.String("abc123")})
		assert.NoError(t, err)
	})

	t.Run("Evidence", func(t *testing.T) {
		err := ValidateEvidence(&taxonomypb.Evidence{ParentFindingId: "test-finding"})
		assert.NoError(t, err)
	})
}
