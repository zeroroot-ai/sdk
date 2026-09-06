// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

func TestNewTaxonomyAdapter_NilResponse(t *testing.T) {
	adapter := NewTaxonomyAdapter(nil)
	if adapter == nil {
		t.Fatal("expected non-nil adapter for nil response")
	}
	if adapter.Version() != "" {
		t.Errorf("expected empty version, got %q", adapter.Version())
	}
	if len(adapter.NodeTypes()) != 0 {
		t.Errorf("expected empty node types, got %d", len(adapter.NodeTypes()))
	}
}

func TestNewTaxonomyAdapter_EmptyResponse(t *testing.T) {
	resp := &harnesspb.GetTaxonomySchemaResponse{
		Version: "1.0.0",
	}
	adapter := NewTaxonomyAdapter(resp)
	if adapter.Version() != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", adapter.Version())
	}
	if len(adapter.NodeTypes()) != 0 {
		t.Errorf("expected empty node types, got %d", len(adapter.NodeTypes()))
	}
}

func TestNewTaxonomyAdapter_FullResponse(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	// Test version
	if adapter.Version() != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", adapter.Version())
	}

	// Test node types count
	nodeTypes := adapter.NodeTypes()
	if len(nodeTypes) != 2 {
		t.Errorf("expected 2 node types, got %d", len(nodeTypes))
	}

	// Test relationship types count
	relTypes := adapter.RelationshipTypes()
	if len(relTypes) != 2 {
		t.Errorf("expected 2 relationship types, got %d", len(relTypes))
	}

	// Test technique IDs count
	techIDs := adapter.TechniqueIDs("")
	if len(techIDs) != 2 {
		t.Errorf("expected 2 techniques, got %d", len(techIDs))
	}
}

func TestTaxonomyAdapter_IsCanonicalNodeType(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	tests := []struct {
		name     string
		typeName string
		expected bool
	}{
		{"existing type", "host", true},
		{"existing type 2", "domain", true},
		{"non-existent type", "unknown", false},
		{"empty type", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.IsCanonicalNodeType(tt.typeName)
			if got != tt.expected {
				t.Errorf("IsCanonicalNodeType(%q) = %v, want %v", tt.typeName, got, tt.expected)
			}
		})
	}
}

func TestTaxonomyAdapter_IsCanonicalRelationType(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	tests := []struct {
		name     string
		typeName string
		expected bool
	}{
		{"existing type", "HAS_PORT", true},
		{"existing type 2", "RESOLVES_TO", true},
		{"non-existent type", "UNKNOWN_REL", false},
		{"empty type", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.IsCanonicalRelationType(tt.typeName)
			if got != tt.expected {
				t.Errorf("IsCanonicalRelationType(%q) = %v, want %v", tt.typeName, got, tt.expected)
			}
		})
	}
}

func TestTaxonomyAdapter_NodeTypeInfo(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	t.Run("existing node type", func(t *testing.T) {
		info := adapter.NodeTypeInfo("host")
		if info == nil {
			t.Fatal("expected non-nil info for 'host'")
		}
		if info.Type != "host" {
			t.Errorf("expected type 'host', got %q", info.Type)
		}
		if info.Name != "Host" {
			t.Errorf("expected name 'Host', got %q", info.Name)
		}
		if info.Category != "asset" {
			t.Errorf("expected category 'asset', got %q", info.Category)
		}
		if len(info.Properties) != 2 {
			t.Errorf("expected 2 properties, got %d", len(info.Properties))
		}
	})

	t.Run("non-existent node type", func(t *testing.T) {
		info := adapter.NodeTypeInfo("unknown")
		if info != nil {
			t.Error("expected nil info for unknown type")
		}
	})

	t.Run("returns copy", func(t *testing.T) {
		info1 := adapter.NodeTypeInfo("host")
		info2 := adapter.NodeTypeInfo("host")
		if info1 == info2 {
			t.Error("expected different pointers for each call (copy)")
		}
	})
}

