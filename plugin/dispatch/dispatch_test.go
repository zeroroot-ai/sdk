// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
)

// ----------------------------------------------------------------------------
// Fake ComponentClient
// ----------------------------------------------------------------------------

// fakeWork represents one scripted work item returned by fakeClient.PollWork.
type fakeWork struct {
	workID   string
	workType string
	payload  []byte
}

// fakeClient is a test double for ComponentClient.
type fakeClient struct {
	mu      sync.Mutex
	queue   []fakeWork
	results []fakeResult
	// pollErr is returned on the next PollWork call when set.
	pollErr error
}

type fakeResult struct {
	workID  string
	result  []byte
	errInfo *pluginpb.PluginError
}

// enqueue adds a work item to the back of the queue.
func (f *fakeClient) enqueue(w fakeWork) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, w)
}

// PollWork dequeues the next item or returns an empty workID on timeout.
func (f *fakeClient) PollWork(_ context.Context, timeout time.Duration) (string, string, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.pollErr != nil {
		err := f.pollErr
		f.pollErr = nil
		return "", "", nil, err
	}
	if len(f.queue) == 0 {
		// Simulate a poll timeout (empty workID, nil error).
		time.Sleep(min(timeout, 5*time.Millisecond))
		return "", "", nil, nil
	}
	item := f.queue[0]
	f.queue = f.queue[1:]
	return item.workID, item.workType, item.payload, nil
}

// SubmitResult records the submitted result for later assertion.
func (f *fakeClient) SubmitResult(_ context.Context, workID string, result []byte, errInfo *pluginpb.PluginError) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, fakeResult{workID: workID, result: result, errInfo: errInfo})
	return nil
}

// waitResult blocks until at least n results have been submitted or timeout.
func (f *fakeClient) waitResult(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		f.mu.Lock()
		count := len(f.results)
		f.mu.Unlock()
		if count >= n {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// lastResult returns the most recently submitted result (or zero value).
func (f *fakeClient) lastResult() fakeResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.results) == 0 {
		return fakeResult{}
	}
	return f.results[len(f.results)-1]
}

// resultByWorkID returns the submitted result for workID, or false if not found.
func (f *fakeClient) resultByWorkID(workID string) (fakeResult, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.results {
		if r.workID == workID {
			return r, true
		}
	}
	return fakeResult{}, false
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// makeInvokePayload builds a proto-marshaled PluginInvokeRequest whose request
// payload is the JSON-encoded string val (the Go-first wire format: the method
// payload rides in the Any's value as raw JSON).
func makeInvokePayload(t *testing.T, method string, val string) []byte {
	t.Helper()
	return makeInvokePayloadWithDeadlineMs(t, method, val, 5000)
}

// makeInvokePayloadWithDeadlineMs builds a request with a specific deadline.
func makeInvokePayloadWithDeadlineMs(t *testing.T, method string, val string, deadlineMs int64) []byte {
	t.Helper()
	jsonVal, err := json.Marshal(val)
	if err != nil {
		t.Fatalf("json.Marshal(val): %v", err)
	}
	req := &pluginpb.PluginInvokeRequest{
		PluginName: "test-plugin",
		Method:     method,
		Request:    &anypb.Any{TypeUrl: "json:string", Value: jsonVal},
		DeadlineMs: deadlineMs,
	}
	b, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("proto.Marshal(PluginInvokeRequest): %v", err)
	}
	return b
}

// echoHandler is a MethodHandler that returns the raw JSON request unchanged.
func echoHandler(_ context.Context, req json.RawMessage) (json.RawMessage, error) {
	return req, nil
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

// TestNormalDispatch verifies: PollWork item claimed → handler called →
// SubmitResult with marshaled response and no error.
func TestNormalDispatch(t *testing.T) {
	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"Echo": echoHandler,
		},
	})

	payload := makeInvokePayload(t, "Echo", "hello")
	client.enqueue(fakeWork{workID: "w1", workType: WorkTypePluginInvoke, payload: payload})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	if !client.waitResult(1, 3*time.Second) {
		t.Fatal("timed out waiting for SubmitResult")
	}
	r, ok := client.resultByWorkID("w1")
	if !ok {
		t.Fatal("no result for w1")
	}
	if r.errInfo != nil {
		t.Fatalf("expected success, got error: %v", r.errInfo)
	}
	if len(r.result) == 0 {
		t.Fatal("expected non-empty result bytes")
	}
}

// TestMethodNotFound verifies that an unknown method name produces a
// KIND_METHOD_NOT_FOUND PluginError submitted via SubmitResult.
func TestMethodNotFound(t *testing.T) {
	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"Echo": echoHandler,
		},
	})

	payload := makeInvokePayload(t, "NoSuchMethod", "data")
	client.enqueue(fakeWork{workID: "w2", workType: WorkTypePluginInvoke, payload: payload})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	if !client.waitResult(1, 3*time.Second) {
		t.Fatal("timed out waiting for SubmitResult")
	}
	r, ok := client.resultByWorkID("w2")
	if !ok {
		t.Fatal("no result for w2")
	}
	if r.errInfo == nil {
		t.Fatal("expected error, got nil errInfo")
	}
	if r.errInfo.GetKind() != pluginpb.PluginError_PLUGIN_ERROR_KIND_METHOD_NOT_FOUND {
		t.Fatalf("expected METHOD_NOT_FOUND, got %v", r.errInfo.GetKind())
	}
}

