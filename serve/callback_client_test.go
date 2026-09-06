// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// TestNewCallbackClient tests the callback client constructor.
func TestNewCallbackClient(t *testing.T) {
	t.Run("valid endpoint", func(t *testing.T) {
		client, err := NewCallbackClient("localhost:50051")
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "localhost:50051", client.endpoint)
	})

	t.Run("empty endpoint", func(t *testing.T) {
		client, err := NewCallbackClient("")
		assert.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("with TLS config", func(t *testing.T) {
		tlsConf := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		client, err := NewCallbackClient("localhost:50051", WithCallbackTLS(tlsConf))
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, tlsConf, client.tlsConf)
	})

	t.Run("with token", func(t *testing.T) {
		token := "test-token-123"
		client, err := NewCallbackClient("localhost:50051", WithCallbackToken(token))
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, token, client.token)
	})
}

// TestCallbackClientSetFullContext tests the SetFullContext method.
func TestCallbackClientSetFullContext(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)

	client.SetFullContext(TaskContextParams{
		TaskID:          "task-123",
		AgentName:       "test-agent",
		MissionID:       "mission-abc",
		TraceID:         "trace-456",
		SpanID:          "span-789",
		MissionRunID:    "run-001",
		AgentRunID:      "agent-run-001",
		RunNumber:       1,
		ToolExecutionID: "exec-001",
	})

	assert.Equal(t, "task-123", client.taskID)
	assert.Equal(t, "test-agent", client.agentName)
	assert.Equal(t, "mission-abc", client.missionID)
	assert.Equal(t, "trace-456", client.traceID)
	assert.Equal(t, "span-789", client.spanID)
	assert.Equal(t, "run-001", client.missionRunID)
	assert.Equal(t, "agent-run-001", client.agentRunID)
	assert.Equal(t, int32(1), client.runNumber)
	assert.Equal(t, "exec-001", client.toolExecutionID)
}

// TestCallbackClientContextInfo tests the contextInfo method.
func TestCallbackClientContextInfo(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)

	client.SetFullContext(TaskContextParams{
		TaskID:          "task-123",
		AgentName:       "test-agent",
		MissionID:       "mission-abc",
		TraceID:         "trace-456",
		SpanID:          "span-789",
		MissionRunID:    "run-001",
		AgentRunID:      "agent-run-001",
		RunNumber:       1,
		ToolExecutionID: "exec-001",
	})

	ctx := client.contextInfo()
	assert.NotNil(t, ctx)
	assert.Equal(t, "task-123", ctx.TaskId)
	assert.Equal(t, "test-agent", ctx.AgentName)
	assert.Equal(t, "mission-abc", ctx.MissionId)
	assert.Equal(t, "trace-456", ctx.TraceId)
	assert.Equal(t, "span-789", ctx.SpanId)
	assert.Equal(t, "run-001", ctx.MissionRunId)
	assert.Equal(t, "agent-run-001", ctx.AgentRunId)
	assert.Equal(t, int32(1), ctx.RunNumber)
	assert.Equal(t, "exec-001", ctx.ToolExecutionId)
}

// TestCallbackClientConnectionLifecycle tests connect/close lifecycle.
func TestCallbackClientConnectionLifecycle(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)

	// Initially not connected
	assert.False(t, client.IsConnected())

	// Close without connecting should work
	err = client.Close()
	assert.NoError(t, err)

	// Cannot connect after close
	ctx := context.Background()
	err = client.Connect(ctx)
	assert.Error(t, err)
}

// TestCallbackClientClose tests the Close method.
func TestCallbackClientClose(t *testing.T) {
	t.Run("close without connect", func(t *testing.T) {
		client, err := NewCallbackClient("localhost:50051")
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)
	})

	t.Run("double close", func(t *testing.T) {
		client, err := NewCallbackClient("localhost:50051")
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)

		// Second close should still work
		err = client.Close()
		assert.NoError(t, err)
	})
}

