// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	"github.com/zeroroot-ai/sdk/capabilitygrant"
	"github.com/zeroroot-ai/sdk/graphrag"
)

// ============================================================================
// Local message types mirroring component.proto
//
// These types are the public API surface for PlatformClient callers. They
// mirror the wire types in componentpb but are deliberately decoupled so that
// callers in the serve package do not need to import the generated proto package
// directly.
// ============================================================================

// ComponentInfo carries registration metadata sent in RegisterComponent.
type ComponentInfo struct {
	// Kind is "agent", "tool", or "plugin".
	Kind string
	Name string
	// Version is the component's semver version string.
	Version string
	// Metadata is arbitrary key/value pairs attached to the registration.
	Metadata map[string]string
	// Capabilities lists agent capabilities (kind == "agent").
	Capabilities []string
	// Methods lists plugin method names (kind == "plugin").
	Methods []string
	// InputMessageType is the fully-qualified proto type for tool input (kind == "tool").
	InputMessageType string
	// OutputMessageType is the fully-qualified proto type for tool output (kind == "tool").
	OutputMessageType string
	// FileDescriptorSet is the serialized proto FileDescriptorSet for tool schema (kind == "tool").
	FileDescriptorSet []byte
	// OntologyExtension is the component's contribution to the daemon's
	// ontology reasoner (hierarchies, equivalences, IFPs, prefixes). Optional;
	// the zero value is skipped on the wire. Components that implement
	// `serve.OntologyContributor` have this field populated automatically by
	// `serve.Tool` / `serve.Agent` from `OntologyExtension()`.
	OntologyExtension graphrag.OntologyExtension
}

// RegistrationResponse carries the server-assigned instance ID and connection
// parameters returned by RegisterComponent.
type RegistrationResponse struct {
	// InstanceID is the unique identifier assigned to this component instance.
	InstanceID string
	// HeartbeatIntervalMs is the recommended heartbeat cadence in milliseconds.
	HeartbeatIntervalMs int32
	// PollIntervalMs is the recommended idle poll cadence in milliseconds.
	PollIntervalMs int32
	// PollTimeoutMs is the server-side long-poll timeout to pass in PollWork.
	PollTimeoutMs int32
	// Config carries initial configuration values for the component.
	Config map[string]string
}

// WorkItem represents a single unit of work returned by PollWork.
// When WorkID is empty the poll timed out with no available work.
type WorkItem struct {
	// WorkID uniquely identifies this work item; must be echoed in SubmitResult.
	WorkID string
	// WorkType describes the kind of work (e.g. "execute", "stream").
	WorkType string
	// Payload is the serialized work payload; interpretation depends on WorkType.
	Payload []byte
	// Context carries arbitrary key/value metadata for this work item.
	Context map[string]string
	// TimeoutMs is the maximum allowed time to complete and submit the result.
	TimeoutMs int64
}

// WorkError represents a structured error outcome for a failed work item.
// Leave nil on success when calling SubmitResult.
type WorkError struct {
	// Code is a short machine-readable error identifier (e.g. "EXECUTION_FAILED").
	Code string
	// Message is a human-readable description of the error.
	Message string
	// Retryable indicates whether the caller should retry the operation.
	Retryable bool
}

// LLMMessage is a single message in an LLM conversation, matching the
// LLMMessage proto message in component.proto.
type LLMMessage struct {
	// Role is "system", "user", or "assistant".
	Role    string
	Content string
}

// TokenUsage reports token consumption for an LLM completion call.
type TokenUsage struct {
	InputTokens  int32
	OutputTokens int32
}

// CompleteResponse carries the LLM reply for a non-streaming completion.
type CompleteResponse struct {
	Response *LLMMessage
	Usage    *TokenUsage
}

// CompleteWithToolsResult holds the LLM reply, any tool calls, and token usage
// for a CompleteWithTools request.
type CompleteWithToolsResult struct {
	// Response is the LLM text reply (may be nil when the model only emitted tool calls).
	Response *LLMMessage
	// ToolCalls is the list of tool-use invocations requested by the model.
	ToolCalls []*componentpb.ToolCallResult
	// Usage reports token consumption for this call.
	Usage *TokenUsage
	// FinishReason is the model's stop reason (e.g. "tool_use", "end_turn").
	FinishReason string
}

// StreamChunk is a single token chunk delivered by StreamCompletion.
type StreamChunk struct {
	// Content is the incremental text fragment for this chunk.
	Content string
	// Done is true on the final chunk; Usage is only set when Done is true.
	Done bool
	// Usage reports token consumption, populated only on the final chunk.
	Usage *TokenUsage
	// Err is non-nil if the stream terminated with an error.
	Err error
}

// ToolStreamEvent is a single event delivered by CallToolStream.
type ToolStreamEvent struct {
	// EventType is one of "progress", "partial", "warning", "error", "result".
	EventType string
	// PayloadJSON carries event-specific data.
	PayloadJSON string
	// Done is true on the final event.
	Done bool
	// Err is non-nil if the stream terminated with an error.
	Err error
}

