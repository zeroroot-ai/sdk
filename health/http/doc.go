// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package http provides an HTTP server for exposing health check endpoints.
//
// This package implements a reusable HTTP health server that can be integrated
// into any Gibson component to expose standard Kubernetes health endpoints:
// /healthz (liveness) and /readyz (readiness).
//
// # Usage
//
// Basic usage with default configuration:
//
//	server := http.NewServer(nil)
//	server.Start()
//	defer server.Stop(context.Background())
//
// Custom configuration with readiness checks:
//
//	cfg := &http.Config{
//	    Port: 8080,
//	    CheckTimeout: 5 * time.Second,
//	}
//	server := http.NewServer(cfg)
//
//	// Register readiness checks for dependencies
//	server.RegisterReadinessCheck("redis", http.RedisCheck(redisClient))
//	server.RegisterReadinessCheck("neo4j", http.Neo4jCheck(neo4jDriver))
//
//	server.Start()
//	defer server.Stop(context.Background())
//
// # Endpoints
//
// The server exposes two endpoints:
//
//   - GET /healthz - Liveness probe. Returns 200 if the process is alive.
//     This endpoint should be lightweight and return quickly (< 100ms).
//
//   - GET /readyz - Readiness probe. Returns 200 if all registered checks pass,
//     503 if any check fails. Checks run concurrently with configurable timeout.
//
// # Response Format
//
// All endpoints return JSON responses with consistent structure:
//
//	{
//	  "status": "healthy|degraded|unhealthy",
//	  "timestamp": "2026-03-06T20:45:00Z",
//	  "message": "optional error message",
//	  "checks": {
//	    "checkName": {
//	      "status": "healthy|degraded|unhealthy",
//	      "message": "check-specific message",
//	      "details": {...}
//	    }
//	  }
//	}
//
// # Integration with SDK Serve
//
// Components using core/sdk/serve expose health endpoints automatically.
// The HealthPort defaults to 8080 and can be overridden via WithHealthPort():
//
//	serve.Tool(myTool,
//	    serve.WithCapabilityGrantFromEnv(),
//	    serve.WithHealthPort(9091),
//	)
//
// # Pre-built Checks
//
// The package provides ready-to-use health check functions for common dependencies:
//
//   - BinaryCheck(name) - Verifies a binary exists in PATH
//   - RedisCheck(client) - Pings Redis server
//   - Neo4jCheck(driver) - Verifies Neo4j connectivity
//   - GRPCCheck(conn) - Calls gRPC health service
//   - TCPCheck(addr) - Tests TCP connectivity
//
// # Custom Checks
//
// Custom health checks can be implemented by providing a CheckFunc:
//
//	customCheck := func(ctx context.Context) types.HealthStatus {
//	    // Perform check logic
//	    if err := checkSomething(); err != nil {
//	        return types.NewUnhealthyStatus("check failed", map[string]any{
//	            "error": err.Error(),
//	        })
//	    }
//	    return types.NewHealthyStatus("check passed")
//	}
//	server.RegisterReadinessCheck("custom", customCheck)
//
// # Error Handling
//
// The server handles errors gracefully:
//
//   - Check timeouts return unhealthy status with "check timed out" message
//   - Panics in check functions are recovered and logged as unhealthy
//   - Server shutdown waits for in-flight requests to complete
//   - Failed checks are isolated (one failure doesn't crash the server)
//
// # Configuration
//
// All configuration parameters have sensible defaults:
//
//   - Port: 8080
//   - ReadTimeout: 5s
//   - WriteTimeout: 10s
//   - CheckTimeout: 5s
//   - BindAddress: 0.0.0.0 (all interfaces)
//
// Use DefaultConfig() to get a configuration with these defaults, then
// customize as needed.
package http