// TestHandlerPanic verifies that a panicking handler is recovered and a
// KIND_HANDLER_FAILED error is submitted.
func TestHandlerPanic(t *testing.T) {
	panicHandler := func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		panic("simulated panic — no secrets here")
	}

	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"PanicMethod": panicHandler,
		},
	})

	payload := makeInvokePayload(t, "PanicMethod", "trigger")
	client.enqueue(fakeWork{workID: "w3", workType: WorkTypePluginInvoke, payload: payload})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	if !client.waitResult(1, 3*time.Second) {
		t.Fatal("timed out waiting for SubmitResult")
	}
	r, ok := client.resultByWorkID("w3")
	if !ok {
		t.Fatal("no result for w3")
	}
	if r.errInfo == nil {
		t.Fatal("expected error, got nil errInfo")
	}
	if r.errInfo.GetKind() != pluginpb.PluginError_PLUGIN_ERROR_KIND_HANDLER_FAILED {
		t.Fatalf("expected HANDLER_FAILED, got %v", r.errInfo.GetKind())
	}
	if r.errInfo.GetMessage() != "handler panicked" {
		t.Fatalf("expected 'handler panicked' message, got %q", r.errInfo.GetMessage())
	}
}

// TestHandlerDeadlineExceeded verifies that a handler that blocks past its
// deadline receives a cancelled context and the error maps to KIND_DEADLINE_EXCEEDED.
func TestHandlerDeadlineExceeded(t *testing.T) {
	slowHandler := func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // context.DeadlineExceeded
		case <-time.After(10 * time.Second):
			return json.RawMessage(`"late"`), nil
		}
	}

	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"SlowMethod": slowHandler,
		},
	})

	// 50ms deadline — the slow handler will always time out.
	payload := makeInvokePayloadWithDeadlineMs(t, "SlowMethod", "data", 50)
	client.enqueue(fakeWork{workID: "w4", workType: WorkTypePluginInvoke, payload: payload})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	if !client.waitResult(1, 3*time.Second) {
		t.Fatal("timed out waiting for SubmitResult")
	}
	r, ok := client.resultByWorkID("w4")
	if !ok {
		t.Fatal("no result for w4")
	}
	if r.errInfo == nil {
		t.Fatal("expected error, got nil errInfo")
	}
	if r.errInfo.GetKind() != pluginpb.PluginError_PLUGIN_ERROR_KIND_DEADLINE_EXCEEDED {
		t.Fatalf("expected DEADLINE_EXCEEDED, got %v", r.errInfo.GetKind())
	}
}

// TestUnsupportedWorkType verifies that an unknown work_type produces a
// KIND_INTERNAL error and does not crash the dispatcher.
func TestUnsupportedWorkType(t *testing.T) {
	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{},
	})

	client.enqueue(fakeWork{workID: "w5", workType: "some_other_type", payload: []byte("raw")})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	if !client.waitResult(1, 3*time.Second) {
		t.Fatal("timed out waiting for SubmitResult")
	}
	r := client.lastResult()
	if r.errInfo == nil {
		t.Fatal("expected error for unsupported work_type")
	}
	if r.errInfo.GetKind() != pluginpb.PluginError_PLUGIN_ERROR_KIND_INTERNAL {
		t.Fatalf("expected INTERNAL, got %v", r.errInfo.GetKind())
	}
}

