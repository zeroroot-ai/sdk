// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
	"github.com/zeroroot-ai/sdk/capabilitygrant"
	"github.com/zeroroot-ai/sdk/plugin/dispatch"
	"github.com/zeroroot-ai/sdk/plugin/egress"
	"github.com/zeroroot-ai/sdk/plugin/events"
	"github.com/zeroroot-ai/sdk/plugin/health"
	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/manifest"
	"github.com/zeroroot-ai/sdk/plugin/metrics"
	pluginsecrets "github.com/zeroroot-ai/sdk/plugin/secrets"
)

// MethodHandler is the low-level, JSON-in/JSON-out dispatch adapter type,
// aliased from [dispatch.MethodHandler]. Plugin authors do NOT implement it
// directly — they register typed Go handlers with [WithHandler], which builds
// the adapter and derives the method schema from the Go types. The alias is
// exported for the dynamic-discovery path ([DiscoveredMethod.Handler]).
//
// Handlers MUST NOT include resolved secret values in any returned error string.
type MethodHandler = dispatch.MethodHandler

// Serve is the single entry point a plugin author calls from main(). It
// orchestrates all plugin SDK components:
//
//  1. Loads and validates the manifest from the path given by [WithManifest].
//  2. Cross-checks registered method handlers against manifest declarations.
//  3. Acquires a daemon connection via capabilitygrant (Bootstrap → Discover → Register).
//  4. Constructs the secrets client wrapping GetCredential.
//  5. Starts the egress enforcer (per GIBSON_PLUGIN_RUNTIME).
//  6. Pre-resolves all manifest secrets with scope=startup, required=true.
//  7. Starts the lifecycle state machine, invokes OnStart, transitions to Ready.
//  8. Starts the health server on the configured port.
//  9. Starts the events subscriber on the component callback stream.
//  10. Starts the dispatch loop (PollWork → handler → SubmitResult).
//  11. Blocks until ctx is cancelled or a fatal error occurs.
//  12. On SIGTERM/SIGINT: stops new work, drains in-flight handlers up to
//     drainTimeout, runs OnStop, exits cleanly.
//  13. On rotation=restart event: drains then exits with code 75.
//
// Serve returns the first fatal error encountered, or nil on clean shutdown.
func Serve(ctx context.Context, opts ...Option) error {
	// t0 anchors the gibson_plugin_startup_seconds histogram. It is observed
	// by the lifecycle observer below the first time the state machine
	// transitions into Ready.
	t0 := time.Now()

	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	cfg.defaults()

	// Surface any error raised while applying options (an underivable handler
	// schema or a duplicate registration) before any daemon connection.
	if len(cfg.optionErrs) > 0 {
		return fmt.Errorf("plugin.Serve: invalid options: %w", errors.Join(cfg.optionErrs...))
	}

	// -------------------------------------------------------------------------
	// Step 1: Load and validate manifest.
	// -------------------------------------------------------------------------
	var m *manifest.Manifest
	switch {
	case cfg.parsedManifest != nil:
		m = cfg.parsedManifest
		manifest.ApplyDefaults(m)
		if err := manifest.Validate(m); err != nil {
			return fmt.Errorf("plugin.Serve: validate manifest: %w", err)
		}
	case cfg.manifestPath != "":
		var err error
		m, err = manifest.Load(cfg.manifestPath)
		if err != nil {
			return fmt.Errorf("plugin.Serve: load manifest: %w", err)
		}
	default:
		return errors.New("plugin.Serve: WithManifest is required")
	}
	slog.Info("plugin: manifest loaded",
		"name", m.Metadata.Name,
		"version", m.Metadata.Version,
		"runtime", m.Spec.Runtime,
	)

	// -------------------------------------------------------------------------
	// Step 2: Validate method handler registration vs manifest declarations.
	// -------------------------------------------------------------------------
	if err := validateMethods(m, cfg.handlers); err != nil {
		return fmt.Errorf("plugin.Serve: method validation: %w", err)
	}
	if m.Spec.DynamicMethods && cfg.methodSource == nil {
		return errors.New("plugin.Serve: manifest declares spec.dynamic_methods " +
			"but no method source is registered; pass WithMethodSource")
	}
	if cfg.methodSource != nil && !m.Spec.DynamicMethods {
		return errors.New("plugin.Serve: WithMethodSource requires " +
			"spec.dynamic_methods: true in the manifest")
	}

	// -------------------------------------------------------------------------
	// Step 3: Check setec_required policy.
	// -------------------------------------------------------------------------
	if m.Spec.Policy.SetecRequired {
		runtime := os.Getenv(egress.EnvRuntimeKey)
		if runtime == "" {
			runtime = egress.RuntimeProcess
		}
		if runtime != egress.RuntimeSetec {
			return fmt.Errorf("plugin.Serve: manifest requires setec runtime "+
				"(spec.policy.setec_required=true) but GIBSON_PLUGIN_RUNTIME=%q", runtime)
		}
	}

	// -------------------------------------------------------------------------
	// Step 4: Build a signal-aware context wrapping the caller's ctx.
	// -------------------------------------------------------------------------
	signalCtx, cancelSignal := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancelSignal()

	// -------------------------------------------------------------------------
	// Step 5: Build the lifecycle state machine.
	// -------------------------------------------------------------------------
	sm := lifecycle.New(cfg.hooks)
	sm.OnTransition(func(from, to lifecycle.State) {
		slog.Info("plugin: lifecycle transition",
			"plugin", m.Metadata.Name,
			"from", from.String(),
			"to", to.String(),
		)
	})

	// Metrics observer: bumps gibson_plugin_lifecycle_transition_total and
	// updates gibson_plugin_state. The install_id label is empty until
	// RegisterComponent assigns one; the gauge is backfilled below via
	// metrics.Default.SetState(plugin, instanceID, Current()) once known.
	//
	// Startup observation: on the first transition into Ready, observe
	// gibson_plugin_startup_seconds(plugin) using the t0 captured above.
	var (
		startupOnce        sync.Once
		instanceIDForGauge atomic.Pointer[string]
	)
	emptyInstance := ""
	instanceIDForGauge.Store(&emptyInstance)
	sm.OnTransition(func(from, to lifecycle.State) {
		iid := *instanceIDForGauge.Load()
		metrics.Default.RecordTransition(m.Metadata.Name, iid, from, to)
		if to == lifecycle.Ready {
			startupOnce.Do(func() {
				metrics.Default.ObserveStartup(m.Metadata.Name, t0)
			})
		}
	})

	// -------------------------------------------------------------------------
	// Step 6: Capability-grant registration (Bootstrap → Discover → Register).
	// -------------------------------------------------------------------------
	if err := sm.Transition(lifecycle.Registering); err != nil {
		return fmt.Errorf("plugin.Serve: lifecycle transition to Registering: %w", err)
	}

	platformURL := cfg.platformURL
	if platformURL == "" {
		platformURL = os.Getenv("GIBSON_URL")
	}
	if platformURL == "" {
		return errors.New("plugin.Serve: platform URL is required; " +
			"set GIBSON_URL or pass WithPlatformURL")
	}

	hostKeyPath, err := pluginHostKeyPath(m.Metadata.Name)
	if err != nil {
		return fmt.Errorf("plugin.Serve: resolve host key path: %w", err)
	}

	cgClient, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
		PlatformURL:    platformURL,
		BootstrapToken: cfg.bootstrapToken,
		HostKeyPath:    hostKeyPath,
		AgentName:      m.Metadata.Name,
		AgentMode:      "autonomous",
	})
	if err != nil {
		return fmt.Errorf("plugin.Serve: capabilitygrant.NewClient: %w", err)
	}

	if err := cgClient.Discover(signalCtx); err != nil {
		return fmt.Errorf("plugin.Serve: capabilitygrant.Discover: %w", err)
	}

	if err := cgClient.Register(signalCtx); err != nil {
		return fmt.Errorf("plugin.Serve: capabilitygrant.Register: %w", err)
	}
	slog.Info("plugin: registered with daemon",
		"plugin", m.Metadata.Name,
		"agent_id", cgClient.AgentID(),
	)

	// -------------------------------------------------------------------------
	// Step 7: Establish gRPC connection to the daemon for ComponentService and
	//         HarnessCallbackService.
	// -------------------------------------------------------------------------
	daemonAddr := resolveDaemonAddr(platformURL)
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(daemonTransportCredentials()),
		grpc.WithPerRPCCredentials(cgClient.GRPCPerRPCCredentials()),
	}
	// A process-mode bridge reaching the daemon through a TLS gateway (Envoy)
	// may need to fix the gRPC :authority — e.g. when the dial target is a local
	// forward — so the gateway routes to the right virtual host. In-cluster
	// (the default), the dial-target authority is already correct.
	if authority := strings.TrimSpace(os.Getenv("GIBSON_DAEMON_AUTHORITY")); authority != "" {
		dialOpts = append(dialOpts, grpc.WithAuthority(authority))
	}
	conn, err := grpc.NewClient(daemonAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("plugin.Serve: grpc.NewClient: %w", err)
	}
	defer conn.Close()

	componentSvcClient := componentpb.NewComponentServiceClient(conn)
	harnessCallbackSvcClient := harnesspb.NewHarnessCallbackServiceClient(conn)

	// Construct the secrets client before RegisterComponent so a method
	// source can resolve declared credentials during discovery (e.g. to start
	// a vendor subprocess that needs an API token in its environment).
	secretsClient := cfg.secretsClient
	if secretsClient == nil {
		secretsClient = buildSecretsClient(m, harnessCallbackSvcClient)
	}

	// Inject the secrets client into the context so that the method source,
	// the OnStart hook, the method handlers (via the dispatch loop), and the
	// OnStop hook can resolve declared secrets through plugin.ResolveSecret /
	// secrets.FromContext. Every context derived from signalCtx below — the
	// method source call, RunOnStart, the errgroup ctx that feeds disp.Run,
	// and the health/events goroutines — inherits this value.
	signalCtx = pluginsecrets.NewContext(signalCtx, secretsClient)

	// Validate and register handlers for any discovered methods.
	var discovered []DiscoveredMethod
	if cfg.methodSource != nil {
		var err error
		discovered, err = cfg.methodSource(signalCtx)
		if err != nil {
			return fmt.Errorf("plugin.Serve: method source: %w", err)
		}
		for _, dm := range discovered {
			if dm.Name == "" {
				return errors.New("plugin.Serve: method source returned a method with empty name")
			}
			if dm.Handler == nil {
				return fmt.Errorf("plugin.Serve: method source returned method %q with nil handler", dm.Name)
			}
			if _, exists := cfg.handlers[dm.Name]; exists {
				return fmt.Errorf("plugin.Serve: method source returned method %q "+
					"which collides with an already-registered method", dm.Name)
			}
			cfg.handlers[dm.Name] = dm.Handler
		}
		slog.Info("plugin: methods discovered",
			"plugin", m.Metadata.Name,
			"count", len(discovered),
		)
	}

	// Assemble the method set: names (back-compat, RegisterComponentRequest.methods)
	// plus rich descriptors (method_descriptors) so the connector catalog and
	// SearchTools can surface per-method descriptions to agents.
	methodNames, methodDescriptors := buildMethodMetadata(m.Spec.Methods, discovered, cfg.methodSchemas)
	if len(methodNames) == 0 {
		return errors.New("plugin.Serve: no methods to register; the method " +
			"source returned an empty set and the manifest declares none")
	}

	// Register as a plugin component. The plugin:* metadata keys are the
	// contract the daemon reads to populate the ComponentInstall record
	// (internal/platform/component/service.go) — runtime mode, setec_required,
	// host id (the RFC-7638 thumbprint that keys per-host install uniqueness),
	// manifest hash, and content-trust classification. gibson#997.
	// Declared secrets this plugin resolves at runtime, sent so the daemon can
	// bind can_resolve for this plugin's principal on each at registration
	// (ADR-0066). A first-party plugin's manifest is operator-approved via
	// GitOps, and can_resolve only reads a value the operator separately
	// provisions — see the daemon's RegisterComponent binding. Comma-joined
	// because a secret ref (e.g. "cred:github_token") contains a colon but never
	// a comma.
	declaredSecrets := make([]string, 0, len(m.Spec.Secrets))
	for _, s := range m.Spec.Secrets {
		if s.Name != "" {
			declaredSecrets = append(declaredSecrets, s.Name)
		}
	}

	regResp, err := componentSvcClient.RegisterComponent(signalCtx, &componentpb.RegisterComponentRequest{
		Kind:              "plugin",
		Name:              m.Metadata.Name,
		Version:           m.Metadata.Version,
		Methods:           methodNames,
		MethodDescriptors: methodDescriptors,
		Metadata: map[string]string{
			"manifest_version":      m.Metadata.Version,
			"runtime":               m.Spec.Runtime,
			"plugin:runtime_mode":   m.Spec.Runtime,
			"plugin:setec_required": strconv.FormatBool(m.Spec.Policy.SetecRequired),
			"plugin:host_id":        cgClient.HostID(),
			"plugin:manifest_hash":  manifestHashFromPath(cfg.manifestPath),
			// plugin:content_trust travels the manifest's trust classification to
			// the daemon, which records it on the ComponentInstall and gates
			// untrusted plugin invocation through the dispatch policy
			// (ADR-0010). Normalised so an unset value is "trusted".
			"plugin:content_trust": normalizeContentTrust(m.Spec.Policy.ContentTrust),
			// plugin:secrets is the comma-joined list of declared secret refs;
			// the daemon binds can_resolve for each (ADR-0066).
			"plugin:secrets": strings.Join(declaredSecrets, ","),
		},
	})
	if err != nil {
		return fmt.Errorf("plugin.Serve: RegisterComponent: %w", err)
	}
	instanceID := regResp.GetInstanceId()
	slog.Info("plugin: component registered",
		"plugin", m.Metadata.Name,
		"instance_id", instanceID,
	)

	// Now that an install_id exists, backfill the per-install gauge to the
	// state machine's current state. Subsequent transitions update the gauge
	// via the observer registered above.
	instanceIDForGauge.Store(&instanceID)
	metrics.Default.SetState(m.Metadata.Name, instanceID, sm.Current())

	// -------------------------------------------------------------------------
	// Step 8: Transition to ResolvingSecrets. The secrets client itself was
	// constructed before RegisterComponent (above) so the method source could
	// use it during discovery.
	// -------------------------------------------------------------------------
	if err := sm.Transition(lifecycle.ResolvingSecrets); err != nil {
		return fmt.Errorf("plugin.Serve: lifecycle transition to ResolvingSecrets: %w", err)
	}

	// -------------------------------------------------------------------------
	// Step 9: Apply egress enforcer.
	// -------------------------------------------------------------------------
	egressEnforcer := egress.New(nil) // nil SetecClient; concrete impl is Setec SDK scope.
	if err := egressEnforcer.Apply(signalCtx, m.Spec.Egress); err != nil {
		return fmt.Errorf("plugin.Serve: egress enforcer: %w", err)
	}

	// -------------------------------------------------------------------------
	// Step 10: Pre-resolve all scope=startup, required=true secrets.
	// -------------------------------------------------------------------------
	for _, s := range m.Spec.Secrets {
		if s.Scope == "startup" && s.Required {
			if _, err := secretsClient.Resolve(signalCtx, s.Name); err != nil {
				return fmt.Errorf("plugin.Serve: required startup secret %q is unavailable: %w",
					s.Name, err)
			}
			slog.Info("plugin: startup secret resolved", "name", s.Name)
		}
	}

	// -------------------------------------------------------------------------
	// Step 11: Transition to Starting, run OnStart, transition to Ready.
	// -------------------------------------------------------------------------
	if err := sm.Transition(lifecycle.Starting); err != nil {
		return fmt.Errorf("plugin.Serve: lifecycle transition to Starting: %w", err)
	}

	if err := sm.RunOnStart(signalCtx); err != nil {
		return fmt.Errorf("plugin.Serve: OnStart hook failed: %w", err)
	}
	// sm is now in Ready state (transitioned by RunOnStart).

	// -------------------------------------------------------------------------
	// Step 12: Start health server.
	// -------------------------------------------------------------------------
	healthSrv := health.New(sm, cfg.healthAddr, m.Spec.Health.EffectiveLivenessInterval())
	boundAddr, err := healthSrv.Start(signalCtx)
	if err != nil {
		return fmt.Errorf("plugin.Serve: start health server: %w", err)
	}
	slog.Info("plugin: health server started", "addr", boundAddr)

	// -------------------------------------------------------------------------
	// Step 13: Build the dispatcher.
	// -------------------------------------------------------------------------
	pollTimeout := time.Duration(regResp.GetPollTimeoutMs()) * time.Millisecond
	if pollTimeout <= 0 {
		pollTimeout = dispatch.DefaultPollTimeout
	}

	compAdapter := &componentClientAdapter{
		client:     componentSvcClient,
		instanceID: instanceID,
	}

	disp := dispatch.New(compAdapter, dispatch.Config{
		Handlers:    cfg.handlers,
		PollTimeout: pollTimeout,
		OnInvocationComplete: func(method string, dur time.Duration, res dispatch.InvocationResult) {
			// dispatch.InvocationResult and metrics.Result share string
			// values; the cast is exact and bounded by the two enums.
			metrics.Default.ObserveInvocation(
				m.Metadata.Name, method, metrics.Result(res), dur,
			)
		},
	})

	// -------------------------------------------------------------------------
	// Step 14: Build the events subscriber and wire the Drainer.
	// -------------------------------------------------------------------------
	eventStream := &componentEventStream{ctx: make(chan struct{})}

	sub := events.NewWithDrainer(eventStream, secretsClient, sm, disp, m)
	pluginName := m.Metadata.Name
	sub.SetOnRotation(func(_ string, lag time.Duration) {
		metrics.Default.ObserveRotationPropagation(pluginName, lag)
	})

	// -------------------------------------------------------------------------
	// Step 15: Run background goroutines.
	// -------------------------------------------------------------------------
	heartbeatInterval := time.Duration(regResp.GetHeartbeatIntervalMs()) * time.Millisecond
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}

	eg, egCtx := errgroup.WithContext(signalCtx)

	// Heartbeat loop.
	eg.Go(func() error {
		return runHeartbeat(egCtx, componentSvcClient, instanceID, heartbeatInterval, healthSrv)
	})

	// Events subscriber.
	eg.Go(func() error {
		if err := sub.Run(egCtx); err != nil {
			return fmt.Errorf("events subscriber: %w", err)
		}
		return nil
	})

	// Dispatch loop.
	eg.Go(func() error {
		if err := disp.Run(egCtx); err != nil {
			return fmt.Errorf("dispatch loop: %w", err)
		}
		return nil
	})

	// Graceful shutdown watcher.
	eg.Go(func() error {
		<-egCtx.Done()
		// Use a fresh background context for shutdown operations because
		// egCtx is already cancelled here. Re-inject the secrets client so the
		// OnStop hook can still resolve declared secrets during drain.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.drainTimeout+5*time.Second)
		defer cancel()
		shutdownCtx = pluginsecrets.NewContext(shutdownCtx, secretsClient)
		return gracefulShutdown(shutdownCtx, sm, disp, cfg.drainTimeout, m.Metadata.Name)
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("plugin.Serve: %w", err)
	}
	return nil
}

