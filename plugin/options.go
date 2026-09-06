// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/manifest"
	"github.com/zeroroot-ai/sdk/plugin/schema"
	"github.com/zeroroot-ai/sdk/plugin/secrets"
)

// DiscoveredMethod describes a single method discovered at startup by a
// [MethodSource]. Discovered methods are merged with the statically declared
// [WithHandler] handlers and registered with the daemon as the plugin's method
// set. A discovered method carries a raw JSON dispatch handler directly (its
// schema, if any, comes from the runtime source, not Go reflection).
type DiscoveredMethod struct {
	// Name is the method identifier. Required, and must not collide with a
	// statically registered method or another discovered method.
	Name string

	// Description is an optional human-readable explanation of the method,
	// surfaced in the component catalog.
	Description string

	// Handler processes invocations of this method. Required.
	Handler MethodHandler
}

// MethodSource discovers a plugin's method set at startup. [Serve] invokes it
// once, after capability-grant registration and daemon connectivity are
// established but before RegisterComponent, so the discovered set is part of
// the component's declared methods. The supplied context carries the plugin
// secrets client (recoverable via secrets.FromContext), allowing the source to
// resolve declared credentials — e.g. to start a vendor subprocess — before
// discovery.
//
// Using a MethodSource requires spec.dynamic_methods: true in the manifest.
type MethodSource func(ctx context.Context) ([]DiscoveredMethod, error)

// config holds the resolved configuration for a [Serve] call.
// It is built by applying the supplied [Option] functions in order.
type config struct {
	// manifestPath is the filesystem path to plugin.yaml. Required unless
	// parsedManifest is supplied.
	manifestPath string

	// parsedManifest is an in-memory manifest that bypasses the file load.
	// Used by wrappers that derive a plugin manifest from another declarative
	// source.
	parsedManifest *manifest.Manifest

	// methodSource discovers additional methods at startup. Optional.
	methodSource MethodSource

	// handlers maps method name → low-level JSON dispatch adapter. Built by
	// [WithHandler] calls (which wrap the author's typed handler).
	handlers map[string]MethodHandler

	// methodSchemas maps method name → the JSON-Schema documents derived from
	// the handler's Go request/response types. Built alongside handlers by
	// [WithHandler]; consumed by Serve to populate the registration descriptors.
	methodSchemas map[string]methodSchema

	// optionErrs collects errors raised while applying options (e.g. a schema
	// that cannot be derived, or a duplicate handler). An Option cannot return
	// an error, so they are surfaced by [Serve] before any daemon connection.
	optionErrs []error

	// hooks are the optional lifecycle callbacks supplied by [WithLifecycle].
	hooks lifecycle.LifecycleHooks

	// secretsClient overrides the production secrets client. Used in tests.
	secretsClient secrets.Client

	// healthAddr is the TCP listen address for the health server (e.g. ":8080").
	// Defaults to ":8080" when empty.
	healthAddr string

	// drainTimeout is the maximum time Serve waits for in-flight handlers to
	// complete after a SIGTERM or SIGINT. Defaults to 30 seconds.
	drainTimeout time.Duration

	// platformURL is the base HTTPS URL of the Gibson platform.
	// Read from GIBSON_URL when not set explicitly.
	platformURL string

	// bootstrapToken is an optional first-time registration token.
	// When provided it overrides the GIBSON_BOOTSTRAP_TOKEN environment variable.
	bootstrapToken string
}

// Option configures [Serve].
type Option func(*config)

// methodSchema holds the JSON-Schema documents derived from a handler's Go
// request and response types.
type methodSchema struct {
	input  string
	output string
}

// WithManifest loads the plugin manifest from path. Required.
//
// The path is passed to [manifest.Load]; the manifest is validated at startup
// before any daemon connection is established.
func WithManifest(path string) Option {
	return func(c *config) {
		c.manifestPath = path
	}
}

