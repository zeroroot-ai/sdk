// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/credentials"
)

// defaultHostKeyPath is used when ClientConfig.HostKeyPath is empty.
const defaultHostKeyPath = ".gibson/host_key.json"

// ClientConfig holds the configuration for a new Client.
type ClientConfig struct {
	// PlatformURL is the base HTTPS URL of the Gibson platform dashboard.
	// Example: "https://platform.example.com"
	PlatformURL string

	// BootstrapToken is an optional one-time credential for first-time host
	// registration. If empty, ResolveBootstrap will fall back to the Kubernetes
	// service account token. After registration the token is not stored.
	BootstrapToken string

	// HostKeyPath is the path to the on-disk host keypair (JWK JSON, 0600).
	// Defaults to ~/.gibson/host_key.json when empty.
	HostKeyPath string

	// AgentName is a human-readable label for this agent instance.
	AgentName string

	// AgentMode is the execution mode declared to the platform.
	// Supported values: "delegated", "autonomous".
	AgentMode string
}

// Capability is a single granted capability returned by the platform after
// successful registration.
type Capability struct {
	// Name is the capability identifier (e.g., "tool:mytool", "mission:create").
	Name string `json:"capability_name"`

	// ComponentRef is an optional reference to the platform component that
	// backs this capability.
	ComponentRef string `json:"component_ref"`

	// Description is a human-readable explanation of the capability.
	Description string `json:"description"`
}

// Client manages the lifecycle of a Gibson platform connection for an external
// agent. It is safe for concurrent use after construction.
//
// Typical usage:
//
//	c, err := capabilitygrant.NewClient(cfg)
//	// handle err
//	if err := c.Discover(ctx); err != nil { ... }
//	if err := c.Register(ctx); err != nil { ... }
//	// c.AgentID(), c.GRPCPerRPCCredentials() are now usable.
type Client struct {
	config   ClientConfig
	hostKey  *HostKey
	agentKey *AgentKey
	agentID  string
	hostID   string
	// componentScope is the FGA component identifier the dashboard assigned
	// to this agent at registration time. It must be included in every
	// agent+jwt minted for daemon RPCs; the daemon's FGA interceptor denies
	// calls whose JWTs lack it (spec R2). Populated by
	// applyRegistrationResponse; accessible via ComponentScope().
	componentScope string
	capabilities   []Capability
	discovery      *DiscoveryDocument
	httpClient     *http.Client
	logger         *slog.Logger

	// Revocation detector — populated lazily on first use by
	// RevocationUnaryInterceptor / RevocationStreamInterceptor.
	revocationInit sync.Once
	revocationDet  *revocationDetector

	// svidSource, when non-nil, fetches SPIFFE JWT-SVIDs from a local SPIRE
	// Workload API and takes precedence over host+jwt/bootstrap as the
	// enrollment credential (see buildRegistrationAuth). Populated by
	// NewClient only when SPIFFE_ENDPOINT_SOCKET is set; nil otherwise, which
	// preserves the pre-existing behavior for non-SPIFFE callers (tests,
	// external agents outside a SPIRE-enabled cluster).
	svidSource jwtSVIDSource

	mu sync.RWMutex
}

// jwtSVIDSource is the subset of *workloadapi.JWTSource the client depends on
// to mint a SPIFFE JWT-SVID enrollment credential. It is an unexported
// interface, rather than a direct *workloadapi.JWTSource field, purely so
// tests can inject a stub without a live SPIRE Workload API socket.
type jwtSVIDSource interface {
	FetchJWTSVID(ctx context.Context, params jwtsvid.Params) (*jwtsvid.SVID, error)
	Close() error
}