// QueuedToolResultEvent is a single result delivered by ToolResults.
type QueuedToolResultEvent struct {
	// Index is the zero-based position in the original inputs array.
	Index int32
	// OutputJSON is the JSON-encoded tool output.
	OutputJSON string
	// Done is true on the final result.
	Done bool
	// Err is non-nil if this invocation failed or the stream terminated with an error.
	Err error
}

// ============================================================================
// PlatformClient
// ============================================================================

// PlatformClient manages the outbound gRPC connection from a Gibson component
// to the Gibson platform's ComponentService. It is thread-safe and supports
// automatic reconnection with exponential backoff.
//
// All outbound gRPC calls are authenticated with Capability Grant JWTs via
// capabilitygrant.Client.GRPCPerRPCCredentials(). Call Authenticate before Connect
// to complete the Capability Grant discover → register bootstrap sequence.
//
// Usage:
//
//	cfg := serve.DefaultConfig()
//	cfg.PlatformURL = "https://platform.gibson.example.com"
//
//	pc := serve.NewPlatformClient(cfg)
//	if err := pc.Authenticate(ctx, componentName, componentMode); err != nil { ... }
//	if err := pc.Connect(ctx); err != nil { ... }
//	defer pc.Close()
//
//	reg, err := pc.Register(ctx, serve.ComponentInfo{Kind: "agent", Name: "k8s-killer", Version: "1.0.0"})
type PlatformClient struct {
	// conn is the underlying gRPC client connection.
	conn *grpc.ClientConn

	// service is the generated ComponentServiceClient used for all RPCs.
	service componentpb.ComponentServiceClient

	// capabilityGrantClient authenticates all outbound gRPC calls with Capability Grant JWTs.
	// Set via Authenticate; nil means unauthenticated (dev/test only).
	capabilityGrantClient *capabilitygrant.Client

	// instanceID is the server-assigned ID after registration.
	instanceID string
	// pollTimeoutMs is the server-recommended long-poll timeout stored after Register.
	pollTimeoutMs int32
	// platformURL is the target Gibson platform endpoint, e.g. "https://platform:443".
	platformURL string
	// bootstrapToken is the one-time registration credential for first-time host registration.
	bootstrapToken string
	// hostKeyPath is the path to the on-disk Ed25519 host keypair.
	hostKeyPath string
	// pollInterval overrides the server-recommended poll cadence (zero = use server value).
	pollInterval time.Duration
	// heartbeatInterval overrides the server-recommended heartbeat cadence (zero = use server value).
	heartbeatInterval time.Duration

	mu     sync.RWMutex
	closed bool
}

// NewPlatformClient constructs a PlatformClient from the provided Config.
// The client is not connected until Authenticate and Connect are called.
//
// Config fields consumed:
//   - PlatformURL    — Gibson platform HTTPS base URL (required)
//   - BootstrapToken — one-time registration credential (optional after first run)
//   - HostKeyPath    — path to Ed25519 host key file (defaults to ~/.gibson/host_key.json)
func NewPlatformClient(cfg *Config) *PlatformClient {
	if cfg == nil {
		return &PlatformClient{}
	}
	return &PlatformClient{
		platformURL:       cfg.PlatformURL,
		bootstrapToken:    cfg.BootstrapToken,
		hostKeyPath:       cfg.HostKeyPath,
		pollInterval:      cfg.PollInterval,
		heartbeatInterval: cfg.HeartbeatInterval,
	}
}

// NewPlatformClientFromConn constructs a PlatformClient from an already-established
// gRPC connection. This is used by SPIFFE mode, which dials with mTLS credentials
// before creating the client. The returned client is ready to use immediately
// (no Connect or Authenticate call needed).
func NewPlatformClientFromConn(conn *grpc.ClientConn) *PlatformClient {
	return &PlatformClient{
		conn:    conn,
		service: componentpb.NewComponentServiceClient(conn),
	}
}

// NewPlatformClientWithOptions constructs a fully configured PlatformClient.
// The second argument (previously an API key) is ignored; Capability Grant credentials
// are supplied via the WithPlatformCapabilityGrant option or by calling Authenticate.
func NewPlatformClientWithOptions(platformURL, _ string, opts ...PlatformClientOption) (*PlatformClient, error) {
	if platformURL == "" {
		return nil, errors.New("platformURL cannot be empty")
	}

	pc := &PlatformClient{
		platformURL: platformURL,
	}

	for _, opt := range opts {
		opt(pc)
	}

	return pc, nil
}

// PlatformClientOption is a functional option for PlatformClient.
type PlatformClientOption func(*PlatformClient)

// WithPlatformPollInterval overrides the server-recommended poll interval.
// Use this to force a faster or slower polling cadence during development.
func WithPlatformPollInterval(d time.Duration) PlatformClientOption {
	return func(pc *PlatformClient) {
		pc.pollInterval = d
	}
}

// WithPlatformHeartbeatInterval overrides the server-recommended heartbeat interval.
func WithPlatformHeartbeatInterval(d time.Duration) PlatformClientOption {
	return func(pc *PlatformClient) {
		pc.heartbeatInterval = d
	}
}

// WithPlatformCapabilityGrant wires an already-bootstrapped capabilitygrant.Client into the
// PlatformClient. All outbound gRPC calls will be signed using the client's
// per-RPC credentials. This option is an alternative to calling Authenticate
// when the caller wants to share an capabilitygrant.Client instance across multiple
// PlatformClient instances.
func WithPlatformCapabilityGrant(c *capabilitygrant.Client) PlatformClientOption {
	return func(pc *PlatformClient) {
		pc.capabilityGrantClient = c
	}
}