// WithHandler registers a typed Go handler for the named method — the single
// plugin-authoring path (ADR-0065 R4, ADR-0027 one code path). The author
// writes a plain function
//
//	func(ctx context.Context, req Req) (Resp, error)
//
// over plain Go request/response structs; the SDK derives the method's tool
// schema/descriptor from Req and Resp by reflection (see the schema package for
// the supported shapes) and installs a JSON⇄struct dispatch adapter. There is
// no hand-written .proto and no per-method codegen.
//
// The method name MUST match a name declared in the manifest's spec.methods[].
// [Serve] validates that every declared method has a registered handler and
// every registered handler is declared in the manifest, and that both Req and
// Resp yield a valid schema — any mismatch or underivable type causes [Serve]
// to return a startup error before the daemon connection attempt.
//
// Registering the same method name twice is a startup error.
func WithHandler[Req, Resp any](name string, fn func(ctx context.Context, req Req) (Resp, error)) Option {
	return func(c *config) {
		if c.handlers == nil {
			c.handlers = make(map[string]MethodHandler)
		}
		if c.methodSchemas == nil {
			c.methodSchemas = make(map[string]methodSchema)
		}
		if _, dup := c.handlers[name]; dup {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler: method %q registered more than once", name))
			return
		}

		reqType := reflect.TypeOf((*Req)(nil)).Elem()
		respType := reflect.TypeOf((*Resp)(nil)).Elem()
		inSchema, err := schema.DeriveJSON(reqType)
		if err != nil {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler %q: derive request schema from %s: %w", name, reqType, err))
			return
		}
		outSchema, err := schema.DeriveJSON(respType)
		if err != nil {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler %q: derive response schema from %s: %w", name, respType, err))
			return
		}
		c.methodSchemas[name] = methodSchema{input: string(inSchema), output: string(outSchema)}

		c.handlers[name] = func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
			var req Req
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &req); err != nil {
					return nil, fmt.Errorf("decode request for method %q: %w", name, err)
				}
			}
			resp, err := fn(ctx, req)
			if err != nil {
				return nil, err
			}
			out, err := json.Marshal(resp)
			if err != nil {
				return nil, fmt.Errorf("encode response for method %q: %w", name, err)
			}
			return out, nil
		}
	}
}

// WithParsedManifest supplies an already-parsed manifest, bypassing the
// [WithManifest] file load. The manifest is validated by [Serve] exactly as a
// file-loaded manifest would be. Intended for wrappers that derive a plugin
// manifest from another declarative source; plain plugins should use
// [WithManifest].
//
// When both WithManifest and WithParsedManifest are supplied, the parsed
// manifest wins.
func WithParsedManifest(m *manifest.Manifest) Option {
	return func(c *config) {
		c.parsedManifest = m
	}
}

// WithMethodSource registers a [MethodSource] that discovers methods at
// startup (e.g. from an MCP server's tools/list). Discovered methods are
// merged with statically registered [WithMethod] handlers — name collisions
// are a startup error — and the combined set is registered with the daemon.
//
// The manifest must declare spec.dynamic_methods: true; conversely, a manifest
// declaring dynamic_methods requires a method source. This keeps the manifest
// the single declarative source of truth for how the plugin's method set is
// produced.
func WithMethodSource(src MethodSource) Option {
	return func(c *config) {
		c.methodSource = src
	}
}

// WithLifecycle supplies optional lifecycle hooks that are called at specific
// state transitions:
//   - OnStart: called when the plugin transitions to Ready.
//   - OnStop: called when the plugin transitions to Draining.
//   - OnDegraded: called when the plugin transitions to Degraded (secret
//     revocation, sustained daemon disconnect).
func WithLifecycle(hooks lifecycle.LifecycleHooks) Option {
	return func(c *config) {
		c.hooks = hooks
	}
}

// WithSecretsClient injects a [secrets.Client] implementation. When nil (the
// default) [Serve] constructs a production client backed by the daemon's
// HarnessCallbackService.GetCredential RPC.
//
// This option is intended for testing; pass a fake client to avoid real daemon
// connectivity in unit tests.
func WithSecretsClient(cl secrets.Client) Option {
	return func(c *config) {
		c.secretsClient = cl
	}
}

// WithHealthAddr sets the TCP listen address for the HTTP health server.
// Default is ":8080". Pass ":0" to bind to a random available port (useful in
// tests).
func WithHealthAddr(addr string) Option {
	return func(c *config) {
		c.healthAddr = addr
	}
}

// WithDrainTimeout sets the maximum time [Serve] waits for in-flight handlers
// to complete after receiving SIGTERM or SIGINT. Default is 30 seconds.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *config) {
		c.drainTimeout = d
	}
}

// WithPlatformURL sets the Gibson platform base URL. When not supplied, [Serve]
// reads GIBSON_URL from the environment. Primarily useful in tests.
func WithPlatformURL(url string) Option {
	return func(c *config) {
		c.platformURL = url
	}
}

// WithBootstrapToken sets the first-time registration token. When not supplied,
// the capability-grant client falls back to GIBSON_BOOTSTRAP_TOKEN or the
// Kubernetes service-account token.
func WithBootstrapToken(token string) Option {
	return func(c *config) {
		c.bootstrapToken = token
	}
}

// defaults fills in zero-value fields after all options have been applied.
func (c *config) defaults() {
	if c.healthAddr == "" {
		c.healthAddr = ":8080"
	}
	if c.drainTimeout <= 0 {
		c.drainTimeout = 30 * time.Second
	}
	if c.handlers == nil {
		c.handlers = make(map[string]MethodHandler)
	}
}
