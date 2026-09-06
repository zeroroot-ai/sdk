// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	grpcCreds "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// CallbackClient manages the gRPC connection to the orchestrator's HarnessCallbackService.
// It provides thread-safe access to all harness operations via RPC callbacks.
type CallbackClient struct {
	// Connection management
	conn   *grpc.ClientConn
	client harnesspb.HarnessCallbackServiceClient
	mu     sync.RWMutex

	// Configuration
	endpoint       string
	tlsConf        *tls.Config
	transportCreds grpcCreds.TransportCredentials
	token          string
	perRPCCreds    grpcCreds.PerRPCCredentials

	// Context tracking
	taskID          string
	agentName       string
	missionID       string
	traceID         string
	spanID          string
	missionRunID    string // Unique ID for this mission execution
	agentRunID      string // Unique ID for this agent execution
	runNumber       int32  // Sequential run number (1, 2, 3...)
	toolExecutionID string // ID for tool execution provenance

	// Connection lifecycle
	connected bool
	closed    bool
}

// NewCallbackClient creates a new callback client with the given endpoint.
// The client is not connected until Connect() is called.
func NewCallbackClient(endpoint string, opts ...CallbackClientOption) (*CallbackClient, error) {
	if endpoint == "" {
		return nil, errors.New("endpoint cannot be empty")
	}

	client := &CallbackClient{
		endpoint: endpoint,
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// CallbackClientOption is a functional option for configuring CallbackClient.
type CallbackClientOption func(*CallbackClient)

// WithCallbackTLS configures TLS for the callback client connection.
func WithCallbackTLS(conf *tls.Config) CallbackClientOption {
	return func(c *CallbackClient) {
		c.tlsConf = conf
	}
}

// WithCallbackToken sets the authentication token for callback requests.
func WithCallbackToken(token string) CallbackClientOption {
	return func(c *CallbackClient) {
		c.token = token
	}
}

// WithCallbackCredentials sets per-RPC credentials for callback requests.
// When provided, these credentials take precedence over a token set by
// WithCallbackToken. Use this to pass an capabilitygrant.Client's GRPCPerRPCCredentials
// so that harness callbacks carry Capability Grant JWTs.
func WithCallbackCredentials(creds grpcCreds.PerRPCCredentials) CallbackClientOption {
	return func(c *CallbackClient) {
		c.perRPCCreds = creds
	}
}

// WithCallbackTransportCredentials sets the gRPC transport-level credentials for
// callback connections. Use this in SPIFFE mode to pass mTLS credentials obtained
// from the SPIRE Workload API, replacing the default insecure transport.
func WithCallbackTransportCredentials(creds grpcCreds.TransportCredentials) CallbackClientOption {
	return func(c *CallbackClient) {
		c.transportCreds = creds
	}
}

// Connect establishes the gRPC connection to the orchestrator.
// This must be called before any RPC methods can be invoked.
func (c *CallbackClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("client is closed")
	}

	// Check if already connected AND connection is actually healthy
	if c.connected && c.conn != nil {
		state := c.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return nil // Already connected and healthy
		}
		// Connection exists but is unhealthy - close and reconnect
		c.conn.Close()
		c.connected = false
	}

	// Build dial options
	var dialOpts []grpc.DialOption

	// Configure transport credentials: explicit TransportCredentials take precedence
	// (used in SPIFFE mode), then a TLS config, then insecure.
	switch {
	case c.transportCreds != nil:
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(c.transportCreds))
	case c.tlsConf != nil:
		creds := grpcCreds.NewTLS(c.tlsConf)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	default:
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Per-RPC credentials take precedence over the static token.
	if c.perRPCCreds != nil {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(c.perRPCCreds))
	}

	// Add keepalive configuration
	dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}))

	// Create context with timeout for connection establishment
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Establish connection
	conn, err := grpc.DialContext(connCtx, c.endpoint, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to orchestrator: %w", err)
	}

	c.conn = conn
	c.client = harnesspb.NewHarnessCallbackServiceClient(conn)
	c.connected = true

	// Wait for connection to be ready (with timeout)
	// This ensures the connection is actually established, not just dialed
	readyCtx, readyCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readyCancel()
	c.conn.WaitForStateChange(readyCtx, connectivity.Idle)
	state := c.conn.GetState()
	if state == connectivity.TransientFailure || state == connectivity.Shutdown {
		return fmt.Errorf("connection failed to establish: state=%s", state)
	}

	return nil
}