// Authenticate performs the Capability Grant discover → register bootstrap sequence
// using the platform URL, bootstrap token, and host key path stored in the
// PlatformClient. After a successful call, all subsequent gRPC calls are
// signed with fresh Capability Grant JWTs.
//
// componentName and componentMode are used as the agent_name / agent_mode fields
// in the registration request (e.g. "my-mytool-tool", "autonomous").
//
// Authenticate is idempotent if called with the same parameters; calling it
// again after a successful bootstrap is a no-op that re-signs with the
// existing credentials.
func (pc *PlatformClient) Authenticate(ctx context.Context, componentName, componentMode string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Already authenticated — re-use existing client.
	if pc.capabilityGrantClient != nil {
		return nil
	}

	cfg := capabilitygrant.ClientConfig{
		PlatformURL:    pc.platformURL,
		BootstrapToken: pc.bootstrapToken,
		HostKeyPath:    pc.hostKeyPath,
		AgentName:      componentName,
		AgentMode:      componentMode,
	}

	client, err := capabilitygrant.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create capability grant client: %w", err)
	}

	if err := client.Discover(ctx); err != nil {
		return fmt.Errorf("capability grant discover: %w", err)
	}

	if err := client.Register(ctx); err != nil {
		return fmt.Errorf("capability grant register: %w", err)
	}

	pc.capabilityGrantClient = client
	return nil
}

// CapabilityGrantClient returns the underlying capabilitygrant.Client after a successful
// Authenticate call. Returns nil if Authenticate has not been called.
func (pc *PlatformClient) CapabilityGrantClient() *capabilitygrant.Client {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.capabilityGrantClient
}

// Connect dials the Gibson platform with appropriate transport credentials.
//
// TLS selection rules:
//   - URL begins with "https://" → TLS with system root CAs.
//   - URL begins with "http://" or "localhost" → insecure transport (dev only).
//
// If an capabilitygrant.Client has been set (via Authenticate or WithPlatformCapabilityGrant),
// all RPCs will carry a freshly signed Capability Grant JWT.
//
// If the connection is already healthy (Ready or Idle) this is a no-op.
// Calling Connect after Close returns an error.
func (pc *PlatformClient) Connect(ctx context.Context) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return errors.New("platform client is closed")
	}

	// Re-use an existing healthy connection.
	if pc.conn != nil {
		state := pc.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return nil
		}
		// Unhealthy — close and reconnect.
		_ = pc.conn.Close()
		pc.conn = nil
		pc.service = nil
	}

	dialOpts, err := pc.buildDialOptions()
	if err != nil {
		return fmt.Errorf("build dial options: %w", err)
	}

	target := pc.grpcTarget()

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	//nolint:staticcheck // DialContext is the idiomatic call site for timeout-bound dial.
	conn, err := grpc.DialContext(connCtx, target, dialOpts...)
	if err != nil {
		return fmt.Errorf("dial platform %s: %w", target, err)
	}

	pc.conn = conn
	pc.service = componentpb.NewComponentServiceClient(conn)

	// Wait briefly to detect an immediate transport failure.
	readyCtx, readyCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readyCancel()
	pc.conn.WaitForStateChange(readyCtx, connectivity.Idle)
	state := pc.conn.GetState()
	if state == connectivity.TransientFailure || state == connectivity.Shutdown {
		return fmt.Errorf("connection failed immediately: state=%s", state)
	}

	return nil
}

// grpcTarget strips the URL scheme and returns a bare "host:port" target
// suitable for grpc.Dial.
func (pc *PlatformClient) grpcTarget() string {
	target := pc.platformURL
	target = strings.TrimPrefix(target, "https://")
	target = strings.TrimPrefix(target, "http://")
	return target
}

// buildDialOptions constructs the grpc.DialOption slice based on the platform URL.
// When an capabilitygrant.Client is present, a per-RPC credential that signs a fresh
// Capability Grant JWT for every call is appended.
func (pc *PlatformClient) buildDialOptions() ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	// Transport credentials: TLS for https, insecure for http/localhost.
	if strings.HasPrefix(pc.platformURL, "https://") {
		creds := credentials.NewClientTLSFromCert(nil, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Per-RPC Capability Grant JWT credentials.
	if pc.capabilityGrantClient != nil {
		opts = append(opts, grpc.WithPerRPCCredentials(pc.capabilityGrantClient.GRPCPerRPCCredentials()))
	}

	// Keepalive: mirror the pattern used by CallbackClient.
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}))

	return opts, nil
}

// ensureConnected checks that the client is connected and not closed.
// Must be called before issuing any RPC. Not safe to call under the write lock.
func (pc *PlatformClient) ensureConnected(op string) error {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return fmt.Errorf("%s: client is closed", op)
	}
	if pc.conn == nil {
		return fmt.Errorf("%s: not connected — call Connect first", op)
	}
	state := pc.conn.GetState()
	if state == connectivity.Shutdown {
		return fmt.Errorf("%s: connection is shut down", op)
	}
	return nil
}

