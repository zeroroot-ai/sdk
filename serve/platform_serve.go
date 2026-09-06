// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	harnessconst "github.com/zeroroot-ai/sdk/harness"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// ============================================================================
// SPIFFE transport helpers
// ============================================================================

// dialSPIFFE opens an X509Source from the SPIRE Workload API and dials the
// daemon at daemonAddress using mTLS. extraOpts are appended after the mTLS
// transport credential option, allowing callers to inject per-RPC credentials
// (e.g. Capability Grant JWTs) alongside the transport. The returned connection
// and source are owned by the caller; source.Close() must be called when done.
func dialSPIFFE(ctx context.Context, socketPath, daemonAddress string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, *workloadapi.X509Source, error) {
	source, err := workloadapi.NewX509Source(ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr("unix://"+socketPath),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to SPIRE workload API at %s: %w", socketPath, err)
	}

	tlsCreds := grpccredentials.MTLSClientCredentials(source, source, tlsconfig.AuthorizeAny())

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}, extraOpts...)

	conn, err := grpc.NewClient(daemonAddress, opts...)
	if err != nil {
		source.Close()
		return nil, nil, fmt.Errorf("dial daemon at %s: %w", daemonAddress, err)
	}

	return conn, source, nil
}

// ============================================================================
// Agent platform mode
// ============================================================================

