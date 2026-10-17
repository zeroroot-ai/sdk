// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package serve connects SDK components to the Gibson platform.
//
// An agent or a tool does not run a server for work. It dials the platform,
// registers itself, and polls for work. A plugin uses plugin.Serve instead.
//
// # Usage
//
// For agents, call sdk.ServeAgent or serve.Agent:
//
//	func main() {
//	    a, err := sdk.NewAgent(
//	        sdk.WithName("my-agent"),
//	        sdk.WithVersion("1.0.0"),
//	        sdk.WithDescription("My agent"),
//	        sdk.WithExecuteFunc(execute),
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    if err := serve.Agent(a); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// For tools:
//
//	func main() {
//	    if err := serve.Tool(&MyTool{}); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// # Configuration
//
// DefaultConfig reads the environment:
//
//   - GIBSON_URL: the platform URL. It is required.
//   - GIBSON_BOOTSTRAP_TOKEN: the token for the first registration.
//   - GIBSON_HOST_KEY_PATH: the path of the host key (optional).
//   - GIBSON_HEALTH_PORT: the port of the health server (default 8080).
//
// # Schema
//
// A tool started with the --schema flag prints its schema and exits:
//
//	./tool --schema
//
// # Shutdown
//
// SIGINT and SIGTERM cancel the work loop, and the component stops cleanly.
//
// # Health Checks
//
// The health server exposes /healthz for liveness and /readyz for readiness.
// /readyz returns 503 until the component is registered with the platform.
package serve
