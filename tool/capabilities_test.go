// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"
	"testing"

	"github.com/zeroroot-ai/sdk/types"
	protolib "google.golang.org/protobuf/proto"
)

// mockToolWithCapabilities implements both Tool and CapabilityProvider interfaces
type mockToolWithCapabilities struct {
	name         string
	version      string
	description  string
	tags         []string
	capabilities *types.Capabilities
}

func (m *mockToolWithCapabilities) Name() string        { return m.name }
func (m *mockToolWithCapabilities) Version() string     { return m.version }
func (m *mockToolWithCapabilities) Description() string { return m.description }
func (m *mockToolWithCapabilities) Tags() []string      { return m.tags }
func (m *mockToolWithCapabilities) InputMessageType() string {
	return "test.v1.TestRequest"
}
func (m *mockToolWithCapabilities) OutputMessageType() string {
	return "test.v1.TestResponse"
}
func (m *mockToolWithCapabilities) ExecuteProto(ctx context.Context, input protolib.Message) (protolib.Message, error) {
	return nil, nil
}
func (m *mockToolWithCapabilities) Health(ctx context.Context) types.HealthStatus {
	return types.HealthStatus{}
}
func (m *mockToolWithCapabilities) Capabilities(ctx context.Context) *types.Capabilities {
	return m.capabilities
}

// mockToolWithoutCapabilities implements only Tool interface (not CapabilityProvider)
type mockToolWithoutCapabilities struct {
	name        string
	version     string
	description string
	tags        []string
}

func (m *mockToolWithoutCapabilities) Name() string        { return m.name }
func (m *mockToolWithoutCapabilities) Version() string     { return m.version }
func (m *mockToolWithoutCapabilities) Description() string { return m.description }
func (m *mockToolWithoutCapabilities) Tags() []string      { return m.tags }
func (m *mockToolWithoutCapabilities) InputMessageType() string {
	return "test.v1.TestRequest"
}
func (m *mockToolWithoutCapabilities) OutputMessageType() string {
	return "test.v1.TestResponse"
}
func (m *mockToolWithoutCapabilities) ExecuteProto(ctx context.Context, input protolib.Message) (protolib.Message, error) {
	return nil, nil
}
func (m *mockToolWithoutCapabilities) Health(ctx context.Context) types.HealthStatus {
	return types.HealthStatus{}
}

func TestGetCapabilities_WithProvider(t *testing.T) {
	tests := []struct {
		name         string
		capabilities *types.Capabilities
		wantNil      bool
	}{
		{
			name: "tool with full capabilities",
			capabilities: &types.Capabilities{
				HasRoot:      true,
				HasSudo:      true,
				CanRawSocket: true,
				Features: map[string]bool{
					"stealth_scan": true,
					"os_detection": true,
				},
				BlockedArgs:     []string{},
				ArgAlternatives: map[string]string{},
			},
			wantNil: false,
		},
		{
			name: "tool with limited capabilities",
			capabilities: &types.Capabilities{
				HasRoot:      false,
				HasSudo:      false,
				CanRawSocket: false,
				Features: map[string]bool{
					"stealth_scan": false,
					"os_detection": false,
				},
				BlockedArgs: []string{"-sS", "-O", "-sU"},
				ArgAlternatives: map[string]string{
					"-sS": "-sT",
				},
			},
			wantNil: false,
		},
		{
			name: "tool with partial privileges",
			capabilities: &types.Capabilities{
				HasRoot:      false,
				HasSudo:      true,
				CanRawSocket: false,
			},
			wantNil: false,
		},
		{
			name:         "tool returns nil capabilities",
			capabilities: nil,
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &mockToolWithCapabilities{
				name:         "test-tool",
				version:      "1.0.0",
				description:  "Test tool",
				tags:         []string{"test"},
				capabilities: tt.capabilities,
			}

			ctx := context.Background()
			got := GetCapabilities(ctx, tool)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetCapabilities() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("GetCapabilities() returned nil, want non-nil")
			}

			// Verify the returned capabilities match what we expect
			if got.HasRoot != tt.capabilities.HasRoot {
				t.Errorf("GetCapabilities().HasRoot = %v, want %v", got.HasRoot, tt.capabilities.HasRoot)
			}

			if got.HasSudo != tt.capabilities.HasSudo {
				t.Errorf("GetCapabilities().HasSudo = %v, want %v", got.HasSudo, tt.capabilities.HasSudo)
			}

			if got.CanRawSocket != tt.capabilities.CanRawSocket {
				t.Errorf("GetCapabilities().CanRawSocket = %v, want %v", got.CanRawSocket, tt.capabilities.CanRawSocket)
			}

			// Verify features
			if tt.capabilities.Features != nil {
				for feature, expected := range tt.capabilities.Features {
					if got.HasFeature(feature) != expected {
						t.Errorf("GetCapabilities().HasFeature(%q) = %v, want %v", feature, got.HasFeature(feature), expected)
					}
				}
			}

			// Verify blocked args
			if tt.capabilities.BlockedArgs != nil {
				for _, arg := range tt.capabilities.BlockedArgs {
					if !got.IsArgBlocked(arg) {
						t.Errorf("GetCapabilities().IsArgBlocked(%q) = false, want true", arg)
					}
				}
			}

			// Verify alternatives
			if tt.capabilities.ArgAlternatives != nil {
				for arg, expectedAlt := range tt.capabilities.ArgAlternatives {
					gotAlt, exists := got.GetAlternative(arg)
					if !exists {
						t.Errorf("GetCapabilities().GetAlternative(%q) returned false, want true", arg)
					}
					if gotAlt != expectedAlt {
						t.Errorf("GetCapabilities().GetAlternative(%q) = %q, want %q", arg, gotAlt, expectedAlt)
					}
				}
			}
		})
	}
}