// servePlatformAgent connects an agent to the Gibson platform in pull mode.
// The agent registers itself outbound, polls for work, executes tasks locally,
// and submits results. No local gRPC server is started.
//
// Capability Grant bootstrap always runs first (ADR-0036). When
// useSPIFFETransport(cfg) is true the gRPC dial uses SPIFFE mTLS transport
// with the CG JWT still attached as the per-RPC credential. Otherwise standard
// CG-authenticated TLS is used.
func servePlatformAgent(a agent.Agent, cfg *Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start the HTTP health server early so /healthz responds immediately,
	// even before registration completes. /readyz will return 503 until
	// registration succeeds.
	_, healthState, stopHealth := startComponentHealthServer(cfg, "agent", a.Name())
	defer stopHealth()

	// Capability Grant bootstrap — always required (ADR-0036).
	client, err := NewPlatformClientWithOptions(cfg.PlatformURL, "",
		WithPlatformPollInterval(cfg.PollInterval),
		WithPlatformHeartbeatInterval(cfg.HeartbeatInterval),
	)
	if err != nil {
		return fmt.Errorf("create platform client: %w", err)
	}
	client.bootstrapToken = cfg.BootstrapToken
	client.hostKeyPath = cfg.HostKeyPath

	slog.Info("bootstrapping capability grant", "component", "agent", "name", a.Name(), "platform_url", cfg.PlatformURL)
	if err := client.Authenticate(ctx, a.Name(), "autonomous"); err != nil {
		return fmt.Errorf("capability grant bootstrap: %w", err)
	}

	// Connect: SPIFFE mTLS transport upgrade when socket is present; CG TLS otherwise.
	if useSPIFFETransport(cfg) {
		slog.Info("upgrading to SPIFFE mTLS transport",
			"component", "agent",
			"name", a.Name(),
			"daemon_address", cfg.DaemonAddress,
			"spiffe_socket", cfg.SPIFFEEndpointSocket,
		)
		cgPerRPC := client.capabilityGrantClient.GRPCPerRPCCredentials()
		conn, source, err := dialSPIFFE(ctx, cfg.SPIFFEEndpointSocket, cfg.DaemonAddress, grpc.WithPerRPCCredentials(cgPerRPC))
		if err != nil {
			return fmt.Errorf("SPIFFE dial: %w", err)
		}
		defer source.Close()
		// Wire the SPIFFE conn into the PlatformClient; CG per-RPC creds travel on every call.
		client.conn = conn
		client.service = componentpb.NewComponentServiceClient(conn)
	} else {
		slog.Info("connecting to platform", "component", "agent", "name", a.Name(), "platform_url", cfg.PlatformURL)
		if err := client.connectWithBackoff(ctx); err != nil {
			return fmt.Errorf("connect to platform: %w", err)
		}
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			slog.Warn("error closing platform client", "component", "agent", "name", a.Name(), "error", cerr)
		}
	}()

	// Register as an agent.
	capabilities := a.Capabilities()
	targetTypes := a.TargetTypes()
	techniqueTypes := a.TechniqueTypes()

	info := ComponentInfo{
		Kind:    "agent",
		Name:    a.Name(),
		Version: a.Version(),
		Metadata: map[string]string{
			"description":     a.Description(),
			"capabilities":    strings.Join(capabilities, ","),
			"target_types":    strings.Join(targetTypes, ","),
			"technique_types": strings.Join(techniqueTypes, ","),
		},
		Capabilities:      capabilities,
		OntologyExtension: ontologyExtensionFrom(a),
	}

	reg, err := client.Register(ctx, info)
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	pollInterval := cfg.PollInterval
	heartbeatInterval := cfg.HeartbeatInterval
	if reg != nil {
		if reg.PollIntervalMs > 0 {
			pollInterval = time.Duration(reg.PollIntervalMs) * time.Millisecond
		}
		if reg.HeartbeatIntervalMs > 0 {
			heartbeatInterval = time.Duration(reg.HeartbeatIntervalMs) * time.Millisecond
		}
	}

	slog.Info("agent registered",
		"name", a.Name(),
		"instance_id", client.InstanceID(),
		"poll_interval", pollInterval,
		"heartbeat_interval", heartbeatInterval,
	)

	// Flip /readyz to healthy now that registration succeeded.
	if healthState != nil {
		healthState.markRegistered(heartbeatInterval)
	}

	// Heartbeat goroutine with automatic re-registration on daemon restart.
	heartbeatStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := client.Heartbeat(ctx); err != nil {
					if errors.Is(err, ErrNotRegistered) {
						slog.Warn("agent no longer registered, re-registering", "name", a.Name())
						if _, regErr := client.Register(ctx, info); regErr != nil {
							slog.Error("agent re-registration failed", "name", a.Name(), "error", regErr)
						} else {
							slog.Info("agent re-registered successfully", "name", a.Name(), "instance_id", client.InstanceID())
						}
					} else {
						slog.Warn("heartbeat failed", "component", "agent", "name", a.Name(), "error", err)
					}
					continue
				}
				if healthState != nil {
					healthState.markHeartbeat()
				}
			}
		}
	}()
	defer close(heartbeatStop)

	// Poll loop.
	slog.Info("agent polling for work", "name", a.Name())
	for {
		select {
		case <-ctx.Done():
			slog.Info("agent shutting down", "component", "agent", "name", a.Name())
			return nil
		default:
		}

		work, err := client.PollWork(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("poll work error", "component", "agent", "name", a.Name(), "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pollInterval):
			}
			continue
		}

		if work == nil {
			// No work available; wait before polling again.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pollInterval):
			}
			continue
		}

		// Execute work item in a goroutine so the poll loop can continue.
		go executePlatformAgentWork(ctx, client, a, work)
	}
}

