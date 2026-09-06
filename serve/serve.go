// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/zeroroot-ai/sdk/extraction"
)

// validateConfig checks that the minimum required configuration is present.
// Capability Grant (PlatformURL) is always required; components cannot start
// without it. Returns a descriptive error if PlatformURL is empty.
func validateConfig(cfg *Config) error {
	if cfg.PlatformURL == "" {
		return errors.New("capability grant is required: set GIBSON_PLATFORM_URL or call WithCapabilityGrant()")
	}
	return nil
}

// useSPIFFETransport reports whether the SPIFFE transport upgrade should be
// used. SPIFFE is an optional in-cluster transport layer on top of Capability
// Grant identity — it is active only when both a socket path is configured and
// the socket file is present on disk.
func useSPIFFETransport(cfg *Config) bool {
	return cfg.SPIFFEEndpointSocket != "" && socketExists(cfg.SPIFFEEndpointSocket)
}

// socketExists reports whether the file at path exists (used for SPIFFE socket detection).
func socketExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EntityExtractor is an alias for extraction.EntityExtractor to avoid
// requiring users to import the extraction package separately.
type EntityExtractor = extraction.EntityExtractor

// Config holds serve configuration.
// It defines the platform connection settings, health check configuration,
// and graceful shutdown behavior.
type Config struct {
	// HealthPort is the TCP port on which the HTTP health server listens
	// in platform mode. The health server exposes /healthz (liveness) and
	// /readyz (readiness) endpoints for Kubernetes probes. Set to a negative
	// value to disable the health server entirely (not recommended in K8s).
	// Default: 8080
	HealthPort int

	// HealthEndpoint is the path for HTTP health checks.
	// This is not currently used but reserved for future HTTP health endpoint.
	// Default: /health
	HealthEndpoint string

	// GracefulTimeout is the maximum duration to wait for active requests
	// to complete during graceful shutdown.
	// Default: 30 seconds
	GracefulTimeout time.Duration

	// PlatformURL is the Gibson platform HTTPS base URL.
	// Required for platform mode. Set via WithCapabilityGrant(), WithCapabilityGrantFromEnv(),
	// or the GIBSON_PLATFORM_URL environment variable read by WithCapabilityGrantFromEnv().
	PlatformURL string

	// BootstrapToken is the one-time host registration credential used on the
	// first call to Register. After the host key is persisted to disk the token
	// is not needed for subsequent runs. Set via WithBootstrapToken() or the
	// GIBSON_AGENT_BOOTSTRAP_TOKEN environment variable read by WithCapabilityGrantFromEnv().
	BootstrapToken string

	// HostKeyPath is the path to the on-disk Ed25519 host keypair (JWK JSON, 0600).
	// Defaults to ~/.gibson/host_key.json when empty. Set via WithHostKey() or the
	// GIBSON_HOST_KEY_PATH environment variable read by WithCapabilityGrantFromEnv().
	HostKeyPath string

	// PollInterval is how often the component polls for work in platform mode.
	// Default: 1 second. Can be overridden by platform config response.
	PollInterval time.Duration

	// HeartbeatInterval is how often the component sends heartbeats in platform mode.
	// Default: 10 seconds. Can be overridden by platform config response.
	HeartbeatInterval time.Duration

	// SkipBinaryCheck disables startup validation of system dependencies
	// declared in component.yaml. Use for development/testing only.
	SkipBinaryCheck bool

	// Extractor is an optional EntityExtractor that auto-populates proto
	// field 100 (DiscoveryResult) on tool responses after ExecuteProto.
	// If nil, no extraction is performed and field 100 is left as-is.
	Extractor EntityExtractor

	// AuthzFailOpen controls the policy when the daemon's Authorize RPC is
	// unreachable:
	//   false (default) — fail-closed; treat Unavailable as deny.
	//   true            — fail-open (dev mode); log WARN and proceed.
	AuthzFailOpen bool

	// SPIFFEEndpointSocket is the path to the SPIRE Workload API Unix socket.
	// When set (and the socket exists), SPIFFE mode is used.
	// Set via WithSPIFFE() or read from SPIFFE_ENDPOINT_SOCKET by WithSPIFFEFromEnv().
	// Default: /run/spire/sockets/agent.sock
	SPIFFEEndpointSocket string

	// DaemonAddress is the gRPC target address for the Gibson daemon in SPIFFE mode.
	// Only used in SPIFFE mode. Set via WithSPIFFE() or read from GIBSON_DAEMON_ADDRESS
	// by WithSPIFFEFromEnv().
	// Default: gibson.gibson.svc.cluster.local:50002
	DaemonAddress string
}

// DefaultConfig returns default serve configuration.
func DefaultConfig() *Config {
	healthPort := 8080
	if envHealth := os.Getenv("GIBSON_HEALTH_PORT"); envHealth != "" {
		if p, err := strconv.Atoi(envHealth); err == nil {
			healthPort = p
		}
	}

	return &Config{
		HealthPort:        healthPort,
		HealthEndpoint:    "/health",
		GracefulTimeout:   30 * time.Second,
		PollInterval:      1 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}
}
