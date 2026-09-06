// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
)

func TestMapRowToProto_Host(t *testing.T) {
	tests := []struct {
		name    string
		row     map[string]any
		want    *taxonomypb.Host
		wantErr bool
	}{
		{
			name: "all fields populated",
			row: map[string]any{
				"id":          "host-1",
				"ip":          "192.168.1.1",
				"hostname":    "server1.local",
				"os":          "Linux",
				"os_version":  "Ubuntu 22.04",
				"mac_address": "00:11:22:33:44:55",
				"state":       "up",
			},
			want: &taxonomypb.Host{
				Id:         "host-1",
				Ip:         proto.String("192.168.1.1"),
				Hostname:   proto.String("server1.local"),
				Os:         proto.String("Linux"),
				OsVersion:  proto.String("Ubuntu 22.04"),
				MacAddress: proto.String("00:11:22:33:44:55"),
				State:      proto.String("up"),
			},
		},
		{
			name: "minimal fields (only required)",
			row: map[string]any{
				"id": "host-2",
			},
			want: &taxonomypb.Host{
				Id: "host-2",
			},
		},
		{
			name: "nil values skipped",
			row: map[string]any{
				"id":       "host-3",
				"ip":       "192.168.1.2",
				"hostname": nil,
				"os":       nil,
			},
			want: &taxonomypb.Host{
				Id: "host-3",
				Ip: proto.String("192.168.1.2"),
			},
		},
		{
			name: "unknown fields ignored",
			row: map[string]any{
				"id":            "host-4",
				"ip":            "192.168.1.3",
				"unknown_field": "should be ignored",
				"extra":         123,
			},
			want: &taxonomypb.Host{
				Id: "host-4",
				Ip: proto.String("192.168.1.3"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &taxonomypb.Host{}
			err := MapRowToProto(tt.row, got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, proto.Equal(tt.want, got), "protos not equal:\nwant: %v\ngot:  %v", tt.want, got)
		})
	}
}

func TestMapRowToProto_Port(t *testing.T) {
	tests := []struct {
		name    string
		row     map[string]any
		want    *taxonomypb.Port
		wantErr bool
	}{
		{
			name: "full port information",
			row: map[string]any{
				"id":             "port-1",
				"number":         int64(80), // Neo4j returns int64
				"protocol":       "tcp",
				"state":          "open",
				"reason":         "syn-ack",
				"parent_host_id": "host-1",
			},
			want: &taxonomypb.Port{
				Id:           "port-1",
				Number:       80,
				Protocol:     "tcp",
				State:        proto.String("open"),
				Reason:       proto.String("syn-ack"),
				ParentHostId: "host-1",
			},
		},
		{
			name: "int64 to int32 conversion",
			row: map[string]any{
				"id":             "port-2",
				"number":         int64(443),
				"protocol":       "tcp",
				"parent_host_id": "host-1",
			},
			want: &taxonomypb.Port{
				Id:           "port-2",
				Number:       443,
				Protocol:     "tcp",
				ParentHostId: "host-1",
			},
		},
		{
			name: "int32 overflow error",
			row: map[string]any{
				"id":             "port-3",
				"number":         int64(3000000000), // > int32 max
				"protocol":       "tcp",
				"parent_host_id": "host-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &taxonomypb.Port{}
			err := MapRowToProto(tt.row, got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, proto.Equal(tt.want, got), "protos not equal:\nwant: %v\ngot:  %v", tt.want, got)
		})
	}
}

func TestMapRowToProto_Finding(t *testing.T) {
	tests := []struct {
		name    string
		row     map[string]any
		want    *taxonomypb.Finding
		wantErr bool
	}{
		{
			name: "complete finding",
			row: map[string]any{
				"id":          "finding-1",
				"title":       "SQL Injection",
				"description": "Potential SQL injection vulnerability",
				"severity":    "high",
				"confidence":  0.95,
				"category":    "injection",
				"cvss_score":  7.5,
				"cve_ids":     "CVE-2024-1234",
			},
			want: &taxonomypb.Finding{
				Id:          "finding-1",
				Title:       "SQL Injection",
				Description: proto.String("Potential SQL injection vulnerability"),
				Severity:    "high",
				Confidence:  proto.Float64(0.95),
				Category:    proto.String("injection"),
				CvssScore:   proto.Float64(7.5),
				CveIds:      proto.String("CVE-2024-1234"),
			},
		},
		{
			name: "float64 conversion from Neo4j",
			row: map[string]any{
				"id":         "finding-2",
				"title":      "XSS",
				"severity":   "medium",
				"confidence": float64(0.87), // Neo4j returns float64
				"cvss_score": float64(6.1),
			},
			want: &taxonomypb.Finding{
				Id:         "finding-2",
				Title:      "XSS",
				Severity:   "medium",
				Confidence: proto.Float64(0.87),
				CvssScore:  proto.Float64(6.1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &taxonomypb.Finding{}
			err := MapRowToProto(tt.row, got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, proto.Equal(tt.want, got), "protos not equal:\nwant: %v\ngot:  %v", tt.want, got)
		})
	}
}

func TestMapRowToProto_GraphNode(t *testing.T) {
	tests := []struct {
		name    string
		row     map[string]any
		want    *graphragpb.GraphNode
		wantErr bool
	}{
		{
			name: "graph node with timestamps",
			row: map[string]any{
				"id":             "node-1",
				"type":           "host",
				"mission_id":     "mission-123",
				"mission_run_id": "run-456",
				"agent_run_id":   "agent-789",
				"discovered_by":  "mytool-agent",
				"discovered_at":  int64(1704067200),
				"created_at":     int64(1704067200),
				"updated_at":     int64(1704067300),
			},
			want: &graphragpb.GraphNode{
				Id:           "node-1",
				Type:         "host",
				MissionId:    "mission-123",
				MissionRunId: "run-456",
				AgentRunId:   "agent-789",
				DiscoveredBy: "mytool-agent",
				DiscoveredAt: 1704067200,
				CreatedAt:    1704067200,
				UpdatedAt:    1704067300,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &graphragpb.GraphNode{}
			err := MapRowToProto(tt.row, got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, proto.Equal(tt.want, got), "protos not equal:\nwant: %v\ngot:  %v", tt.want, got)
		})
	}
}

func TestMapRowsToProtos(t *testing.T) {
	rows := []map[string]any{
		{
			"id":       "host-1",
			"ip":       "192.168.1.1",
			"hostname": "server1.local",
		},
		{
			"id":       "host-2",
			"ip":       "192.168.1.2",
			"hostname": "server2.local",
		},
		{
			"id": "host-3",
			"ip": "192.168.1.3",
			// No hostname
		},
	}

	want := []*taxonomypb.Host{
		{
			Id:       "host-1",
			Ip:       proto.String("192.168.1.1"),
			Hostname: proto.String("server1.local"),
		},
		{
			Id:       "host-2",
			Ip:       proto.String("192.168.1.2"),
			Hostname: proto.String("server2.local"),
		},
		{
			Id: "host-3",
			Ip: proto.String("192.168.1.3"),
		},
	}

	got, err := MapRowsToProtos(rows, func() *taxonomypb.Host {
		return &taxonomypb.Host{}
	})

	require.NoError(t, err)
	assert.Len(t, got, len(want))

	for i := range want {
		assert.True(t, proto.Equal(want[i], got[i]), "row %d: protos not equal:\nwant: %v\ngot:  %v", i, want[i], got[i])
	}
}

func TestMapRowsToProtos_EmptySlice(t *testing.T) {
	got, err := MapRowsToProtos([]map[string]any{}, func() *taxonomypb.Host {
		return &taxonomypb.Host{}
	})

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestMapRowsToProtos_NilSlice(t *testing.T) {
	got, err := MapRowsToProtos[*taxonomypb.Host](nil, func() *taxonomypb.Host {
		return &taxonomypb.Host{}
	})

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMapRowsToProtos_Error(t *testing.T) {
	rows := []map[string]any{
		{"id": "host-1", "ip": "192.168.1.1"},
		{"id": "host-2", "number": int64(3000000000)}, // Invalid - number field doesn't exist on Host
		{"id": "host-3", "ip": "192.168.1.3"},
	}

	_, err := MapRowsToProtos(rows, func() *taxonomypb.Host {
		return &taxonomypb.Host{}
	})

	// Should not error - unknown fields are ignored
	require.NoError(t, err)
}

func TestMapFieldsFromProto(t *testing.T) {
	host := &taxonomypb.Host{
		Id:         "host-1",
		Ip:         proto.String("192.168.1.1"),
		Hostname:   proto.String("server1.local"),
		Os:         proto.String("Linux"),
		OsVersion:  proto.String("Ubuntu 22.04"),
		MacAddress: proto.String("00:11:22:33:44:55"),
	}

	tests := []struct {
		name   string
		fields []string
		want   map[string]any
	}{
		{
			name:   "all fields",
			fields: nil,
			want: map[string]any{
				"id":          "host-1",
				"ip":          "192.168.1.1",
				"hostname":    "server1.local",
				"os":          "Linux",
				"os_version":  "Ubuntu 22.04",
				"mac_address": "00:11:22:33:44:55",
			},
		},
		{
			name:   "specific fields",
			fields: []string{"id", "ip", "hostname"},
			want: map[string]any{
				"id":       "host-1",
				"ip":       "192.168.1.1",
				"hostname": "server1.local",
			},
		},
		{
			name:   "single field",
			fields: []string{"ip"},
			want: map[string]any{
				"ip": "192.168.1.1",
			},
		},
		{
			name:   "non-existent field ignored",
			fields: []string{"ip", "non_existent"},
			want: map[string]any{
				"ip": "192.168.1.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapFieldsFromProto(host, tt.fields)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapFieldsFromProto_NilMessage(t *testing.T) {
	got := MapFieldsFromProto(nil, []string{"id"})
	assert.Nil(t, got)
}

func TestExtractIDFields(t *testing.T) {
	tests := []struct {
		name string
		msg  proto.Message
		want map[string]any
	}{
		{
			name: "host with id",
			msg: &taxonomypb.Host{
				Id:       "host-1",
				Ip:       proto.String("192.168.1.1"),
				Hostname: proto.String("server1"),
			},
			want: map[string]any{
				"id": "host-1",
			},
		},
		{
			name: "port with parent_host_id",
			msg: &taxonomypb.Port{
				Id:           "port-1",
				Number:       80,
				Protocol:     "tcp",
				ParentHostId: "host-1",
			},
			want: map[string]any{
				"id":             "port-1",
				"parent_host_id": "host-1",
			},
		},
		{
			name: "service with parent_port_id",
			msg: &taxonomypb.Service{
				Id:           "service-1",
				Name:         "http",
				ParentPortId: "port-1",
			},
			want: map[string]any{
				"id":             "service-1",
				"parent_port_id": "port-1",
			},
		},
		{
			name: "evidence with parent_finding_id",
			msg: &taxonomypb.Evidence{
				Id:              "evidence-1",
				Type:            "screenshot",
				ParentFindingId: "finding-1",
			},
			want: map[string]any{
				"id":                "evidence-1",
				"parent_finding_id": "finding-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractIDFields(tt.msg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractIDFields_NilMessage(t *testing.T) {
	got := ExtractIDFields(nil)
	assert.Nil(t, got)
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		msg     proto.Message
		wantErr bool
	}{
		{
			name: "all required fields set",
			msg: &taxonomypb.Host{
				Id: "host-1",
			},
			wantErr: false,
		},
		{
			name: "port with required fields",
			msg: &taxonomypb.Port{
				Id:           "port-1",
				Number:       80,
				Protocol:     "tcp",
				ParentHostId: "host-1",
			},
			wantErr: false,
		},
		{
			name: "finding with required fields",
			msg: &taxonomypb.Finding{
				Id:       "finding-1",
				Title:    "SQL Injection",
				Severity: "high",
			},
			wantErr: false,
		},
		{
			name:    "missing required id",
			msg:     &taxonomypb.Host{},
			wantErr: true,
		},
		{
			name: "missing required number",
			msg: &taxonomypb.Port{
				Id:           "port-1",
				Protocol:     "tcp",
				ParentHostId: "host-1",
				// Missing: Number
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequiredFields(tt.msg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRequiredFields_NilMessage(t *testing.T) {
	err := ValidateRequiredFields(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestSnakeToCamel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"snake_case", "snakeCase"},
		{"parent_host_id", "parentHostId"},
		{"single", "single"},
		{"multi_word_field", "multiWordField"},
		{"_leading_underscore", "LeadingUnderscore"},
		{"trailing_underscore_", "trailingUnderscore"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := snakeToCamel(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapRowToProto_TypeConversions(t *testing.T) {
	tests := []struct {
		name    string
		row     map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name: "various int types to int32",
			row: map[string]any{
				"id":             "port-1",
				"number":         int(80),
				"protocol":       "tcp",
				"parent_host_id": "host-1",
			},
			wantErr: false,
		},
		{
			name: "uint32 to int32",
			row: map[string]any{
				"id":             "port-1",
				"number":         uint32(443),
				"protocol":       "tcp",
				"parent_host_id": "host-1",
			},
			wantErr: false,
		},
		{
			name: "float to int conversion",
			row: map[string]any{
				"id":             "port-1",
				"number":         float64(22),
				"protocol":       "tcp",
				"parent_host_id": "host-1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &taxonomypb.Port{}
			err := MapRowToProto(tt.row, got)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMapRowToProto_EdgeCases(t *testing.T) {
	t.Run("nil target", func(t *testing.T) {
		err := MapRowToProto(map[string]any{"id": "test"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("nil row", func(t *testing.T) {
		err := MapRowToProto(nil, &taxonomypb.Host{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("empty row", func(t *testing.T) {
		host := &taxonomypb.Host{}
		err := MapRowToProto(map[string]any{}, host)
		assert.NoError(t, err)
		// All fields should remain unset
		assert.Empty(t, host.Id)
	})
}
