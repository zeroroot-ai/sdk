// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package extraction

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewExtractorRegistry(t *testing.T) {
	r := NewExtractorRegistry()
	assert.NotNil(t, r)
	assert.Empty(t, r.ListTools())
}

func TestRegistry_Register(t *testing.T) {
	r := NewExtractorRegistry()

	err := r.Register(&mockExtractor{name: "mytool", canExtract: true})
	require.NoError(t, err)
	assert.True(t, r.Has("mytool"))
}

func TestRegistry_Register_Nil(t *testing.T) {
	r := NewExtractorRegistry()
	err := r.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestRegistry_Register_EmptyName(t *testing.T) {
	r := NewExtractorRegistry()
	err := r.Register(&mockExtractor{name: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewExtractorRegistry()
	require.NoError(t, r.Register(&mockExtractor{name: "mytool"}))

	err := r.Register(&mockExtractor{name: "mytool"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewExtractorRegistry()
	require.NoError(t, r.Register(&mockExtractor{name: "mytool"}))

	err := r.Unregister("mytool")
	require.NoError(t, err)
	assert.False(t, r.Has("mytool"))
}

func TestRegistry_Unregister_NotFound(t *testing.T) {
	r := NewExtractorRegistry()
	err := r.Unregister("nonexistent")
	assert.Error(t, err)
}

func TestRegistry_Get(t *testing.T) {
	r := NewExtractorRegistry()
	ext := &mockExtractor{name: "mytool"}
	require.NoError(t, r.Register(ext))

	got, err := r.Get("mytool")
	require.NoError(t, err)
	assert.Equal(t, ext, got)
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewExtractorRegistry()
	_, err := r.Get("nonexistent")
	assert.Error(t, err)
}

func TestRegistry_Has(t *testing.T) {
	r := NewExtractorRegistry()
	assert.False(t, r.Has("mytool"))

	require.NoError(t, r.Register(&mockExtractor{name: "mytool"}))
	assert.True(t, r.Has("mytool"))
}

func TestRegistry_ListTools(t *testing.T) {
	r := NewExtractorRegistry()
	require.NoError(t, r.Register(&mockExtractor{name: "mytool"}))
	require.NoError(t, r.Register(&mockExtractor{name: "mytool-b"}))

	tools := r.ListTools()
	assert.Len(t, tools, 2)
	assert.ElementsMatch(t, []string{"mytool", "mytool-b"}, tools)
}

func TestRegistry_ExtractFromResponse_Success(t *testing.T) {
	r := NewExtractorRegistry()
	discovery := &graphragpb.DiscoveryResult{
		Hosts: []*graphragpb.Host{{Ip: "10.0.0.1"}},
	}
	require.NoError(t, r.Register(&mockExtractor{
		name:       "mytool",
		canExtract: true,
		result:     discovery,
	}))

	result, err := r.ExtractFromResponse(context.Background(), "mytool", nil)
	require.NoError(t, err)
	assert.Equal(t, discovery, result)
}

func TestRegistry_ExtractFromResponse_NotRegistered(t *testing.T) {
	r := NewExtractorRegistry()
	_, err := r.ExtractFromResponse(context.Background(), "mytool", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no extractor registered")
}

func TestRegistry_ExtractFromResponse_CannotExtract(t *testing.T) {
	r := NewExtractorRegistry()
	require.NoError(t, r.Register(&mockExtractor{
		name:       "mytool",
		canExtract: false,
	}))

	_, err := r.ExtractFromResponse(context.Background(), "mytool", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot process")
}

func TestRegistry_ExtractFromResponse_ExtractError(t *testing.T) {
	r := NewExtractorRegistry()
	require.NoError(t, r.Register(&mockExtractor{
		name:       "mytool",
		canExtract: true,
		err:        errors.New("parse failed"),
	}))

	_, err := r.ExtractFromResponse(context.Background(), "mytool", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extraction failed")
	assert.Contains(t, err.Error(), "parse failed")
}
