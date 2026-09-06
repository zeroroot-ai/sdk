// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package egress applies a plugin manifest's egress declaration to the
// runtime environment.
//
// The enforcement behavior is runtime-mode-specific:
//
//   - process: informational logging only. The plugin runs on a developer
//     laptop or in CI where the host network is unrestricted. The declared
//     egress is surfaced in the dashboard plugin detail page (Spec 4) but no
//     active enforcement occurs here.
//
//   - pod: no-op. Kubernetes NetworkPolicy enforcement is emitted daemon-side
//     at plugin registration time and applied by the Kubernetes control plane.
//     The SDK takes no action at runtime.
//
//   - setec: the egress declaration is forwarded to the Setec microVM
//     orchestrator via [SetecClient.ApplyNetworkPolicy]. The Setec orchestrator
//     enforces the policy at the microVM boundary; outbound traffic to
//     undeclared targets is dropped.
//
// The runtime mode is read from the GIBSON_PLUGIN_RUNTIME environment variable
// (values: "process", "pod", "setec"). When the variable is absent or empty the
// mode defaults to "process".
//
// Call [New] to get the appropriate [Enforcer] for the current environment.
package egress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// EnvRuntimeKey is the environment variable that selects the plugin runtime mode.
const EnvRuntimeKey = "GIBSON_PLUGIN_RUNTIME"

// Runtime mode constants mirror manifest.DefaultRuntime and its peers.
const (
	RuntimeProcess = "process"
	RuntimePod     = "pod"
	RuntimeSetec   = "setec"
)

// Enforcer applies a plugin manifest's egress declaration to the runtime
// environment. Obtain an implementation via [New].
type Enforcer interface {
	// Apply enforces (or records) the supplied egress declarations for the
	// current runtime mode. The ctx is used for any outbound calls (setec mode
	// only). Returns nil on success.
	Apply(ctx context.Context, decls []manifest.EgressDecl) error
}

// SetecClient is the interface through which the setec enforcer forwards egress
// declarations to the Setec microVM orchestrator. A concrete implementation is
// provided by the Setec SDK (out of scope for this spec). In process and pod
// mode the client can be nil.
type SetecClient interface {
	// ApplyNetworkPolicy instructs the Setec orchestrator to enforce the
	// supplied egress declarations as the microVM's outbound network policy.
	ApplyNetworkPolicy(ctx context.Context, decls []manifest.EgressDecl) error
}

// New returns the [Enforcer] matching the current runtime mode.
//
// The mode is read from GIBSON_PLUGIN_RUNTIME. An absent or empty variable
// defaults to "process". An unrecognised value is treated as "process" with
// a warning log so plugins remain operable in unexpected environments.
//
// setecClient is only required when the runtime mode is "setec". In process
// and pod mode it may be nil. If the mode is "setec" and setecClient is nil,
// [Enforcer.Apply] will return an error rather than panicking.
func New(setecClient SetecClient) Enforcer {
	mode := os.Getenv(EnvRuntimeKey)
	if mode == "" {
		mode = RuntimeProcess
	}
	switch mode {
	case RuntimeProcess:
		return &processEnforcer{}
	case RuntimePod:
		return &podEnforcer{}
	case RuntimeSetec:
		return &setecEnforcer{client: setecClient}
	default:
		slog.Warn("egress: unrecognised GIBSON_PLUGIN_RUNTIME value; defaulting to process mode",
			"value", mode)
		return &processEnforcer{}
	}
}

// ----------------------------------------------------------------------------
// processEnforcer
// ----------------------------------------------------------------------------

// processEnforcer is the [Enforcer] for process (laptop/CI) mode. It logs
// each declared egress target at Info level for visibility and returns nil.
// No enforcement is applied; the host network is unrestricted in this mode.
type processEnforcer struct{}

// Apply logs each egress declaration and returns nil. No network enforcement
// occurs in process mode. The declarations are informational only; the
// dashboard plugin detail page (Spec 4) surfaces them to operators.
func (e *processEnforcer) Apply(_ context.Context, decls []manifest.EgressDecl) error {
	if len(decls) == 0 {
		slog.Info("egress(process): no egress declarations; network is unrestricted")
		return nil
	}
	for _, d := range decls {
		slog.Info("egress(process): declared egress target (informational, unenforced)",
			"host", d.Host,
			"protocol", d.Protocol,
			"port", d.Port,
			"purpose", d.Purpose,
		)
	}
	return nil
}

// ----------------------------------------------------------------------------
// podEnforcer
// ----------------------------------------------------------------------------

// podEnforcer is the [Enforcer] for pod (Kubernetes) mode. Apply is a no-op
// because NetworkPolicy enforcement is emitted daemon-side at plugin
// registration time and applied by the Kubernetes network plugin. The SDK
// does not duplicate that work at runtime.
type podEnforcer struct{}

// Apply is intentionally a no-op in pod mode. The daemon (or the operator
// deploying the plugin) emits a Kubernetes NetworkPolicy matching the
// manifest's spec.egress[] at registration time; the cluster's network
// plugin enforces it. This function always returns nil.
func (e *podEnforcer) Apply(_ context.Context, _ []manifest.EgressDecl) error {
	// Intentional no-op. See package-level documentation.
	return nil
}

// ----------------------------------------------------------------------------
// setecEnforcer
// ----------------------------------------------------------------------------

// setecEnforcer is the [Enforcer] for setec (microVM) mode. It forwards the
// egress declarations to the Setec orchestrator, which applies them as the
// microVM's outbound network policy. Outbound traffic to undeclared targets
// is dropped at the microVM boundary.
type setecEnforcer struct {
	client SetecClient
}

// Apply forwards decls to the Setec orchestrator via SetecClient.ApplyNetworkPolicy.
// It returns an error if setecClient is nil or if the orchestrator call fails.
//
// An error here causes plugin startup to fail (called from plugin.Serve during
// the startup sequence) which is the correct behaviour: a setec plugin that
// cannot enforce its network policy must not serve work.
func (e *setecEnforcer) Apply(ctx context.Context, decls []manifest.EgressDecl) error {
	if e.client == nil {
		return errors.New("egress(setec): setec mode requires a SetecClient; " +
			"pass a non-nil SetecClient to egress.New")
	}
	if err := e.client.ApplyNetworkPolicy(ctx, decls); err != nil {
		return fmt.Errorf("egress(setec): ApplyNetworkPolicy: %w", err)
	}
	return nil
}
