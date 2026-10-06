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
// Plugin authors write business logic, declare the plugin in code, and call
// [Serve] from main. The SDK handles registration, secret resolution, method dispatch,
// lifecycle management, health probes, SIGTERM drain, and rotation events.
//
// # The Serve Entry Point
//
// [Serve] is the single function a plugin author calls from main:
//
//	func main() {
//	    if err := plugin.Serve(context.Background(),
//	        plugin.WithName("my-plugin"),
//	        plugin.WithVersion("0.1.0"),
//	        plugin.WithHandler("Echo", "echoes the request back unchanged", echoHandler),
//	        plugin.WithLifecycle(lifecycle.LifecycleHooks{OnStart: requireAPIKey}),
//	    ); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// [Serve] does not return until the plugin has shut down cleanly or the context
// is cancelled. It returns the first fatal error, or nil on clean shutdown.
//
// # The Declaration in Code
//
// A plugin declares itself in code and reports the declaration at start, over
// the RegisterComponent RPC (ADR-0097). No manifest file exists.
//
//   - [WithName] and [WithVersion] name the plugin. Both are required.
//   - Each [WithHandler] adds one method with its description. The handlers
//     are the method set; at least one is required.
//
// The plugin declares no secrets. A tenant admin grants the plugin access to
// named secrets in the deploy wizard, and the daemon checks that grant on each
// resolve.
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
// A plugin that cannot start without a secret resolves it in its OnStart hook
// and returns the error. [Serve] then fails at boot with the secret named:
//
//	func requireAPIKey(ctx context.Context) error {
//	    if _, err := plugin.ResolveSecret(ctx, "cred:api_key"); err != nil {
//	        return fmt.Errorf("resolve startup secret %q: %w", "cred:api_key", err)
//	    }
//	    return nil
//	}
//
// Rules:
//   - NEVER log a resolved value, write it to stdout/stderr, include it in OTel
//     span attributes, or include it in any error message returned to a caller.
//   - The daemon decides each resolve with the FGA relation can_resolve. The
//     SDK keeps no allow-list.
//   - Resolved values are cached in-process with a default TTL of 60 seconds.
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
// # SIGTERM, Revocation and Rotation
//
// On SIGTERM or SIGINT [Serve]:
//  1. Stops accepting new work from PollWork.
//  2. Waits for in-flight handlers to complete up to drainTimeout (default 30s).
//  3. Calls OnStop, transitions to Stopped, and returns nil.
//
// [Serve] subscribes to the daemon's ComponentService.WatchComponentEvents
// stream. The daemon publishes secret_access_revoked and secret_rotated for
// the calling plugin on it, and a heartbeat the SDK drops. When the stream
// fails for a reason other than shutdown, the SDK reconnects with capped
// exponential backoff, so a daemon rollout does not end the subscription.
// The daemon replays nothing on subscribe: a plugin that connects after a
// revocation learns it on its next credential resolve, which the daemon
// denies.
//
// When the operator revokes a secret grant of the plugin:
//  1. The events subscriber receives the secret_access_revoked event.
//  2. The secrets client marks the name revoked and drops it from the cache.
//  3. The lifecycle state machine moves to Degraded and calls OnDegraded.
//  4. The next heartbeat reports health_status "degraded" with the reason,
//     so the daemon marks the install DEGRADED within one heartbeat interval.
//
// When the operator rotates a secret, the events subscriber receives the
// secret_rotated event and drops the cached value. The next resolve fetches
// the new value.
//
// # See Also
//
//   - [lifecycle.StateMachine] — lifecycle state machine.
//   - [health.Server] — health probe endpoints.
//   - [pluginsecrets.Client] — credential resolution with caching.
//   - [events.Subscriber] — rotation and revocation event handling.
//   - [dispatch.Dispatcher] — PollWork → handler → SubmitResult dispatch loop.
package plugin
