// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 8080, cfg.HealthPort)
	assert.Equal(t, "/health", cfg.HealthEndpoint)
	assert.Equal(t, 30*time.Second, cfg.GracefulTimeout)
}

func TestValidateConfig_RequiresPlatformURL(t *testing.T) {
	cfg := DefaultConfig()
	// PlatformURL is empty by default
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability grant")
}

func TestValidateConfig_AcceptsFullCG(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PlatformURL = "https://platform.example.com"
	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestUseSPIFFETransport_SocketAbsent(t *testing.T) {
	cfg := DefaultConfig()
	// No socket path set — should return false
	assert.False(t, useSPIFFETransport(cfg))

	// Non-existent path — should return false
	cfg.SPIFFEEndpointSocket = "/tmp/nonexistent-spiffe-socket-does-not-exist"
	assert.False(t, useSPIFFETransport(cfg))
}

func TestUseSPIFFETransport_SocketPresent(t *testing.T) {
	f, err := os.CreateTemp("", "spiffe-socket-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.Close()

	cfg := DefaultConfig()
	cfg.SPIFFEEndpointSocket = f.Name()
	assert.True(t, useSPIFFETransport(cfg))
}