// executePlatformAgentWork deserializes a work item payload, invokes the agent,
// and submits the result back to the platform.
func executePlatformAgentWork(ctx context.Context, client *PlatformClient, a agent.Agent, work *WorkItem) {
	slog.Info("agent executing work item",
		"component", "agent",
		"name", a.Name(),
		"work_id", work.WorkID,
		"work_type", work.WorkType,
	)

	// Deserialize task from proto-encoded payload.
	var protoTask typespb.Task
	if err := protolib.Unmarshal(work.Payload, &protoTask); err != nil {
		// Fall back to JSON deserialization for flexibility.
		var taskJSON struct {
			ID   string `json:"id"`
			Goal string `json:"goal"`
		}
		if jsonErr := json.Unmarshal(work.Payload, &taskJSON); jsonErr != nil {
			slog.Error("failed to deserialize agent task payload",
				"work_id", work.WorkID,
				"proto_error", err,
				"json_error", jsonErr,
			)
			_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
				Code:      "DESERIALIZATION_ERROR",
				Message:   fmt.Sprintf("failed to decode task payload: %v", err),
				Retryable: false,
			})
			return
		}
		protoTask.Id = taskJSON.ID
		protoTask.Goal = taskJSON.Goal
	}

	task := ProtoToTask(&protoTask)

	// Apply work item timeout if provided.
	taskCtx := ctx
	var taskCancel context.CancelFunc
	if work.TimeoutMs > 0 {
		taskCtx, taskCancel = context.WithTimeout(ctx, time.Duration(work.TimeoutMs)*time.Millisecond)
		defer taskCancel()
	}

	// Build a harness backed by the platform connection so the agent can use
	// LLM completion, tools, plugins, and findings through the platform RPCs.
	harness := NewPlatformHarness(client,
		WithPlatformWorkID(work.WorkID),
		WithPlatformMission(missionContextFromWorkItem(work)),
		WithPlatformTarget(targetInfoFromWorkItem(work)),
		WithPlatformLogger(slog.Default().With("agent", a.Name(), "work_id", work.WorkID)),
		// tracer: use no-op until trace context is propagated via work item
	)
	// Populate mission execution context from the work item if the daemon
	// provided it (key: "mission_execution_context_json").
	harness.SetMissionExecutionContext(missionExecutionContextFromWorkItem(work))

	result, err := a.Execute(taskCtx, harness, task)

	if err != nil {
		slog.Warn("agent task execution failed",
			"component", "agent",
			"name", a.Name(),
			"work_id", work.WorkID,
			"error", err,
		)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "EXECUTION_ERROR",
			Message:   err.Error(),
			Retryable: false,
		})
		return
	}

	// Serialize result as proto bytes.
	protoResult := ResultToProto(result)
	resultBytes, marshalErr := protolib.Marshal(protoResult)
	if marshalErr != nil {
		slog.Error("failed to marshal agent result",
			"work_id", work.WorkID,
			"error", marshalErr,
		)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "SERIALIZATION_ERROR",
			Message:   fmt.Sprintf("failed to marshal result: %v", marshalErr),
			Retryable: false,
		})
		return
	}

	if err := client.SubmitResult(ctx, work.WorkID, resultBytes, nil); err != nil {
		slog.Warn("failed to submit agent result",
			"component", "agent",
			"name", a.Name(),
			"work_id", work.WorkID,
			"error", err,
		)
		return
	}

	slog.Info("agent work item completed",
		"component", "agent",
		"name", a.Name(),
		"work_id", work.WorkID,
		"status", result.Status,
	)
}

// ============================================================================
// Tool platform mode
// ============================================================================

