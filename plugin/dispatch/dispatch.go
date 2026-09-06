// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package dispatch implements the plugin-side PollWork → handler → SubmitResult
// dispatch loop for the Gibson plugin SDK.
//
// The [Dispatcher] claims work items of type "plugin_invoke" from the daemon
// via the existing ComponentService PollWork RPC, routes each item to the
// matching [MethodHandler] registered by the plugin author, and submits the
// result via SubmitResult. Concurrency is bounded by a semaphore channel;
// graceful shutdown is provided via [Dispatcher.Drain] and
// [Dispatcher.DrainThenExit].
//
// # Wire format (Go-first, ADR-0065 R4)
//
// The PollWork/SubmitResult transport frames (PluginInvokeRequest, PluginError)
// stay protobuf, but the method PAYLOAD is JSON. The daemon places the
// JSON-encoded request in PluginInvokeRequest.request.value (the type_url names
// the method's derived request schema and is informational); the dispatcher
// hands those raw JSON bytes to the handler and submits the handler's raw JSON
// response bytes verbatim. There is no per-method proto message and no proto
// descriptor registry: the handler adapter built by plugin.WithHandler owns the
// JSON⇄Go-struct marshalling, using the schema derived from the author's typed
// request/response structs.
//
// Plugin authors do not use this package directly; it is wired by the top-level
// plugin.Serve entry point.
package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
	"google.golang.org/protobuf/proto"
)

// WorkTypePluginInvoke is the work_type string that identifies a plugin
// method invocation work item in the ComponentService PollWork stream.
const WorkTypePluginInvoke = "plugin_invoke"

// DefaultConcurrency is the maximum number of handler goroutines that run
// concurrently when [Config.Concurrency] is zero or unset.
const DefaultConcurrency = 10

// DefaultPollTimeout is the long-poll timeout passed to [ComponentClient.PollWork]
// when [Config.PollTimeout] is zero or unset.
const DefaultPollTimeout = 20 * time.Second

// defaultDrainTimeout is the timeout used by [Dispatcher.DrainThenExit].
const defaultDrainTimeout = 30 * time.Second

// exitCode75 is the rotation-restart sentinel exit code per Spec 2 Requirement 10.3.
const exitCode75 = 75

// exiter is the function called by [Dispatcher.DrainThenExit] after draining.
// It defaults to os.Exit. Tests replace this package-level variable to capture
// the call rather than actually exiting.
var exiter = os.Exit

// SetExiterForTest replaces the package-level exiter function used by
// [Dispatcher.DrainThenExit] and returns the previous value so callers can
// restore it with defer. This function is intended for use in tests only;
// it is not safe for concurrent use.
func SetExiterForTest(fn func(int)) func(int) {
	prev := exiter
	exiter = fn
	return prev
}

// MethodHandler is the low-level, JSON-in/JSON-out dispatch function for one
// method. The dispatcher passes the raw JSON request payload from
// PluginInvokeRequest.request.value and submits the returned raw JSON bytes as
// the result verbatim.
//
// Plugin authors do NOT implement this directly. They register typed Go
// handlers with plugin.WithHandler; the SDK wraps each in an adapter of this
// type that unmarshals the JSON into the author's request struct and marshals
// the response struct back to JSON. This raw type is exported only so Serve and
// the discovery path can carry handlers.
//
// Handlers must not include resolved secret values in returned errors; the error
// message is forwarded verbatim to the tool caller.
type MethodHandler func(ctx context.Context, req json.RawMessage) (json.RawMessage, error)

// ComponentClient is the subset of the ComponentService client used by the
// dispatcher. It is defined as an interface so that tests can supply a fake
// implementation without a live daemon connection.
type ComponentClient interface {
	// PollWork performs a long-poll for the next work item assigned to this
	// plugin install. It returns an empty workID on timeout (not an error).
	// ctx cancellation terminates the poll immediately.
	PollWork(ctx context.Context, timeout time.Duration) (workID string, workType string, payload []byte, err error)

	// SubmitResult submits the result (or error) for a previously claimed work
	// item identified by workID. errInfo is nil on success.
	SubmitResult(ctx context.Context, workID string, result []byte, errInfo *pluginpb.PluginError) error
}

