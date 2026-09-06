// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	toolpb "github.com/zeroroot-ai/sdk/api/gen/gibson/tool/v1"
	"github.com/zeroroot-ai/sdk/enum"
	"github.com/zeroroot-ai/sdk/startup"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// SchemaFlag is the command-line flag used to request schema output.
const SchemaFlag = "--schema"

// Tool serves a tool implementation.
//
// If --schema flag is passed, outputs tool schema to stdout and exits.
// Otherwise, performs the Capability Grant bootstrap (discover → register), connects
// to the Gibson platform in pull mode, and polls for work.
//
// WithCapabilityGrant or WithCapabilityGrantFromEnv must be provided; Tool returns an error
// if PlatformURL is not configured.
//
// Example:
//
//	err := serve.Tool(myTool, serve.WithCapabilityGrantFromEnv())
//	if err != nil {
//	    log.Fatal(err)
//	}
func Tool(t tool.Tool, opts ...Option) error {
	// Check for --schema flag
	if hasSchemaFlag() {
		return OutputSchema(t)
	}

	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// System binary validation: validate before any network connections
	if !cfg.SkipBinaryCheck {
		logger := slog.Default().With("component", "tool", "name", t.Name())

		manifestPath := discoverComponentYAML()
		if manifestPath == "" {
			logger.Warn("component.yaml not found -- skipping binary checks",
				"searched", []string{"./component.yaml", "/etc/gibson/component.yaml"},
			)
		} else {
			result, err := startup.ValidateSystemDependencies(logger, manifestPath)
			if err != nil {
				return fmt.Errorf("system dependency check failed for tool %q: %w", t.Name(), err)
			}
			// Store degraded state for health endpoint reporting
			if result != nil && len(result.Degraded) > 0 {
				setDegradedDependencies(result.Degraded)
			}
		}
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}
	slog.Info("connecting to platform", "name", t.Name(), "platform_url", cfg.PlatformURL)
	return servePlatformTool(t, cfg)
}

// discoverComponentYAML looks for component.yaml in standard locations.
// Returns the first path found, or empty string if not found.
func discoverComponentYAML() string {
	candidates := []string{
		"./component.yaml",
		"/etc/gibson/component.yaml",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

var (
	degradedMu   sync.RWMutex
	degradedDeps []startup.MissingBinary
)

// setDegradedDependencies stores optional binaries that were missing at startup.
func setDegradedDependencies(deps []startup.MissingBinary) {
	degradedMu.Lock()
	defer degradedMu.Unlock()
	degradedDeps = deps
}

// GetDegradedDependencies returns optional binaries that were missing at startup.
func GetDegradedDependencies() []startup.MissingBinary {
	degradedMu.RLock()
	defer degradedMu.RUnlock()
	return degradedDeps
}

// CheckBinaryAvailable checks if a binary is in the degraded (missing optional) list.
// Returns an error if the binary is missing, nil if it is available.
// Tools should call this before attempting to use an optional binary capability.
func CheckBinaryAvailable(binaryName string) error {
	degradedMu.RLock()
	defer degradedMu.RUnlock()
	for _, dep := range degradedDeps {
		if dep.Name == binaryName {
			return fmt.Errorf("capability requires binary '%s' which is not available", binaryName)
		}
	}
	return nil
}

// DegradedDependenciesCheck returns a health check function suitable for
// registration with the HTTP health server's readiness endpoint.
//
// When optional binaries are missing, the check returns a degraded status
// with the list of missing binary names. When all binaries are present,
// it returns healthy. The HTTP status code remains 200 in both cases
// since the tool is still ready to handle requests for available capabilities.
//
// Example:
//
//	healthServer.RegisterReadinessCheck("system-dependencies", serve.DegradedDependenciesCheck())
func DegradedDependenciesCheck() func(ctx context.Context) types.HealthStatus {
	return func(_ context.Context) types.HealthStatus {
		deps := GetDegradedDependencies()
		if len(deps) == 0 {
			return types.NewHealthyStatus("all system dependencies available")
		}

		var names []string
		for _, dep := range deps {
			names = append(names, dep.Name)
		}

		return types.NewDegradedStatus(
			"some optional capabilities are unavailable",
			map[string]any{
				"missing_optional": names,
			},
		)
	}
}

// hasSchemaFlag checks if --schema was passed as a command-line argument.
func hasSchemaFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == SchemaFlag {
			return true
		}
	}
	return false
}

