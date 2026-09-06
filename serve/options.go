// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"os"
	"time"
)

const (
	// DefaultSPIFFEEndpointSocket is the default path for the SPIRE Workload API socket.
	DefaultSPIFFEEndpointSocket = "/run/spire/sockets/agent.sock"

	// DefaultDaemonAddress is the default gRPC address for the Gibson daemon when using SPIFFE mTLS transport.
	DefaultDaemonAddress = "gibson.gibson.svc.cluster.local:50002"
)

// Option is a functional option for configuring a Server.
// Options provide a flexible way to customize server behavior
// without requiring a large number of constructor parameters.
type Option func(*Config)

// WithHealthEndpoint sets the path for the health check endpoint.
// This is reserved for future HTTP health endpoint support.
// Currently, health checks are exposed via gRPC health checking protocol.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithHealthEndpoint("/healthz"))
func WithHealthEndpoint(path string) Option {
	return func(c *Config) {
		c.HealthEndpoint = path
	}
}

// WithHealthPort sets the TCP port for the HTTP health server that runs
// alongside the platform serve loop. The health server exposes /healthz
// (liveness) and /readyz (readiness) endpoints for Kubernetes probes.
//
// Set to a negative value to disable the health server entirely. This is
// NOT recommended when running under Kubernetes, since the default helm
// chart probes will fail without it.
//
// The port may also be set via the GIBSON_HEALTH_PORT environment variable,
// which is read by DefaultConfig(). This option takes precedence over the
// env var.
//
// Default: 8080
//
// Example:
//
//	serve.Tool(myTool, serve.WithHealthPort(9091))
func WithHealthPort(port int) Option {
	return func(c *Config) {
		c.HealthPort = port
	}
}

// WithGracefulShutdown sets the maximum duration to wait for active
// requests to complete during graceful shutdown.
// After this timeout, the server will force shutdown.
//
// A longer timeout gives more time for long-running requests to complete,
// but delays shutdown. A shorter timeout causes faster shutdown but may
// interrupt active requests.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithGracefulShutdown(60*time.Second))
func WithGracefulShutdown(timeout time.Duration) Option {
	return func(c *Config) {
		c.GracefulTimeout = timeout
	}
}

// WithCapabilityGrant configures Capability Grant Protocol authentication.
// platformURL is the HTTPS URL of the Gibson dashboard
// (e.g., "https://platform.example.com").
//
// **Required.** The serve entry points return an error if PlatformURL is not set.
// Use this option or WithCapabilityGrantFromEnv to satisfy the requirement.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithCapabilityGrant("https://platform.gibson.ai"))
func WithCapabilityGrant(platformURL string) Option {
	return func(cfg *Config) {
		cfg.PlatformURL = platformURL
	}
}

// WithBootstrapToken sets the one-time host registration token used on the
// first call to Register. After the host key is persisted to disk the token
// is no longer needed for subsequent runs.
//
// Example:
//
//	serve.Agent(myAgent,
//	    serve.WithCapabilityGrant("https://platform.gibson.ai"),
//	    serve.WithBootstrapToken(os.Getenv("MY_BOOTSTRAP_TOKEN")),
//	)
func WithBootstrapToken(token string) Option {
	return func(cfg *Config) {
		cfg.BootstrapToken = token
	}
}

// WithHostKey sets the path to the Ed25519 host key file.
// The file is created automatically on first registration and reused on
// subsequent runs. The parent directory must be writable.
//
// Defaults to ~/.gibson/host_key.json when this option is not provided.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithHostKey("/var/lib/gibson/host_key.json"))
func WithHostKey(path string) Option {
	return func(cfg *Config) {
		cfg.HostKeyPath = path
	}
}