// InvocationResult is the bounded enumeration the dispatcher passes to the
// [Config.OnInvocationComplete] callback to identify the outcome of a single
// handler invocation. The strings are stable; the values match
// metrics.Result so the callsite can pass them straight through without an
// import-cycle-creating dependency on the metrics package.
type InvocationResult string

const (
	// ResultOK indicates the handler returned a successful response.
	ResultOK InvocationResult = "ok"

	// ResultError indicates the handler returned a non-deadline error.
	ResultError InvocationResult = "error"

	// ResultDeadlineExceeded indicates the handler context deadline was hit.
	ResultDeadlineExceeded InvocationResult = "deadline_exceeded"

	// ResultPanic indicates the handler panicked and was recovered.
	ResultPanic InvocationResult = "panic"

	// ResultMethodNotFound indicates no handler is registered for the
	// requested method name.
	ResultMethodNotFound InvocationResult = "method_not_found"
)

// InvocationCompleteFn is the optional callback signature passed to
// [Config.OnInvocationComplete]. The dispatcher invokes it after every
// plugin_invoke work item is processed (success or any failure).
//
// duration is the wall-clock time from handler entry to handler return for
// successful and error outcomes; for the dispatch-side outcomes
// (method_not_found, panic-during-unmarshal) duration is the time spent in
// the dispatcher's invocation handling.
type InvocationCompleteFn func(method string, duration time.Duration, result InvocationResult)

// Config configures a [Dispatcher].
type Config struct {
	// Handlers maps method names to the plugin-author-supplied handler
	// functions. The map must not be modified after [New] is called.
	Handlers map[string]MethodHandler

	// Concurrency is the maximum number of handler goroutines that may run
	// concurrently. Zero or negative values fall back to [DefaultConcurrency].
	Concurrency int

	// PollTimeout is the duration passed to ComponentClient.PollWork for each
	// long-poll. Zero falls back to [DefaultPollTimeout].
	PollTimeout time.Duration

	// OnInvocationComplete is an optional callback invoked after every
	// plugin_invoke work item is processed. The metrics package wires this
	// to record the gibson_plugin_invoke_duration_seconds histogram and the
	// gibson_plugin_invoke_total counter. When nil, no metric callback is
	// invoked.
	//
	// The callback is invoked synchronously from the dispatcher goroutine
	// after the result has been submitted; it must not block for long.
	OnInvocationComplete InvocationCompleteFn
}

// Dispatcher claims plugin_invoke work items from the daemon, routes them to
// the appropriate [MethodHandler], and submits results. Use [New] to construct
// one.
type Dispatcher struct {
	client   ComponentClient
	cfg      Config
	sem      chan struct{} // semaphore — capacity == Concurrency
	inflight atomic.Int64  // current number of running handler goroutines

	stopOnce sync.Once
	stopCh   chan struct{} // closed to signal Run to stop polling
}

// New constructs a [Dispatcher] from client and cfg. The dispatcher is idle
// until [Dispatcher.Run] is called.
//
// cfg.Concurrency defaults to [DefaultConcurrency] when <= 0.
// cfg.PollTimeout defaults to [DefaultPollTimeout] when zero.
func New(client ComponentClient, cfg Config) *Dispatcher {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultConcurrency
	}
	if cfg.PollTimeout == 0 {
		cfg.PollTimeout = DefaultPollTimeout
	}
	return &Dispatcher{
		client: client,
		cfg:    cfg,
		sem:    make(chan struct{}, cfg.Concurrency),
		stopCh: make(chan struct{}),
	}
}

// Run blocks until ctx is cancelled. It long-polls for work, dispatches each
// claimed item to the matching handler in a bounded goroutine, and submits
// results. When ctx is cancelled Run drains in-flight handlers and returns nil.
//
// Run does not return until all in-flight handlers have completed or ctx is
// cancelled (whichever comes first when draining).
func (d *Dispatcher) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-d.stopCh:
			return nil
		default:
		}

		workID, workType, payload, err := d.client.PollWork(ctx, d.cfg.PollTimeout)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			slog.Error("dispatch: PollWork error", "err", err)
			// Brief pause to avoid a tight error loop, then retry.
			select {
			case <-ctx.Done():
				return nil
			case <-d.stopCh:
				return nil
			case <-time.After(250 * time.Millisecond):
			}
			continue
		}

		// Empty workID means the long-poll timed out with no work — normal.
		if workID == "" {
			continue
		}

		// Dispatch the work item.
		d.dispatch(ctx, workID, workType, payload)
	}
}