// toolServiceServer implements the gRPC ToolService for an SDK tool.
// It bridges the gRPC protocol to the tool.Tool interface.
type toolServiceServer struct {
	toolpb.UnimplementedToolServiceServer
	tool      tool.Tool
	extractor EntityExtractor
}

// GetDescriptor returns the tool's descriptor including name, version, description, and tags.
//
// The input_schema and output_schema fields in ToolDescriptor are intentionally left empty.
// These JSON Schema fields remain in tool.proto for wire compatibility with older clients but
// carry no semantic value for tools built with this SDK. All type information is conveyed
// through the fully-qualified proto message type names exposed by tool.InputMessageType() and
// tool.OutputMessageType(), which the platform receives via RegisterComponent's
// input_message_type and output_message_type fields. Once task 1.3 adds those fields
// directly to ToolDescriptor, clients can read them from the descriptor response as well.
func (s *toolServiceServer) GetDescriptor(ctx context.Context, req *toolpb.GetDescriptorRequest) (*toolpb.GetDescriptorResponse, error) {
	return &toolpb.GetDescriptorResponse{
		Name:        s.tool.Name(),
		Description: s.tool.Description(),
		Version:     s.tool.Version(),
		Tags:        s.tool.Tags(),
		// InputSchema/OutputSchema intentionally not populated: tools use proto message types,
		// not JSON schemas. See comment on GetDescriptor for details.
	}, nil
}

// Execute runs the tool with the provided input.
// The input is serialized as JSON in the request and the output is
// serialized as JSON in the response.
func (s *toolServiceServer) Execute(ctx context.Context, req *toolpb.ExecuteRequest) (*toolpb.ExecuteResponse, error) {
	// Apply timeout if specified
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// Get the tool's input message type
	inputTypeName := s.tool.InputMessageType()
	if inputTypeName == "" {
		return nil, status.Errorf(codes.Unimplemented, "tool does not specify InputMessageType")
	}

	// Find the proto message type in the global registry
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(inputTypeName))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find message type %q: %v", inputTypeName, err)
	}

	// Create a new instance of the proto message
	protoReq := messageType.New().Interface()

	// Apply enum normalization using the centralized enum.Normalize function
	normalizedJSON := enum.Normalize(s.tool.Name(), req.InputJson)

	// Unmarshal JSON input into the proto message with lenient settings
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Ignore unknown fields
	}
	if err := unmarshaler.Unmarshal([]byte(normalizedJSON), protoReq); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid input JSON for type %s: %v", inputTypeName, err)
	}

	// Execute the tool using ExecuteProto
	protoResp, err := s.tool.ExecuteProto(ctx, protoReq)

	// Build response
	resp := &toolpb.ExecuteResponse{}

	// Handle execution result
	if err == nil {
		// Run extraction if configured — populate field 100 before serializing.
		if s.extractor != nil && protoResp != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("extractor panicked, continuing without discovery",
							"tool", s.tool.Name(), "panic", r)
					}
				}()
				if discovery, extractErr := s.extractor.Extract(ctx, protoResp); extractErr != nil {
					slog.Warn("extraction failed, continuing without discovery",
						"tool", s.tool.Name(), "error", extractErr)
				} else if discovery != nil {
					setDiscoveryField(protoResp, discovery)
				}
			}()
		}

		// Marshal proto response to JSON
		outputJSON, jsonErr := protojson.Marshal(protoResp)
		if jsonErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to marshal output: %v", jsonErr)
		}
		resp.OutputJson = string(outputJSON)
	} else {
		// Map error to proto error
		resp.Error = &commonpb.Error{
			Code:      "EXECUTION_ERROR",
			Message:   err.Error(),
			Retryable: false,
		}
	}

	return resp, nil
}

// Health returns the current health status of the tool.
func (s *toolServiceServer) Health(ctx context.Context, req *toolpb.HealthRequest) (*toolpb.HealthResponse, error) {
	health := s.tool.Health(ctx)

	return &toolpb.HealthResponse{
		Status: &commonpb.HealthStatus{
			Status:    health.Status,
			Message:   health.Message,
			CheckedAt: time.Now().UnixMilli(),
		},
	}, nil
}