// connectWithBackoff attempts to re-establish the connection using exponential
// backoff (1 s → 2 s → 4 s … capped at 30 s). It is safe to call concurrently;
// only one goroutine will perform the dial at a time.
func (pc *PlatformClient) connectWithBackoff(ctx context.Context) error {
	const (
		initialDelay = 1 * time.Second
		maxDelay     = 30 * time.Second
	)

	delay := initialDelay
	for {
		err := pc.Connect(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("reconnect aborted: %w", ctx.Err())
		case <-time.After(delay):
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// InstanceID returns the server-assigned instance ID after a successful
// Register call. Returns an empty string before registration.
func (pc *PlatformClient) InstanceID() string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.instanceID
}

// ============================================================================
// ComponentService RPC methods
// ============================================================================

// Register calls the RegisterComponent RPC and stores the returned instance ID
// and poll timeout. The RegistrationResponse carries connection parameters
// (heartbeat/poll intervals) that callers should use to configure their
// polling and heartbeat loops.
func (pc *PlatformClient) Register(ctx context.Context, info ComponentInfo) (*RegistrationResponse, error) {
	if err := pc.ensureConnected("Register"); err != nil {
		return nil, err
	}

	req := &componentpb.RegisterComponentRequest{
		Kind:              info.Kind,
		Name:              info.Name,
		Version:           info.Version,
		Metadata:          info.Metadata,
		Capabilities:      info.Capabilities,
		Methods:           info.Methods,
		InputMessageType:  info.InputMessageType,
		OutputMessageType: info.OutputMessageType,
		FileDescriptorSet: info.FileDescriptorSet,
	}
	if !info.OntologyExtension.IsZero() {
		req.OntologyExtension = graphrag.OntologyExtensionToProto(info.OntologyExtension)
	}

	resp, err := pc.service.RegisterComponent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RegisterComponent: %w", err)
	}

	pc.mu.Lock()
	pc.instanceID = resp.InstanceId
	pc.pollTimeoutMs = resp.PollTimeoutMs
	pc.mu.Unlock()

	return &RegistrationResponse{
		InstanceID:          resp.InstanceId,
		HeartbeatIntervalMs: resp.HeartbeatIntervalMs,
		PollIntervalMs:      resp.PollIntervalMs,
		PollTimeoutMs:       resp.PollTimeoutMs,
		Config:              resp.Config,
	}, nil
}

// PollWork calls the PollWork RPC and returns the next available work item.
// Returns (nil, nil) when the poll timed out with no work available.
//
// The caller is responsible for re-calling PollWork in a loop and for
// honouring the poll interval returned by Register.
func (pc *PlatformClient) PollWork(ctx context.Context) (*WorkItem, error) {
	if err := pc.ensureConnected("PollWork"); err != nil {
		return nil, err
	}

	pc.mu.RLock()
	instanceID := pc.instanceID
	pollTimeoutMs := pc.pollTimeoutMs
	pc.mu.RUnlock()

	req := &componentpb.PollWorkRequest{
		InstanceId: instanceID,
		TimeoutMs:  pollTimeoutMs,
	}

	resp, err := pc.service.PollWork(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("PollWork: %w", err)
	}

	// Empty work_id means the long-poll timed out with no available work.
	if resp.WorkId == "" {
		return nil, nil
	}

	item := &WorkItem{
		WorkID:    resp.WorkId,
		WorkType:  resp.WorkType,
		Payload:   resp.Payload,
		Context:   resp.Context,
		TimeoutMs: resp.TimeoutMs,
	}

	return item, nil
}

// SubmitResult delivers the execution outcome of a work item back to Gibson.
// Pass a non-nil workErr to signal failure; leave nil for success.
func (pc *PlatformClient) SubmitResult(ctx context.Context, workID string, result []byte, workErr *WorkError) error {
	if err := pc.ensureConnected("SubmitResult"); err != nil {
		return err
	}
	if workID == "" {
		return errors.New("SubmitResult: workID cannot be empty")
	}

	req := &componentpb.SubmitResultRequest{
		WorkId: workID,
		Result: result,
	}
	if workErr != nil {
		req.Error = &componentpb.ComponentError{
			Code:      workErr.Code,
			Message:   workErr.Message,
			Retryable: workErr.Retryable,
		}
	}

	_, err := pc.service.SubmitResult(ctx, req)
	if err != nil {
		return fmt.Errorf("SubmitResult: %w", err)
	}
	return nil
}

// ErrNotRegistered is returned by Heartbeat when the daemon reports that the
// instance is no longer in the registry (e.g., after a daemon restart). The
// serve loop must call Register again when it sees this error.
var ErrNotRegistered = errors.New("instance is no longer registered")

// Heartbeat sends a health pulse to Gibson. Returns ErrNotRegistered when the
// daemon signals that the instance needs to re-register (the serve loop should
// call Register again). Other errors are transient and can be retried.
func (pc *PlatformClient) Heartbeat(ctx context.Context) error {
	if err := pc.ensureConnected("Heartbeat"); err != nil {
		return err
	}

	pc.mu.RLock()
	instanceID := pc.instanceID
	pc.mu.RUnlock()

	req := &componentpb.HeartbeatRequest{
		InstanceId:    instanceID,
		HealthStatus:  "healthy",
		HealthMessage: "",
	}

	resp, err := pc.service.Heartbeat(ctx, req)
	if err != nil {
		return fmt.Errorf("Heartbeat: %w", err)
	}
	if !resp.Registered {
		return ErrNotRegistered
	}
	return nil
}

// Complete proxies a non-streaming LLM completion request through the agent
// harness. workID ties the request to the active work item for authorization
// and billing. slot is the named LLM slot (e.g. "primary") declared by the agent.
func (pc *PlatformClient) Complete(ctx context.Context, workID, slot string, messages []LLMMessage) (*CompleteResponse, error) {
	if err := pc.ensureConnected("Complete"); err != nil {
		return nil, err
	}
	if workID == "" {
		return nil, errors.New("Complete: workID cannot be empty")
	}
	if slot == "" {
		return nil, errors.New("Complete: slot cannot be empty")
	}

	protoMsgs := make([]*componentpb.LLMMessage, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &componentpb.LLMMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	req := &componentpb.CompleteRequest{
		WorkId:   workID,
		Slot:     slot,
		Messages: protoMsgs,
	}

	resp, err := pc.service.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Complete: %w", err)
	}

	result := &CompleteResponse{}
	if resp.Response != nil {
		result.Response = &LLMMessage{
			Role:    resp.Response.Role,
			Content: resp.Response.Content,
		}
	}
	if resp.Usage != nil {
		result.Usage = &TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		}
	}
	return result, nil
}