// servePlatformTool connects a tool to the Gibson platform in pull mode.
// The tool registers itself outbound, polls for work, executes locally, and
// submits results. No local gRPC server is started.
//
// Capability Grant bootstrap always runs first (ADR-0036). When
// useSPIFFETransport(cfg) is true the gRPC dial uses SPIFFE mTLS transport
// with the CG JWT still attached as the per-RPC credential. Otherwise standard
// CG-authenticated TLS is used.
func servePlatformTool(t tool.Tool, cfg *Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start the HTTP health server early so /healthz responds immediately,
	// even before registration completes. /readyz will return 503 until
	// registration succeeds.
	_, healthState, stopHealth := startComponentHealthServer(cfg, "tool", t.Name())
	defer stopHealth()

	// Capability Grant bootstrap — always required (ADR-0036).
	client, err := NewPlatformClientWithOptions(cfg.PlatformURL, "",
		WithPlatformPollInterval(cfg.PollInterval),
		WithPlatformHeartbeatInterval(cfg.HeartbeatInterval),
	)
	if err != nil {
		return fmt.Errorf("create platform client: %w", err)
	}
	client.bootstrapToken = cfg.BootstrapToken
	client.hostKeyPath = cfg.HostKeyPath

	slog.Info("bootstrapping capability grant", "component", "tool", "name", t.Name(), "platform_url", cfg.PlatformURL)
	if err := client.Authenticate(ctx, t.Name(), "autonomous"); err != nil {
		return fmt.Errorf("capability grant bootstrap: %w", err)
	}

	// Create a CallbackClient for per-work-item authorization checks. A nil
	// callbackClient disables Authorize calls (dev mode without a daemon).
	var callbackClient *CallbackClient

	// Connect: SPIFFE mTLS transport upgrade when socket is present; CG TLS otherwise.
	if useSPIFFETransport(cfg) {
		slog.Info("upgrading to SPIFFE mTLS transport",
			"component", "tool",
			"name", t.Name(),
			"daemon_address", cfg.DaemonAddress,
			"spiffe_socket", cfg.SPIFFEEndpointSocket,
		)
		cgPerRPC := client.capabilityGrantClient.GRPCPerRPCCredentials()
		conn, source, err := dialSPIFFE(ctx, cfg.SPIFFEEndpointSocket, cfg.DaemonAddress, grpc.WithPerRPCCredentials(cgPerRPC))
		if err != nil {
			return fmt.Errorf("SPIFFE dial: %w", err)
		}
		defer source.Close()
		// Wire the SPIFFE conn into the PlatformClient; CG per-RPC creds travel on every call.
		client.conn = conn
		client.service = componentpb.NewComponentServiceClient(conn)

		// Callback client: SPIFFE mTLS transport + CG per-RPC for Authorize calls.
		tlsCreds := grpccredentials.MTLSClientCredentials(source, source, tlsconfig.AuthorizeAny())
		cc, ccErr := NewCallbackClient(cfg.DaemonAddress,
			WithCallbackTransportCredentials(tlsCreds),
			WithCallbackCredentials(cgPerRPC),
		)
		if ccErr != nil {
			slog.Warn("failed to create callback client for tool authz — Authorize calls will be no-op",
				"tool", t.Name(), "error", ccErr)
		} else if connErr := cc.Connect(ctx); connErr != nil {
			slog.Warn("callback client connect failed — Authorize calls will be no-op",
				"tool", t.Name(), "error", connErr)
		} else {
			callbackClient = cc
			defer func() {
				if cerr := callbackClient.Close(); cerr != nil {
					slog.Warn("error closing callback client", "tool", t.Name(), "error", cerr)
				}
			}()
		}
	} else {
		slog.Info("connecting to platform", "component", "tool", "name", t.Name(), "platform_url", cfg.PlatformURL)
		if err := client.connectWithBackoff(ctx); err != nil {
			return fmt.Errorf("connect to platform: %w", err)
		}
		defer func() {
			if cerr := client.Close(); cerr != nil {
				slog.Warn("error closing platform client", "component", "tool", "name", t.Name(), "error", cerr)
			}
		}()

		if cfg.PlatformURL != "" {
			var callbackOpts []CallbackClientOption
			if ac := client.CapabilityGrantClient(); ac != nil {
				callbackOpts = append(callbackOpts, WithCallbackCredentials(ac.GRPCPerRPCCredentials()))
			}
			cc, ccErr := NewCallbackClient(cfg.PlatformURL, callbackOpts...)
			if ccErr != nil {
				slog.Warn("failed to create callback client for tool authz — Authorize calls will be no-op",
					"tool", t.Name(), "error", ccErr)
			} else if connErr := cc.Connect(ctx); connErr != nil {
				slog.Warn("callback client connect failed — Authorize calls will be no-op",
					"tool", t.Name(), "error", connErr)
			} else {
				callbackClient = cc
				defer func() {
					if cerr := callbackClient.Close(); cerr != nil {
						slog.Warn("error closing callback client", "tool", t.Name(), "error", cerr)
					}
				}()
			}
		}
	}

	// Build registration metadata — mirrors what serveToolGRPC populates.
	metadata := map[string]string{
		"description":         t.Description(),
		"input_message_type":  t.InputMessageType(),
		"output_message_type": t.OutputMessageType(),
	}
	if len(t.Tags()) > 0 {
		metadata["tags"] = strings.Join(t.Tags(), ",")
	}

	// Attempt to extract the FileDescriptorSet for schema introspection.
	var fdsBytes []byte
	fds, fdsErr := ExtractFileDescriptorSet(t)
	if fdsErr != nil {
		slog.Warn("schema extraction failed",
			"component", "tool",
			"name", t.Name(),
			"input_type", t.InputMessageType(),
			"output_type", t.OutputMessageType(),
			"error", fdsErr,
		)
	} else if fds != nil {
		fdsBytes = fds
		metadata["file_descriptor_set"] = base64.StdEncoding.EncodeToString(fds)
		slog.Debug("extracted tool schema", "tool", t.Name(), "size_bytes", len(fds))
	} else {
		slog.Warn("tool registered without FileDescriptorSet - dynamic type resolution will fail",
			"tool", t.Name(),
			"input_type", t.InputMessageType(),
			"output_type", t.OutputMessageType(),
		)
	}

	info := ComponentInfo{
		Kind:              "tool",
		Name:              t.Name(),
		Version:           t.Version(),
		Metadata:          metadata,
		InputMessageType:  t.InputMessageType(),
		OutputMessageType: t.OutputMessageType(),
		FileDescriptorSet: fdsBytes,
		OntologyExtension: ontologyExtensionFrom(t),
	}

	reg, err := client.Register(ctx, info)
	if err != nil {
		return fmt.Errorf("register tool: %w", err)
	}

	pollInterval := cfg.PollInterval
	heartbeatInterval := cfg.HeartbeatInterval
	if reg != nil {
		if reg.PollIntervalMs > 0 {
			pollInterval = time.Duration(reg.PollIntervalMs) * time.Millisecond
		}
		if reg.HeartbeatIntervalMs > 0 {
			heartbeatInterval = time.Duration(reg.HeartbeatIntervalMs) * time.Millisecond
		}
	}

	slog.Info("tool registered",
		"name", t.Name(),
		"instance_id", client.InstanceID(),
		"poll_interval", pollInterval,
		"heartbeat_interval", heartbeatInterval,
	)

	// Flip /readyz to healthy now that registration succeeded.
	if healthState != nil {
		healthState.markRegistered(heartbeatInterval)
	}

	// Heartbeat goroutine with automatic re-registration on daemon restart.
	heartbeatStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := client.Heartbeat(ctx); err != nil {
					if errors.Is(err, ErrNotRegistered) {
						slog.Warn("tool no longer registered, re-registering", "name", t.Name())
						if _, regErr := client.Register(ctx, info); regErr != nil {
							slog.Error("tool re-registration failed", "name", t.Name(), "error", regErr)
						} else {
							slog.Info("tool re-registered successfully", "name", t.Name(), "instance_id", client.InstanceID())
						}
					} else {
						slog.Warn("heartbeat failed", "component", "tool", "name", t.Name(), "error", err)
					}
					continue
				}
				if healthState != nil {
					healthState.markHeartbeat()
				}
			}
		}
	}()
	defer close(heartbeatStop)

	slog.Info("tool polling for work", "name", t.Name())
	for {
		select {
		case <-ctx.Done():
			slog.Info("tool shutting down", "component", "tool", "name", t.Name())
			return nil
		default:
		}

		work, err := client.PollWork(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("poll work error", "component", "tool", "name", t.Name(), "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pollInterval):
			}
			continue
		}

		if work == nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pollInterval):
			}
			continue
		}

		go executePlatformToolWork(ctx, client, t, work, cfg.Extractor, cfg.AuthzFailOpen, callbackClient)
	}
}

