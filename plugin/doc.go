// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package plugin is the Gibson plugin SDK.
//
// # Overview
//
// A plugin is a credential-bearing, stateful service integration — the only
// component class permitted to call [ResolveSecret]. Plugins are registered
// with the Gibson daemon, expose one or more typed RPC methods, and are invoked
// by tools via the daemon's PluginInvoke RPC (gibson.plugin.v1.PluginInvokeService).
//
// Plugin authors write business logic, declare a manifest, and call [Serve]
// from main. The SDK handles registration, secret resolution, method dispatch,
// lifecycle management, health probes, SIGTERM drain, and rotation events.
//
// # The Serve Entry Point
//
// [Serve] is the single function a plugin author calls from main:
//
//	func main() {
//	    if err := plugin.Serve(context.Background(),
//	        plugin.WithManifest("./plugin.yaml"),
//	        plugin.WithHandler("Echo", echoHandler),
//	    ); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// [Serve] does not return until the plugin has shut down cleanly or the context
// is cancelled. It returns the first fatal error, or nil on clean shutdown.
//
// # The Manifest Contract
//
// Every plugin declares a YAML manifest (plugin.yaml) at the root of its source
// directory. The manifest is validated at startup by the SDK, by
// `gibson component validate`, and by the daemon at registration time — the
// same [manifest.Validate] function backs all three call-sites.
//
// Minimal manifest example:
//
//	apiVersion: plugin.gibson.zeroroot.ai/v1
//	kind: Plugin
//	metadata:
//	  name: my-plugin
//	  version: 0.1.0
//	  description: My first plugin
//	spec:
//	  workload_class: plugin
//	  methods:
//	    - name: Echo
//	      description: Echo the request back to the caller
//	  secrets:
//	    - name: cred:api_key
//	      scope: startup
//	      rotation: live
//	      required: true
//	  runtime: process
//
// The manifest schema version is "plugin.gibson.zeroroot.ai/v1". Future
// versions are gated via the apiVersion field so parsing remains backward
// compatible.
//
// # Secret Consumption Model
//
// Plugins are the ONLY component class that may access credentials. [Serve]
// injects a broker-backed secrets client into every handler and lifecycle-hook
// context; recover the value with [ResolveSecret] (or [SecretsFromContext] for
// the client handle):
//
//	func echoHandler(ctx context.Context, req EchoRequest) (EchoResponse, error) {
//	    apiKey, err := plugin.ResolveSecret(ctx, "cred:api_key")
//	    if err != nil {
//	        return EchoResponse{}, fmt.Errorf("resolve api_key: %w", err)
//	    }
//	    // use apiKey — never log it, never include it in error messages
//	    _ = apiKey
//	    return EchoResponse{Echoed: req.Message}, nil
//	}
//
// Rules:
//   - NEVER log a resolved value, write it to stdout/stderr, include it in OTel
//     span attributes, or include it in any error message returned to a caller.
//   - Only names declared in spec.secrets may be resolved; the SDK rejects
//     undeclared names before any RPC.
//   - Resolved values are cached in-process with a default TTL of 60 seconds.
//
// # Three Runtime Modes
//
// Plugin author code is the same across all three modes. The SDK selects mode
// behaviour based on the GIBSON_PLUGIN_RUNTIME environment variable:
//
//   - process (default): laptop and CI. Network egress is informational only;
//     no enforcement occurs. Use `gibson component run` locally.
//
//   - pod: Kubernetes deployment. The daemon emits a NetworkPolicy at
//     registration time matching spec.egress[]. The SDK itself is a no-op in
//     the egress path; the cluster enforces.
//
//   - setec: Setec microVM. The SDK registers spec.egress[] with the Setec
//     orchestrator at startup; outbound traffic to undeclared targets is dropped
//     at the microVM boundary.
//
// # Lifecycle States
//
// A plugin progresses through:
//
//	Bootstrapping → Registering → ResolvingSecrets → Starting → Ready
//	Ready → Draining → Stopped
//	Ready → Degraded → Ready  (secret revocation / reconnect recovery)
//
// State transitions are logged with structured slog at Info level.
//
// Health endpoints (HTTP on the port configured by [WithHealthAddr]):
//   - /healthz: returns 200 once Ready; 503 before.
//   - /livez:   returns 200 when Ready or Degraded AND daemon heartbeat is fresh;
//     503 otherwise.
//
// # SIGTERM and Rotation-Restart Contracts
//
// On SIGTERM or SIGINT [Serve]:
//  1. Stops accepting new work from PollWork.
//  2. Waits for in-flight handlers to complete up to drainTimeout (default 30s).
//  3. Calls OnStop, transitions to Stopped, and returns nil.
//
// When a manifest secret with rotation=restart is rotated by the operator:
//  1. The events subscriber receives the secret_rotated event.
//  2. In-flight handlers are drained.
//  3. The process exits with code 75 — the rotation-restart sentinel.
//  4. The orchestrator (systemd, Kubernetes, Setec) restarts the plugin; the
//     new process resolves the rotated secret value at scope=startup.
//
// Exit code 75 is the canonical rotation-restart sentinel. Operators and
// orchestrators MUST restart the plugin when this code is observed.
//
// # See Also
//
//   - [manifest.Manifest] — manifest schema and loader.
//   - [lifecycle.StateMachine] — lifecycle state machine.
//   - [health.Server] — health probe endpoints.
//   - [pluginsecrets.Client] — credential resolution with caching.
//   - [events.Subscriber] — rotation and revocation event handling.
//   - [dispatch.Dispatcher] — PollWork → handler → SubmitResult dispatch loop.
//   - [egress.Enforcer] — runtime-mode-specific egress enforcement.
package plugin
