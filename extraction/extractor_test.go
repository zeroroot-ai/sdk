// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package extraction

import (
	"context"

	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

// mockExtractor is a test EntityExtractor.
type mockExtractor struct {
	name       string
	canExtract bool
	result     *graphragpb.DiscoveryResult
	err        error
}

func (m *mockExtractor) ToolName() string              { return m.name }
func (m *mockExtractor) CanExtract(proto.Message) bool { return m.canExtract }
func (m *mockExtractor) Extract(_ context.Context, _ proto.Message) (*graphragpb.DiscoveryResult, error) {
	return m.result, m.err
}