// toolCallbackAuthorizer is a minimal harnessconst.Authorizer backed by a
// CallbackClient. One instance is created per work-item execution and carries
// the verified run_id from the work envelope's AuthzContext.
type toolCallbackAuthorizer struct {
	client   *CallbackClient
	runID    string
	failOpen bool
}

func (a *toolCallbackAuthorizer) Authorize(ctx context.Context, action, resource string) error {
	if a.client == nil || !a.client.IsConnected() {
		// No connection — dev mode or startup race; allow (matches CallbackHarness dev-mode behaviour).
		return nil
	}
	if a.runID == "" {
		return nil
	}

	req := &harnesspb.AuthorizeRequest{
		RunId:    a.runID,
		Action:   action,
		Resource: resource,
	}
	resp, err := a.client.Authorize(ctx, req)
	if err != nil {
		st, ok := grpcstatus.FromError(err)
		if !ok {
			if a.failOpen {
				slog.WarnContext(ctx, "tool authz unavailable — proceeding (fail-open)", "action", action, "resource", resource, "error", err)
				return nil
			}
			slog.ErrorContext(ctx, "tool authz unavailable — denying (fail-closed)", "action", action, "resource", resource, "error", err)
			return harnessconst.ErrAuthzServiceUnavailable
		}
		switch st.Code() {
		case codes.Unimplemented:
			slog.Debug("daemon does not support Authorize RPC; defaulting to allow", "action", action, "resource", resource)
			return nil
		case codes.Unavailable, codes.DeadlineExceeded:
			if a.failOpen {
				slog.WarnContext(ctx, "tool authz unavailable — proceeding (fail-open)", "action", action, "resource", resource)
				return nil
			}
			slog.ErrorContext(ctx, "tool authz unavailable — denying (fail-closed)", "action", action, "resource", resource)
			return harnessconst.ErrAuthzServiceUnavailable
		case codes.NotFound, codes.FailedPrecondition:
			slog.WarnContext(ctx, "tool authz denied", "action", action, "resource", resource, "code", st.Code())
			return harnessconst.ErrUnauthorized
		default:
			return harnessconst.ErrUnauthorized
		}
	}
	if !resp.Allowed {
		slog.InfoContext(ctx, "tool authz denied by FGA",
			"action", action, "resource", resource, "run_id", a.runID)
		return harnessconst.ErrUnauthorized
	}
	return nil
}

