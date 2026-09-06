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

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg == nil {
		t.Fatal("NewConfig() returned nil")
	}

	if cfg.version != "1.0.0" {
		t.Errorf("NewConfig() default version = %v, want %v", cfg.version, "1.0.0")
	}

	if cfg.tags == nil {
		t.Error("NewConfig() tags should not be nil")
	}

	if len(cfg.tags) != 0 {
		t.Errorf("NewConfig() tags length = %v, want 0", len(cfg.tags))
	}
}

func TestConfig_Setters(t *testing.T) {
	cfg := NewConfig()

	// Test SetName
	cfg.SetName("test-tool")
	if cfg.name != "test-tool" {
		t.Errorf("SetName() name = %v, want %v", cfg.name, "test-tool")
	}

	// Test SetVersion
	cfg.SetVersion("2.0.0")
	if cfg.version != "2.0.0" {
		t.Errorf("SetVersion() version = %v, want %v", cfg.version, "2.0.0")
	}

	// Test SetDescription
	cfg.SetDescription("A test tool")
	if cfg.description != "A test tool" {
		t.Errorf("SetDescription() description = %v, want %v", cfg.description, "A test tool")
	}

	// Test SetTags
	tags := []string{"test", "mock"}
	cfg.SetTags(tags)
	if len(cfg.tags) != len(tags) {
		t.Errorf("SetTags() tags length = %v, want %v", len(cfg.tags), len(tags))
	}

	// Test SetInputMessageType
	cfg.SetInputMessageType("test.v1.TestRequest")
	if cfg.inputMessageType != "test.v1.TestRequest" {
		t.Errorf("SetInputMessageType() type = %v, want %v", cfg.inputMessageType, "test.v1.TestRequest")
	}

	// Test SetOutputMessageType
	cfg.SetOutputMessageType("test.v1.TestResponse")
	if cfg.outputMessageType != "test.v1.TestResponse" {
		t.Errorf("SetOutputMessageType() type = %v, want %v", cfg.outputMessageType, "test.v1.TestResponse")
	}

	// Test SetExecuteProtoFunc
	executeProtoFunc := func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
		result, _ := structpb.NewStruct(map[string]any{"result": "success"})
		return result, nil
	}
	cfg.SetExecuteProtoFunc(executeProtoFunc)
	if cfg.executeProtoFunc == nil {
		t.Error("SetExecuteProtoFunc() executeProtoFunc should not be nil")
	}
}

func TestConfig_MethodChaining(t *testing.T) {
	cfg := NewConfig().
		SetName("chained-tool").
		SetVersion("1.2.3").
		SetDescription("Chained configuration").
		SetTags([]string{"test"})

	if cfg.name != "chained-tool" {
		t.Errorf("method chaining name = %v, want %v", cfg.name, "chained-tool")
	}

	if cfg.version != "1.2.3" {
		t.Errorf("method chaining version = %v, want %v", cfg.version, "1.2.3")
	}

	if cfg.description != "Chained configuration" {
		t.Errorf("method chaining description = %v, want %v", cfg.description, "Chained configuration")
	}

	if len(cfg.tags) != 1 || cfg.tags[0] != "test" {
		t.Errorf("method chaining tags = %v, want [test]", cfg.tags)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "config cannot be nil",
		},
		{
			name:    "missing name",
			config:  NewConfig().SetExecuteProtoFunc(func(ctx context.Context, input protolib.Message) (protolib.Message, error) { return nil, nil }),
			wantErr: true,
			errMsg:  "tool name is required",
		},
		{
			name:    "valid config with name only",
			config:  NewConfig().SetName("test-tool"),
			wantErr: false,
		},
		{
			name: "valid config with proto execution",
			config: NewConfig().
				SetName("valid-tool").
				SetInputMessageType("google.protobuf.Struct").
				SetOutputMessageType("google.protobuf.Struct").
				SetExecuteProtoFunc(func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
					result, _ := structpb.NewStruct(map[string]any{"result": "ok"})
					return result, nil
				}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := New(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("New() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("New() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("New() unexpected error = %v", err)
				return
			}

			if tool == nil {
				t.Error("New() returned nil tool")
			}
		})
	}
}