func TestTaxonomyAdapter_RelationshipTypeInfo(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	t.Run("existing relationship type", func(t *testing.T) {
		info := adapter.RelationshipTypeInfo("HAS_PORT")
		if info == nil {
			t.Fatal("expected non-nil info for 'HAS_PORT'")
		}
		if info.Type != "HAS_PORT" {
			t.Errorf("expected type 'HAS_PORT', got %q", info.Type)
		}
		if len(info.FromTypes) != 1 || info.FromTypes[0] != "host" {
			t.Errorf("expected FromTypes=['host'], got %v", info.FromTypes)
		}
		if len(info.ToTypes) != 1 || info.ToTypes[0] != "port" {
			t.Errorf("expected ToTypes=['port'], got %v", info.ToTypes)
		}
	})

	t.Run("non-existent relationship type", func(t *testing.T) {
		info := adapter.RelationshipTypeInfo("UNKNOWN_REL")
		if info != nil {
			t.Error("expected nil info for unknown type")
		}
	})
}

func TestTaxonomyAdapter_TechniqueIDs_Filter(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	tests := []struct {
		name     string
		source   string
		expected int
	}{
		{"all techniques", "", 2},
		{"mitre only", "mitre", 1},
		{"arcanum only", "arcanum", 1},
		{"unknown source", "unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := adapter.TechniqueIDs(tt.source)
			if len(ids) != tt.expected {
				t.Errorf("TechniqueIDs(%q) returned %d, want %d", tt.source, len(ids), tt.expected)
			}
		})
	}
}

func TestTaxonomyAdapter_TechniqueInfo(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	t.Run("existing technique", func(t *testing.T) {
		info := adapter.TechniqueInfo("T1190")
		if info == nil {
			t.Fatal("expected non-nil info for 'T1190'")
		}
		if info.ID != "T1190" {
			t.Errorf("expected ID 'T1190', got %q", info.ID)
		}
		if info.Taxonomy != "mitre" {
			t.Errorf("expected taxonomy 'mitre', got %q", info.Taxonomy)
		}
	})

	t.Run("non-existent technique", func(t *testing.T) {
		info := adapter.TechniqueInfo("T9999")
		if info != nil {
			t.Error("expected nil info for unknown technique")
		}
	})
}

func TestTaxonomyAdapter_ValidateNodeType(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	// Valid type should return true
	if !adapter.ValidateNodeType("host") {
		t.Error("expected ValidateNodeType('host') to return true")
	}

	// Invalid type should return false (and log warning)
	if adapter.ValidateNodeType("unknown") {
		t.Error("expected ValidateNodeType('unknown') to return false")
	}
}

func TestTaxonomyAdapter_ValidateRelationType(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	// Valid type should return true
	if !adapter.ValidateRelationType("HAS_PORT") {
		t.Error("expected ValidateRelationType('HAS_PORT') to return true")
	}

	// Invalid type should return false (and log warning)
	if adapter.ValidateRelationType("UNKNOWN_REL") {
		t.Error("expected ValidateRelationType('UNKNOWN_REL') to return false")
	}
}

func TestTaxonomyAdapter_ToJSON(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	json := adapter.ToJSON()
	if json == nil {
		t.Fatal("expected non-nil JSON")
	}

	if version, ok := json["version"].(string); !ok || version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %v", json["version"])
	}

	if nodeTypes, ok := json["node_types"].([]string); !ok || len(nodeTypes) != 2 {
		t.Errorf("expected 2 node types in JSON, got %v", json["node_types"])
	}
}

func TestTaxonomyAdapter_ToJSONString(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	jsonStr := adapter.ToJSONString()
	if jsonStr == "" || jsonStr == "{}" {
		t.Error("expected non-empty JSON string")
	}
}

func TestTaxonomyAdapter_GetTargetType(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	t.Run("existing target type", func(t *testing.T) {
		info, ok := adapter.GetTargetType("http_api")
		if !ok {
			t.Fatal("expected to find target type 'http_api'")
		}
		if info.Name != "HTTP API" {
			t.Errorf("expected name 'HTTP API', got %q", info.Name)
		}
	})

	t.Run("non-existent target type", func(t *testing.T) {
		_, ok := adapter.GetTargetType("unknown")
		if ok {
			t.Error("expected not to find target type 'unknown'")
		}
	})
}

func TestTaxonomyAdapter_GetTechniqueType(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	t.Run("existing technique type", func(t *testing.T) {
		info, ok := adapter.GetTechniqueType("ssrf")
		if !ok {
			t.Fatal("expected to find technique type 'ssrf'")
		}
		if info.Name != "Server-Side Request Forgery" {
			t.Errorf("expected name 'Server-Side Request Forgery', got %q", info.Name)
		}
	})

	t.Run("non-existent technique type", func(t *testing.T) {
		_, ok := adapter.GetTechniqueType("unknown")
		if ok {
			t.Error("expected not to find technique type 'unknown'")
		}
	})
}