// WithCapabilityGrantFromEnv reads Capability Grant configuration from environment variables:
//
//	GIBSON_PLATFORM_URL          — platform HTTPS base URL (required)
//	GIBSON_AGENT_BOOTSTRAP_TOKEN — one-time registration token (only needed on first run; host key persists after that)
//	GIBSON_HOST_KEY_PATH         — path to Ed25519 host key file (optional)
//
// **Required.** The serve entry points return an error if PlatformURL is not set.
// GIBSON_PLATFORM_URL is the only required variable; GIBSON_AGENT_BOOTSTRAP_TOKEN
// is only needed on first run, after which the host key is persisted to disk and
// reused on subsequent runs.
//
// Environment variables that are empty or unset are silently ignored so that
// this option can be combined with WithBootstrapToken or WithHostKey without
// conflict.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithCapabilityGrantFromEnv())
func WithCapabilityGrantFromEnv() Option {
	return func(cfg *Config) {
		if v := os.Getenv("GIBSON_PLATFORM_URL"); v != "" {
			cfg.PlatformURL = v
		}
		if v := os.Getenv("GIBSON_AGENT_BOOTSTRAP_TOKEN"); v != "" {
			cfg.BootstrapToken = v
		}
		if v := os.Getenv("GIBSON_HOST_KEY_PATH"); v != "" {
			cfg.HostKeyPath = v
		}
	}
}

// WithPollInterval sets how often the component polls the platform for work.
// The platform config response may override this value at runtime.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithPollInterval(500*time.Millisecond))
func WithPollInterval(d time.Duration) Option {
	return func(c *Config) {
		c.PollInterval = d
	}
}

// WithHeartbeatInterval sets how often the component sends heartbeats to the platform.
// The platform config response may override this value at runtime.
//
// Example:
//
//	serve.Agent(myAgent, serve.WithHeartbeatInterval(30*time.Second))
func WithHeartbeatInterval(d time.Duration) Option {
	return func(c *Config) {
		c.HeartbeatInterval = d
	}
}

// WithSkipBinaryCheck disables the startup binary validation that normally
// checks all system dependencies declared in component.yaml before registering
// with the platform. Use this for development and testing scenarios where the
// real binary is not available.
//
// Example:
//
//	serve.Tool(myTool, serve.WithSkipBinaryCheck())
func WithSkipBinaryCheck() Option {
	return func(c *Config) {
		c.SkipBinaryCheck = true
	}
}

// WithExtractor sets an EntityExtractor that auto-populates proto field 100
// (DiscoveryResult) on tool responses after each successful ExecuteProto call.
// The extraction runs in the tool process before results are submitted to the
// platform. If extraction fails, the tool result is still submitted without
// discovery data.
//
// Example:
//
//	serve.Tool(myTool, serve.WithExtractor(&MyToolExtractor{}))
func WithExtractor(e EntityExtractor) Option {
	return func(c *Config) {
		c.Extractor = e
	}
}

// WithPlatform is a convenience option that sets both the platform URL and the
// bootstrap token in a single call. It is equivalent to calling WithCapabilityGrant
// and WithBootstrapToken separately.
//
// Example:
//
//	serve.Tool(myTool, serve.WithPlatform("https://platform.gibson.ai", os.Getenv("BOOTSTRAP_TOKEN")))
func WithPlatform(platformURL, bootstrapToken string) Option {
	return func(cfg *Config) {
		cfg.PlatformURL = platformURL
		cfg.BootstrapToken = bootstrapToken
	}
}

// WithPlatformFromEnv reads all platform configuration from environment variables.
// This is the recommended option for in-cluster components that may also have
// SPIFFE transport available.
//
// Environment variables read:
//
//	GIBSON_PLATFORM_URL          — platform HTTPS base URL (required)
//	GIBSON_AGENT_BOOTSTRAP_TOKEN — one-time registration token (only needed on first run)
//	GIBSON_HOST_KEY_PATH         — path to Ed25519 host key file (optional)
//	SPIFFE_ENDPOINT_SOCKET       — SPIRE Workload API socket (default: /run/spire/sockets/agent.sock)
//	GIBSON_DAEMON_ADDRESS        — daemon gRPC address for SPIFFE transport (default: gibson.gibson.svc.cluster.local:50002)
//
// CG vars (GIBSON_PLATFORM_URL) are required. If SPIFFE_ENDPOINT_SOCKET is set
// and the socket exists at runtime, the serve loop upgrades to SPIFFE mTLS
// transport while continuing to present CG JWTs as the application-level
// credential. SPIFFE is a transport upgrade, not a competing auth mode.
//
// Example:
//
//	serve.Tool(myTool, serve.WithPlatformFromEnv())
func WithPlatformFromEnv() Option {
	return func(cfg *Config) {
		// SPIFFE config
		if v := os.Getenv("SPIFFE_ENDPOINT_SOCKET"); v != "" {
			cfg.SPIFFEEndpointSocket = v
		} else if cfg.SPIFFEEndpointSocket == "" {
			cfg.SPIFFEEndpointSocket = DefaultSPIFFEEndpointSocket
		}
		if v := os.Getenv("GIBSON_DAEMON_ADDRESS"); v != "" {
			cfg.DaemonAddress = v
		} else if cfg.DaemonAddress == "" {
			cfg.DaemonAddress = DefaultDaemonAddress
		}
		// Capability Grant config
		if v := os.Getenv("GIBSON_PLATFORM_URL"); v != "" {
			cfg.PlatformURL = v
		}
		if v := os.Getenv("GIBSON_AGENT_BOOTSTRAP_TOKEN"); v != "" {
			cfg.BootstrapToken = v
		}
		if v := os.Getenv("GIBSON_HOST_KEY_PATH"); v != "" {
			cfg.HostKeyPath = v
		}
	}
}

