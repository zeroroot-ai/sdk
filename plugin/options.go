// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/schema"
	"github.com/zeroroot-ai/sdk/plugin/secrets"
)

// config holds the resolved configuration for a [Serve] call.
// It is built by applying the supplied [Option] functions in order.
type config struct {
	// name is the plugin name the daemon registers. Required, set by [WithName].
	name string

	// version is the plugin version the daemon records. Required, set by
	// [WithVersion].
	version string

	// handlers maps method name → low-level JSON dispatch adapter. Built by
	// [WithHandler] calls (which wrap the author's typed handler).
	handlers map[string]MethodHandler

	// methodSchemas maps method name → the JSON-Schema documents derived from
	// the handler's Go request/response types. Built alongside handlers by
	// [WithHandler]; consumed by Serve to populate the registration descriptors.
	methodSchemas map[string]methodSchema

	// methodDescriptions maps method name → the human-readable description an
	// agent reads to disambiguate tools in the catalog. Required per method,
	// supplied to WithHandler beside the handler it describes.
	methodDescriptions map[string]string

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

	// httpClient is the client for the platform HTTP calls, discovery and
	// registration. nil means the default client.
	httpClient *http.Client
}

// Option configures [Serve].
type Option func(*config)

// methodSchema holds the JSON-Schema documents derived from a handler's Go
// request and response types.
type methodSchema struct {
	input string
}

// WithName sets the plugin name that [Serve] registers with the daemon.
// Required. The plugin declares itself in code and reports the declaration at
// start (ADR-0097). No manifest file exists.
func WithName(name string) Option {
	return func(c *config) {
		c.name = name
	}
}

// WithVersion sets the plugin version that [Serve] registers with the daemon.
// Required.
func WithVersion(version string) Option {
	return func(c *config) {
		c.version = version
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
// The registered handlers are the plugin's method set: [Serve] registers each
// one with the daemon. Both Req and Resp must yield a valid schema; an
// underivable type causes [Serve] to return a startup error before the daemon
// connection attempt.
//
// Registering the same method name twice is a startup error.
//
// description is REQUIRED and must be non-empty. It is what an agent reads to
// choose between tools in the catalog (RegisterComponent.method_descriptors →
// SearchTools), so an empty one silently degrades tool selection rather than
// failing anything. It lives beside the handler whose contract it describes
// (sdk#127, ADR-0097).
func WithHandler[Req, Resp any](name, description string, fn func(ctx context.Context, req Req) (Resp, error)) Option {
	return func(c *config) {
		if c.handlers == nil {
			c.handlers = make(map[string]MethodHandler)
		}
		if c.methodSchemas == nil {
			c.methodSchemas = make(map[string]methodSchema)
		}
		if c.methodDescriptions == nil {
			c.methodDescriptions = make(map[string]string)
		}
		if _, dup := c.handlers[name]; dup {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler: method %q registered more than once", name))
			return
		}
		if strings.TrimSpace(description) == "" {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler %q: description is required; "+
				"an agent reads it to choose between tools in the catalog, and an empty one degrades "+
				"tool selection without failing anything", name))
			return
		}
		c.methodDescriptions[name] = description

		reqType := reflect.TypeOf((*Req)(nil)).Elem()
		respType := reflect.TypeOf((*Resp)(nil)).Elem()
		inSchema, err := schema.DeriveJSON(reqType)
		if err != nil {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler %q: derive request schema from %s: %w", name, reqType, err))
			return
		}
		if _, err := schema.DeriveJSON(respType); err != nil {
			c.optionErrs = append(c.optionErrs, fmt.Errorf("WithHandler %q: derive response schema from %s: %w", name, respType, err))
			return
		}
		c.methodSchemas[name] = methodSchema{input: string(inSchema)}

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

// WithLifecycle supplies optional lifecycle hooks that are called at specific
// state transitions:
//   - OnStart: called when the plugin transitions to Ready. A plugin that
//     needs a secret to start resolves it here with [ResolveSecret] and
//     returns the error, so [Serve] fails at boot with the secret named.
//   - OnStop: called when the plugin transitions to Draining.
//   - OnDegraded: called when the plugin transitions to Degraded (secret
//     revocation, sustained daemon disconnect).
func WithLifecycle(hooks lifecycle.LifecycleHooks) Option {
	return func(c *config) {
		c.hooks = hooks
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