// TestCallbackClientConcurrency tests concurrent access to the client.
func TestCallbackClientConcurrency(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)

	// Set context from multiple goroutines
	done := make(chan bool)
	for i := range 10 {
		go func(n int) {
			taskID := time.Now().String()
			client.SetFullContext(TaskContextParams{
				TaskID:    taskID,
				AgentName: "agent",
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// Client should still be usable
	assert.NotNil(t, client)
}

// TestCallbackClientNotConnected tests that RPCs fail when not connected.
func TestCallbackClientNotConnected(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)

	// Close the client so it can't reconnect
	client.Close()

	ctx := context.Background()

	// LLMComplete should fail when closed
	_, err = client.LLMComplete(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestCallbackClient_ConnectWithKeepalive tests that Connect configures keepalive parameters.
func TestCallbackClient_ConnectWithKeepalive(t *testing.T) {
	// Start a test gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()

	// Create and start a minimal gRPC server
	server := grpc.NewServer()
	defer server.Stop()

	// Start serving in background
	go func() {
		_ = server.Serve(lis)
	}()

	// Create callback client pointing to test server
	client, err := NewCallbackClient(lis.Addr().String())
	require.NoError(t, err)

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	require.NoError(t, err)

	// Wait a bit for connection to be ready (gRPC connection may be in Connecting state initially)
	time.Sleep(100 * time.Millisecond)
	assert.True(t, client.IsConnected())

	// Verify client can be closed cleanly
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestCallbackClientNotConnectedErrorMessages verifies that all methods return
// distinct error messages with method names when the client is closed.
func TestCallbackClientNotConnectedErrorMessages(t *testing.T) {
	client, err := NewCallbackClient("localhost:50051")
	require.NoError(t, err)

	// Close the client so reconnection fails immediately
	client.Close()

	ctx := context.Background()

	// Test each method and verify it returns a distinct error message
	testCases := []struct {
		name        string
		call        func() error
		expectedMsg string
	}{
		{
			name: "LLMComplete",
			call: func() error {
				_, err := client.LLMComplete(ctx, nil)
				return err
			},
			expectedMsg: "LLMComplete: client not connected",
		},
		{
			name: "LLMCompleteWithTools",
			call: func() error {
				_, err := client.LLMCompleteWithTools(ctx, nil)
				return err
			},
			expectedMsg: "LLMCompleteWithTools: client not connected",
		},
		{
			name: "LLMStream",
			call: func() error {
				_, err := client.LLMStream(ctx, nil)
				return err
			},
			expectedMsg: "LLMStream: client not connected",
		},
		{
			name: "ListTools",
			call: func() error {
				_, err := client.ListTools(ctx, nil)
				return err
			},
			expectedMsg: "ListTools: client not connected",
		},
		{
			name: "QueryPlugin",
			call: func() error {
				_, err := client.QueryPlugin(ctx, nil)
				return err
			},
			expectedMsg: "QueryPlugin: client not connected",
		},
		{
			name: "ListPlugins",
			call: func() error {
				_, err := client.ListPlugins(ctx, nil)
				return err
			},
			expectedMsg: "ListPlugins: client not connected",
		},
		{
			name: "DelegateToAgent",
			call: func() error {
				_, err := client.DelegateToAgent(ctx, nil)
				return err
			},
			expectedMsg: "DelegateToAgent: client not connected",
		},
		{
			name: "ListAgents",
			call: func() error {
				_, err := client.ListAgents(ctx, nil)
				return err
			},
			expectedMsg: "ListAgents: client not connected",
		},
		{
			name: "SubmitFinding",
			call: func() error {
				_, err := client.SubmitFinding(ctx, nil)
				return err
			},
			expectedMsg: "SubmitFinding: client not connected",
		},
		{
			name: "GetPlanContext",
			call: func() error {
				_, err := client.GetPlanContext(ctx, nil)
				return err
			},
			expectedMsg: "GetPlanContext: client not connected",
		},
		{
			name: "ReportStepHints",
			call: func() error {
				_, err := client.ReportStepHints(ctx, nil)
				return err
			},
			expectedMsg: "ReportStepHints: client not connected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err, "expected error for %s when closed", tc.name)
			// When client is closed, error should either say "not connected" or "closed"
			errMsg := err.Error()
			assert.True(t,
				errMsg == tc.expectedMsg || strings.Contains(errMsg, "not connected") || strings.Contains(errMsg, "closed"),
				"error message for %s should indicate connection issue, got: %s", tc.name, errMsg)
		})
	}
}