func TestSdkTool_Name(t *testing.T) {
	cfg := NewConfig().SetName("test-tool")

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := tool.Name(); got != "test-tool" {
		t.Errorf("Name() = %v, want %v", got, "test-tool")
	}
}

func TestSdkTool_Version(t *testing.T) {
	cfg := NewConfig().
		SetName("test-tool").
		SetVersion("2.5.1")

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := tool.Version(); got != "2.5.1" {
		t.Errorf("Version() = %v, want %v", got, "2.5.1")
	}
}

func TestSdkTool_Description(t *testing.T) {
	cfg := NewConfig().
		SetName("test-tool").
		SetDescription("Test description")

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := tool.Description(); got != "Test description" {
		t.Errorf("Description() = %v, want %v", got, "Test description")
	}
}

func TestSdkTool_Tags(t *testing.T) {
	tags := []string{"test", "mock", "utility"}
	cfg := NewConfig().
		SetName("test-tool").
		SetTags(tags)

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := tool.Tags()
	if len(got) != len(tags) {
		t.Fatalf("Tags() length = %v, want %v", len(got), len(tags))
	}

	for i, tag := range got {
		if tag != tags[i] {
			t.Errorf("Tags()[%d] = %v, want %v", i, tag, tags[i])
		}
	}
}

func TestSdkTool_InputMessageType(t *testing.T) {
	cfg := NewConfig().
		SetName("test-tool").
		SetInputMessageType("test.v1.TestRequest")

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := tool.InputMessageType()
	if got != "test.v1.TestRequest" {
		t.Errorf("InputMessageType() = %v, want %v", got, "test.v1.TestRequest")
	}
}

func TestSdkTool_OutputMessageType(t *testing.T) {
	cfg := NewConfig().
		SetName("test-tool").
		SetOutputMessageType("test.v1.TestResponse")

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := tool.OutputMessageType()
	if got != "test.v1.TestResponse" {
		t.Errorf("OutputMessageType() = %v, want %v", got, "test.v1.TestResponse")
	}
}

func TestSdkTool_ExecuteProto(t *testing.T) {
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
				st := input.(*structpb.Struct)
				value := st.Fields["value"].GetNumberValue()
				result, _ := structpb.NewStruct(map[string]any{"doubled": value * 2})
				return result, nil
			},
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"value": structpb.NewNumberValue(5),
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output protolib.Message) {
				st := output.(*structpb.Struct)
				if st.Fields["doubled"].GetNumberValue() != 10 {
					t.Errorf("ExecuteProto() doubled = %v, want 10", st.Fields["doubled"].GetNumberValue())
				}
			},
		},
		{
			name: "execution error",
			executeProtoFunc: func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
				return nil, errors.New("execution failed")
			},
			input:   &structpb.Struct{},
			wantErr: true,
		},
		{
			name:             "no execution function configured",
			executeProtoFunc: nil,
			input:            &structpb.Struct{},
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig().
				SetName("test-tool").
				SetInputMessageType("google.protobuf.Struct").
				SetOutputMessageType("google.protobuf.Struct").
				SetExecuteProtoFunc(tt.executeProtoFunc)

			tool, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			got, err := tool.ExecuteProto(context.Background(), tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExecuteProto() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteProto() unexpected error = %v", err)
				return
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, got)
			}
		})
	}
}

func TestSdkTool_ExecuteProto_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := NewConfig().
		SetName("test-tool").
		SetExecuteProtoFunc(func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				result, _ := structpb.NewStruct(map[string]any{"result": "ok"})
				return result, nil
			}
		})

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cancel() // Cancel context before execution

	_, err = tool.ExecuteProto(ctx, &structpb.Struct{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ExecuteProto() with canceled context error = %v, want %v", err, context.Canceled)
	}
}

func TestSdkTool_Health(t *testing.T) {
	cfg := NewConfig().SetName("test-tool")

	tool, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	status := tool.Health(context.Background())

	if status.Status != types.StatusHealthy {
		t.Errorf("Health() status = %v, want %v", status.Status, types.StatusHealthy)
	}

	if status.Message != "tool is operational" {
		t.Errorf("Health() message = %v, want %v", status.Message, "tool is operational")
	}
}

func TestSdkTool_InterfaceCompliance(t *testing.T) {
	var _ Tool = (*sdkTool)(nil)
}
