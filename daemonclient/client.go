// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package daemonclient provides a client library for connecting to the Gibson daemon.
//
// The daemonclient package implements the client-side of the daemon-client architecture,
// allowing CLI commands and SDK consumers to connect to the running daemon and invoke
// operational RPCs via gRPC. It handles connection management, streaming RPCs, and
// provides high-level convenience methods for common daemon operations.
//
// This package exposes only operational RPC methods (DaemonService). Admin RPCs
// (Shutdown, tenant management, billing, etc.) are not included.
package daemonclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	daemonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
	manifestpb "github.com/zeroroot-ai/sdk/api/gen/gibson/manifest/v1"
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

// bearerTokenCredential implements credentials.PerRPCCredentials for static bearer token auth.
// Used by the CLI and dashboard for API key and Auth.js / Zitadel session token authentication.
// Agents use capabilitygrant.Client.GRPCPerRPCCredentials() instead (see core/sdk/capabilitygrant/).
type bearerTokenCredential struct {
	token string
}

// NewTokenCredentials creates a PerRPCCredentials that injects
// "Authorization: Bearer <token>" into every outbound gRPC call.
//
// Accepted token shape: a Zitadel-issued JWT obtained via either
//   - OAuth2 client_credentials grant (machine-to-machine / service-acting), or
//   - OIDC authorization code flow (user-acting, forwarded by the dashboard).
//
// The Gibson daemon does NOT validate JWTs itself. All bearer tokens flow
// through Envoy, which runs jwt_authn against the platform Zitadel issuer and
// enforces audience == "gibson-platform". Any bearer string that is not a
// well-formed JWT validated by Envoy will be rejected with HTTP 401 before the
// request reaches the daemon.
//
// This helper is NOT a generic bearer-token wrapper. Do NOT use it with:
//   - Opaque tokens (Zitadel ACCESS_TOKEN_TYPE_BEARER — these are rejected by
//     Envoy jwt_authn because they lack a Header.Payload.Signature structure).
//   - API keys, session cookies, or third-party IdP tokens.
//   - Any token whose audience is not "gibson-platform".
//
// Agents and tools use capabilitygrant.Client.GRPCPerRPCCredentials() instead,
// which carries capability-grant JWTs scoped to a specific mission task.
//
// Spec: zero-trust-hardening Requirement 10.5.
func NewTokenCredentials(token string) credentials.PerRPCCredentials {
	return &bearerTokenCredential{token: token}
}

func (c *bearerTokenCredential) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	if c.token == "" {
		return nil, nil
	}
	return map[string]string{"authorization": "Bearer " + c.token}, nil
}

func (c *bearerTokenCredential) RequireTransportSecurity() bool { return true }

// Client represents a connection to the Gibson daemon.
//
// The client wraps a gRPC connection and provides high-level methods for
// interacting with the daemon. It supports both Unix socket and TCP connections,
// automatically handling the appropriate connection setup based on address format.
//
// Example usage:
//
//	// Connect using default address or GIBSON_DAEMON_ADDRESS env var
//	client, err := ConnectOrFail(ctx)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
//	// Or connect directly to an address
//	client, err := Connect(ctx, "localhost:50002")
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
//	// Use the client
//	status, err := client.Status(ctx)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Daemon running: %v\n", status.Running)
type Client struct {
	// conn is the underlying gRPC connection
	conn *grpc.ClientConn

	// daemon is the gRPC service client for daemon operational RPCs
	daemon daemonpb.DaemonServiceClient
}

// New auto-detects credentials from the environment and returns a connected client.
//
// Detection order:
//  1. SPIRE Workload API socket at /run/spire/sockets/agent.sock → mTLS
//  2. OIDC_CLIENT_CREDENTIALS_* env vars → OAuth2 client-credentials grant
//
// TLS is mandatory. The system trust store is used; set GIBSON_DAEMON_CA to a
// PEM file path to additionally trust a self-signed CA (Kind/dev clusters).
//
// Returns an error if no credential is detected or TLS cannot be configured.
func New(ctx context.Context, daemonAddr string, opts ...Option) (*Client, error) {
	perRPC, tc, err := detectCredentials(ctx)
	if err != nil {
		return nil, err
	}
	return dial(ctx, daemonAddr, perRPC, tc)
}