// NewClient creates a Client from cfg. It resolves the host key path and
// generates an ephemeral agent key, but does NOT yet contact the platform.
// Call Discover then Register to complete the bootstrap sequence.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.PlatformURL == "" {
		return nil, errors.New("capabilitygrant: NewClient: PlatformURL is required")
	}
	if cfg.AgentName == "" {
		return nil, errors.New("capabilitygrant: NewClient: AgentName is required")
	}
	if cfg.AgentMode == "" {
		cfg.AgentMode = "autonomous"
	}

	if cfg.HostKeyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("capabilitygrant: resolve home dir for host key: %w", err)
		}
		cfg.HostKeyPath = filepath.Join(home, defaultHostKeyPath)
	}

	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(cfg.HostKeyPath), 0700); err != nil {
		return nil, fmt.Errorf("capabilitygrant: create host key directory: %w", err)
	}

	hostKey, err := LoadOrGenerateHostKey(cfg.HostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: load or generate host key: %w", err)
	}

	agentKey, err := GenerateAgentKey()
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: generate agent key: %w", err)
	}

	c := &Client{
		config:     cfg,
		hostKey:    hostKey,
		agentKey:   agentKey,
		hostID:     hostKey.ID,
		httpClient: &http.Client{},
	}

	// When a SPIRE Workload API socket is available (in-cluster, mounted by the
	// SPIFFE CSI driver or hostPath), prefer presenting a JWT-SVID over the
	// host+jwt/bootstrap chain for the enrollment credential — see
	// buildRegistrationAuth. Absence of the socket is normal for external
	// agents and tests, so a missing or unreachable socket is not fatal here:
	// log it and fall back to the existing behavior.
	if socket := os.Getenv("SPIFFE_ENDPOINT_SOCKET"); socket != "" {
		// NewClient has no caller-supplied context and the source is long-lived
		// (it outlives any single Register call, refreshing SVIDs as needed for
		// re-registration on every process start), so context.Background is
		// deliberate here, not a shortcut.
		src, err := workloadapi.NewJWTSource(context.Background(),
			workloadapi.WithClientOptions(workloadapi.WithAddr(socket)))
		if err != nil {
			// c.logger is only populated by callers that opt into it; it is nil
			// at this point in construction, so log through slog.Default()
			// rather than risk a nil-pointer call (mirrors newRevocationDetector's
			// nil handling in revocation.go).
			slog.Default().Warn("capabilitygrant: create JWT-SVID source failed, "+
				"falling back to host+jwt/bootstrap enrollment credential",
				"socket", socket, "error", err)
		} else {
			c.svidSource = src
		}
	}

	return c, nil
}

// Close releases resources held by the client. In particular, it closes the
// SPIRE Workload API connection backing the JWT-SVID source, if one was
// created. Close is a no-op when no such source exists. It is safe to call
// Close even if NewClient's SPIFFE branch was never taken.
func (c *Client) Close() error {
	c.mu.RLock()
	src := c.svidSource
	c.mu.RUnlock()

	if src == nil {
		return nil
	}
	if err := src.Close(); err != nil {
		return fmt.Errorf("capabilitygrant: close JWT-SVID source: %w", err)
	}
	return nil
}

// Discover fetches the /.well-known/agent-configuration document from the
// platform and caches it. Must be called before Register.
func (c *Client) Discover(ctx context.Context) error {
	doc, err := Discover(ctx, c.config.PlatformURL, c.httpClient)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.discovery = doc
	c.mu.Unlock()

	return nil
}

