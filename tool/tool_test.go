// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"
	"errors"
	"testing"

	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/zeroroot-ai/sdk/types"
)

// mockTool is a test implementation of the Tool interface.
type mockTool struct {
	name              string
	version           string
	description       string
	tags              []string
	inputMessageType  string
	outputMessageType string
	executeProtoFunc  func(ctx context.Context, input protolib.Message) (protolib.Message, error)
	healthFunc        func(ctx context.Context) types.HealthStatus
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Version() string {
	return m.version
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Tags() []string {
	return m.tags
}

func (m *mockTool) InputMessageType() string {
	if m.inputMessageType != "" {
		return m.inputMessageType
	}
	return "google.protobuf.Struct"
}

func (m *mockTool) OutputMessageType() string {
	if m.outputMessageType != "" {
		return m.outputMessageType
	}
	return "google.protobuf.Struct"
}

func (m *mockTool) ExecuteProto(ctx context.Context, input protolib.Message) (protolib.Message, error) {
	if m.executeProtoFunc != nil {
		return m.executeProtoFunc(ctx, input)
	}
	return nil, errors.New("proto execution not implemented")
}

func (m *mockTool) Health(ctx context.Context) types.HealthStatus {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return types.NewHealthyStatus("mock tool healthy")
}

func TestMockTool_Name(t *testing.T) {
	tool := &mockTool{
		name: "test-tool",
	}

	if got := tool.Name(); got != "test-tool" {
		t.Errorf("Name() = %v, want %v", got, "test-tool")
	}
}

func TestMockTool_Version(t *testing.T) {
	tool := &mockTool{
		version: "1.0.0",
	}

	if got := tool.Version(); got != "1.0.0" {
		t.Errorf("Version() = %v, want %v", got, "1.0.0")
	}
}

func TestMockTool_Description(t *testing.T) {
	tool := &mockTool{
		description: "A test tool",
	}

	if got := tool.Description(); got != "A test tool" {
		t.Errorf("Description() = %v, want %v", got, "A test tool")
	}
}

func TestMockTool_Tags(t *testing.T) {
	want := []string{"test", "mock"}
	tool := &mockTool{
		tags: want,
	}

	got := tool.Tags()
	if len(got) != len(want) {
		t.Fatalf("Tags() length = %v, want %v", len(got), len(want))
	}

	for i, tag := range got {
		if tag != want[i] {
			t.Errorf("Tags()[%d] = %v, want %v", i, tag, want[i])
		}
	}
}

func TestMockTool_InputMessageType(t *testing.T) {
	tool := &mockTool{
		inputMessageType: "test.v1.TestRequest",
	}

	got := tool.InputMessageType()
	if got != "test.v1.TestRequest" {
		t.Errorf("InputMessageType() = %v, want %v", got, "test.v1.TestRequest")
	}
}

func TestMockTool_OutputMessageType(t *testing.T) {
	tool := &mockTool{
		outputMessageType: "test.v1.TestResponse",
	}

	got := tool.OutputMessageType()
	if got != "test.v1.TestResponse" {
		t.Errorf("OutputMessageType() = %v, want %v", got, "test.v1.TestResponse")
	}
}

func TestMockTool_ExecuteProto(t *testing.T) {
	tests := []struct {
		name             string
		executeProtoFunc func(ctx context.Context, input protolib.Message) (protolib.Message, error)
		input            protolib.Message
		wantErr          bool
		checkOutput      func(t *testing.T, output protolib.Message)
	}{
		{
			name: "successful execution",
			executeProtoFunc: func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
				result, _ := structpb.NewStruct(map[string]any{"result": "success"})
				return result, nil
			},
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"message": structpb.NewStringValue("hello"),
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output protolib.Message) {
				st := output.(*structpb.Struct)
				if st.Fields["result"].GetStringValue() != "success" {
					t.Errorf("ExecuteProto() result = %v, want success", st.Fields["result"].GetStringValue())
				}
			},
		},
		{
			name: "execution with error",
			executeProtoFunc: func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
				return nil, errors.New("execution failed")
			},
			input:   &structpb.Struct{},
			wantErr: true,
		},
		{
			name:             "no execute function",
			executeProtoFunc: nil,
			input:            &structpb.Struct{},
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &mockTool{
				executeProtoFunc: tt.executeProtoFunc,
			}

			got, err := tool.ExecuteProto(context.Background(), tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteProto() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkOutput != nil {
				tt.checkOutput(t, got)
			}
		})
	}
}

func TestMockTool_ExecuteProto_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tool := &mockTool{
		executeProtoFunc: func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				result, _ := structpb.NewStruct(map[string]any{"result": "success"})
				return result, nil
			}
		},
	}

	_, err := tool.ExecuteProto(ctx, &structpb.Struct{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ExecuteProto() with canceled context error = %v, want %v", err, context.Canceled)
	}
}

func TestMockTool_Health(t *testing.T) {
	tests := []struct {
		name       string
		healthFunc func(ctx context.Context) types.HealthStatus
		wantStatus string
	}{
		{
			name: "healthy status",
			healthFunc: func(ctx context.Context) types.HealthStatus {
				return types.NewHealthyStatus("all systems operational")
			},
			wantStatus: types.StatusHealthy,
		},
		{
			name: "degraded status",
			healthFunc: func(ctx context.Context) types.HealthStatus {
				return types.NewDegradedStatus("some issues", nil)
			},
			wantStatus: types.StatusDegraded,
		},
		{
			name: "unhealthy status",
			healthFunc: func(ctx context.Context) types.HealthStatus {
				return types.NewUnhealthyStatus("critical failure", nil)
			},
			wantStatus: types.StatusUnhealthy,
		},
		{
			name:       "default healthy",
			healthFunc: nil,
			wantStatus: types.StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &mockTool{
				healthFunc: tt.healthFunc,
			}

			status := tool.Health(context.Background())
			if status.Status != tt.wantStatus {
				t.Errorf("Health() status = %v, want %v", status.Status, tt.wantStatus)
			}
		})
	}
}

func TestMockTool_InterfaceCompliance(t *testing.T) {
	var _ Tool = (*mockTool)(nil)
}
