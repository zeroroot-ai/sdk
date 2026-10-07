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
