// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package http

import (
	"context"
	"time"

	"github.com/zeroroot-ai/sdk/types"
)

// Config holds configuration for the HTTP health server.
// All fields have sensible defaults via DefaultConfig().
type Config struct {
	// Port is the HTTP port for health endpoints.
	// Default: 8080
	Port int

	// ReadTimeout is the maximum duration for reading requests.
	// This prevents slow clients from holding connections open indefinitely.
	// Default: 5s
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration for writing responses.
	// This ensures responses are sent in a timely manner.
	// Default: 10s
	WriteTimeout time.Duration

	// CheckTimeout is the maximum duration for running all health checks.
	// If checks exceed this timeout, they are marked as unhealthy.
	// Default: 5s
	CheckTimeout time.Duration

	// BindAddress is the IP address to bind the server to.
	// Use "0.0.0.0" for all interfaces (default) or "127.0.0.1"
	// for localhost-only binding.
	// Default: "0.0.0.0"
	BindAddress string
}

// DefaultConfig returns a Config with sensible defaults for production use.
// These defaults follow Kubernetes best practices for health probes:
//   - Quick liveness checks (< 100ms)
//   - Reasonable readiness check timeouts (5s)
//   - Standard health port (8080)
//   - Bind to all interfaces for container environments
func DefaultConfig() *Config {
	return &Config{
		Port:         8080,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		CheckTimeout: 5 * time.Second,
		BindAddress:  "0.0.0.0",
	}
}

// CheckFunc is a function that performs a health check and returns the result.
// Check functions should:
//   - Respect the context deadline for timeout handling
//   - Return quickly for liveness checks (< 100ms)
//   - Be safe to call concurrently
//   - Not panic (panics are recovered but should be avoided)
//
// Example:
//
//	func redisCheck(ctx context.Context) types.HealthStatus {
//	    if err := client.Ping(ctx).Err(); err != nil {
//	        return types.NewUnhealthyStatus("redis ping failed", map[string]any{
//	            "error": err.Error(),
//	        })
//	    }
//	    return types.NewHealthyStatus("redis is healthy")
//	}
type CheckFunc func(ctx context.Context) types.HealthStatus

// Response represents the JSON response structure for health endpoints.
// It follows a consistent format across all Gibson components.
type Response struct {
	// Status is the overall health state: "healthy", "degraded", or "unhealthy".
	Status string `json:"status"`

	// Timestamp is the time the health check was performed in RFC3339 format.
	Timestamp string `json:"timestamp"`

	// Message provides a human-readable summary of the health status.
	// Only included for non-healthy states.
	Message string `json:"message,omitempty"`

	// Checks contains detailed results for individual health checks.
	// Only included in readiness endpoint responses.
	Checks map[string]CheckResult `json:"checks,omitempty"`
}

// CheckResult represents the result of an individual health check.
// Each registered check produces one CheckResult in the response.
type CheckResult struct {
	// Status is the check's health state: "healthy", "degraded", or "unhealthy".
	Status string `json:"status"`

	// Message provides a human-readable description of the check result.
	Message string `json:"message,omitempty"`

	// Details contains additional diagnostic information specific to this check.
	// This can include error details, latency metrics, or connection info.
	Details map[string]any `json:"details,omitempty"`
}