// gracefulShutdown performs the SIGTERM drain sequence:
//  1. Transitions lifecycle to Draining (calls OnStop hook via RunOnStop).
//  2. Drains the dispatcher up to drainTimeout.
//  3. Transitions to Stopped.
func gracefulShutdown(
	ctx context.Context,
	sm *lifecycle.StateMachine,
	disp *dispatch.Dispatcher,
	drainTimeout time.Duration,
	pluginName string,
) error {
	slog.Info("plugin: initiating graceful shutdown", "plugin", pluginName)

	// RunOnStop transitions to Draining and calls OnStop.
	if err := sm.RunOnStop(ctx); err != nil {
		slog.Warn("plugin: OnStop hook returned error",
			"plugin", pluginName, "err", err)
	}

	// Drain in-flight handlers.
	if err := disp.Drain(ctx, drainTimeout); err != nil {
		slog.Warn("plugin: drain completed with error",
			"plugin", pluginName, "err", err)
	}

	// Transition to Stopped.
	if err := sm.Transition(lifecycle.Stopped); err != nil {
		slog.Warn("plugin: transition to Stopped failed",
			"plugin", pluginName, "err", err)
	}
	slog.Info("plugin: shutdown complete", "plugin", pluginName)
	return nil
}

// buildMethodMetadata assembles, from the manifest-declared methods and the
// methods returned by a method source, the parallel name list (back-compat,
// RegisterComponentRequest.methods) and the rich per-method descriptors
// (method_descriptors). Descriptions flow through so the connector catalog and
// SearchTools can surface them; declared methods come first, then discovered.
func buildMethodMetadata(declared []manifest.MethodDecl, discovered []DiscoveredMethod, schemas map[string]methodSchema) ([]string, []*componentpb.ComponentMethod) {
	n := len(declared) + len(discovered)
	names := make([]string, 0, n)
	detailed := make([]*componentpb.ComponentMethod, 0, n)
	for _, d := range declared {
		names = append(names, d.Name)
		cm := &componentpb.ComponentMethod{Name: d.Name, Description: d.Description}
		// The Go-first request schema derived from the handler's typed struct
		// travels to the daemon as the method's tool-input contract.
		if s, ok := schemas[d.Name]; ok {
			cm.InputSchemaJson = s.input
		}
		detailed = append(detailed, cm)
	}
	for _, dm := range discovered {
		names = append(names, dm.Name)
		detailed = append(detailed, &componentpb.ComponentMethod{Name: dm.Name, Description: dm.Description})
	}
	return names, detailed
}