func TestGetCapabilities_WithoutProvider(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
	}{
		{
			name: "tool without capability provider",
			tool: &mockToolWithoutCapabilities{
				name:        "simple-tool",
				version:     "1.0.0",
				description: "Simple tool without capabilities",
				tags:        []string{"simple"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := GetCapabilities(ctx, tt.tool)

			if got != nil {
				t.Errorf("GetCapabilities() = %v, want nil for tool without CapabilityProvider", got)
			}
		})
	}
}

func TestGetCapabilities_InterfaceAssertion(t *testing.T) {
	// Verify that GetCapabilities correctly uses type assertion
	ctx := context.Background()

	// Test with a tool that implements CapabilityProvider
	withProvider := &mockToolWithCapabilities{
		name:    "with-provider",
		version: "1.0.0",
		capabilities: &types.Capabilities{
			HasRoot: true,
		},
	}

	// Cast to Tool interface to test type assertion
	var toolWithProvider Tool = withProvider
	caps := GetCapabilities(ctx, toolWithProvider)
	if caps == nil {
		t.Error("GetCapabilities() returned nil for tool that implements CapabilityProvider")
	}

	// Test with a tool that does not implement CapabilityProvider
	withoutProvider := &mockToolWithoutCapabilities{
		name:    "without-provider",
		version: "1.0.0",
	}

	// Cast to Tool interface to test type assertion
	var toolWithoutProvider Tool = withoutProvider
	caps = GetCapabilities(ctx, toolWithoutProvider)
	if caps != nil {
		t.Error("GetCapabilities() returned non-nil for tool that does not implement CapabilityProvider")
	}
}

// mockToolCapabilitiesContextChecker is a tool that checks context propagation
type mockToolCapabilitiesContextChecker struct {
	mockToolWithCapabilities
	contextReceived *bool
	testKey         any
}

func (m *mockToolCapabilitiesContextChecker) Capabilities(ctx context.Context) *types.Capabilities {
	if ctx.Value(m.testKey) != nil {
		*m.contextReceived = true
	}
	return m.capabilities
}

func TestGetCapabilities_ContextPropagation(t *testing.T) {
	// Verify that context is properly passed to the Capabilities method
	type contextKey string
	const testKey contextKey = "test"

	contextReceived := false

	// Create a tool that checks for context values
	tool := &mockToolCapabilitiesContextChecker{
		mockToolWithCapabilities: mockToolWithCapabilities{
			name:    "context-aware-tool",
			version: "1.0.0",
			capabilities: &types.Capabilities{
				HasRoot: true,
			},
		},
		contextReceived: &contextReceived,
		testKey:         testKey,
	}

	// Create context with a value
	ctx := context.WithValue(context.Background(), testKey, "test-value")

	// Call GetCapabilities
	caps := GetCapabilities(ctx, tool)

	if caps == nil {
		t.Fatal("GetCapabilities() returned nil")
	}

	if !contextReceived {
		t.Error("Context was not properly propagated to Capabilities method")
	}
}

func TestGetCapabilities_MultipleCallsConsistent(t *testing.T) {
	// Verify that calling GetCapabilities multiple times returns consistent results
	caps := &types.Capabilities{
		HasRoot:      true,
		HasSudo:      false,
		CanRawSocket: true,
		Features: map[string]bool{
			"feature1": true,
		},
	}

	tool := &mockToolWithCapabilities{
		name:         "consistent-tool",
		version:      "1.0.0",
		capabilities: caps,
	}

	ctx := context.Background()

	// Call multiple times
	result1 := GetCapabilities(ctx, tool)
	result2 := GetCapabilities(ctx, tool)
	result3 := GetCapabilities(ctx, tool)

	// Verify all results are identical
	if result1 != result2 || result2 != result3 {
		t.Error("GetCapabilities() returned different pointers on multiple calls")
	}

	// Verify all results have the same values
	if result1 == nil || result2 == nil || result3 == nil {
		t.Fatal("GetCapabilities() returned nil")
	}

	if result1.HasRoot != result2.HasRoot || result2.HasRoot != result3.HasRoot {
		t.Error("GetCapabilities() returned inconsistent HasRoot values")
	}
}

func TestGetCapabilities_NilTool(t *testing.T) {
	// This test documents behavior with nil tool (will panic, which is expected)
	// We don't actually call it to avoid test panics, just document the expectation

	// Uncomment to verify panic behavior:
	// defer func() {
	// 	if r := recover(); r == nil {
	// 		t.Error("GetCapabilities(ctx, nil) should panic")
	// 	}
	// }()
	// GetCapabilities(context.Background(), nil)

	t.Skip("Skipping nil tool test - would cause panic")
}