// executePlatformToolWork deserializes a work item payload, invokes the tool's
// ExecuteProto, and submits the result back to the platform.
//
// callbackClient is the harness callback connection used to forward Authorize calls
// to the daemon. When nil, authorization is skipped (dev mode).
func executePlatformToolWork(ctx context.Context, client *PlatformClient, t tool.Tool, work *WorkItem, extractor EntityExtractor, failOpen bool, callbackClient *CallbackClient) {
	slog.Info("tool executing work item",
		"component", "tool",
		"name", t.Name(),
		"work_id", work.WorkID,
		"work_type", work.WorkType,
	)

	// run_id is carried in the work item context by the daemon; Capability Grant JWTs
	// authenticate the transport-level connection, not individual work items.
	var verifiedRunID string

	inputTypeName := t.InputMessageType()
	if inputTypeName == "" {
		slog.Error("tool does not specify InputMessageType", "tool", t.Name(), "work_id", work.WorkID)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "CONFIGURATION_ERROR",
			Message:   "tool does not specify InputMessageType",
			Retryable: false,
		})
		return
	}

	messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(inputTypeName))
	if err != nil {
		slog.Error("failed to find input message type",
			"tool", t.Name(),
			"work_id", work.WorkID,
			"input_type", inputTypeName,
			"error", err,
		)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "TYPE_NOT_FOUND",
			Message:   fmt.Sprintf("failed to find message type %q: %v", inputTypeName, err),
			Retryable: false,
		})
		return
	}

	protoReq := messageType.New().Interface()

	// Payload is JSON-encoded input for tools.
	inputJSON := string(work.Payload)

	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshaler.Unmarshal([]byte(inputJSON), protoReq); err != nil {
		slog.Error("failed to unmarshal tool input",
			"tool", t.Name(),
			"work_id", work.WorkID,
			"input_type", inputTypeName,
			"error", err,
		)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "INVALID_INPUT",
			Message:   fmt.Sprintf("invalid input JSON for type %s: %v", inputTypeName, err),
			Retryable: false,
		})
		return
	}

	// Apply work item timeout if provided.
	toolCtx := ctx
	var toolCancel context.CancelFunc
	if work.TimeoutMs > 0 {
		toolCtx, toolCancel = context.WithTimeout(ctx, time.Duration(work.TimeoutMs)*time.Millisecond)
		defer toolCancel()
	}

	// Inject a per-work-item Authorizer into the context so that ExecuteProto
	// implementations can call harness.AuthorizerFromContext(ctx).Authorize(...)
	// for defense-in-depth checks without receiving the full agent.Harness interface.
	// When callbackClient is nil (dev mode) the toolCallbackAuthorizer no-ops safely.
	toolCtx = harnessconst.ContextWithAuthorizer(toolCtx, &toolCallbackAuthorizer{
		client:   callbackClient,
		runID:    verifiedRunID,
		failOpen: failOpen,
	})

	protoResp, err := t.ExecuteProto(toolCtx, protoReq)
	if err != nil {
		slog.Warn("tool execution failed",
			"component", "tool",
			"name", t.Name(),
			"work_id", work.WorkID,
			"error", err,
		)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "EXECUTION_ERROR",
			Message:   err.Error(),
			Retryable: false,
		})
		return
	}

	// Run extraction if configured — populate field 100 before serializing.
	if extractor != nil && protoResp != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("extractor panicked, continuing without discovery",
						"tool", t.Name(), "work_id", work.WorkID, "panic", r)
				}
			}()
			if discovery, extractErr := extractor.Extract(toolCtx, protoResp); extractErr != nil {
				slog.Warn("extraction failed, continuing without discovery",
					"tool", t.Name(), "work_id", work.WorkID, "error", extractErr)
			} else if discovery != nil {
				if ok := setDiscoveryField(protoResp, discovery); ok {
					slog.Debug("extraction populated field 100",
						"tool", t.Name(), "work_id", work.WorkID)
				}
			}
		}()
	}

	// Serialize output as JSON bytes.
	outputJSON, marshalErr := protojson.Marshal(protoResp)
	if marshalErr != nil {
		slog.Error("failed to marshal tool output",
			"tool", t.Name(),
			"work_id", work.WorkID,
			"error", marshalErr,
		)
		_ = client.SubmitResult(ctx, work.WorkID, nil, &WorkError{
			Code:      "SERIALIZATION_ERROR",
			Message:   fmt.Sprintf("failed to marshal output: %v", marshalErr),
			Retryable: false,
		})
		return
	}

	if err := client.SubmitResult(ctx, work.WorkID, outputJSON, nil); err != nil {
		slog.Warn("failed to submit tool result",
			"component", "tool",
			"name", t.Name(),
			"work_id", work.WorkID,
			"error", err,
		)
		return
	}

	slog.Info("tool work item completed",
		"component", "tool",
		"name", t.Name(),
		"work_id", work.WorkID,
	)
}