// validateMethods cross-checks the handler map against the manifest's method
// declarations. Returns a descriptive error listing any mismatches.
func validateMethods(m *manifest.Manifest, handlers map[string]MethodHandler) error {
	declared := make(map[string]struct{}, len(m.Spec.Methods))
	for _, meth := range m.Spec.Methods {
		declared[meth.Name] = struct{}{}
	}

	var undeclared, unregistered []string
	for name := range handlers {
		if _, ok := declared[name]; !ok {
			undeclared = append(undeclared, name)
		}
	}
	for _, meth := range m.Spec.Methods {
		if _, ok := handlers[meth.Name]; !ok {
			unregistered = append(unregistered, meth.Name)
		}
	}

	sort.Strings(undeclared)
	sort.Strings(unregistered)

	var msgs []string
	if len(undeclared) > 0 {
		msgs = append(msgs, fmt.Sprintf(
			"handlers registered for undeclared methods: [%s]",
			strings.Join(undeclared, ", "),
		))
	}
	if len(unregistered) > 0 {
		msgs = append(msgs, fmt.Sprintf(
			"manifest methods without registered handlers: [%s]",
			strings.Join(unregistered, ", "),
		))
	}
	if len(msgs) > 0 {
		return fmt.Errorf("method mismatch: %s", strings.Join(msgs, "; "))
	}
	return nil
}