// NewWithCredentials lets the caller provide credentials explicitly.
// The supplied PerRPCCredentials are used directly; TLS is still mandatory and
// is configured from the system trust store + GIBSON_DAEMON_CA as in New.
// Intended for testing and advanced use cases; prefer New in production code.
func NewWithCredentials(ctx context.Context, daemonAddr string, creds credentials.PerRPCCredentials, opts ...Option) (*Client, error) {
	tc, err := buildTLSTransport()
	if err != nil {
		return nil, fmt.Errorf("TLS setup failed: %w", err)
	}
	return dial(ctx, daemonAddr, creds, tc)
}

// Connect establishes a connection to the Gibson daemon at the specified address.
//
// Deprecated: use New which auto-detects credentials and enforces TLS. Connect
// is kept for backward compatibility; it auto-detects credentials identically to New.
func Connect(ctx context.Context, address string) (*Client, error) {
	if address == "" {
		return nil, errors.New("daemon address cannot be empty")
	}
	return New(ctx, address)
}

// connectInternal is used by ConnectWithOptions (legacy path with explicit token).
func connectInternal(ctx context.Context, address, token string) (*Client, error) {
	if address == "" {
		return nil, errors.New("daemon address cannot be empty")
	}
	tc, err := buildTLSTransport()
	if err != nil {
		return nil, fmt.Errorf("TLS setup failed: %w", err)
	}
	var perRPC credentials.PerRPCCredentials
	if token != "" {
		perRPC = NewTokenCredentials(token)
	} else {
		var detErr error
		perRPC, _, detErr = detectCredentials(ctx)
		if detErr != nil {
			return nil, detErr
		}
	}
	return dial(ctx, address, perRPC, tc)
}

// dial is the shared low-level dialer. Both tc and perRPC may be non-nil.
func dial(ctx context.Context, address string, perRPC credentials.PerRPCCredentials, tc credentials.TransportCredentials) (*Client, error) {
	if address == "" {
		return nil, errors.New("daemon address cannot be empty")
	}

	// Add default timeout if context has none.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	// Format target.
	var target string
	if strings.HasPrefix(address, "unix://") {
		target = address
	} else if strings.HasPrefix(address, "/") {
		target = "unix://" + address
	} else {
		target = address
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(tc),
		grpc.WithBlock(),
	}
	if perRPC != nil {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(perRPC))
	}

	//nolint:staticcheck // grpc.DialContext is deprecated in newer grpc but required for WithBlock semantics.
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon at %s: %w", address, err)
	}

	return &Client{
		conn:   conn,
		daemon: daemonpb.NewDaemonServiceClient(conn),
	}, nil
}

// Close closes the connection to the daemon.
//
// This method should be called when the client is no longer needed, typically
// using defer immediately after successful connection:
//
//	client, err := Connect(ctx, address)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
// Returns:
//   - error: Non-nil if connection close fails
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Ping checks if the daemon is responsive.
//
// This is a lightweight health check that can be used to verify the daemon
// is running and responding to requests. Useful for status commands and
// connection validation.
//
// Parameters:
//   - ctx: Context for the RPC call
//
// Returns:
//   - error: Non-nil if daemon doesn't respond or responds with an error
//
// Example:
//
//	if err := client.Ping(ctx); err != nil {
//	    return fmt.Errorf("daemon not responding: %w", err)
//	}
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.daemon.Ping(ctx, &daemonpb.PingRequest{})
	if err != nil {
		// Wrap error with user-friendly message
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return errors.New("daemon not responding (connection unavailable)")
			case codes.DeadlineExceeded:
				return errors.New("daemon ping timeout")
			default:
				return fmt.Errorf("daemon ping failed: %s", st.Message())
			}
		}
		return fmt.Errorf("daemon ping failed: %w", err)
	}
	return nil
}

// Status retrieves the daemon's current status and health information.
//
// This method queries the daemon for comprehensive status including:
//   - Process state (running, PID, uptime)
//   - Service endpoints (registry, callback, gRPC)
//   - Component counts (agents, missions, active missions)
//
// Parameters:
//   - ctx: Context for the RPC call
//
// Returns:
//   - *DaemonStatus: Complete status information
//   - error: Non-nil if RPC fails or daemon is unhealthy
//
// Example:
//
//	status, err := client.Status(ctx)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Daemon uptime: %s\n", status.Uptime)
//	fmt.Printf("Registered agents: %d\n", status.AgentCount)
func (c *Client) Status(ctx context.Context) (*DaemonStatus, error) {
	resp, err := c.daemon.Status(ctx, &daemonpb.StatusRequest{})
	if err != nil {
		// Wrap error with user-friendly message
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon status request timeout")
			default:
				return nil, fmt.Errorf("failed to get daemon status: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to get daemon status: %w", err)
	}

	return convertProtoStatus(resp), nil
}

