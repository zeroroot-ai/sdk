// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secrets

import "context"

// ctxKey is the unexported context key under which the active plugin
// [Client] is stored. Using a struct{} key avoids collisions with any
// other package's context values.
type ctxKey struct{}

// NewContext returns a copy of ctx carrying c. The plugin SDK calls this
// inside plugin.Serve so that method handlers and lifecycle hooks can recover
// the broker-backed secrets client from their context via [FromContext].
//
// Plugin authors do not normally call NewContext; it is exported for tests and
// for advanced callers that drive the dispatch loop directly.
func NewContext(ctx context.Context, c Client) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the plugin secrets [Client] carried by ctx, if any.
//
// Inside a plugin.Serve method handler or lifecycle hook the second return
// value is true. Outside that scope (e.g. a unit test that calls a handler
// directly without injecting a client) it is false, and callers must treat the
// secrets client as unavailable rather than dereferencing a nil interface.
func FromContext(ctx context.Context) (Client, bool) {
	c, ok := ctx.Value(ctxKey{}).(Client)
	return c, ok
}