// pluginHostKeyPath returns the host key path for a plugin install.
// Path: ~/.gibson/plugin/<plugin-name>/host_key.json
func pluginHostKeyPath(pluginName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return home + "/.gibson/plugin/" + pluginName + "/host_key.json", nil
}

// daemonTransportCredentials selects the gRPC transport security for the daemon
// connection. The default is insecure: in-cluster (the hosted/setec path) the
// daemon callback is reached over a plaintext mesh hop. A process-mode /
// customer-network bridge reaching the daemon through a TLS gateway (Envoy) sets
// GIBSON_DAEMON_TLS=1 to dial with TLS instead, so its capability-grant JWT
// authenticates through ext-authz at the gateway. The system cert pool — which
// honors SSL_CERT_FILE — supplies the trust anchors for the gateway certificate.
func daemonTransportCredentials() credentials.TransportCredentials {
	if v := strings.TrimSpace(os.Getenv("GIBSON_DAEMON_TLS")); v != "" && v != "0" && v != "false" {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return grpcInsecure.NewCredentials()
}

// resolveDaemonAddr derives the gRPC dial target from the platform base URL.
// GIBSON_DAEMON_ADDR overrides when set (useful in tests and local dev).
//
// An explicit port in the platform URL wins; only when the URL carries no port
// does the scheme default apply (:443 for https, :80 for http). This mirrors
// agent.Connect's normalizeTarget and matters beyond routing: grpc-go mints the
// CG-JWT `aud` claim from the dial authority, and ext-authz pins component
// audiences with an exact-string allowlist (gibson#1282, deploy#1245). The old
// behavior — rewriting every port to the daemon's in-cluster :50051 — minted a
// divergent `:50051` audience whenever a plugin dialed the edge, forcing the
// deploy chart to carry dedicated `:50051` allowlist entries (sdk#452).
//
// In-cluster callers are unaffected as long as the platform URL names the
// daemon gRPC port explicitly (e.g. http://<release>-gibson-workloads:50051),
// which is what the chart's cluster-internal default does.
func resolveDaemonAddr(platformURL string) string {
	if addr := os.Getenv("GIBSON_DAEMON_ADDR"); addr != "" {
		return addr
	}
	// Strip the scheme; it decides the default port when none is present.
	// A bare host with no scheme defaults to :443, like agent.Connect.
	defaultPort := "443"
	addr := platformURL
	switch {
	case strings.HasPrefix(addr, "https://"):
		addr = strings.TrimPrefix(addr, "https://")
	case strings.HasPrefix(addr, "http://"):
		addr = strings.TrimPrefix(addr, "http://")
		defaultPort = "80"
	}
	// Strip trailing path components.
	if idx := strings.Index(addr, "/"); idx >= 0 {
		addr = addr[:idx]
	}
	// Explicit port wins: the dial authority — and therefore the CG-JWT
	// audience — must follow the URL the operator configured.
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return addr + ":" + defaultPort
}

// buildSecretsClient constructs the production secrets client backed by the
// daemon's HarnessCallbackService.GetCredential RPC.
//
// The Credential proto is a oneof (ApiKey | BearerToken | Basic | OAuth |
// CustomSecret). The plugin secrets client works with raw []byte, so we
// extract the raw credential value from whichever field is populated.
func buildSecretsClient(m *manifest.Manifest, hc harnesspb.HarnessCallbackServiceClient) pluginsecrets.Client {
	return pluginsecrets.New(m, func(ctx context.Context, name string) ([]byte, error) {
		resp, err := hc.GetCredential(ctx, &harnesspb.GetCredentialRequest{
			Name: name,
		})
		if err != nil {
			return nil, fmt.Errorf("GetCredential %q: %w", name, err)
		}
		if resp.GetError() != nil {
			return nil, fmt.Errorf("GetCredential %q: daemon error: %s",
				name, resp.GetError().GetMessage())
		}
		cred := resp.GetCredential()
		if cred == nil {
			return nil, fmt.Errorf("GetCredential %q: nil credential in response", name)
		}
		// Extract the raw secret value from the oneof SecretData field.
		// Plugin handlers receive bytes and cast as needed for their protocol.
		switch data := cred.SecretData.(type) {
		case *harnesspb.Credential_ApiKey:
			return []byte(data.ApiKey), nil
		case *harnesspb.Credential_BearerToken:
			return []byte(data.BearerToken), nil
		case *harnesspb.Credential_CustomSecret:
			return []byte(data.CustomSecret), nil
		case *harnesspb.Credential_Basic:
			if data.Basic != nil {
				return []byte(data.Basic.GetPassword()), nil
			}
		}
		return nil, fmt.Errorf("GetCredential %q: unsupported or empty credential type", name)
	}, pluginsecrets.CacheConfig{})
}

// runHeartbeat sends periodic HeartbeatRequests until ctx is cancelled.
// On each successful heartbeat it records the event in the health server.
func runHeartbeat(
	ctx context.Context,
	client componentpb.ComponentServiceClient,
	instanceID string,
	interval time.Duration,
	srv *health.Server,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, err := client.Heartbeat(ctx, &componentpb.HeartbeatRequest{
				InstanceId:    instanceID,
				HealthStatus:  "serving",
				HealthMessage: "ok",
			})
			if err != nil {
				slog.Warn("plugin: heartbeat failed", "err", err)
				continue
			}
			srv.RecordHeartbeat()
		}
	}
}

