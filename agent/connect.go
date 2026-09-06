// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package agent: connect.go implements the single-call Connect API that handles
// runtime-credential loading, SPIFFE auto-detection, and gRPC dial so agent
// authors write zero auth code.
//
// Usage:
//
//	client, err := agent.Connect(ctx, agent.ConnectConfig{})
//	if err != nil { ... }
//	defer client.Close()
//
// Connect loads the component's runtime credential (written by
// `gibson component register`, or GIBSON_AGENT_KEY for headless contexts) and
// dials the Gibson Envoy gateway at the registered URL, presenting a freshly
// signed Capability-Grant JWT in the x-capability-grant header on every RPC.
// This is the same credential the runtime serve loop and `gibson inspect` use —
// no Zitadel client_credentials token is ever minted (ADR-0045).
package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
	"github.com/zeroroot-ai/sdk/spiffe"
)

// ConnectConfig tunes Connect. All fields are optional: unset fields fall back
// to the registered install, environment variables, or documented defaults.
type ConnectConfig struct {
	// GibsonURL overrides the dial target. Defaults to the URL recorded in the
	// runtime install, then the GIBSON_URL environment variable.
	GibsonURL string

	// Name selects the install when more than one agent is registered on this
	// host (~/.gibson/agent/<name>.runtime.json). When empty, Connect
	// auto-detects the single agent install (or uses GIBSON_AGENT_KEY).
	Name string

	// Logger receives a single structured log line on successful connect
	// (gibson_url + agent id). If nil, no logging is performed.
	Logger ConnectLogger
}

// ConnectLogger is the minimal logging interface required by Connect.
type ConnectLogger interface {
	// Info logs a single key/value message. Called once on successful connect.
	Info(msg string, keysAndValues ...any)
}

// Client wraps a *grpc.ClientConn whose per-RPC credentials attach a fresh
// Capability-Grant JWT on every call. It is the sole type returned by Connect.
type Client struct {
	conn *grpc.ClientConn
}

// Conn returns the underlying gRPC connection. Use this to create service
// stubs (e.g. daemonpb.NewDaemonServiceClient(client.Conn())).
func (c *Client) Conn() *grpc.ClientConn { return c.conn }

// Close releases the connection. Safe to call on a nil receiver.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Typed sentinel errors returned by Connect.
var (
	// ErrMissingCredentials is returned when no runtime credential can be
	// resolved (the component has not been registered, or GIBSON_AGENT_KEY is
	// unset and no install file exists).
	ErrMissingCredentials = errors.New("agent: runtime credential not found")

	// ErrAmbiguousInstall is returned when multiple agent installs exist and
	// ConnectConfig.Name was not set to disambiguate.
	ErrAmbiguousInstall = errors.New("agent: multiple agent installs; set ConnectConfig.Name")

	// ErrDial is returned when the gRPC dial to the Gibson gateway fails.
	ErrDial = errors.New("agent: failed to dial Gibson gateway")
)

// Connect is the single-call entry point for agent authentication. It:
//  1. Resolves the runtime credential (GIBSON_AGENT_KEY, else the registered
//     install at ~/.gibson/agent/<name>.runtime.json).
//  2. Resolves the dial target (cfg.GibsonURL, else the install URL, else
//     GIBSON_URL).
//  3. Auto-detects the SPIFFE Workload API socket (mTLS when available, plain
//     TLS otherwise).
//  4. Returns a *Client whose per-RPC credentials sign a fresh agent+jwt and
//     attach it as the x-capability-grant header on every RPC.
func Connect(ctx context.Context, cfg ConnectConfig) (*Client, error) {
	return connect(ctx, cfg)
}

// connect is the testable core; extraDialOpts let tests substitute the
// transport (e.g. insecure creds against a bufconn server). They are appended
// last so a test-supplied WithTransportCredentials overrides the SPIFFE one.
func connect(ctx context.Context, cfg ConnectConfig, extraDialOpts ...grpc.DialOption) (*Client, error) {
	install, err := resolveAgentInstall(cfg.Name)
	if err != nil {
		return nil, err
	}

	gibsonURL := cfg.GibsonURL
	if gibsonURL == "" {
		gibsonURL = install.GibsonURL
	}
	if gibsonURL == "" {
		gibsonURL = os.Getenv(capabilitygrant.EnvGibsonURL)
	}
	if gibsonURL == "" {
		return nil, fmt.Errorf("%w: GIBSON_URL is not set and not in the runtime install", ErrDial)
	}

	perRPC, err := install.Credential.PerRPCCredentials()
	if err != nil {
		return nil, fmt.Errorf("agent: build capability-grant credentials: %w", err)
	}

	dialOpts, _, err := spiffe.DialOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: SPIFFE dial options: %w", ErrDial, err)
	}
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(perRPC))
	dialOpts = append(dialOpts, extraDialOpts...)

	target := normalizeTarget(gibsonURL)
	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s — %w", ErrDial, target, err)
	}

	if cfg.Logger != nil {
		cfg.Logger.Info("agent.Connect: connected",
			"gibson_url", gibsonURL,
			"agent_id", install.Credential.AgentID,
		)
	}

	return &Client{conn: conn}, nil
}

// resolveAgentInstall loads the agent runtime install. With GIBSON_AGENT_KEY set
// or an explicit name, it resolves directly; otherwise it auto-detects the
// single agent install on this host.
func resolveAgentInstall(name string) (capabilitygrant.RuntimeInstall, error) {
	if os.Getenv(capabilitygrant.EnvRuntimeCredential) != "" || name != "" {
		install, err := capabilitygrant.ResolveRuntimeInstall("agent", name)
		if err != nil {
			return capabilitygrant.RuntimeInstall{}, wrapResolveErr(err)
		}
		return install, nil
	}

	installs, err := capabilitygrant.ListInstalls()
	if err != nil {
		return capabilitygrant.RuntimeInstall{}, err
	}
	var agents []capabilitygrant.InstallRef
	for _, in := range installs {
		if in.Kind == "agent" {
			agents = append(agents, in)
		}
	}
	switch len(agents) {
	case 0:
		return capabilitygrant.RuntimeInstall{}, fmt.Errorf("%w: no agent registered — run `gibson component register --token <bootstrap-token>` first", ErrMissingCredentials)
	case 1:
		install, err := capabilitygrant.ResolveRuntimeInstall("agent", agents[0].Name)
		if err != nil {
			return capabilitygrant.RuntimeInstall{}, wrapResolveErr(err)
		}
		return install, nil
	default:
		return capabilitygrant.RuntimeInstall{}, ErrAmbiguousInstall
	}
}

// wrapResolveErr maps a not-found resolve error to ErrMissingCredentials.
func wrapResolveErr(err error) error {
	if err != nil && strings.Contains(err.Error(), "no runtime credential") {
		return fmt.Errorf("%w: %w", ErrMissingCredentials, err)
	}
	return err
}

// normalizeTarget converts a user-facing URL into a gRPC dial target.
// Rules: strip scheme, strip trailing /, append :443 if no port present.
func normalizeTarget(rawURL string) string {
	s := rawURL
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	if !strings.Contains(s, ":") {
		s += ":443"
	}
	return s
}
