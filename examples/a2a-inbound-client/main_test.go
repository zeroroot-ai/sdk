// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePlatform serves the enrollment endpoints and the inbound A2A endpoint of
// one agent. The strict copy of the token checks of the daemon is in the test
// of the package a2ainbound. This fake checks that each request has a token
// for its own path.
type fakePlatform struct {
	a2asrv.RequestHandler // nil: the endpoint serves three methods only
	srv                   *httptest.Server

	mu sync.Mutex
	// endAfter is the number of reads after which the Task is completed. A
	// negative number keeps the Task in the state working.
	endAfter int
	reads    int
	canceled bool
	targets  any
	// fail names one method that answers with an error.
	fail string
}

func (f *fakePlatform) failure(method string) error {
	if f.fail == method {
		return a2a.ErrInternalError
	}
	return nil
}

func newFakePlatform(t *testing.T, endAfter int) *fakePlatform {
	t.Helper()
	f := &fakePlatform{endAfter: endAfter}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"protocol_version": "1.0",
			"endpoints":        map[string]string{"register": f.srv.URL + "/register"},
		})
	})
	mux.HandleFunc("POST /register", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"agent_id": "agent-1", "component_scope": "component:agent/caller"})
	})
	mux.Handle("POST /a2a/inbound/recon", boundToPath(a2asrv.NewJSONRPCHandler(f)))
	mux.Handle("GET /a2a/inbound/recon/.well-known/agent-card.json", boundToPath(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(&a2a.AgentCard{Name: "recon", Description: "finds hosts"})
		})))
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// boundToPath refuses a request whose token does not name the path.
func boundToPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var claims struct {
			Method string `json:"method"`
		}
		parts := strings.Split(r.Header.Get("x-capability-grant"), ".")
		if len(parts) == 3 {
			raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
			_ = json.Unmarshal(raw, &claims)
		}
		if claims.Method != r.URL.Path {
			http.Error(w, "no grant for this path", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (f *fakePlatform) task(state a2a.TaskState) *a2a.Task {
	return &a2a.Task{ID: "mission-1", ContextID: "ctx-1", Status: a2a.TaskStatus{State: state}}
}

func (f *fakePlatform) SendMessage(_ context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range req.Message.Parts {
		if data, ok := p.Data().(map[string]any); ok {
			f.targets = data["targets"]
		}
	}
	return f.task(a2a.TaskStateSubmitted), f.failure("send")
}

func (f *fakePlatform) GetTask(context.Context, *a2a.GetTaskRequest) (*a2a.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.endAfter >= 0 && f.reads >= f.endAfter {
		return f.task(a2a.TaskStateCompleted), nil
	}
	return f.task(a2a.TaskStateWorking), f.failure("get")
}

func (f *fakePlatform) CancelTask(context.Context, *a2a.CancelTaskRequest) (*a2a.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.canceled = true
	return f.task(a2a.TaskStateCanceled), f.failure("cancel")
}

func (f *fakePlatform) options(t *testing.T) options {
	t.Helper()
	t.Setenv("SPIFFE_ENDPOINT_SOCKET", "")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "boot-1")
	return options{
		platformURL: f.srv.URL, agent: "recon", text: "scan the hosts",
		hostKeyPath: filepath.Join(t.TempDir(), "host_key.json"),
		wait:        time.Minute, poll: time.Millisecond,
		transport: f.srv.Client().Transport,
	}
}

func TestRunSendsAMessageAndWaitsForTheTask(t *testing.T) {
	f := newFakePlatform(t, 2)
	opts := f.options(t)
	opts.targets = []string{"target-1"}
	var out bytes.Buffer

	require.NoError(t, run(context.Background(), opts, &out))
	assert.Contains(t, out.String(), "agent recon: finds hosts")
	assert.Contains(t, out.String(), "task mission-1: TASK_STATE_SUBMITTED")
	assert.Contains(t, out.String(), "task mission-1: TASK_STATE_COMPLETED")
	assert.Equal(t, 2, f.reads)
	assert.False(t, f.canceled)
	assert.Equal(t, []any{"target-1"}, f.targets)
}

func TestRunCancelsATaskThatRunsTooLong(t *testing.T) {
	f := newFakePlatform(t, -1)
	opts := f.options(t)
	opts.wait = 20 * time.Millisecond
	var out bytes.Buffer

	require.NoError(t, run(context.Background(), opts, &out))
	assert.True(t, f.canceled)
	assert.Contains(t, out.String(), "task mission-1: TASK_STATE_CANCELED")
}

func TestRunStopsWhenTheContextEnds(t *testing.T) {
	f := newFakePlatform(t, -1)
	opts := f.options(t)
	opts.poll = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, run(ctx, opts, &bytes.Buffer{}), context.DeadlineExceeded)
}

func TestRunFailsWithNoPlatform(t *testing.T) {
	f := newFakePlatform(t, 0)
	opts := f.options(t)
	opts.platformURL = "http://plain.example.com"
	require.ErrorContains(t, run(context.Background(), opts, &bytes.Buffer{}), "make the identity")

	opts = f.options(t)
	opts.agent = "not-published"
	require.ErrorContains(t, run(context.Background(), opts, &bytes.Buffer{}), "read the agent card")
}

func TestRunReportsAFailedCall(t *testing.T) {
	for method, want := range map[string]string{
		"send": "send the message", "get": "read the task", "cancel": "cancel the task",
	} {
		f := newFakePlatform(t, -1)
		f.fail = method
		opts := f.options(t)
		if method == "cancel" {
			opts.wait = 0
		}
		require.ErrorContains(t, run(context.Background(), opts, &bytes.Buffer{}), want, method)
	}
}

func TestOptionsFrom(t *testing.T) {
	env := map[string]string{
		envPlatformURL: "https://api.example.com", envAgent: "recon",
		envTargets: "target-1, target-2,", envHostKey: "/tmp/key.json",
	}
	getenv := func(k string) string { return env[k] }

	opts, err := optionsFrom(getenv, []string{"scan", "the", "hosts"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com", opts.platformURL)
	assert.Equal(t, "recon", opts.agent)
	assert.Equal(t, "scan the hosts", opts.text)
	assert.Equal(t, []string{"target-1", "target-2"}, opts.targets)
	assert.Equal(t, "/tmp/key.json", opts.hostKeyPath)

	_, err = optionsFrom(getenv, nil)
	require.ErrorContains(t, err, "usage")
	delete(env, envAgent)
	_, err = optionsFrom(getenv, []string{"scan"})
	require.ErrorContains(t, err, envAgent)
	delete(env, envPlatformURL)
	_, err = optionsFrom(getenv, []string{"scan"})
	require.ErrorContains(t, err, envPlatformURL)
}