// TestConcurrencyCap verifies that the dispatcher never exceeds Concurrency
// simultaneous handler goroutines.
func TestConcurrencyCap(t *testing.T) {
	const cap = 3
	const totalItems = 10

	var concurrent atomic.Int64
	var maxConcurrent atomic.Int64

	blockCh := make(chan struct{})
	blockingHandler := func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		n := concurrent.Add(1)
		defer concurrent.Add(-1)
		for {
			cur := maxConcurrent.Load()
			if n <= cur || maxConcurrent.CompareAndSwap(cur, n) {
				break
			}
		}
		<-blockCh
		return json.RawMessage(`"ok"`), nil
	}

	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"Block": blockingHandler,
		},
		Concurrency: cap,
	})

	for i := range totalItems {
		payload := makeInvokePayload(t, "Block", fmt.Sprintf("item-%d", i))
		client.enqueue(fakeWork{
			workID:   fmt.Sprintf("cc-%d", i),
			workType: WorkTypePluginInvoke,
			payload:  payload,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	// Give the dispatcher a moment to saturate its concurrency limit.
	time.Sleep(150 * time.Millisecond)

	observed := maxConcurrent.Load()
	if observed > int64(cap) {
		t.Fatalf("concurrency cap violated: max observed %d > cap %d", observed, cap)
	}

	// Unblock all handlers so goroutines drain cleanly.
	close(blockCh)

	if !client.waitResult(totalItems, 5*time.Second) {
		t.Fatalf("timed out waiting for all %d results", totalItems)
	}
}

// TestDrainWaitsForInFlight verifies that Drain blocks until in-flight
// handlers complete.
func TestDrainWaitsForInFlight(t *testing.T) {
	releaseCh := make(chan struct{})
	var handlerCalled atomic.Bool

	slowHandler := func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		handlerCalled.Store(true)
		select {
		case <-releaseCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return json.RawMessage(`"done"`), nil
	}

	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"Slow": slowHandler,
		},
	})

	payload := makeInvokePayload(t, "Slow", "data")
	client.enqueue(fakeWork{workID: "drain-w1", workType: WorkTypePluginInvoke, payload: payload})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	// Wait until the handler has started.
	for !handlerCalled.Load() {
		time.Sleep(5 * time.Millisecond)
	}

	// Start Drain in background.
	drainDone := make(chan error, 1)
	go func() {
		drainDone <- d.Drain(context.Background(), 5*time.Second)
	}()

	// Drain should not finish while the handler is blocked.
	select {
	case err := <-drainDone:
		t.Fatalf("Drain returned prematurely with err=%v", err)
	case <-time.After(50 * time.Millisecond):
	}

	// Release the handler.
	close(releaseCh)

	// Now Drain should complete.
	select {
	case err := <-drainDone:
		if err != nil {
			t.Fatalf("Drain returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Drain timed out after handler release")
	}
}

// TestDrainCtxCancelForcesExit verifies that cancelling the context passed to
// Drain causes it to return context.Canceled immediately.
func TestDrainCtxCancelForcesExit(t *testing.T) {
	blockCh := make(chan struct{})
	blockHandler := func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		select {
		case <-blockCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return json.RawMessage(`"done"`), nil
	}

	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"Block": blockHandler,
		},
	})

	var handlerCalled atomic.Bool
	wrapHandler := func(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
		handlerCalled.Store(true)
		return blockHandler(ctx, req)
	}
	d.cfg.Handlers = map[string]MethodHandler{"Block": wrapHandler}

	payload := makeInvokePayload(t, "Block", "data")
	client.enqueue(fakeWork{workID: "dctx-w1", workType: WorkTypePluginInvoke, payload: payload})

	runCtx, runCancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(runCtx) }()
	defer runCancel()

	for !handlerCalled.Load() {
		time.Sleep(5 * time.Millisecond)
	}

	drainCtx, drainCancel := context.WithCancel(context.Background())
	drainDone := make(chan error, 1)
	go func() {
		drainDone <- d.Drain(drainCtx, 30*time.Second)
	}()

	// Cancel the drain context after a brief delay.
	time.AfterFunc(30*time.Millisecond, drainCancel)

	select {
	case err := <-drainDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Drain did not respect context cancellation")
	}

	close(blockCh)
}

// TestDrainThenExitCallsExiter verifies that DrainThenExit calls the injected
// exiter with exit code 75 after draining.
func TestDrainThenExitCallsExiter(t *testing.T) {
	var exitCode int
	var exitCalled atomic.Bool

	original := exiter
	exiter = func(code int) {
		exitCode = code
		exitCalled.Store(true)
	}
	t.Cleanup(func() { exiter = original })

	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{},
	})

	// No in-flight work; DrainThenExit should drain immediately and exit.
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.DrainThenExit("test rotation reason")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("DrainThenExit did not return in time")
	}

	if !exitCalled.Load() {
		t.Fatal("exiter was not called")
	}
	if exitCode != exitCode75 {
		t.Fatalf("expected exit code %d, got %d", exitCode75, exitCode)
	}
}

// TestMultipleHandlers exercises two distinct methods in a single dispatcher.
func TestMultipleHandlers(t *testing.T) {
	client := &fakeClient{}
	d := New(client, Config{
		Handlers: map[string]MethodHandler{
			"Echo":    echoHandler,
			"Reverse": echoHandler, // simplification: same logic, different name
		},
	})

	client.enqueue(fakeWork{workID: "mh-1", workType: WorkTypePluginInvoke,
		payload: makeInvokePayload(t, "Echo", "ping")})
	client.enqueue(fakeWork{workID: "mh-2", workType: WorkTypePluginInvoke,
		payload: makeInvokePayload(t, "Reverse", "pong")})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = d.Run(ctx) }()
	defer cancel()

	if !client.waitResult(2, 3*time.Second) {
		t.Fatal("timed out waiting for both results")
	}
	for _, id := range []string{"mh-1", "mh-2"} {
		r, ok := client.resultByWorkID(id)
		if !ok {
			t.Fatalf("no result for %s", id)
		}
		if r.errInfo != nil {
			t.Fatalf("expected success for %s, got %v", id, r.errInfo)
		}
	}
}

// min returns the smaller of a and b (for time.Duration).
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
