// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

func TestExtractDiscovery_WithDiscoveryField(t *testing.T) {
	// Create a DiscoveryResult
	discovery := &graphragpb.DiscoveryResult{
		Hosts: []*graphragpb.Host{
			{Ip: "192.168.1.1", Hostname: stringPtr("host1")},
		},
	}

	// Test extraction from a proto that has DiscoveryResult directly
	extracted := ExtractDiscovery(discovery)
	if extracted == nil {
		t.Fatal("expected non-nil result")
	}

	if len(extracted.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(extracted.Hosts))
	}

	if extracted.Hosts[0].Ip != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", extracted.Hosts[0].Ip)
	}
}

func TestExtractDiscovery_NilMessage(t *testing.T) {
	result := ExtractDiscovery(nil)
	if result != nil {
		t.Errorf("expected nil for nil message, got %v", result)
	}
}

func TestExtractDiscovery_EmptyDiscovery(t *testing.T) {
	// Create an empty DiscoveryResult
	discovery := &graphragpb.DiscoveryResult{}

	extracted := ExtractDiscovery(discovery)
	if extracted == nil {
		t.Fatal("expected non-nil result for empty discovery")
	}

	if len(extracted.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(extracted.Hosts))
	}
}

func TestExtractDiscovery_WrongMessageType(t *testing.T) {
	// Test with a different proto message type (Any)
	anyMsg := &anypb.Any{}

	result := ExtractDiscovery(anyMsg)
	if result != nil {
		t.Errorf("expected nil for non-discovery message, got %v", result)
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