// dispatch processes a single work item in a bounded goroutine. It acquires a
// semaphore slot before launching the goroutine; callers that exceed Concurrency
// block here until a slot is available.
func (d *Dispatcher) dispatch(ctx context.Context, workID, workType string, payload []byte) {
	// Acquire semaphore slot (blocks if at capacity).
	select {
	case d.sem <- struct{}{}:
	case <-ctx.Done():
		// Context cancelled while waiting; submit internal error and return.
		_ = d.client.SubmitResult(ctx, workID, nil, &pluginpb.PluginError{
			Kind:    pluginpb.PluginError_PLUGIN_ERROR_KIND_INTERNAL,
			Message: "context cancelled while waiting for dispatch slot",
		})
		return
	}

	d.inflight.Add(1)

	go func() {
		defer func() {
			<-d.sem
			d.inflight.Add(-1)
		}()

		switch workType {
		case WorkTypePluginInvoke:
			d.handlePluginInvoke(ctx, workID, payload)
		default:
			slog.Warn("dispatch: unsupported work_type", "work_id", workID, "work_type", workType)
			_ = d.client.SubmitResult(ctx, workID, nil, &pluginpb.PluginError{
				Kind:    pluginpb.PluginError_PLUGIN_ERROR_KIND_INTERNAL,
				Message: fmt.Sprintf("unsupported work_type %q", workType),
			})
		}
	}()
}

// handlePluginInvoke processes a single plugin_invoke work item end-to-end.
// All errors are translated into PluginError and submitted via SubmitResult;
// this function never returns an error to its caller.
//
// Metrics: a single OnInvocationComplete callback fires from the deferred
// reporter regardless of which exit path is taken. The method label is
// best-effort: it is the parsed method name when available and "unknown"
// when the envelope failed to unmarshal.
func (d *Dispatcher) handlePluginInvoke(ctx context.Context, workID string, payload []byte) {
	start := time.Now()
	method := "unknown"
	result := ResultError
	defer func() {
		if d.cfg.OnInvocationComplete != nil {
			d.cfg.OnInvocationComplete(method, time.Since(start), result)
		}
	}()

	// 1. Unmarshal the PluginInvokeRequest envelope.
	var req pluginpb.PluginInvokeRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		slog.Error("dispatch: failed to unmarshal PluginInvokeRequest",
			"work_id", workID, "err", err)
		d.submitErr(ctx, workID, pluginpb.PluginError_PLUGIN_ERROR_KIND_INTERNAL,
			"failed to unmarshal request envelope")
		return
	}
	method = req.GetMethod()

	// 2. Look up the handler.
	handler, ok := d.cfg.Handlers[method]
	if !ok {
		result = ResultMethodNotFound
		d.submitErr(ctx, workID, pluginpb.PluginError_PLUGIN_ERROR_KIND_METHOD_NOT_FOUND,
			fmt.Sprintf("method %q not declared in handler registry", method))
		return
	}

	// 3. Extract the raw JSON request payload. The transport wraps it in an
	//    Any whose value is the JSON bytes; a nil Any means an empty request,
	//    which is legal (a method may take no arguments).
	var reqJSON json.RawMessage
	if reqAny := req.GetRequest(); reqAny != nil {
		reqJSON = json.RawMessage(reqAny.GetValue())
	}

	// 4. Build the handler context with the deadline from the request.
	handlerCtx := ctx
	if req.GetDeadlineMs() > 0 {
		deadline := time.Now().Add(time.Duration(req.GetDeadlineMs()) * time.Millisecond)
		var cancel context.CancelFunc
		handlerCtx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	// 5. Call the handler with panic recovery. callHandlerSafe sets result
	//    via the pointer to one of ResultOK / ResultError /
	//    ResultDeadlineExceeded / ResultPanic.
	respJSON, handlerErr := d.callHandlerSafe(handlerCtx, workID, method, handler, reqJSON, &result)
	if handlerErr != nil {
		return // callHandlerSafe submitted the error result and set result.
	}

	// 6. Submit the handler's raw JSON response verbatim.
	if submitErr := d.client.SubmitResult(ctx, workID, respJSON, nil); submitErr != nil {
		slog.Error("dispatch: SubmitResult failed",
			"work_id", workID, "method", method, "err", submitErr)
		// SubmitResult failure is a transport problem after a successful
		// handler run; keep result=OK so business-side success counts.
	}
	result = ResultOK
}