// TaskContextParams contains all the context parameters for RPC calls.
type TaskContextParams struct {
	TaskID          string
	AgentName       string
	MissionID       string
	TraceID         string
	SpanID          string
	MissionRunID    string // Unique ID for this mission execution
	AgentRunID      string // Unique ID for this agent execution
	RunNumber       int32  // Sequential run number (1, 2, 3...)
	ToolExecutionID string // ID for tool execution provenance
}

// SetFullContext updates the complete task context for subsequent RPC calls.
// This should be called at the start of each task execution with all available context.
func (c *CallbackClient) SetFullContext(params TaskContextParams) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.taskID = params.TaskID
	c.agentName = params.AgentName
	c.missionID = params.MissionID
	c.traceID = params.TraceID
	c.spanID = params.SpanID
	c.missionRunID = params.MissionRunID
	c.agentRunID = params.AgentRunID
	c.runNumber = params.RunNumber
	c.toolExecutionID = params.ToolExecutionID
}

// contextInfo builds the ContextInfo proto message with current task context.
func (c *CallbackClient) contextInfo() *harnesspb.ContextInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &harnesspb.ContextInfo{
		TaskId:          c.taskID,
		AgentName:       c.agentName,
		MissionId:       c.missionID,
		TraceId:         c.traceID,
		SpanId:          c.spanID,
		MissionRunId:    c.missionRunID,
		AgentRunId:      c.agentRunID,
		RunNumber:       c.runNumber,
		ToolExecutionId: c.toolExecutionID,
	}
}

// contextWithMetadata creates a context with authentication metadata if a token is set.
func (c *CallbackClient) contextWithMetadata(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + c.token,
	})
	return metadata.NewOutgoingContext(ctx, md)
}

// Close closes the gRPC connection and cleans up resources.
// The client cannot be reused after Close() is called.
func (c *CallbackClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.connected = false

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns true if the client is connected to the orchestrator.
// This checks both the internal state and the actual gRPC connection state.
func (c *CallbackClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.closed || c.conn == nil {
		return false
	}

	// Check actual gRPC connection state
	// Accept Ready, Idle, and Connecting as valid states (connection may be establishing)
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle || state == connectivity.Connecting
}

// ============================================================================
// LLM Operations
// ============================================================================

// LLMComplete performs an LLM completion request via the orchestrator.
func (c *CallbackClient) LLMComplete(ctx context.Context, req *harnesspb.LLMCompleteRequest) (*harnesspb.LLMCompleteResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("LLMComplete: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.LLMComplete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLMComplete: %w", err)
	}
	return resp, nil
}

// LLMCompleteWithTools performs an LLM completion with tool calling enabled.
func (c *CallbackClient) LLMCompleteWithTools(ctx context.Context, req *harnesspb.LLMCompleteWithToolsRequest) (*harnesspb.LLMCompleteWithToolsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("LLMCompleteWithTools: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.LLMCompleteWithTools(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLMCompleteWithTools: %w", err)
	}
	return resp, nil
}

// LLMCompleteStructured performs an LLM completion with structured output via the orchestrator.
func (c *CallbackClient) LLMCompleteStructured(ctx context.Context, req *harnesspb.LLMCompleteStructuredRequest) (*harnesspb.LLMCompleteStructuredResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("LLMCompleteStructured: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.LLMCompleteStructured(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLMCompleteStructured: %w", err)
	}
	return resp, nil
}