// Register registers the host and agent with the platform.
//
// On first use (no prior registration), it requires a bootstrap credential —
// either ClientConfig.BootstrapToken or a Kubernetes service account token
// found at the well-known path. The bootstrap credential is used only for this
// request and is never stored.
//
// On subsequent calls (host key already on disk from a previous run), the client
// signs a host+jwt with the persisted key and presents that instead.
//
// After successful registration the client stores the returned agent ID and
// capabilities. Call AgentID() and Capabilities() to retrieve them.
func (c *Client) Register(ctx context.Context) error {
	c.mu.RLock()
	doc := c.discovery
	c.mu.RUnlock()

	if doc == nil {
		return errors.New("capabilitygrant: Register called before Discover")
	}

	registerURL := doc.Endpoints.Register
	if registerURL == "" {
		return errors.New("capabilitygrant: discovery document has no register endpoint")
	}

	// Build the Authorization header value. Prefer a SPIFFE JWT-SVID when a
	// Workload API source is available, then host+jwt when the host key was
	// loaded from disk (meaning it was previously registered), then fall back
	// to the bootstrap credential for first-time registration.
	authHeader, err := c.buildRegistrationAuth(ctx, registerURL)
	if err != nil {
		return fmt.Errorf("capabilitygrant: build registration auth: %w", err)
	}

	body, err := c.buildRegistrationBody()
	if err != nil {
		return fmt.Errorf("capabilitygrant: build registration body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("capabilitygrant: build registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("capabilitygrant: registration request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("capabilitygrant: read registration response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("capabilitygrant: registration returned HTTP %d: %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return c.applyRegistrationResponse(respBody)
}

// buildRegistrationAuth returns the Authorization header value for registration.
// Prefers a SPIFFE JWT-SVID (audience = registerURL) when a Workload API
// source is available — this is what lets a pod plugin enroll (and
// re-enroll on every restart) without a bootstrap token or a persisted host
// key ever leaving the pod. Otherwise uses host+jwt when we have an existing
// registration (host key already on disk), and the bootstrap credential for
// first-time registration.
func (c *Client) buildRegistrationAuth(ctx context.Context, registerURL string) (string, error) {
	c.mu.RLock()
	src := c.svidSource
	c.mu.RUnlock()

	// The SVID is re-presentable on every start (not just first registration):
	// unlike the bootstrap token, it is not a one-time credential, so there is
	// no first-vs-re-registration branch here — just try it whenever a source
	// exists, and fall through to the existing chain on any failure.
	if src != nil {
		svid, err := src.FetchJWTSVID(ctx, jwtsvid.Params{Audience: registerURL})
		if err == nil {
			return "Bearer " + svid.Marshal(), nil
		}
		slog.Default().Warn("capabilitygrant: fetch JWT-SVID failed, "+
			"falling back to host+jwt/bootstrap enrollment credential",
			"register_url", registerURL, "error", err)
	}

	// Try host+jwt next — works whenever the key file already existed before
	// NewClient was called (i.e., the host has registered before).
	hostJWT, err := SignHostJWT(c.hostKey, registerURL)
	if err == nil {
		// Determine whether this key was freshly generated or loaded from an
		// existing file. We do this by checking whether BootstrapToken is set
		// or whether the K8s SA token file exists.
		bootstrap, _ := ResolveBootstrap(c.config.BootstrapToken)
		if bootstrap != nil {
			// We have a bootstrap credential — use it for first-time registration.
			return "Bearer " + bootstrap.Token, nil
		}
		// No bootstrap credential — fall through to host+jwt (re-registration).
		return "Bearer " + hostJWT, nil
	}

	// If signing failed, fall back to bootstrap.
	bootstrap, bErr := ResolveBootstrap(c.config.BootstrapToken)
	if bErr != nil {
		return "", fmt.Errorf("sign host JWT failed (%w) and no bootstrap credential available: %w", err, bErr)
	}
	return "Bearer " + bootstrap.Token, nil
}

// registrationRequest is the JSON body sent to the register endpoint.
type registrationRequest struct {
	HostID      string          `json:"host_id"`
	AgentName   string          `json:"agent_name"`
	AgentMode   string          `json:"agent_mode"`
	HostKeyJWK  json.RawMessage `json:"host_key_jwk"`
	AgentKeyJWK json.RawMessage `json:"agent_key_jwk"`
}

// registrationResponse is the JSON body returned by the register endpoint.
type registrationResponse struct {
	AgentID      string       `json:"agent_id"`
	Capabilities []Capability `json:"capabilities"`
	// ComponentScope is the FGA component identifier the platform bound to
	// this installation. Required; an empty value means the platform is
	// running a pre-component_scope build and the agent will fail every
	// subsequent daemon RPC. No grace period exists on the daemon side.
	ComponentScope string `json:"component_scope"`
}

// buildRegistrationBody serialises the registration request JSON.
func (c *Client) buildRegistrationBody() ([]byte, error) {
	req := registrationRequest{
		HostID:      c.hostID,
		AgentName:   c.config.AgentName,
		AgentMode:   c.config.AgentMode,
		HostKeyJWK:  c.hostKey.PublicKeyJWK(),
		AgentKeyJWK: c.agentKey.PublicKeyJWK(),
	}
	return json.Marshal(req)
}

// applyRegistrationResponse parses the server response and updates the client
// with the assigned agent ID and granted capabilities.
func (c *Client) applyRegistrationResponse(body []byte) error {
	var resp registrationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("capabilitygrant: parse registration response: %w", err)
	}
	if resp.AgentID == "" {
		return errors.New("capabilitygrant: registration response missing agent_id")
	}
	if resp.ComponentScope == "" {
		return errors.New("capabilitygrant: registration response missing component_scope " +
			"(dashboard is running a pre-component_scope build; the daemon will " +
			"deny every subsequent RPC)")
	}

	c.mu.Lock()
	c.agentID = resp.AgentID
	c.capabilities = resp.Capabilities
	c.componentScope = resp.ComponentScope
	c.mu.Unlock()

	return nil
}

// ComponentScope returns the FGA component identifier bound to this agent
// installation by the platform at registration time. Returns an empty string
// if Register has not been called successfully yet.
func (c *Client) ComponentScope() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.componentScope
}

// AgentID returns the agent ID assigned by the platform after registration.
// Returns an empty string if Register has not been called successfully yet.
func (c *Client) AgentID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentID
}

// HostID returns the host ID (RFC 7638 JWK thumbprint of the host public key).
func (c *Client) HostID() string {
	// hostID is set at construction time and never changes.
	return c.hostID
}

// HasCapability returns true if the named capability was granted to this agent
// by the platform during registration.
func (c *Client) HasCapability(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, cap := range c.capabilities {
		if cap.Name == name {
			return true
		}
	}
	return false
}

// Capabilities returns a snapshot of all capabilities granted to this agent.
// The returned slice is a copy; mutations do not affect the client.
func (c *Client) Capabilities() []Capability {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Capability, len(c.capabilities))
	copy(out, c.capabilities)
	return out
}