// WithAuthzFailOpen enables fail-open mode for authorization service unavailability.
// When the daemon's Authorize RPC is unreachable, the component will log a WARN
// and proceed with the operation instead of denying it (fail-closed default).
//
// This mode is intended for development environments. Do not enable in production.
func WithAuthzFailOpen() Option {
	return func(c *Config) {
		c.AuthzFailOpen = true
	}
}

// WithSPIFFE is an optional in-cluster transport upgrade that configures SPIFFE
// mTLS by setting the SPIRE Workload API socket path and the daemon gRPC address.
// When the socket exists at runtime, the serve loop dials the daemon over mTLS
// using SPIFFE SVIDs as the transport credential.
//
// This option upgrades the transport channel only; it does not replace Capability
// Grant as the application-level identity. Capability Grant (WithCapabilityGrant
// or WithCapabilityGrantFromEnv) must also be configured — CG JWTs are still
// presented to the platform even when SPIFFE mTLS is active.
//
// socketPath is the path to the SPIRE Agent Workload API Unix socket
// (e.g., "/run/spire/sockets/agent.sock"). If empty, DefaultSPIFFEEndpointSocket
// is used. daemonAddress is the gRPC host:port for the Gibson daemon
// (e.g., "gibson.gibson.svc.cluster.local:50002"). If empty, DefaultDaemonAddress
// is used.
//
// Example:
//
//	serve.Tool(myTool,
//	    serve.WithCapabilityGrantFromEnv(), // required
//	    serve.WithSPIFFE("", ""),           // optional transport upgrade
//	)
func WithSPIFFE(socketPath, daemonAddress string) Option {
	return func(cfg *Config) {
		if socketPath == "" {
			socketPath = DefaultSPIFFEEndpointSocket
		}
		if daemonAddress == "" {
			daemonAddress = DefaultDaemonAddress
		}
		cfg.SPIFFEEndpointSocket = socketPath
		cfg.DaemonAddress = daemonAddress
	}
}

// WithSPIFFEFromEnv is an optional in-cluster transport upgrade that reads SPIFFE
// configuration from environment variables:
//
//	SPIFFE_ENDPOINT_SOCKET — path to the SPIRE Workload API Unix socket
//	                         (default: /run/spire/sockets/agent.sock)
//	GIBSON_DAEMON_ADDRESS  — gRPC host:port of the Gibson daemon
//	                         (default: gibson.gibson.svc.cluster.local:50002)
//
// This option upgrades the transport channel only; it does not replace Capability
// Grant as the application-level identity. Capability Grant (WithCapabilityGrant
// or WithCapabilityGrantFromEnv) must also be configured — CG JWTs are still
// presented to the platform even when SPIFFE mTLS is active.
//
// This option is designed for in-cluster components where the SPIRE Agent socket
// is mounted at the standard path and the daemon address is injected via env var.
// Unset variables fall back to the defaults above.
//
// Example:
//
//	serve.Tool(myTool,
//	    serve.WithCapabilityGrantFromEnv(), // required
//	    serve.WithSPIFFEFromEnv(),          // optional transport upgrade
//	)
func WithSPIFFEFromEnv() Option {
	return func(cfg *Config) {
		socketPath := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
		if socketPath == "" {
			socketPath = DefaultSPIFFEEndpointSocket
		}
		daemonAddress := os.Getenv("GIBSON_DAEMON_ADDRESS")
		if daemonAddress == "" {
			daemonAddress = DefaultDaemonAddress
		}
		cfg.SPIFFEEndpointSocket = socketPath
		cfg.DaemonAddress = daemonAddress
	}
}