// CallTool proxies a tool execution request through the agent harness.
// inputJSON must be a JSON object matching the named tool's input schema.
// Returns the JSON-encoded tool output on success.
func (pc *PlatformClient) CallTool(ctx context.Context, workID, toolName, inputJSON string) (string, error) {
	if err := pc.ensureConnected("CallTool"); err != nil {
		return "", err
	}
	if workID == "" {
		return "", errors.New("CallTool: workID cannot be empty")
	}
	if toolName == "" {
		return "", errors.New("CallTool: toolName cannot be empty")
	}

	req := &componentpb.CallToolRequest{
		WorkId:    workID,
		ToolName:  toolName,
		InputJson: inputJSON,
	}

	resp, err := pc.service.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("CallTool: %w", err)
	}
	if resp.Error != nil && resp.Error.Code != "" {
		return "", fmt.Errorf("CallTool %s: [%s] %s", toolName, resp.Error.Code, resp.Error.Message)
	}
	return resp.OutputJson, nil
}

// QueryPlugin proxies a plugin method invocation through the agent harness.
// paramsJSON must be a JSON object matching the named plugin method's parameter schema.
// Returns the JSON-encoded method result on success.
func (pc *PlatformClient) QueryPlugin(ctx context.Context, workID, pluginName, method, paramsJSON string) (string, error) {
	if err := pc.ensureConnected("QueryPlugin"); err != nil {
		return "", err
	}
	if workID == "" {
		return "", errors.New("QueryPlugin: workID cannot be empty")
	}
	if pluginName == "" {
		return "", errors.New("QueryPlugin: pluginName cannot be empty")
	}
	if method == "" {
		return "", errors.New("QueryPlugin: method cannot be empty")
	}

	req := &componentpb.QueryPluginRequest{
		WorkId:     workID,
		PluginName: pluginName,
		Method:     method,
		ParamsJson: paramsJSON,
	}

	resp, err := pc.service.QueryPlugin(ctx, req)
	if err != nil {
		return "", fmt.Errorf("QueryPlugin: %w", err)
	}
	if resp.Error != nil && resp.Error.Code != "" {
		return "", fmt.Errorf("QueryPlugin %s.%s: [%s] %s", pluginName, method, resp.Error.Code, resp.Error.Message)
	}
	return resp.ResultJson, nil
}

// SubmitFinding proxies a security finding submission through the agent harness.
// finding must be a proto-encoded Finding message. Returns the server-assigned
// finding ID on success.
func (pc *PlatformClient) SubmitFinding(ctx context.Context, workID string, finding []byte) (string, error) {
	if err := pc.ensureConnected("SubmitFinding"); err != nil {
		return "", err
	}
	if workID == "" {
		return "", errors.New("SubmitFinding: workID cannot be empty")
	}
	if len(finding) == 0 {
		return "", errors.New("SubmitFinding: finding payload cannot be empty")
	}

	req := &componentpb.SubmitFindingRequest{
		WorkId:  workID,
		Finding: finding,
	}

	resp, err := pc.service.SubmitFinding(ctx, req)
	if err != nil {
		return "", fmt.Errorf("SubmitFinding: %w", err)
	}
	return resp.FindingId, nil
}

// ============================================================================
// New harness-parity methods — added in task 2
// ============================================================================