// GRPCPerRPCCredentials returns a credentials.PerRPCCredentials implementation
// that signs a fresh agent+jwt for every outbound gRPC call. Pass this to the
// gRPC dial call used for harness callbacks to the daemon.
//
// Example:
//
//	conn, err := grpc.NewClient(c.PlatformURL(),
//	    grpc.WithPerRPCCredentials(c.GRPCPerRPCCredentials()),
//	    grpc.WithTransportCredentials(credentials.NewTLS(nil)),
//	)
func (c *Client) GRPCPerRPCCredentials() credentials.PerRPCCredentials {
	return &capabilityGrantCredentials{client: c}
}

// PlatformURL returns the platform base URL supplied at construction time.
// Used by the serve loop to determine the gRPC dial target.
func (c *Client) PlatformURL() string {
	return c.config.PlatformURL
}

// SetHTTPClient replaces the underlying *http.Client used for discovery and
// registration requests. This is useful when the caller needs a custom
// transport — for example, to trust a test server's self-signed TLS certificate
// or to configure mTLS.
//
// SetHTTPClient must not be called concurrently with Discover or Register.
func (c *Client) SetHTTPClient(hc *http.Client) {
	if hc == nil {
		return
	}
	c.httpClient = hc
}

// PatchDiscoveryRegisterURL overwrites the register endpoint URL in the cached
// discovery document. This is intended for tests that need to redirect
// registration traffic to a mock server whose address is only known at runtime
// (e.g., httptest.Server.URL).
//
// PatchDiscoveryRegisterURL is a no-op when Discover has not been called yet.
func (c *Client) PatchDiscoveryRegisterURL(registerURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.discovery != nil {
		c.discovery.Endpoints.Register = registerURL
	}
}

// capabilityGrantCredentials implements credentials.PerRPCCredentials.
// It signs a fresh agent+jwt for each gRPC call using the agent's ephemeral key.
type capabilityGrantCredentials struct {
	client *Client
}

// MetadataHeaderCapabilityGrant is the gRPC/HTTP metadata key carrying a
// component's self-signed per-RPC agent+jwt (ADR-0045, decision A). It is a
// DEDICATED header, NOT Authorization: a component never presents a Zitadel
// Bearer to the daemon, and ext-authz reads the CG-JWT from here so the
// Authorization slot stays free for human/S2S Zitadel traffic.
const MetadataHeaderCapabilityGrant = "x-capability-grant"

// perRPCMetadata signs a fresh agent+jwt bound to method and returns it under
// the dedicated x-capability-grant metadata header. Single source of truth for
// both the Client-backed and RuntimeCredential-backed per-RPC credentials.
func perRPCMetadata(key *AgentKey, hostID, agentID, componentScope, method string) (map[string]string, error) {
	token, err := SignAgentJWT(key, hostID, agentID, method, componentScope)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: sign per-RPC JWT: %w", err)
	}
	return map[string]string{MetadataHeaderCapabilityGrant: token}, nil
}

// requestMethodFromContext returns the full gRPC method ("/pkg.Service/Method")
// grpc-go recorded for this call. It is the only place the RPC method is
// available inside PerRPCCredentials: the uri arguments grpc-go passes to
// GetRequestMetadata carry the audience ("https://<authority>/<service>") and
// omit the method name, so they cannot bind the token to a single RPC.
func requestMethodFromContext(ctx context.Context) (string, error) {
	ri, ok := credentials.RequestInfoFromContext(ctx)
	if !ok || ri.Method == "" {
		return "", errors.New("capabilitygrant: no gRPC RequestInfo in context — cannot bind the agent+jwt to its method")
	}
	return ri.Method, nil
}

// GetRequestMetadata signs a fresh agent+jwt bound to the RPC method and returns
// it under the x-capability-grant header. The method is read from the gRPC
// RequestInfo in ctx (not from the uri arguments, which only name the service),
// so the token is useless at any other endpoint.
func (a *capabilityGrantCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	method, err := requestMethodFromContext(ctx)
	if err != nil {
		return nil, err
	}

	a.client.mu.RLock()
	hostID := a.client.hostID
	agentID := a.client.agentID
	key := a.client.agentKey
	scope := a.client.componentScope
	a.client.mu.RUnlock()

	return perRPCMetadata(key, hostID, agentID, scope, method)
}

// RequireTransportSecurity returns false to allow use over both TLS and
// plaintext connections (e.g., in tests). Production deployments should use TLS.
func (a *capabilityGrantCredentials) RequireTransportSecurity() bool { return false }
