// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"errors"

	"github.com/zeroroot-ai/sdk/plugin/secrets"
)

// ErrNoSecretsClient is returned by [ResolveSecret] and [SecretsFromContext]
// when ctx carries no secrets client. This happens when the function is called
// outside a [Serve] handler or lifecycle hook — for example from a unit test
// that invokes a handler directly without injecting a client. Test code that
// needs secrets should inject one with [secrets.NewContext].
var ErrNoSecretsClient = errors.New("plugin: no secrets client in context; " +
	"ResolveSecret must be called from within a plugin.Serve method handler or lifecycle hook")

// ResolveSecret fetches the value of a manifest-declared secret by name, using
// the broker-backed secrets client that [Serve] injects into every handler and
// lifecycle-hook context.
//
// This is the supported way for a plugin to read a credential value at runtime.
// Plugins are the only component class permitted to resolve secrets, and the
// broker is the only credential channel — never read secrets from environment
// variables or config files.
//
//	func handler(ctx context.Context, req proto.Message) (proto.Message, error) {
//	    token, err := plugin.ResolveSecret(ctx, "cred:api_key")
//	    if err != nil {
//	        return nil, fmt.Errorf("resolve api_key: %w", err) // never embed token in the error
//	    }
//	    // use token — never log it, never include it in error messages
//	    _ = token
//	    return req, nil
//	}
//
// name MUST be declared in the manifest's spec.secrets; the SDK rejects
// undeclared names before any RPC. Resolved values are cached in-process with a
// default 60s TTL; pass [secrets.WithCache](false) to force a fresh fetch.
//
// ResolveSecret returns [ErrNoSecretsClient] when called outside a Serve
// handler or lifecycle-hook context.
func ResolveSecret(ctx context.Context, name string, opts ...secrets.Option) ([]byte, error) {
	c, ok := secrets.FromContext(ctx)
	if !ok {
		return nil, ErrNoSecretsClient
	}
	return c.Resolve(ctx, name, opts...)
}

// SecretsFromContext returns the broker-backed secrets [secrets.Client] that
// [Serve] injected into ctx. Most plugins should call [ResolveSecret] instead;
// SecretsFromContext is for callers that need the client handle directly (for
// example to invalidate a cache entry). The second return value is false when
// ctx carries no client.
func SecretsFromContext(ctx context.Context) (secrets.Client, bool) {
	return secrets.FromContext(ctx)
}