// ----------------------------------------------------------------------------
// componentClientAdapter adapts the generated ComponentServiceClient to the
// dispatch.ComponentClient interface expected by dispatch.Dispatcher.
// ----------------------------------------------------------------------------

type componentClientAdapter struct {
	client     componentpb.ComponentServiceClient
	instanceID string
}

// PollWork implements dispatch.ComponentClient by mapping the ComponentService
// PollWork proto call to the interface signature used by the dispatcher.
func (a *componentClientAdapter) PollWork(ctx context.Context, timeout time.Duration) (workID, workType string, payload []byte, err error) {
	resp, e := a.client.PollWork(ctx, &componentpb.PollWorkRequest{
		InstanceId: a.instanceID,
		TimeoutMs:  int32(timeout.Milliseconds()),
	})
	if e != nil {
		return "", "", nil, e
	}
	return resp.GetWorkId(), resp.GetWorkType(), resp.GetPayload(), nil
}

// SubmitResult implements dispatch.ComponentClient by translating the
// plugin-level PluginError into the ComponentError shape used by the wire
// protocol.
func (a *componentClientAdapter) SubmitResult(ctx context.Context, workID string, result []byte, errInfo *pluginpb.PluginError) error {
	req := &componentpb.SubmitResultRequest{
		WorkId: workID,
		Result: result,
	}
	if errInfo != nil {
		req.Error = &componentpb.ComponentError{
			Code:    errInfo.GetKind().String(),
			Message: errInfo.GetMessage(),
		}
	}
	_, err := a.client.SubmitResult(ctx, req)
	return err
}

