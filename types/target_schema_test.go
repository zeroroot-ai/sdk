// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/schema"
)

func TestTargetSchema_Validate(t *testing.T) {
	tests := []struct {
		name    string
		schema  TargetSchema
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid schema with required fields",
			schema: TargetSchema{
				Type:        "http_api",
				Version:     "1.0",
				Description: "HTTP API target",
				Schema: schema.Object(map[string]schema.JSON{
					"url": schema.StringWithDesc("Target URL"),
				}, "url"),
			},
			wantErr: false,
		},
		{
			name: "valid schema without required fields",
			schema: TargetSchema{
				Type:        "custom",
				Version:     "1.0",
				Description: "Custom target",
				Schema: schema.Object(map[string]schema.JSON{
					"host": schema.String(),
					"port": schema.Int(),
				}),
			},
			wantErr: false,
		},
		{
			name: "missing type",
			schema: TargetSchema{
				Version: "1.0",
				Schema:  schema.Object(map[string]schema.JSON{}),
			},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name: "missing version",
			schema: TargetSchema{
				Type:   "test",
				Schema: schema.Object(map[string]schema.JSON{}),
			},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "schema not object type",
			schema: TargetSchema{
				Type:        "test",
				Version:     "1.0",
				Description: "Test target",
				Schema:      schema.String(),
			},
			wantErr: true,
			errMsg:  "must be of type 'object'",
		},
		{
			name: "required field not in properties",
			schema: TargetSchema{
				Type:        "test",
				Version:     "1.0",
				Description: "Test target",
				Schema: schema.Object(map[string]schema.JSON{
					"host": schema.String(),
				}, "url"), // "url" is required but not in properties
			},
			wantErr: true,
			errMsg:  "required field 'url'",
		},
		{
			name: "required fields with no properties",
			schema: TargetSchema{
				Type:        "test",
				Version:     "1.0",
				Description: "Test target",
				Schema: schema.JSON{
					Type:     "object",
					Required: []string{"url"},
				},
			},
			wantErr: true,
			errMsg:  "must define at least one property",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTargetSchema_ValidateConnection(t *testing.T) {
	httpSchema := TargetSchema{
		Type:        "http_api",
		Version:     "1.0",
		Description: "HTTP API target",
		Schema: schema.Object(map[string]schema.JSON{
			"url":     schema.StringWithDesc("Target URL"),
			"method":  schema.String(),
			"timeout": schema.Int(),
		}, "url"),
	}

	tests := []struct {
		name       string
		schema     TargetSchema
		connection map[string]any
		wantErr    bool
		errMsg     string
	}{
		{
			name:   "valid connection with required fields",
			schema: httpSchema,
			connection: map[string]any{
				"url": "https://api.example.com",
			},
			wantErr: false,
		},
		{
			name:   "valid connection with optional fields",
			schema: httpSchema,
			connection: map[string]any{
				"url":     "https://api.example.com",
				"method":  "POST",
				"timeout": 30,
			},
			wantErr: false,
		},
		{
			name:   "missing required field",
			schema: httpSchema,
			connection: map[string]any{
				"method": "GET",
			},
			wantErr: true,
			errMsg:  "required field url is missing",
		},
		{
			name:       "nil connection",
			schema:     httpSchema,
			connection: nil,
			wantErr:    true,
			errMsg:     "connection parameters cannot be nil",
		},
		{
			name:   "invalid field type",
			schema: httpSchema,
			connection: map[string]any{
				"url":     "https://api.example.com",
				"timeout": "not-a-number",
			},
			wantErr: true,
			errMsg:  "expected integer",
		},
		{
			name:       "empty connection when required fields exist",
			schema:     httpSchema,
			connection: map[string]any{},
			wantErr:    true,
			errMsg:     "required field url is missing",
		},
		{
			name: "valid connection with no required fields",
			schema: TargetSchema{
				Type:        "custom",
				Version:     "1.0",
				Description: "Custom target",
				Schema: schema.Object(map[string]schema.JSON{
					"host": schema.String(),
					"port": schema.Int(),
				}),
			},
			connection: map[string]any{},
			wantErr:    false,
		},
		{
			name: "connection with extra fields is valid",
			schema: TargetSchema{
				Type:        "test",
				Version:     "1.0",
				Description: "Test target",
				Schema: schema.Object(map[string]schema.JSON{
					"url": schema.String(),
				}, "url"),
			},
			connection: map[string]any{
				"url":         "https://example.com",
				"extra_key":   "extra_value",
				"another_key": 123,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.ValidateConnection(tt.connection)
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

func TestTargetSchema_ValidateConnection_ComplexSchema(t *testing.T) {
	// Test with a more complex schema including enums and nested objects
	k8sSchema := TargetSchema{
		Type:        "kubernetes",
		Version:     "1.0",
		Description: "Kubernetes cluster target",
		Schema: schema.Object(map[string]schema.JSON{
			"cluster":    schema.StringWithDesc("Cluster name or context"),
			"namespace":  schema.String(),
			"kubeconfig": schema.String(),
			"api_server": schema.StringWithDesc("API server URL"),
		}, "cluster"),
	}

	tests := []struct {
		name       string
		connection map[string]any
		wantErr    bool
	}{
		{
			name: "valid minimal kubernetes connection",
			connection: map[string]any{
				"cluster": "prod-cluster",
			},
			wantErr: false,
		},
		{
			name: "valid full kubernetes connection",
			connection: map[string]any{
				"cluster":    "prod-cluster",
				"namespace":  "default",
				"kubeconfig": "/path/to/kubeconfig",
				"api_server": "https://api.k8s.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing required cluster field",
			connection: map[string]any{
				"namespace": "default",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := k8sSchema.ValidateConnection(tt.connection)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