func TestTaxonomyAdapter_GetCapability(t *testing.T) {
	resp := createTestTaxonomyResponse()
	adapter := NewTaxonomyAdapter(resp)

	t.Run("existing capability", func(t *testing.T) {
		info, ok := adapter.GetCapability("capability.web_scanning")
		if !ok {
			t.Fatal("expected to find capability 'capability.web_scanning'")
		}
		if info.Name != "Web Scanning" {
			t.Errorf("expected name 'Web Scanning', got %q", info.Name)
		}
	})

	t.Run("non-existent capability", func(t *testing.T) {
		_, ok := adapter.GetCapability("unknown")
		if ok {
			t.Error("expected not to find capability 'unknown'")
		}
	})
}

// createTestTaxonomyResponse creates a test taxonomy response with sample data.
func createTestTaxonomyResponse() *harnesspb.GetTaxonomySchemaResponse {
	return &harnesspb.GetTaxonomySchemaResponse{
		Version: "1.2.3",
		NodeTypes: []*harnesspb.TaxonomyNodeType{
			{
				Id:                    "node.asset.host",
				Name:                  "Host",
				Type:                  "host",
				Category:              "asset",
				Description:           "A host system",
				IdentifyingProperties: []string{"ip"},
				Properties: []*harnesspb.TaxonomyProperty{
					{Name: "ip", Type: "string", Required: true, Description: "IP address"},
					{Name: "hostname", Type: "string", Required: false, Description: "Hostname"},
				},
			},
			{
				Id:                    "node.asset.domain",
				Name:                  "Domain",
				Type:                  "domain",
				Category:              "asset",
				Description:           "A domain name",
				IdentifyingProperties: []string{"name"},
				Properties: []*harnesspb.TaxonomyProperty{
					{Name: "name", Type: "string", Required: true, Description: "Domain name"},
				},
			},
		},
		RelationshipTypes: []*harnesspb.TaxonomyRelationshipType{
			{
				Id:            "rel.asset.has_port",
				Name:          "HAS_PORT",
				Type:          "HAS_PORT",
				Category:      "asset_hierarchy",
				Description:   "Host has open port",
				FromTypes:     []string{"host"},
				ToTypes:       []string{"port"},
				Bidirectional: false,
			},
			{
				Id:            "rel.asset.resolves_to",
				Name:          "RESOLVES_TO",
				Type:          "RESOLVES_TO",
				Category:      "asset_hierarchy",
				Description:   "Domain resolves to host",
				FromTypes:     []string{"domain"},
				ToTypes:       []string{"host"},
				Bidirectional: false,
			},
		},
		Techniques: []*harnesspb.TaxonomyTechnique{
			{
				TechniqueId: "T1190",
				Name:        "Exploit Public-Facing Application",
				Taxonomy:    "mitre",
				Category:    "initial_access",
				Description: "Adversaries exploit public-facing applications",
				Tactic:      "Initial Access",
			},
			{
				TechniqueId: "ARC-T001",
				Name:        "Direct Prompt Injection",
				Taxonomy:    "arcanum",
				Category:    "attack_technique",
				Description: "Injecting malicious prompts directly",
			},
		},
		TargetTypes: []*harnesspb.TaxonomyTargetType{
			{
				Id:             "target.web.http_api",
				Type:           "http_api",
				Name:           "HTTP API",
				Category:       "web",
				Description:    "HTTP API endpoint",
				RequiredFields: []string{"url"},
				OptionalFields: []string{"headers", "timeout"},
			},
		},
		TechniqueTypes: []*harnesspb.TaxonomyTechniqueType{
			{
				Id:              "technique.initial_access.ssrf",
				Type:            "ssrf",
				Name:            "Server-Side Request Forgery",
				Category:        "initial_access",
				Description:     "SSRF vulnerability testing",
				MitreIds:        []string{"T1190"},
				DefaultSeverity: "high",
			},
		},
		Capabilities: []*harnesspb.TaxonomyCapability{
			{
				Id:             "capability.web_scanning",
				Name:           "Web Scanning",
				Description:    "Web vulnerability scanning capability",
				TechniqueTypes: []string{"ssrf", "sqli", "xss"},
			},
		},
	}
}