// ----------------------------------------------------------------------------
// componentEventStream provides a context-aware events.EventStream that
// blocks until cancelled. It is replaced by a real stream in Phase 9 (CLI run).
// In production integration, the daemon pushes events over a streaming RPC;
// this placeholder allows the subscriber goroutine to park safely without
// spinning. Tests inject a fake EventStream via the events.Subscriber API.
// ----------------------------------------------------------------------------

// componentEventStream implements events.EventStream.
// The production streaming path is wired by the per-plugin daemon subscription;
// this placeholder blocks on ctx cancellation so the subscriber goroutine exits
// cleanly and does not spin.
type componentEventStream struct {
	// ctx is a sentinel channel closed when Serve exits; it is unused in
	// production but satisfies the interface for the placeholder implementation.
	ctx chan struct{}
}

// Recv blocks until ctx is cancelled.
func (s *componentEventStream) Recv(ctx context.Context) (events.Event, error) {
	<-ctx.Done()
	return events.Event{}, ctx.Err()
}

// normalizeContentTrust maps a manifest content_trust value to the canonical
// "trusted"/"untrusted" the daemon expects, defaulting any empty or unrecognised
// value to "trusted" (fail-safe: only an explicit "untrusted" opts a plugin into
// dispatch-policy gating). See gibson#997.
func normalizeContentTrust(v string) string {
	if v == manifest.ContentTrustUntrusted {
		return manifest.ContentTrustUntrusted
	}
	return manifest.ContentTrustTrusted
}

// manifestHashFromPath returns the SHA-256 hex digest of the manifest YAML at
// path, forwarded as plugin:manifest_hash. Returns "" when path is empty (an
// in-memory parsed manifest) or unreadable — the daemon treats a missing hash
// as "not provided", matching the prior behaviour. See gibson#997.
func manifestHashFromPath(path string) string {
	if path == "" {
		return ""
	}
	// path is the operator-supplied manifest path (WithManifest / the same path
	// manifest.Load already read above), not untrusted input.
	raw, err := os.ReadFile(path) //nolint:gosec // G304: operator-provided manifest path, not user input
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