// CompleteWithTools proxies an LLM completion with tool definitions for
// function-calling support. tools is a slice of componentpb.ToolDefinition
// describing the tools available to the model.
func (pc *PlatformClient) CompleteWithTools(ctx context.Context, workID, slot string, messages []LLMMessage, tools []*componentpb.ToolDefinition) (*CompleteWithToolsResult, error) {
	if err := pc.ensureConnected("CompleteWithTools"); err != nil {
		return nil, err
	}

	protoMsgs := make([]*componentpb.LLMMessage, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &componentpb.LLMMessage{Role: m.Role, Content: m.Content}
	}

	resp, err := pc.service.CompleteWithTools(ctx, &componentpb.CompleteWithToolsRequest{
		WorkId:   workID,
		Slot:     slot,
		Messages: protoMsgs,
		Tools:    tools,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.CompleteWithTools: %w", err)
	}

	result := &CompleteWithToolsResult{
		ToolCalls:    resp.ToolCalls,
		FinishReason: resp.FinishReason,
	}
	if resp.Response != nil {
		result.Response = &LLMMessage{Role: resp.Response.Role, Content: resp.Response.Content}
	}
	if resp.Usage != nil {
		result.Usage = &TokenUsage{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}
	}
	return result, nil
}

// CompleteStructured proxies an LLM completion requesting JSON output conforming
// to the provided JSON Schema. Returns the result_json string on success.
func (pc *PlatformClient) CompleteStructured(ctx context.Context, workID, slot string, messages []LLMMessage, schemaJSON string) (string, error) {
	if err := pc.ensureConnected("CompleteStructured"); err != nil {
		return "", err
	}

	protoMsgs := make([]*componentpb.LLMMessage, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &componentpb.LLMMessage{Role: m.Role, Content: m.Content}
	}

	resp, err := pc.service.CompleteStructured(ctx, &componentpb.CompleteStructuredRequest{
		WorkId:     workID,
		Slot:       slot,
		Messages:   protoMsgs,
		SchemaJson: schemaJSON,
	})
	if err != nil {
		return "", fmt.Errorf("PlatformClient.CompleteStructured: %w", err)
	}
	return resp.ResultJson, nil
}

// StreamCompletion proxies a streaming LLM completion request. It opens a
// server-side stream and returns a channel that receives chunks as they arrive.
// The channel is closed after the final chunk or on error. Callers should drain
// the channel until it is closed; the last event will have Done==true or a
// non-nil Err.
func (pc *PlatformClient) StreamCompletion(ctx context.Context, workID, slot string, messages []LLMMessage) (<-chan StreamChunk, error) {
	if err := pc.ensureConnected("StreamCompletion"); err != nil {
		return nil, err
	}

	protoMsgs := make([]*componentpb.LLMMessage, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &componentpb.LLMMessage{Role: m.Role, Content: m.Content}
	}

	stream, err := pc.service.CompleteStream(ctx, &componentpb.CompleteStreamRequest{
		WorkId:   workID,
		Slot:     slot,
		Messages: protoMsgs,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.StreamCompletion: %w", err)
	}

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		for {
			msg, recvErr := stream.Recv()
			if recvErr != nil {
				if !errors.Is(recvErr, io.EOF) {
					ch <- StreamChunk{Err: fmt.Errorf("PlatformClient.StreamCompletion: %w", recvErr)}
				}
				return
			}
			chunk := StreamChunk{
				Content: msg.Content,
				Done:    msg.Done,
			}
			if msg.Usage != nil {
				chunk.Usage = &TokenUsage{InputTokens: msg.Usage.InputTokens, OutputTokens: msg.Usage.OutputTokens}
			}
			ch <- chunk
			if msg.Done {
				return
			}
		}
	}()
	return ch, nil
}

// QueueToolWork submits a batch of tool invocations for parallel execution and
// returns the assigned job ID for tracking with ToolResults.
func (pc *PlatformClient) QueueToolWork(ctx context.Context, workID, toolName string, inputsJSON []string) (string, error) {
	if err := pc.ensureConnected("QueueToolWork"); err != nil {
		return "", err
	}

	resp, err := pc.service.QueueToolWork(ctx, &componentpb.QueueToolWorkRequest{
		WorkId:     workID,
		ToolName:   toolName,
		InputsJson: inputsJSON,
	})
	if err != nil {
		return "", fmt.Errorf("PlatformClient.QueueToolWork: %w", err)
	}
	return resp.JobId, nil
}

// CallToolStream proxies a tool execution with server-side streaming of progress,
// partial results, warnings, and the final output. Returns a channel that
// receives events until the stream is complete or encounters an error.
func (pc *PlatformClient) CallToolStream(ctx context.Context, workID, toolName, inputJSON string) (<-chan ToolStreamEvent, error) {
	if err := pc.ensureConnected("CallToolStream"); err != nil {
		return nil, err
	}

	stream, err := pc.service.CallToolStream(ctx, &componentpb.CallToolStreamRequest{
		WorkId:    workID,
		ToolName:  toolName,
		InputJson: inputJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.CallToolStream: %w", err)
	}

	ch := make(chan ToolStreamEvent, 32)
	go func() {
		defer close(ch)
		for {
			msg, recvErr := stream.Recv()
			if recvErr != nil {
				if !errors.Is(recvErr, io.EOF) {
					ch <- ToolStreamEvent{Err: fmt.Errorf("PlatformClient.CallToolStream: %w", recvErr)}
				}
				return
			}
			evt := ToolStreamEvent{
				EventType:   msg.EventType,
				PayloadJSON: msg.PayloadJson,
				Done:        msg.Done,
			}
			if msg.Error != nil && msg.Error.Code != "" {
				evt.Err = fmt.Errorf("CallToolStream %s: [%s] %s", toolName, msg.Error.Code, msg.Error.Message)
			}
			ch <- evt
			if msg.Done {
				return
			}
		}
	}()
	return ch, nil
}

// ToolResults streams results for a previously queued tool batch as each
// invocation completes. Returns a channel that receives one event per completed
// invocation; the final event has Done==true.
func (pc *PlatformClient) ToolResults(ctx context.Context, workID, jobID string) (<-chan QueuedToolResultEvent, error) {
	if err := pc.ensureConnected("ToolResults"); err != nil {
		return nil, err
	}

	stream, err := pc.service.ToolResults(ctx, &componentpb.ToolResultsRequest{
		WorkId: workID,
		JobId:  jobID,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.ToolResults: %w", err)
	}

	ch := make(chan QueuedToolResultEvent, 32)
	go func() {
		defer close(ch)
		for {
			msg, recvErr := stream.Recv()
			if recvErr != nil {
				if !errors.Is(recvErr, io.EOF) {
					ch <- QueuedToolResultEvent{Err: fmt.Errorf("PlatformClient.ToolResults: %w", recvErr)}
				}
				return
			}
			evt := QueuedToolResultEvent{
				Index:      msg.Index,
				OutputJSON: msg.OutputJson,
				Done:       msg.Done,
			}
			if msg.Error != nil && msg.Error.Code != "" {
				evt.Err = fmt.Errorf("ToolResults index %d: [%s] %s", msg.Index, msg.Error.Code, msg.Error.Message)
			}
			ch <- evt
			if msg.Done {
				return
			}
		}
	}()
	return ch, nil
}

// ListTools returns descriptors for all tools visible to the caller's tenant.
func (pc *PlatformClient) ListTools(ctx context.Context, workID string) ([]*componentpb.ToolDescriptorProto, error) {
	if err := pc.ensureConnected("ListTools"); err != nil {
		return nil, err
	}

	resp, err := pc.service.ListTools(ctx, &componentpb.ListToolsRequest{WorkId: workID})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.ListTools: %w", err)
	}
	return resp.Tools, nil
}

// DelegateToAgent dispatches a sub-task to another agent and returns its
// JSON-encoded result.
func (pc *PlatformClient) DelegateToAgent(ctx context.Context, workID, agentName string, taskJSON []byte) ([]byte, error) {
	if err := pc.ensureConnected("DelegateToAgent"); err != nil {
		return nil, err
	}

	resp, err := pc.service.DelegateToAgent(ctx, &componentpb.DelegateToAgentRequest{
		WorkId:    workID,
		AgentName: agentName,
		TaskJson:  taskJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.DelegateToAgent: %w", err)
	}
	return resp.ResultJson, nil
}

// ListAgents returns descriptors for all agents visible to the caller's tenant.
func (pc *PlatformClient) ListAgents(ctx context.Context, workID string) ([]*componentpb.AgentDescriptorProto, error) {
	if err := pc.ensureConnected("ListAgents"); err != nil {
		return nil, err
	}

	resp, err := pc.service.ListAgents(ctx, &componentpb.ListAgentsRequest{WorkId: workID})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.ListAgents: %w", err)
	}
	return resp.Agents, nil
}

// GetRunFindings queries findings scoped to a specific mission run or across
// all runs. scope is "previous" or "all". Returns JSON-encoded findings.
func (pc *PlatformClient) GetRunFindings(ctx context.Context, workID, scope string, filterJSON []byte) ([]byte, error) {
	if err := pc.ensureConnected("GetRunFindings"); err != nil {
		return nil, err
	}

	resp, err := pc.service.GetRunFindings(ctx, &componentpb.GetRunFindingsRequest{
		WorkId:     workID,
		Scope:      scope,
		FilterJson: filterJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.GetRunFindings: %w", err)
	}
	return resp.FindingsJson, nil
}

// CreateMission creates a new sub-mission and returns its JSON-encoded info.
// missionDefinitionJSON is the JSON-encoded mission definition; targetID is the mission
// target; optsJSON is optional JSON-encoded creation options.
func (pc *PlatformClient) CreateMission(ctx context.Context, workID string, missionDefinitionJSON []byte, targetID string, optsJSON []byte) ([]byte, error) {
	if err := pc.ensureConnected("CreateMission"); err != nil {
		return nil, err
	}

	resp, err := pc.service.CreateMission(ctx, &componentpb.CreateMissionRequest{
		WorkId:                workID,
		MissionDefinitionJson: missionDefinitionJSON,
		TargetId:              targetID,
		OptsJson:              optsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.CreateMission: %w", err)
	}
	return resp.MissionJson, nil
}

// RunMission queues a mission for execution. optsJSON is optional JSON-encoded
// run options; pass nil for default behaviour.
func (pc *PlatformClient) RunMission(ctx context.Context, workID, missionID string, optsJSON []byte) error {
	if err := pc.ensureConnected("RunMission"); err != nil {
		return err
	}

	_, err := pc.service.RunMission(ctx, &componentpb.RunMissionRequest{
		WorkId:    workID,
		MissionId: missionID,
		OptsJson:  optsJSON,
	})
	if err != nil {
		return fmt.Errorf("PlatformClient.RunMission: %w", err)
	}
	return nil
}

// GetMissionStatus returns the JSON-encoded current status of a mission.
func (pc *PlatformClient) GetMissionStatus(ctx context.Context, workID, missionID string) ([]byte, error) {
	if err := pc.ensureConnected("GetMissionStatus"); err != nil {
		return nil, err
	}

	resp, err := pc.service.GetMissionStatus(ctx, &componentpb.GetMissionStatusRequest{
		WorkId:    workID,
		MissionId: missionID,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.GetMissionStatus: %w", err)
	}
	return resp.StatusJson, nil
}

// WaitMission blocks until a mission completes or the timeout expires. Returns
// the JSON-encoded mission result. timeoutMs of 0 means wait indefinitely.
func (pc *PlatformClient) WaitMission(ctx context.Context, workID, missionID string, timeoutMs int64) ([]byte, error) {
	if err := pc.ensureConnected("WaitMission"); err != nil {
		return nil, err
	}

	resp, err := pc.service.WaitMission(ctx, &componentpb.WaitMissionRequest{
		WorkId:    workID,
		MissionId: missionID,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.WaitMission: %w", err)
	}
	return resp.ResultJson, nil
}

// ListMissions returns JSON-encoded missions matching the given filter.
// filterJSON is an optional JSON-encoded mission.MissionFilter; pass nil for all.
func (pc *PlatformClient) ListMissions(ctx context.Context, workID string, filterJSON []byte) ([]byte, error) {
	if err := pc.ensureConnected("ListMissions"); err != nil {
		return nil, err
	}

	resp, err := pc.service.ListMissions(ctx, &componentpb.ListMissionsRequest{
		WorkId:     workID,
		FilterJson: filterJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.ListMissions: %w", err)
	}
	return resp.MissionsJson, nil
}

// CancelMission requests cancellation of a running mission.
func (pc *PlatformClient) CancelMission(ctx context.Context, workID, missionID string) error {
	if err := pc.ensureConnected("CancelMission"); err != nil {
		return err
	}

	_, err := pc.service.CancelMission(ctx, &componentpb.CancelMissionRequest{
		WorkId:    workID,
		MissionId: missionID,
	})
	if err != nil {
		return fmt.Errorf("PlatformClient.CancelMission: %w", err)
	}
	return nil
}

// GetMissionResults returns the JSON-encoded final results of a completed mission.
func (pc *PlatformClient) GetMissionResults(ctx context.Context, workID, missionID string) ([]byte, error) {
	if err := pc.ensureConnected("GetMissionResults"); err != nil {
		return nil, err
	}

	resp, err := pc.service.GetMissionResults(ctx, &componentpb.GetMissionResultsRequest{
		WorkId:    workID,
		MissionId: missionID,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.GetMissionResults: %w", err)
	}
	return resp.ResultJson, nil
}

// GetCredential retrieves a tenant-scoped credential by name. Returns the
// JSON-encoded credential value.
func (pc *PlatformClient) GetCredential(ctx context.Context, workID, name string) ([]byte, error) {
	if err := pc.ensureConnected("GetCredential"); err != nil {
		return nil, err
	}

	resp, err := pc.service.GetCredential(ctx, &componentpb.GetCredentialRequest{
		WorkId: workID,
		Name:   name,
	})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.GetCredential: %w", err)
	}
	return resp.CredentialJson, nil
}

// GetTaxonomySchema returns the JSON-encoded current taxonomy definition.
func (pc *PlatformClient) GetTaxonomySchema(ctx context.Context, workID string) ([]byte, error) {
	if err := pc.ensureConnected("GetTaxonomySchema"); err != nil {
		return nil, err
	}

	resp, err := pc.service.GetTaxonomySchema(ctx, &componentpb.GetTaxonomySchemaRequest{WorkId: workID})
	if err != nil {
		return nil, fmt.Errorf("PlatformClient.GetTaxonomySchema: %w", err)
	}
	return resp.SchemaJson, nil
}

// ReportStepHints reports planning step hints from an agent back to the
// orchestrator. hintsJSON is the JSON-encoded planning.StepHints value.
func (pc *PlatformClient) ReportStepHints(ctx context.Context, workID string, hintsJSON []byte) error {
	if err := pc.ensureConnected("ReportStepHints"); err != nil {
		return err
	}

	_, err := pc.service.ReportStepHints(ctx, &componentpb.ReportStepHintsRequest{
		WorkId:    workID,
		HintsJson: hintsJSON,
	})
	if err != nil {
		return fmt.Errorf("PlatformClient.ReportStepHints: %w", err)
	}
	return nil
}

// Close closes the underlying gRPC connection and marks the client as closed.
// Subsequent RPC calls will return errors. Close is idempotent.
func (pc *PlatformClient) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil
	}

	pc.closed = true
	pc.service = nil

	if pc.conn != nil {
		err := pc.conn.Close()
		pc.conn = nil
		return err
	}

	return nil
}

// IsConnected reports whether the client currently has a healthy gRPC connection.
func (pc *PlatformClient) IsConnected() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed || pc.conn == nil {
		return false
	}

	state := pc.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle || state == connectivity.Connecting
}