// LLMStream performs a streaming LLM completion request.
func (c *CallbackClient) LLMStream(ctx context.Context, req *harnesspb.LLMStreamRequest) (harnesspb.HarnessCallbackService_LLMStreamClient, error) {
	if !c.IsConnected() {
		return nil, errors.New("LLMStream: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.LLMStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLMStream: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Tool Operations
// ============================================================================

// CallToolProto invokes a tool via the orchestrator using proto-serialized JSON.
func (c *CallbackClient) CallToolProto(ctx context.Context, req *harnesspb.CallToolProtoRequest) (*harnesspb.CallToolProtoResponse, error) {
	// Try to reconnect if not connected
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return nil, fmt.Errorf("CallToolProto: client not connected and reconnect failed: %w", err)
		}
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.CallToolProto(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CallToolProto: %w", err)
	}
	return resp, nil
}

// CallToolProtoStream opens a server-streaming RPC for tool execution with callbacks.
// The returned stream must be consumed by the caller (or closed on context cancel).
func (c *CallbackClient) CallToolProtoStream(ctx context.Context, req *harnesspb.CallToolProtoStreamRequest) (harnesspb.HarnessCallbackService_CallToolProtoStreamClient, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return nil, fmt.Errorf("CallToolProtoStream: client not connected and reconnect failed: %w", err)
		}
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	stream, err := c.client.CallToolProtoStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CallToolProtoStream: %w", err)
	}
	return stream, nil
}

