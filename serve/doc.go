// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package serve provides gRPC server infrastructure for Gibson SDK components.
//
// This package enables agents, tools, and plugins to expose their functionality
// over gRPC, allowing them to be used by the Gibson framework and other clients.
// It handles server lifecycle, graceful shutdown, health checks, and signal handling.
//
// # Usage
//
// For Agents:
//
//	func main() {
//	    agent := &MyAgent{}
//	    if err := agent.Initialize(context.Background(), nil); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    err := serve.Agent(agent,
//	        serve.WithPort(50051),
//	        serve.WithGracefulShutdown(30*time.Second),
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// For Tools:
//
//	func main() {
//	    tool := &MyTool{}
//
//	    err := serve.Tool(tool,
//	        serve.WithPort(50052),
//	        serve.WithHealthEndpoint("/health"),
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// For Plugins:
//
//	func main() {
//	    plugin := &MyPlugin{}
//
//	    err := serve.Plugin(plugin,
//	        serve.WithPort(50053),
//	        serve.WithTLS("cert.pem", "key.pem"),
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// # Server Configuration
//
// The serve package provides flexible configuration through functional options:
//
//   - WithPort: Set the gRPC server port (default: 50051)
//   - WithHealthEndpoint: Set the health check endpoint path (default: /health)
//   - WithGracefulShutdown: Set the graceful shutdown timeout (default: 30s)
//   - WithTLS: Enable TLS with certificate and key files
//
// # Graceful Shutdown
//
// All servers handle SIGINT and SIGTERM signals for graceful shutdown:
//
//  1. Signal received
//  2. Server stops accepting new connections
//  3. Active requests complete within timeout period
//  4. Resources are cleaned up
//  5. Process exits
//
// # Health Checks
//
// All servers automatically expose gRPC health checks compatible with
// the standard gRPC health checking protocol. This allows load balancers
// and orchestration systems to monitor server health.
package serve