// callHandlerSafe calls handler(ctx, req) with panic recovery. On success it
// returns the response message and nil. On handler error or panic it submits the
// appropriate PluginError and returns nil, non-nil so the caller knows submission
// was already handled.
//
// resultOut is set to one of the InvocationResult variants on every exit path
// so the deferred metric reporter in handlePluginInvoke records the right
// label. resultOut may be nil for callers that do not need metric reporting.
func (d *Dispatcher) callHandlerSafe(
	ctx context.Context,
	workID, method string,
	handler MethodHandler,
	req json.RawMessage,
	resultOut *InvocationResult,
) (resp json.RawMessage, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("dispatch: handler panicked",
				"work_id", workID, "method", method,
				"panic", fmt.Sprintf("%v", r),
				// Emit only the first 512 bytes of the stack to avoid log flooding;
				// do NOT include any values that could contain secrets.
				"stack_head", truncate(string(stack), 512),
			)
			d.submitErr(ctx, workID, pluginpb.PluginError_PLUGIN_ERROR_KIND_HANDLER_FAILED,
				"handler panicked")
			if resultOut != nil {
				*resultOut = ResultPanic
			}
			retErr = errors.New("handler panicked")
		}
	}()

	result, err := handler(ctx, req)
	if err != nil {
		kind := pluginpb.PluginError_PLUGIN_ERROR_KIND_HANDLER_FAILED
		invResult := ResultError
		if errors.Is(err, context.DeadlineExceeded) {
			kind = pluginpb.PluginError_PLUGIN_ERROR_KIND_DEADLINE_EXCEEDED
			invResult = ResultDeadlineExceeded
		}
		d.submitErr(ctx, workID, kind, err.Error())
		if resultOut != nil {
			*resultOut = invResult
		}
		return nil, err
	}
	return result, nil
}

// submitErr is a convenience wrapper that submits a PluginError result for
// workID without a payload. Errors from SubmitResult are logged but not
// returned because the caller has no useful recovery path.
func (d *Dispatcher) submitErr(ctx context.Context, workID string, kind pluginpb.PluginError_Kind, msg string) {
	if err := d.client.SubmitResult(ctx, workID, nil, &pluginpb.PluginError{
		Kind:    kind,
		Message: msg,
	}); err != nil {
		slog.Error("dispatch: SubmitResult(error) failed",
			"work_id", workID, "kind", kind, "err", err)
	}
}

// InFlight returns the current number of running handler goroutines. This is
// used by the Drainer to wait for completion before exit.
func (d *Dispatcher) InFlight() int {
	return int(d.inflight.Load())
}

// Drain stops polling new work and waits for all in-flight handlers to complete
// or timeout to elapse, whichever comes first. After Drain returns, the
// dispatcher is stopped and Run will return on its next iteration.
//
// Drain is idempotent; calling it multiple times is safe.
func (d *Dispatcher) Drain(ctx context.Context, timeout time.Duration) error {
	// Signal the poll loop to stop accepting new work.
	d.stopOnce.Do(func() { close(d.stopCh) })

	deadline := time.Now().Add(timeout)
	for {
		if d.InFlight() == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			n := d.InFlight()
			return fmt.Errorf("drain timeout: %d handler(s) still in flight", n)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// DrainThenExit drains in-flight handlers with a 30-second timeout and then
// calls os.Exit(75). The exit code 75 is the rotation-restart sentinel per
// Spec 2 Requirement 10.3; the orchestrator restarts the plugin to pick up
// rotated credentials.
//
// reason is logged at Info level before draining so operators can identify the
// cause of the restart.
func (d *Dispatcher) DrainThenExit(reason string) {
	slog.Info("dispatch: initiating rotation-restart",
		"reason", reason,
		"exit_code", exitCode75,
	)
	ctx, cancel := context.WithTimeout(context.Background(), defaultDrainTimeout)
	defer cancel()
	if err := d.Drain(ctx, defaultDrainTimeout); err != nil {
		slog.Warn("dispatch: drain completed with error before exit", "err", err)
	}
	exiter(exitCode75)
}

// truncate returns at most n bytes of s, appending "…" if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