// ListTools retrieves the list of available tools.
func (c *CallbackClient) ListTools(ctx context.Context, req *harnesspb.ListToolsRequest) (*harnesspb.ListToolsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ListTools: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ListTools(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListTools: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Plugin Operations
// ============================================================================

// QueryPlugin sends a query to a plugin via the orchestrator.
func (c *CallbackClient) QueryPlugin(ctx context.Context, req *harnesspb.QueryPluginRequest) (*harnesspb.QueryPluginResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("QueryPlugin: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.QueryPlugin(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("QueryPlugin: %w", err)
	}
	return resp, nil
}

// ListPlugins retrieves the list of available plugins.
func (c *CallbackClient) ListPlugins(ctx context.Context, req *harnesspb.ListPluginsRequest) (*harnesspb.ListPluginsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ListPlugins: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ListPlugins(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListPlugins: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Agent Operations
// ============================================================================

// DelegateToAgent delegates a task to another agent.
func (c *CallbackClient) DelegateToAgent(ctx context.Context, req *harnesspb.DelegateToAgentRequest) (*harnesspb.DelegateToAgentResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("DelegateToAgent: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.DelegateToAgent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("DelegateToAgent: %w", err)
	}
	return resp, nil
}

// ListAgents retrieves the list of available agents.
func (c *CallbackClient) ListAgents(ctx context.Context, req *harnesspb.ListAgentsRequest) (*harnesspb.ListAgentsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ListAgents: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ListAgents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListAgents: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Finding Operations
// ============================================================================

// SubmitFinding submits a security finding to the orchestrator.
func (c *CallbackClient) SubmitFinding(ctx context.Context, req *harnesspb.SubmitFindingRequest) (*harnesspb.SubmitFindingResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("SubmitFinding: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.SubmitFinding(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("SubmitFinding: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Memory Operations
// ============================================================================

// ============================================================================
// Mission Memory Operations
// ============================================================================

// ============================================================================
// Long-Term Memory Operations
// ============================================================================

// ============================================================================
// GraphRAG Query Operations
// ============================================================================

// ============================================================================
// GraphRAG Storage Operations
// ============================================================================

// ============================================================================
// Planning Operations
// ============================================================================

// GetPlanContext retrieves the planning context from the orchestrator.
func (c *CallbackClient) GetPlanContext(ctx context.Context, req *harnesspb.GetPlanContextRequest) (*harnesspb.GetPlanContextResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetPlanContext: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetPlanContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetPlanContext: %w", err)
	}
	return resp, nil
}

// ReportStepHints reports step hints to the orchestrator.
func (c *CallbackClient) ReportStepHints(ctx context.Context, req *harnesspb.ReportStepHintsRequest) (*harnesspb.ReportStepHintsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ReportStepHints: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ReportStepHints(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ReportStepHints: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Tracing Operations
// ============================================================================

// RecordSpans sends a batch of spans to the orchestrator for distributed tracing.
func (c *CallbackClient) RecordSpans(ctx context.Context, req *harnesspb.RecordSpansRequest) (*harnesspb.RecordSpansResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("RecordSpans: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.RecordSpans(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RecordSpans: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Credential Operations
// ============================================================================

// GetCredential retrieves a credential by name from the orchestrator's credential store.
func (c *CallbackClient) GetCredential(ctx context.Context, req *harnesspb.GetCredentialRequest) (*harnesspb.GetCredentialResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetCredential: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetCredential(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetCredential: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Taxonomy Operations
// ============================================================================

// GetTaxonomySchema retrieves the full taxonomy schema from the orchestrator.
func (c *CallbackClient) GetTaxonomySchema(ctx context.Context, req *harnesspb.GetTaxonomySchemaRequest) (*harnesspb.GetTaxonomySchemaResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetTaxonomySchema: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetTaxonomySchema(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetTaxonomySchema: %w", err)
	}
	return resp, nil
}

// GenerateNodeID generates a deterministic node ID using taxonomy templates.
func (c *CallbackClient) GenerateNodeID(ctx context.Context, req *harnesspb.GenerateNodeIDRequest) (*harnesspb.GenerateNodeIDResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GenerateNodeID: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GenerateNodeID(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GenerateNodeID: %w", err)
	}
	return resp, nil
}

// ValidateFinding validates a finding against the taxonomy schema.
func (c *CallbackClient) ValidateFinding(ctx context.Context, req *harnesspb.ValidateFindingRequest) (*harnesspb.ValidateFindingResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ValidateFinding: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ValidateFinding(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ValidateFinding: %w", err)
	}
	return resp, nil
}

// ValidateGraphNode validates a graph node against the taxonomy schema.
func (c *CallbackClient) ValidateGraphNode(ctx context.Context, req *harnesspb.ValidateGraphNodeRequest) (*harnesspb.ValidateGraphNodeResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ValidateGraphNode: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ValidateGraphNode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ValidateGraphNode: %w", err)
	}
	return resp, nil
}

// ValidateRelationship validates a relationship against the taxonomy schema.
func (c *CallbackClient) ValidateRelationship(ctx context.Context, req *harnesspb.ValidateRelationshipRequest) (*harnesspb.ValidateRelationshipResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ValidateRelationship: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ValidateRelationship(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ValidateRelationship: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Proto-Canonical GraphRAG Operations
// ============================================================================

// Observe emits a typed observation into the World (ADR-0007).
func (c *CallbackClient) Observe(ctx context.Context, req *harnesspb.ObserveRequest) (*harnesspb.ObserveResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("Observe: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.Observe(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Observe: %w", err)
	}
	return resp, nil
}

// WorldView fetches the caller's slice of the tenant World (ADR-0012). The
// context is stamped here, as on every other callback: it addresses the harness
// the daemon should consult, and the daemon reads tenant and scope off that
// harness's mission record rather than off anything sent here.
func (c *CallbackClient) WorldView(ctx context.Context, req *harnesspb.WorldViewRequest) (*harnesspb.WorldViewResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorldView: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorldView(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorldView: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Tool Work Queue Operations
// ============================================================================

// QueueToolWork queues multiple tool invocations for parallel execution.
// Returns a job ID that can be used to retrieve results via ToolResults.
func (c *CallbackClient) QueueToolWork(ctx context.Context, req *harnesspb.QueueToolWorkRequest) (*harnesspb.QueueToolWorkResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("QueueToolWork: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.QueueToolWork(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("QueueToolWork: %w", err)
	}
	return resp, nil
}

// ToolResults returns a streaming client for receiving job results.
// The caller should call Recv() on the returned stream to receive results.
func (c *CallbackClient) ToolResults(ctx context.Context, req *harnesspb.ToolResultsRequest) (harnesspb.HarnessCallbackService_ToolResultsClient, error) {
	if !c.IsConnected() {
		return nil, errors.New("ToolResults: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	stream, err := c.client.ToolResults(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ToolResults: %w", err)
	}
	return stream, nil
}

// ============================================================================
// Mission Management Operations
// ============================================================================

// CreateMission creates a new mission from a mission definition.
func (c *CallbackClient) CreateMission(ctx context.Context, req *harnesspb.CreateMissionRequest) (*harnesspb.CreateMissionResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("CreateMission: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.CreateMission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CreateMission: %w", err)
	}
	return resp, nil
}

// RunMission queues a mission for execution.
func (c *CallbackClient) RunMission(ctx context.Context, req *harnesspb.RunMissionRequest) (*harnesspb.RunMissionResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("RunMission: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.RunMission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RunMission: %w", err)
	}
	return resp, nil
}

// GetMissionStatus retrieves the current status of a mission.
func (c *CallbackClient) GetMissionStatus(ctx context.Context, req *harnesspb.GetMissionStatusRequest) (*harnesspb.GetMissionStatusResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetMissionStatus: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetMissionStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetMissionStatus: %w", err)
	}
	return resp, nil
}

// WaitForMission blocks until a mission completes or times out.
func (c *CallbackClient) WaitForMission(ctx context.Context, req *harnesspb.WaitForMissionRequest) (*harnesspb.WaitForMissionResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WaitForMission: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WaitForMission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WaitForMission: %w", err)
	}
	return resp, nil
}

// ListMissions returns missions matching the provided filter criteria.
func (c *CallbackClient) ListMissions(ctx context.Context, req *harnesspb.ListMissionsRequest) (*harnesspb.ListMissionsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ListMissions: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ListMissions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListMissions: %w", err)
	}
	return resp, nil
}

// CancelMission requests cancellation of a running mission.
func (c *CallbackClient) CancelMission(ctx context.Context, req *harnesspb.CancelMissionRequest) (*harnesspb.CancelMissionResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("CancelMission: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.CancelMission(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CancelMission: %w", err)
	}
	return resp, nil
}

// GetMissionResults retrieves the final results of a completed mission.
func (c *CallbackClient) GetMissionResults(ctx context.Context, req *harnesspb.GetMissionResultsRequest) (*harnesspb.GetMissionResultsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetMissionResults: client not connected")
	}

	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetMissionResults(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetMissionResults: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Authorization Operations
// ============================================================================

// Authorize checks whether the current work execution is permitted to perform
// the given action on the given resource by forwarding the call to the daemon's
// HarnessCallbackService.Authorize RPC.
//
// The runID must be the mission run ID from the work envelope's AuthzContext so
// the daemon can resolve the calling user and tenant for the FGA check.
func (c *CallbackClient) Authorize(ctx context.Context, req *harnesspb.AuthorizeRequest) (*harnesspb.AuthorizeResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("Authorize: client not connected")
	}

	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.Authorize(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Authorize: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Workspace Operations
//
// Mirror the in-process workspace.Workspace surface (file IO + commit/push)
// for out-of-process callback agents. v1 is unary-only; streaming variants
// for files > 16 MB are deferred to a follow-on spec.
// ============================================================================

// WorkspaceList enumerates workspaces configured for the calling mission.
func (c *CallbackClient) WorkspaceList(ctx context.Context, req *harnesspb.WorkspaceListRequest) (*harnesspb.WorkspaceListResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspaceList: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspaceList(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceList: %w", err)
	}
	return resp, nil
}

// WorkspaceGetInfo returns name + path for the named workspace (empty
// name resolves to the mission's primary workspace).
func (c *CallbackClient) WorkspaceGetInfo(ctx context.Context, req *harnesspb.WorkspaceGetInfoRequest) (*harnesspb.WorkspaceGetInfoResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspaceGetInfo: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspaceGetInfo(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceGetInfo: %w", err)
	}
	return resp, nil
}

// WorkspaceReadFile reads a file from a workspace.
func (c *CallbackClient) WorkspaceReadFile(ctx context.Context, req *harnesspb.WorkspaceReadFileRequest) (*harnesspb.WorkspaceReadFileResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspaceReadFile: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspaceReadFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceReadFile: %w", err)
	}
	return resp, nil
}

// WorkspaceWriteFile writes content to a file in a workspace.
func (c *CallbackClient) WorkspaceWriteFile(ctx context.Context, req *harnesspb.WorkspaceWriteFileRequest) (*harnesspb.WorkspaceWriteFileResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspaceWriteFile: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspaceWriteFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceWriteFile: %w", err)
	}
	return resp, nil
}

// WorkspaceListFiles enumerates files matching a glob pattern.
func (c *CallbackClient) WorkspaceListFiles(ctx context.Context, req *harnesspb.WorkspaceListFilesRequest) (*harnesspb.WorkspaceListFilesResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspaceListFiles: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspaceListFiles(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceListFiles: %w", err)
	}
	return resp, nil
}

// WorkspaceCommit stages all changes and creates a commit.
func (c *CallbackClient) WorkspaceCommit(ctx context.Context, req *harnesspb.WorkspaceCommitRequest) (*harnesspb.WorkspaceCommitResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspaceCommit: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspaceCommit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceCommit: %w", err)
	}
	return resp, nil
}

// WorkspacePush pushes committed changes to the remote.
func (c *CallbackClient) WorkspacePush(ctx context.Context, req *harnesspb.WorkspacePushRequest) (*harnesspb.WorkspacePushResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("WorkspacePush: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.WorkspacePush(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("WorkspacePush: %w", err)
	}
	return resp, nil
}

// ============================================================================
// Knowledge Operations
// ============================================================================
//
// Thin passthroughs, in the shape every other call on this client uses: attach
// the task ContextInfo, attach the auth metadata, forward. The daemon resolves
// tenant from that context — no method here takes a tenant, so an agent cannot
// name another tenant's graph.

// QueryNodes searches the tenant knowledge graph over the task-scoped callback transport.
func (c *CallbackClient) QueryNodes(ctx context.Context, req *harnesspb.QueryNodesRequest) (*harnesspb.QueryNodesResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("QueryNodes: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.QueryNodes(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("QueryNodes: %w", err)
	}
	return resp, nil
}

// FindSimilarAttacks returns attack patterns semantically close to content over the task-scoped callback transport.
func (c *CallbackClient) FindSimilarAttacks(ctx context.Context, req *harnesspb.FindSimilarAttacksRequest) (*harnesspb.FindSimilarAttacksResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("FindSimilarAttacks: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.FindSimilarAttacks(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("FindSimilarAttacks: %w", err)
	}
	return resp, nil
}

// GetAttackChains returns multi-hop technique paths from a starting technique over the task-scoped callback transport.
func (c *CallbackClient) GetAttackChains(ctx context.Context, req *harnesspb.GetAttackChainsRequest) (*harnesspb.GetAttackChainsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetAttackChains: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetAttackChains(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetAttackChains: %w", err)
	}
	return resp, nil
}

// FindSimilarFindings returns findings semantically close to the given one over the task-scoped callback transport.
func (c *CallbackClient) FindSimilarFindings(ctx context.Context, req *harnesspb.FindSimilarFindingsRequest) (*harnesspb.FindSimilarFindingsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("FindSimilarFindings: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.FindSimilarFindings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("FindSimilarFindings: %w", err)
	}
	return resp, nil
}

// GetRelatedFindings returns findings reachable from the given one by graph relationship over the task-scoped callback transport.
func (c *CallbackClient) GetRelatedFindings(ctx context.Context, req *harnesspb.GetRelatedFindingsRequest) (*harnesspb.GetRelatedFindingsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetRelatedFindings: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetRelatedFindings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetRelatedFindings: %w", err)
	}
	return resp, nil
}

// GetFindings returns previously submitted findings matching a filter over the task-scoped callback transport.
func (c *CallbackClient) GetFindings(ctx context.Context, req *harnesspb.GetFindingsRequest) (*harnesspb.GetFindingsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetFindings: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetFindings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetFindings: %w", err)
	}
	return resp, nil
}

// ApplicationFindings returns one Application's Findings with their lifecycle
// reachability and exposure.
func (c *CallbackClient) ApplicationFindings(ctx context.Context, req *harnesspb.ApplicationFindingsRequest) (*harnesspb.ApplicationFindingsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("ApplicationFindings: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.ApplicationFindings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ApplicationFindings: %w", err)
	}
	return resp, nil
}

// GetRunFindings returns findings from earlier runs of this mission over the task-scoped callback transport.
func (c *CallbackClient) GetRunFindings(ctx context.Context, req *harnesspb.GetRunFindingsRequest) (*harnesspb.GetRunFindingsResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetRunFindings: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetRunFindings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetRunFindings: %w", err)
	}
	return resp, nil
}

// GetMissionRunHistory returns every run of the caller’s mission over the task-scoped callback transport.
func (c *CallbackClient) GetMissionRunHistory(ctx context.Context, req *harnesspb.GetMissionRunHistoryRequest) (*harnesspb.GetMissionRunHistoryResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("GetMissionRunHistory: client not connected")
	}
	req.Context = c.contextInfo()
	ctx = c.contextWithMetadata(ctx)
	resp, err := c.client.GetMissionRunHistory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetMissionRunHistory: %w", err)
	}
	return resp, nil
}