// ============================================================================
// Work item context helpers
// ============================================================================

// missionContextFromWorkItem extracts a types.MissionContext from the work item's
// Context metadata. Keys used: "mission_id", "mission_name", "mission_phase".
// Returns a zero-value MissionContext if none of the keys are present.
func missionContextFromWorkItem(work *WorkItem) types.MissionContext {
	if work.Context == nil {
		return types.MissionContext{}
	}
	return types.MissionContext{
		ID:    work.Context["mission_id"],
		Name:  work.Context["mission_name"],
		Phase: work.Context["mission_phase"],
	}
}

// targetInfoFromWorkItem extracts a types.TargetInfo from the work item's
// Context metadata. Keys used: "target_id", "target_name", "target_type", "target_provider".
// Returns a zero-value TargetInfo if none of the keys are present.
func targetInfoFromWorkItem(work *WorkItem) types.TargetInfo {
	if work.Context == nil {
		return types.TargetInfo{}
	}
	return types.TargetInfo{
		ID:       work.Context["target_id"],
		Name:     work.Context["target_name"],
		Type:     work.Context["target_type"],
		Provider: work.Context["target_provider"],
	}
}

// missionExecutionContextFromWorkItem extracts a types.MissionExecutionContext
// from the work item's context map. The daemon encodes the full context as JSON
// under the key "mission_execution_context_json" (set in PollWork).
// Returns a zero-value context if the key is absent or the JSON is malformed.
func missionExecutionContextFromWorkItem(work *WorkItem) types.MissionExecutionContext {
	if work.Context == nil {
		return types.MissionExecutionContext{}
	}
	raw, ok := work.Context["mission_execution_context_json"]
	if !ok || raw == "" {
		return types.MissionExecutionContext{}
	}
	var execCtx types.MissionExecutionContext
	if err := json.Unmarshal([]byte(raw), &execCtx); err != nil {
		slog.Warn("platform: failed to parse mission_execution_context_json from work item",
			"error", err)
		return types.MissionExecutionContext{}
	}
	return execCtx
}
