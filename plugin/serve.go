// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
	"github.com/zeroroot-ai/sdk/capabilitygrant"
	"github.com/zeroroot-ai/sdk/plugin/dispatch"
	"github.com/zeroroot-ai/sdk/plugin/events"
	"github.com/zeroroot-ai/sdk/plugin/health"
	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/metrics"
	pluginsecrets "github.com/zeroroot-ai/sdk/plugin/secrets"
)

// MethodHandler is the low-level, JSON-in/JSON-out dispatch adapter type,
// aliased from [dispatch.MethodHandler]. Plugin authors do NOT implement it
// directly — they register typed Go handlers with [WithHandler], which builds
// the adapter and derives the method schema from the Go types.
//
// Handlers MUST NOT include resolved secret values in any returned error string.
type MethodHandler = dispatch.MethodHandler

// Serve is the single entry point a plugin author calls from main(). It
// orchestrates all plugin SDK components:
//
//  1. Checks the declaration in code: [WithName], [WithVersion] and at least
//     one [WithHandler]. No manifest file exists (ADR-0097).
//  2. Acquires a daemon connection via capabilitygrant (Bootstrap → Discover → Register).
//  3. Registers the handlers as the plugin's method set.
//  4. Constructs the secrets client wrapping GetCredential.
//  5. Starts the lifecycle state machine and invokes OnStart. A plugin that
//     needs a secret to start resolves it there; an error fails Serve.
//  6. Transitions to Ready.
//  8. Starts the health server on the configured port.
//  9. Starts the events subscriber on the WatchComponentEvents stream.
//  10. Starts the dispatch loop (PollWork → handler → SubmitResult).
//  11. Blocks until ctx is cancelled or a fatal error occurs.
//  12. On SIGTERM/SIGINT: stops new work, drains in-flight handlers up to
//     drainTimeout, runs OnStop, exits cleanly.
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
	// Step 1: Check the declaration in code (ADR-0097).
	// -------------------------------------------------------------------------
	if cfg.name == "" {
		return errors.New("plugin.Serve: WithName is required")
	}
	if cfg.version == "" {
		return errors.New("plugin.Serve: WithVersion is required")
	}
	if len(cfg.handlers) == 0 {
		return errors.New("plugin.Serve: no methods to register; pass at least one WithHandler")
	}
	slog.Info("plugin: declared", "name", cfg.name, "version", cfg.version, "methods", len(cfg.handlers))

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
			"plugin", cfg.name,
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
		metrics.Default.RecordTransition(cfg.name, iid, from, to)
		if to == lifecycle.Ready {
			startupOnce.Do(func() {
				metrics.Default.ObserveStartup(cfg.name, t0)
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

	hostKeyPath, err := pluginHostKeyPath(cfg.name)
	if err != nil {
		return fmt.Errorf("plugin.Serve: resolve host key path: %w", err)
	}

	cgClient, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
		PlatformURL:    platformURL,
		BootstrapToken: cfg.bootstrapToken,
		HostKeyPath:    hostKeyPath,
		AgentName:      cfg.name,
		AgentMode:      "autonomous",
	})
	if err != nil {
		return fmt.Errorf("plugin.Serve: capabilitygrant.NewClient: %w", err)
	}
	cgClient.SetHTTPClient(cfg.httpClient)

	if err := cgClient.Discover(signalCtx); err != nil {
		return fmt.Errorf("plugin.Serve: capabilitygrant.Discover: %w", err)
	}

	if err := cgClient.Register(signalCtx); err != nil {
		return fmt.Errorf("plugin.Serve: capabilitygrant.Register: %w", err)
	}
	slog.Info("plugin: registered with daemon",
		"plugin", cfg.name,
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

	// The secrets client wraps GetCredential.
	secretsClient := cfg.secretsClient
	if secretsClient == nil {
		secretsClient = buildSecretsClient(harnessCallbackSvcClient)
	}

	// Inject the secrets client into the context so that the OnStart hook,
	// the method handlers (via the dispatch loop), and the OnStop hook can
	// resolve secrets through plugin.ResolveSecret / secrets.FromContext. Every
	// context derived from signalCtx below — RunOnStart, the errgroup ctx that
	// feeds disp.Run, and the health/events goroutines — inherits this value.
	signalCtx = pluginsecrets.NewContext(signalCtx, secretsClient)

	// The registered handlers are the method set: names (RegisterComponentRequest.methods)
	// plus rich descriptors (method_descriptors) so the connector catalog and
	// SearchTools can surface per-method descriptions to agents.
	methodNames, methodDescriptors := buildMethodMetadata(cfg.methodSchemas, cfg.methodDescriptions)

	// Register as a plugin component. plugin:host_id is the RFC-7638
	// thumbprint that keys per-host install uniqueness in the daemon. The
	// daemon decides every authorization itself: a tenant admin grants a
	// plugin its secrets, and no metadata key declares one (sdk#129).
	regResp, err := componentSvcClient.RegisterComponent(signalCtx, &componentpb.RegisterComponentRequest{
		Kind:              "plugin",
		Name:              cfg.name,
		Version:           cfg.version,
		Methods:           methodNames,
		MethodDescriptors: methodDescriptors,
		Metadata: map[string]string{
			"plugin:host_id": cgClient.HostID(),
		},
	})
	if err != nil {
		return fmt.Errorf("plugin.Serve: RegisterComponent: %w", err)
	}
	instanceID := regResp.GetInstanceId()
	slog.Info("plugin: component registered",
		"plugin", cfg.name,
		"instance_id", instanceID,
	)

	// Now that an install_id exists, backfill the per-install gauge to the
	// state machine's current state. Subsequent transitions update the gauge
	// via the observer registered above.
	instanceIDForGauge.Store(&instanceID)
	metrics.Default.SetState(cfg.name, instanceID, sm.Current())

	// -------------------------------------------------------------------------
	// Step 8: Transition to ResolvingSecrets. The secrets client is ready; a
	// plugin that needs a secret to start resolves it in OnStart below.
	// -------------------------------------------------------------------------
	if err := sm.Transition(lifecycle.ResolvingSecrets); err != nil {
		return fmt.Errorf("plugin.Serve: lifecycle transition to ResolvingSecrets: %w", err)
	}

	// -------------------------------------------------------------------------
	// Step 9: Transition to Starting	// -------------------------------------------------------------------------
	// Step 11: Transition to Starting, run OnStart, transition to Ready.
	// -------------------------------------------------------------------------
	if err := sm.Transition(lifecycle.Starting); err != nil {
		return fmt.Errorf("plugin.Serve: lifecycle transition to Starting: %w", err)
	}

	// A plugin that needs a secret resolves it here and returns the error, so
	// a missing grant fails the start with the secret named.
	if err := sm.RunOnStart(signalCtx); err != nil {
		return fmt.Errorf("plugin.Serve: OnStart hook failed: %w", err)
	}
	// sm is now in Ready state (transitioned by RunOnStart).

	// -------------------------------------------------------------------------
	// Step 10: Start health server.
	// -------------------------------------------------------------------------
	healthSrv := health.New(sm, cfg.healthAddr, health.DefaultLivenessInterval)
	boundAddr, err := healthSrv.Start(signalCtx)
	if err != nil {
		return fmt.Errorf("plugin.Serve: start health server: %w", err)
	}
	slog.Info("plugin: health server started", "addr", boundAddr)

	// -------------------------------------------------------------------------
	// Step 11: Build the dispatcher.
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
				cfg.name, method, metrics.Result(res), dur,
			)
		},
	})

	// -------------------------------------------------------------------------
	// Step 12: Build the events subscriber.
	// -------------------------------------------------------------------------
	eventStream := newComponentEventStream(componentSvcClient, cfg.name)

	sub := events.New(eventStream, secretsClient, sm)
	pluginName := cfg.name
	sub.SetOnRotation(func(_ string, lag time.Duration) {
		metrics.Default.ObserveRotationPropagation(pluginName, lag)
	})

	// -------------------------------------------------------------------------
	// Step 13: Run background goroutines.
	// -------------------------------------------------------------------------
	heartbeatInterval := time.Duration(regResp.GetHeartbeatIntervalMs()) * time.Millisecond
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}

	eg, egCtx := errgroup.WithContext(signalCtx)

	// Heartbeat loop.
	eg.Go(func() error {
		return runHeartbeat(egCtx, componentSvcClient, instanceID, heartbeatInterval, sm, healthSrv)
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
		return gracefulShutdown(shutdownCtx, sm, disp, cfg.drainTimeout, cfg.name)
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

// buildMethodMetadata assembles, from the registered handlers, the parallel
// name list (RegisterComponentRequest.methods) and the rich per-method
// descriptors (method_descriptors). Descriptions flow through so the connector
// catalog and SearchTools can surface them. Sorted, because ranging a map
// would let Go's randomised iteration order into the RegisterComponent payload
// and make the registered method list differ between two runs of the same
// plugin.
func buildMethodMetadata(schemas map[string]methodSchema, descriptions map[string]string) ([]string, []*componentpb.ComponentMethod) {
	names := make([]string, 0, len(descriptions))
	for name := range descriptions {
		names = append(names, name)
	}
	sort.Strings(names)
	detailed := make([]*componentpb.ComponentMethod, 0, len(names))
	for _, name := range names {
		cm := &componentpb.ComponentMethod{Name: name, Description: descriptions[name]}
		// The Go-first request schema derived from the handler's typed struct
		// travels to the daemon as the method's tool-input contract.
		if s, ok := schemas[name]; ok {
			cm.InputSchemaJson = s.input
		}
		detailed = append(detailed, cm)
	}
	return names, detailed
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
func buildSecretsClient(hc harnesspb.HarnessCallbackServiceClient) pluginsecrets.Client {
	return pluginsecrets.New(func(ctx context.Context, name string) ([]byte, error) {
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

// Heartbeat health_status values the daemon maps to an install status
// (gibson internal/platform/component InstallStatusFromHealth): "degraded"
// marks the install DEGRADED, every other value keeps it SERVING.
const (
	heartbeatHealthServing  = "serving"
	heartbeatHealthDegraded = "degraded"
)

// lifecycleStatus is the read side of the state machine the heartbeat
// reports. *lifecycle.StateMachine satisfies it.
type lifecycleStatus interface {
	Status() (lifecycle.State, string)
}

// heartbeatHealth maps a lifecycle state and its Degraded reason to the
// health_status and health_message fields of a HeartbeatRequest. Degraded
// reports "degraded" with the reason the state machine recorded (for example
// "secret_revoked: cred:github_token"). Every other state the heartbeat loop
// runs in reports "serving": the loop starts after Ready and stops with the
// errgroup before the drain, so Draining and Stopped never reach the wire.
func heartbeatHealth(state lifecycle.State, reason string) (healthStatus, healthMessage string) {
	if state == lifecycle.Degraded {
		if reason == "" {
			reason = "degraded"
		}
		return heartbeatHealthDegraded, reason
	}
	return heartbeatHealthServing, "ok"
}

// runHeartbeat sends periodic HeartbeatRequests until ctx is cancelled. Every
// tick reads the lifecycle state, so the daemon learns of a Degraded plugin
// within one interval. On each successful heartbeat it records the event in
// the health server.
func runHeartbeat(
	ctx context.Context,
	client componentpb.ComponentServiceClient,
	instanceID string,
	interval time.Duration,
	sm lifecycleStatus,
	srv *health.Server,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			healthStatus, healthMessage := heartbeatHealth(sm.Status())
			_, err := client.Heartbeat(ctx, &componentpb.HeartbeatRequest{
				InstanceId:    instanceID,
				HealthStatus:  healthStatus,
				HealthMessage: healthMessage,
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
		IdempotencyKey: uuid.NewString(),
		WorkId:         workID,
		Result:         result,
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
// componentEventStream adapts ComponentService.WatchComponentEvents to
// events.EventStream. The daemon keys the subscription on the caller's
// identity, so the request names nothing. The daemon replays nothing on
// subscribe: a plugin that connects after a revocation learns it on its next
// GetCredential, which the daemon denies.
// ----------------------------------------------------------------------------

// eventTypeHeartbeat is the keepalive the daemon sends on an idle stream. The
// adapter drops it after a debug log so the subscriber never sees it.
const eventTypeHeartbeat = "heartbeat"

const (
	// eventStreamInitialBackoff is the first wait after a stream error.
	eventStreamInitialBackoff = 500 * time.Millisecond
	// eventStreamMaxBackoff caps the wait between reconnect attempts.
	eventStreamMaxBackoff = 30 * time.Second
)

// componentEventStream implements events.EventStream over the generated
// WatchComponentEvents stream. One instance serves one subscriber goroutine;
// it is not safe for concurrent Recv calls.
type componentEventStream struct {
	client     componentpb.ComponentServiceClient
	pluginName string

	// stream is the open server stream, nil until the first Recv opens one
	// and again after a stream error.
	stream grpc.ServerStreamingClient[componentpb.ComponentEvent]
	// cancelStream ends the open stream's context on reconnect.
	cancelStream context.CancelFunc

	// backoff is the wait before the next reconnect attempt. It doubles on
	// each consecutive failure up to maxBackoff and resets on a received
	// message.
	backoff        time.Duration
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// newComponentEventStream returns an adapter that opens the stream lazily on
// the first Recv call.
func newComponentEventStream(client componentpb.ComponentServiceClient, pluginName string) *componentEventStream {
	return &componentEventStream{
		client:         client,
		pluginName:     pluginName,
		initialBackoff: eventStreamInitialBackoff,
		maxBackoff:     eventStreamMaxBackoff,
	}
}

// Recv returns the next secret event. It opens the stream on first use,
// drops heartbeats, and on any stream error other than the caller's
// cancellation reconnects with capped exponential backoff, so a daemon
// rollout does not end the subscription for good. Recv returns an error only
// when ctx is done.
func (s *componentEventStream) Recv(ctx context.Context) (events.Event, error) {
	for {
		if ctx.Err() != nil {
			s.closeStream()
			return events.Event{}, fmt.Errorf("event stream: %w", ctx.Err())
		}
		if s.stream == nil {
			if err := s.open(ctx); err != nil {
				if waitErr := s.waitBackoff(ctx, "open", err); waitErr != nil {
					return events.Event{}, waitErr
				}
				continue
			}
		}

		msg, err := s.stream.Recv()
		if err != nil {
			s.closeStream()
			if waitErr := s.waitBackoff(ctx, "recv", err); waitErr != nil {
				return events.Event{}, waitErr
			}
			continue
		}
		s.backoff = s.initialBackoff

		if msg.GetType() == eventTypeHeartbeat {
			slog.Debug("plugin: event stream heartbeat",
				"plugin", s.pluginName)
			continue
		}
		return componentEventToEvent(msg), nil
	}
}

// open starts a WatchComponentEvents stream on a context derived from ctx.
func (s *componentEventStream) open(ctx context.Context) error {
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := s.client.WatchComponentEvents(streamCtx, &componentpb.WatchComponentEventsRequest{})
	if err != nil {
		cancel()
		return fmt.Errorf("WatchComponentEvents: %w", err)
	}
	s.stream = stream
	s.cancelStream = cancel
	slog.Info("plugin: event stream subscribed", "plugin", s.pluginName)
	return nil
}

// closeStream cancels the open stream, if any, and forgets it.
func (s *componentEventStream) closeStream() {
	if s.cancelStream != nil {
		s.cancelStream()
	}
	s.stream = nil
	s.cancelStream = nil
}

// waitBackoff logs the failure, sleeps for the current backoff, and doubles
// it up to maxBackoff. It returns the context error when ctx ends first, and
// nil after the sleep. A failure while ctx is already done is the caller's
// cancellation, not a stream fault, so it is not logged.
func (s *componentEventStream) waitBackoff(ctx context.Context, op string, cause error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("event stream: %w", ctx.Err())
	}
	if s.backoff <= 0 {
		s.backoff = s.initialBackoff
	}
	slog.Warn("plugin: event stream failed, reconnecting",
		"plugin", s.pluginName,
		"op", op,
		"code", status.Code(cause).String(),
		"retry_in", s.backoff,
		"err", cause,
	)
	timer := time.NewTimer(s.backoff)
	defer timer.Stop()
	s.backoff = min(s.backoff*2, s.maxBackoff)
	select {
	case <-ctx.Done():
		return fmt.Errorf("event stream: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// componentEventToEvent maps one wire message to the subscriber's Event.
func componentEventToEvent(msg *componentpb.ComponentEvent) events.Event {
	ev := events.Event{
		Type:    msg.GetType(),
		Name:    msg.GetSecretName(),
		Reason:  msg.GetReason(),
		Version: int(msg.GetVersion()),
	}
	if ts := msg.GetOccurredAt(); ts != nil {
		ev.OccurredAt = ts.AsTime()
	}
	return ev
}