// ListAgents retrieves a list of all registered agents from the daemon.
//
// Parameters:
//   - ctx: Context for the RPC call
//
// Returns:
//   - []AgentInfo: List of agent information
//   - error: Non-nil if RPC fails
func (c *Client) ListAgents(ctx context.Context) ([]AgentInfo, error) {
	resp, err := c.daemon.ListAgents(ctx, &daemonpb.ListAgentsRequest{})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while listing agents")
			default:
				return nil, fmt.Errorf("failed to list agents: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return convertProtoAgents(resp.Agents), nil
}

// ListTools retrieves a list of all registered tools from the daemon.
//
// Parameters:
//   - ctx: Context for the RPC call
//
// Returns:
//   - []ToolInfo: List of tool information
//   - error: Non-nil if RPC fails
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	resp, err := c.daemon.ListTools(ctx, &daemonpb.ListToolsRequest{})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while listing tools")
			default:
				return nil, fmt.Errorf("failed to list tools: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return convertProtoTools(resp.Tools), nil
}

// ListPlugins retrieves a list of all registered plugins from the daemon.
//
// Parameters:
//   - ctx: Context for the RPC call
//
// Returns:
//   - []PluginInfo: List of plugin information
//   - error: Non-nil if RPC fails
func (c *Client) ListPlugins(ctx context.Context) ([]PluginInfo, error) {
	resp, err := c.daemon.ListPlugins(ctx, &daemonpb.ListPluginsRequest{})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while listing plugins")
			default:
				return nil, fmt.Errorf("failed to list plugins: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}

	return convertProtoPlugins(resp.Plugins), nil
}

// GetCapabilityManifest fetches the signed capability manifest for the
// caller's tenant. When agentPrincipalID is non-empty, the caller must
// be a tenant admin (enforced daemon-side) — the returned manifest
// describes the specified agent_principal's capabilities rather than
// the caller's own.
func (c *Client) GetCapabilityManifest(ctx context.Context, agentPrincipalID string) (*manifestpb.CapabilityManifest, error) {
	resp, err := c.daemon.GetCapabilityManifest(ctx, &manifestpb.GetCapabilityManifestRequest{
		AgentPrincipalId: agentPrincipalID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, fmt.Errorf("daemon not responding: %s", st.Message())
			case codes.PermissionDenied:
				return nil, fmt.Errorf("permission denied: %s", st.Message())
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while fetching manifest")
			default:
				return nil, fmt.Errorf("failed to fetch manifest: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	if resp == nil || resp.Manifest == nil {
		return nil, errors.New("daemon returned empty manifest response")
	}
	return resp.Manifest, nil
}

// QueryPlugin executes a method on a plugin and returns the result.
//
// Parameters:
//   - ctx: Context for the RPC call
//   - name: Plugin name (e.g., "scope-ingestion")
//   - method: Method name to execute (e.g., "list_programs")
//   - params: Parameters for the method (will be JSON-encoded)
//
// Returns:
//   - *PluginQueryResult: Query result including duration
//   - error: Non-nil if RPC fails or plugin returns an error
func (c *Client) QueryPlugin(ctx context.Context, name, method string, params map[string]any) (*PluginQueryResult, error) {
	resp, err := c.daemon.QueryPlugin(ctx, &daemonpb.QueryPluginRequest{
		Name:   name,
		Method: method,
		Params: mapToTypedMap(params),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while querying plugin")
			default:
				return nil, fmt.Errorf("failed to query plugin: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to query plugin: %w", err)
	}

	// Check for error in response
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	// Convert TypedValue result to any
	result := typedValueToAny(resp.Result)

	return &PluginQueryResult{
		Result:     result,
		DurationMs: resp.DurationMs,
	}, nil
}

// RunMission executes a registered mission definition against a registered
// target and streams events. Missions are invoked by reference only —
// mission YAML / file path invocation was removed under spec
// mission-api-only-cleanup.
//
// Parameters:
//   - ctx: Context for the mission execution (cancellation stops the mission)
//   - missionDefinitionID: ID of a mission definition previously registered via CreateMissionDefinition
//   - targetID: ID of a target previously registered via the target API
//   - variables: runtime variable overrides (nil for none)
//   - memoryContinuity: Memory continuity mode (isolated, inherit, shared) — empty defaults to isolated
//
// Returns:
//   - <-chan MissionEvent: Channel receiving mission events
//   - error: Non-nil if mission start fails
func (c *Client) RunMission(ctx context.Context, missionDefinitionID, targetID string, variables map[string]string, memoryContinuity string) (<-chan MissionEvent, error) {
	if missionDefinitionID == "" {
		return nil, errors.New("mission_definition_id is required")
	}
	if targetID == "" {
		return nil, errors.New("target_id is required")
	}

	req := &daemonpb.RunMissionRequest{
		MissionDefinitionId: missionDefinitionID,
		TargetId:            targetID,
		Variables:           variables,
		MemoryContinuity:    memoryContinuity,
	}

	// Start streaming RPC
	stream, err := c.daemon.RunMission(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, errors.New("mission definition or target not found")
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid mission run request: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to start mission: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to start mission: %w", err)
	}

	// Create event channel and spawn goroutine to read from stream
	eventChan := make(chan MissionEvent, 10) // Buffer to avoid blocking daemon
	go func() {
		defer close(eventChan)
		for {
			event, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case eventChan <- MissionEvent{
					Type:      "error",
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("stream error: %v", err),
				}:
				default:
				}
				return
			}

			select {
			case eventChan <- convertProtoMissionEvent(event):
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

// ListMissions retrieves a list of missions from the daemon.
//
// Parameters:
//   - ctx: Context for the RPC call
//   - activeOnly: If true, only return running/paused missions
//   - statusFilter: Filter by specific status (running, completed, failed, cancelled) - empty means all
//   - namePattern: Filter by mission name using pattern matching - empty means all
//   - limit: Maximum number of missions to return (0 = use default)
//   - offset: Number of missions to skip for pagination
//
// Returns:
//   - []MissionInfo: List of missions matching the filter
//   - int: Total count of missions (for pagination)
//   - error: Non-nil if RPC fails
func (c *Client) ListMissions(ctx context.Context, activeOnly bool, statusFilter, namePattern string, limit, offset int) ([]MissionInfo, int, error) {
	resp, err := c.daemon.ListMissions(ctx, &daemonpb.ListMissionsRequest{
		ActiveOnly:   activeOnly,
		StatusFilter: statusFilter,
		NamePattern:  namePattern,
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, 0, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, 0, errors.New("daemon request timeout while listing missions")
			default:
				return nil, 0, fmt.Errorf("failed to list missions: %s", st.Message())
			}
		}
		return nil, 0, fmt.Errorf("failed to list missions: %w", err)
	}

	missions := make([]MissionInfo, len(resp.Missions))
	for i, m := range resp.Missions {
		missions[i] = MissionInfo{
			ID:                  m.Id,
			Name:                m.Name,
			MissionDefinitionID: m.MissionDefinitionId,
			TargetID:            m.TargetId,
			Status:              m.Status,
			StartTime:           time.Unix(m.StartTime, 0),
			EndTime:             time.Unix(m.EndTime, 0),
			FindingCount:        int(m.FindingCount),
		}
	}

	return missions, int(resp.Total), nil
}

// StopMission stops a running mission via the daemon.
//
// Parameters:
//   - ctx: Context for the RPC call
//   - missionID: The unique identifier of the mission to stop
//   - force: If true, force-kill the mission immediately
//
// Returns:
//   - error: Non-nil if stop request fails
func (c *Client) StopMission(ctx context.Context, missionID string, force bool) error {
	resp, err := c.daemon.StopMission(ctx, &daemonpb.StopMissionRequest{
		MissionId: missionID,
		Force:     force,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return fmt.Errorf("mission not found: %s", missionID)
			case codes.FailedPrecondition:
				return fmt.Errorf("mission is not running: %s", missionID)
			default:
				return fmt.Errorf("failed to stop mission: %s", st.Message())
			}
		}
		return fmt.Errorf("failed to stop mission: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to stop mission: %s", resp.Message)
	}

	return nil
}

// CreateMission creates a new mission run referencing a pre-registered target
// and mission definition. Inline construction was removed under spec
// mission-api-only-cleanup — callers must register the definition via
// CreateMissionDefinition and the target via the target API first.
func (c *Client) CreateMission(ctx context.Context, opts CreateMissionOptions) (*CreateMissionResult, error) {
	if opts.Name == "" {
		return nil, errors.New("mission name is required")
	}
	if opts.TargetID == "" {
		return nil, errors.New("target_id is required")
	}
	if opts.MissionDefinitionID == "" {
		return nil, errors.New("mission_definition_id is required")
	}

	req := &daemonpb.CreateMissionRequest{
		Name:                opts.Name,
		Description:         opts.Description,
		TargetId:            opts.TargetID,
		MissionDefinitionId: opts.MissionDefinitionID,
		Variables:           opts.Variables,
		MemoryContinuity:    opts.MemoryContinuity,
		Metadata:            opts.Metadata,
	}

	resp, err := c.daemon.CreateMission(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid mission configuration: %s", st.Message())
			case codes.NotFound:
				return nil, fmt.Errorf("referenced entity not found: %s", st.Message())
			case codes.AlreadyExists:
				return nil, fmt.Errorf("mission already exists: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to create mission: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to create mission: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to create mission: %s", resp.Message)
	}

	result := &CreateMissionResult{
		Name:                opts.Name,
		MissionDefinitionID: opts.MissionDefinitionID,
	}

	if resp.Mission != nil {
		result.MissionID = resp.Mission.Id
		result.TargetID = resp.Mission.TargetId
		result.Name = resp.Mission.Name
		result.Status = "pending"
	}

	return result, nil
}

// CreateMissionDefinition registers a structured mission definition with the
// daemon. This is the API-only replacement for the removed InstallMission flow
// — the definition is sent as a proto message (in the CLI, serialized from
// JSON via protojson) and validated server-side.
func (c *Client) CreateMissionDefinition(ctx context.Context, def *missionpb.MissionDefinition) (*CreateMissionDefinitionResult, error) {
	if def == nil {
		return nil, errors.New("definition is required")
	}

	resp, err := c.daemon.CreateMissionDefinition(ctx, &daemonpb.CreateMissionDefinitionRequest{
		Definition: def,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid mission definition: %s", st.Message())
			case codes.AlreadyExists:
				return nil, fmt.Errorf("mission definition already exists: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to create mission definition: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to create mission definition: %w", err)
	}

	result := &CreateMissionDefinitionResult{
		MissionDefinitionID: resp.MissionDefinitionId,
	}
	if resp.Info != nil {
		result.Info = MissionDefinitionInfo{
			Name:        resp.Info.Name,
			Version:     resp.Info.Version,
			Description: resp.Info.Description,
			Source:      resp.Info.Source,
			InstalledAt: time.Unix(resp.Info.InstalledAt, 0),
			UpdatedAt:   time.Unix(resp.Info.UpdatedAt, 0),
			NodeCount:   int(resp.Info.NodeCount),
		}
	}
	return result, nil
}

// UpdateMissionDefinition replaces the content of an existing mission definition.
// The Name field of def is the lookup key; the server preserves the original
// server-assigned ID and timestamps. Returns an error wrapping codes.NotFound
// when no definition with that name exists.
// Spec: gibson#437.
func (c *Client) UpdateMissionDefinition(ctx context.Context, def *missionpb.MissionDefinition) (string, error) {
	if def == nil {
		return "", errors.New("definition is required")
	}
	if def.GetName() == "" {
		return "", errors.New("definition name is required")
	}

	resp, err := c.daemon.UpdateMissionDefinition(ctx, &daemonpb.UpdateMissionDefinitionRequest{
		Definition: def,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				return "", fmt.Errorf("mission definition %q not found", def.GetName())
			case codes.Unavailable:
				return "", errors.New("daemon not responding (is it running?)")
			case codes.InvalidArgument:
				return "", fmt.Errorf("invalid mission definition: %s", st.Message())
			default:
				return "", fmt.Errorf("failed to update mission definition: %s", st.Message())
			}
		}
		return "", fmt.Errorf("failed to update mission definition: %w", err)
	}
	return resp.GetMissionDefinitionId(), nil
}

// Subscribe subscribes to all daemon events for real-time updates.
//
// Parameters:
//   - ctx: Context for the subscription (cancellation stops the stream)
//
// Returns:
//   - <-chan Event: Channel receiving all daemon events
//   - error: Non-nil if subscription setup fails
func (c *Client) Subscribe(ctx context.Context) (<-chan Event, error) {
	stream, err := c.daemon.Subscribe(ctx, &daemonpb.SubscribeRequest{})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.PermissionDenied:
				return nil, errors.New("permission denied for event subscription")
			default:
				return nil, fmt.Errorf("failed to subscribe to events: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	eventChan := make(chan Event, 50) // Larger buffer for high-frequency events
	go func() {
		defer close(eventChan)
		for {
			event, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case eventChan <- Event{
					Type:      "error",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"error": err.Error(),
					},
				}:
				default:
				}
				return
			}

			select {
			case eventChan <- convertProtoEvent(event):
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

// StartAgent starts an agent by name.
func (c *Client) StartAgent(ctx context.Context, name string) (*StartResult, error) {
	return c.startComponent(ctx, "agent", name)
}

// StopAgent stops an agent by name.
func (c *Client) StopAgent(ctx context.Context, name string) (*StopResult, error) {
	return c.stopComponent(ctx, "agent", name, false)
}

// StartTool starts a tool by name.
func (c *Client) StartTool(ctx context.Context, name string) (*StartResult, error) {
	return c.startComponent(ctx, "tool", name)
}

// StopTool stops a tool by name.
func (c *Client) StopTool(ctx context.Context, name string) (*StopResult, error) {
	return c.stopComponent(ctx, "tool", name, false)
}

// StartPlugin starts a plugin by name.
func (c *Client) StartPlugin(ctx context.Context, name string) (*StartResult, error) {
	return c.startComponent(ctx, "plugin", name)
}

// StopPlugin stops a plugin by name.
func (c *Client) StopPlugin(ctx context.Context, name string) (*StopResult, error) {
	return c.stopComponent(ctx, "plugin", name, false)
}

// startComponent is the internal method that starts a component via the daemon.
func (c *Client) startComponent(ctx context.Context, kind, name string) (*StartResult, error) {
	resp, err := c.daemon.StartComponent(ctx, &daemonpb.StartComponentRequest{
		Kind: kind,
		Name: name,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, fmt.Errorf("component '%s' not found", name)
			case codes.AlreadyExists:
				return nil, fmt.Errorf("component '%s' is already running", name)
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid component kind or name: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to start component: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to start component: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to start component: %s", resp.Message)
	}

	return &StartResult{
		PID:     int(resp.Pid),
		Port:    int(resp.Port),
		LogPath: resp.LogPath,
	}, nil
}

// stopComponent is the internal method that stops a component via the daemon.
func (c *Client) stopComponent(ctx context.Context, kind, name string, force bool) (*StopResult, error) {
	resp, err := c.daemon.StopComponent(ctx, &daemonpb.StopComponentRequest{
		Kind:  kind,
		Name:  name,
		Force: force,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, fmt.Errorf("component '%s' is not running", name)
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid component kind or name: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to stop component: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to stop component: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to stop component: %s", resp.Message)
	}

	return &StopResult{
		StoppedCount: int(resp.StoppedCount),
		TotalCount:   int(resp.TotalCount),
	}, nil
}

// BuildAgent rebuilds an agent from source.
func (c *Client) BuildAgent(ctx context.Context, name string) (*BuildResult, error) {
	return c.buildComponent(ctx, "agent", name)
}

// BuildTool rebuilds a tool from source.
func (c *Client) BuildTool(ctx context.Context, name string) (*BuildResult, error) {
	return c.buildComponent(ctx, "tool", name)
}

// BuildPlugin rebuilds a plugin from source.
func (c *Client) BuildPlugin(ctx context.Context, name string) (*BuildResult, error) {
	return c.buildComponent(ctx, "plugin", name)
}

// buildComponent is the internal method that builds a component via the daemon.
func (c *Client) buildComponent(ctx context.Context, kind, name string) (*BuildResult, error) {
	resp, err := c.daemon.BuildComponent(ctx, &daemonpb.BuildComponentRequest{
		Kind: kind,
		Name: name,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, fmt.Errorf("component '%s' not found", name)
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid component kind or name: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to build component: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to build component: %w", err)
	}

	return &BuildResult{
		Success:  resp.Success,
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
		Duration: time.Duration(resp.DurationMs) * time.Millisecond,
	}, nil
}

// ShowAgent retrieves detailed information about an agent.
func (c *Client) ShowAgent(ctx context.Context, name string) (*ComponentInfo, error) {
	return c.showComponent(ctx, "agent", name)
}

// ShowTool retrieves detailed information about a tool.
func (c *Client) ShowTool(ctx context.Context, name string) (*ComponentInfo, error) {
	return c.showComponent(ctx, "tool", name)
}

// ShowPlugin retrieves detailed information about a plugin.
func (c *Client) ShowPlugin(ctx context.Context, name string) (*ComponentInfo, error) {
	return c.showComponent(ctx, "plugin", name)
}

// showComponent is the internal method that retrieves component details via the daemon.
func (c *Client) showComponent(ctx context.Context, kind, name string) (*ComponentInfo, error) {
	resp, err := c.daemon.ShowComponent(ctx, &daemonpb.ShowComponentRequest{
		Kind: kind,
		Name: name,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, fmt.Errorf("component '%s' not found", name)
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid component kind or name: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to get component info: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to get component info: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get component info: %s", resp.Message)
	}

	info := &ComponentInfo{
		Name:      resp.Name,
		Version:   resp.Version,
		Kind:      resp.Kind,
		Status:    resp.Status,
		Source:    resp.Source,
		RepoPath:  resp.RepoPath,
		BinPath:   resp.BinPath,
		Port:      int(resp.Port),
		PID:       int(resp.Pid),
		CreatedAt: time.Unix(resp.CreatedAt, 0),
		UpdatedAt: time.Unix(resp.UpdatedAt, 0),
		Manifest:  resp.ManifestInfo,
	}

	if resp.StartedAt > 0 {
		t := time.Unix(resp.StartedAt, 0)
		info.StartedAt = &t
	}
	if resp.StoppedAt > 0 {
		t := time.Unix(resp.StoppedAt, 0)
		info.StoppedAt = &t
	}

	return info, nil
}

// GetAgentLogs retrieves log entries for an agent.
func (c *Client) GetAgentLogs(ctx context.Context, name string, opts LogsOptions) (<-chan LogEntry, error) {
	return c.getComponentLogs(ctx, "agent", name, opts)
}

// GetToolLogs retrieves log entries for a tool.
func (c *Client) GetToolLogs(ctx context.Context, name string, opts LogsOptions) (<-chan LogEntry, error) {
	return c.getComponentLogs(ctx, "tool", name, opts)
}

// GetPluginLogs retrieves log entries for a plugin.
func (c *Client) GetPluginLogs(ctx context.Context, name string, opts LogsOptions) (<-chan LogEntry, error) {
	return c.getComponentLogs(ctx, "plugin", name, opts)
}

// getComponentLogs is the internal method that retrieves component logs via the daemon.
func (c *Client) getComponentLogs(ctx context.Context, kind, name string, opts LogsOptions) (<-chan LogEntry, error) {
	stream, err := c.daemon.GetComponentLogs(ctx, &daemonpb.GetComponentLogsRequest{
		Kind:   kind,
		Name:   name,
		Follow: opts.Follow,
		Lines:  int32(opts.Lines),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, fmt.Errorf("component '%s' not found or no logs available", name)
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid component kind or name: %s", st.Message())
			default:
				return nil, fmt.Errorf("failed to get component logs: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to get component logs: %w", err)
	}

	logChan := make(chan LogEntry, 50)
	go func() {
		defer close(logChan)
		for {
			entry, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				return
			}

			// Convert TypedMap fields to map[string]string
			var fields map[string]string
			if entry.Fields != nil {
				rawFields := typedMapToMap(entry.Fields)
				fields = make(map[string]string, len(rawFields))
				for k, v := range rawFields {
					fields[k] = fmt.Sprintf("%v", v)
				}
			}

			select {
			case logChan <- LogEntry{
				Timestamp: time.Unix(entry.Timestamp, 0),
				Level:     entry.Level,
				Message:   entry.Message,
				Fields:    fields,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, nil
}

// PauseMission pauses a running mission at the next clean checkpoint.
//
// Parameters:
//   - ctx: Context for the RPC call
//   - missionID: ID of the mission to pause
//   - force: If true, pause immediately without waiting for clean boundary
//
// Returns:
//   - checkpointID: ID of the checkpoint created during pause
//   - error: Non-nil if the pause operation fails
func (c *Client) PauseMission(ctx context.Context, missionID string, force bool) (string, error) {
	resp, err := c.daemon.PauseMission(ctx, &daemonpb.PauseMissionRequest{
		MissionId: missionID,
		Force:     force,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return "", errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return "", fmt.Errorf("mission not found: %s", missionID)
			case codes.FailedPrecondition:
				return "", fmt.Errorf("mission is not running: %s", missionID)
			default:
				return "", fmt.Errorf("failed to pause mission: %s", st.Message())
			}
		}
		return "", fmt.Errorf("failed to pause mission: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("failed to pause mission: %s", resp.Message)
	}

	return resp.CheckpointId, nil
}

// ResumeMission resumes a paused mission from its last checkpoint.
//
// Parameters:
//   - ctx: Context for the RPC call
//   - missionID: ID of the mission to resume
//   - fromCheckpoint: Optional specific checkpoint ID to resume from (empty for latest)
//
// Returns:
//   - <-chan MissionEvent: Channel streaming mission events during execution
//   - error: Non-nil if the resume operation fails to start
func (c *Client) ResumeMission(ctx context.Context, missionID string, fromCheckpoint string) (<-chan MissionEvent, error) {
	stream, err := c.daemon.ResumeMission(ctx, &daemonpb.ResumeMissionRequest{
		MissionId:    missionID,
		CheckpointId: fromCheckpoint,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, fmt.Errorf("mission not found: %s", missionID)
			case codes.FailedPrecondition:
				return nil, errors.New("mission cannot be resumed (wrong status or no checkpoint)")
			default:
				return nil, fmt.Errorf("failed to resume mission: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to resume mission: %w", err)
	}

	eventChan := make(chan MissionEvent, 10)
	go func() {
		defer close(eventChan)
		for {
			event, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				eventChan <- MissionEvent{
					Type:      "error",
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("stream error: %v", err),
				}
				return
			}

			var data map[string]interface{}
			if event.Data != nil {
				data = typedMapToMap(event.Data)
			}

			eventChan <- MissionEvent{
				Type:      event.EventType,
				Timestamp: time.Unix(event.Timestamp, 0),
				Message:   event.Message,
				Data:      data,
			}
		}
	}()

	return eventChan, nil
}

// GetMissionHistory returns all runs for a mission name.
//
// Parameters:
//   - ctx: Context for the RPC call
//   - name: Name of the mission to get history for
//   - limit: Maximum number of runs to return (0 for all)
//   - offset: Number of runs to skip for pagination
//
// Returns:
//   - []MissionRun: List of mission runs
//   - int: Total number of runs available
//   - error: Non-nil if the query fails
func (c *Client) GetMissionHistory(ctx context.Context, name string, limit, offset int) ([]MissionRun, int, error) {
	resp, err := c.daemon.GetMissionHistory(ctx, &daemonpb.GetMissionHistoryRequest{
		Name:   name,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, 0, errors.New("daemon not responding (is it running?)")
			case codes.NotFound:
				return nil, 0, fmt.Errorf("no missions found with name: %s", name)
			default:
				return nil, 0, fmt.Errorf("failed to get mission history: %s", st.Message())
			}
		}
		return nil, 0, fmt.Errorf("failed to get mission history: %w", err)
	}

	runs := make([]MissionRun, len(resp.Runs))
	for i, r := range resp.Runs {
		run := MissionRun{
			MissionID:     r.MissionId,
			RunNumber:     int(r.RunNumber),
			Status:        r.Status,
			CreatedAt:     time.Unix(r.CreatedAt, 0),
			FindingsCount: int(r.FindingsCount),
		}
		if r.CompletedAt > 0 {
			t := time.Unix(r.CompletedAt, 0)
			run.CompletedAt = &t
		}
		runs[i] = run
	}

	return runs, int(resp.Total), nil
}

// GetMissionDefinition retrieves the full structured proto for a single
// installed mission definition by name. Returns an error wrapping
// codes.NotFound when the name is not registered.
func (c *Client) GetMissionDefinition(ctx context.Context, name string) (*missionpb.MissionDefinition, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	resp, err := c.daemon.GetMissionDefinition(ctx, &daemonpb.GetMissionDefinitionRequest{Name: name})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				return nil, fmt.Errorf("mission definition %q not found", name)
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while fetching mission definition")
			default:
				return nil, fmt.Errorf("failed to get mission definition: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to get mission definition: %w", err)
	}
	if resp.GetDefinition() == nil {
		return nil, fmt.Errorf("daemon returned empty definition for %q", name)
	}
	return resp.GetDefinition(), nil
}

// ListMissionDefinitions lists all installed mission definitions.
func (c *Client) ListMissionDefinitions(ctx context.Context) ([]*MissionDefinition, error) {
	resp, err := c.daemon.ListMissionDefinitions(ctx, &daemonpb.ListMissionDefinitionsRequest{})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unavailable:
				return nil, errors.New("daemon not responding (is it running?)")
			case codes.DeadlineExceeded:
				return nil, errors.New("daemon request timeout while listing mission definitions")
			default:
				return nil, fmt.Errorf("failed to list mission definitions: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("failed to list mission definitions: %w", err)
	}

	defs := make([]*MissionDefinition, len(resp.Missions))
	for i, m := range resp.Missions {
		defs[i] = &MissionDefinition{
			Name:        m.Name,
			Version:     m.Version,
			Description: m.Description,
			Source:      m.Source,
			InstalledAt: time.Unix(m.InstalledAt, 0),
		}
	}

	return defs, nil
}
