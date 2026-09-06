// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
)

var (
	// ErrLSPTimeout is returned when an LSP operation exceeds its deadline.
	ErrLSPTimeout = errors.New("lsp operation timed out")

	// ErrLSPNotInitialized is returned when an operation is attempted before initialization.
	ErrLSPNotInitialized = errors.New("lsp server not initialized")

	// ErrLSPShutdown is returned when an operation is attempted after shutdown.
	ErrLSPShutdown = errors.New("lsp server is shut down")
)

// jsonrpcRequest represents a JSON-RPC 2.0 request message.
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response message.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcNotification represents a JSON-RPC 2.0 notification message.
type jsonrpcNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcError represents a JSON-RPC 2.0 error object.
type jsonrpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *jsonrpcError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// lspClient provides a base implementation of JSON-RPC 2.0 over stdio for LSP servers.
// It handles message framing, request/response correlation, and notification dispatching.
type lspClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Request/response correlation
	nextID      atomic.Int64
	pending     map[int64]chan *jsonrpcResponse
	pendingLock sync.Mutex

	// Notification handling
	notificationHandlers     map[string]func(json.RawMessage)
	notificationHandlersLock sync.RWMutex

	// Lifecycle management
	initialized atomic.Bool
	shutdown    atomic.Bool
	readDone    chan struct{}
	errChan     chan error

	logger *slog.Logger
}

// newLSPClient creates a new LSP client for the given command.
// The command should be configured but not started.
func newLSPClient(cmd *exec.Cmd, logger *slog.Logger) (*lspClient, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &lspClient{
		cmd:                  cmd,
		stdin:                stdin,
		stdout:               stdout,
		stderr:               stderr,
		pending:              make(map[int64]chan *jsonrpcResponse),
		notificationHandlers: make(map[string]func(json.RawMessage)),
		readDone:             make(chan struct{}),
		errChan:              make(chan error, 1),
		logger:               logger,
	}

	// Start the language server process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start language server: %w", err)
	}

	// Start reading messages from stdout
	go client.readLoop()

	// Start reading stderr for logging
	go client.stderrLoop()

	return client, nil
}

// readLoop reads JSON-RPC messages from the language server's stdout.
// It handles both responses (correlated with requests) and notifications.
func (c *lspClient) readLoop() {
	defer close(c.readDone)

	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Support up to 1MB messages

	var contentLength int

	for scanner.Scan() {
		line := scanner.Text()

		// Parse Content-Length header
		if line == "" {
			// Empty line indicates end of headers, read content
			if contentLength > 0 {
				content := make([]byte, contentLength)
				n, err := io.ReadFull(c.stdout, content)
				if err != nil {
					c.logger.Error("failed to read message content",
						"error", err,
						"expected", contentLength,
						"got", n)
					return
				}

				c.handleMessage(content)
				contentLength = 0
			}
		} else {
			// Parse header
			var length int
			if _, err := fmt.Sscanf(line, "Content-Length: %d", &length); err == nil {
				contentLength = length
			}
		}
	}

	if err := scanner.Err(); err != nil {
		c.logger.Error("error reading from language server", "error", err)
		select {
		case c.errChan <- err:
		default:
		}
	}
}

// handleMessage processes a JSON-RPC message from the server.
func (c *lspClient) handleMessage(data []byte) {
	// Try to parse as response first
	var resp jsonrpcResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.ID != 0 {
		// This is a response to a request
		c.pendingLock.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendingLock.Unlock()

		if ok {
			select {
			case ch <- &resp:
			default:
				c.logger.Warn("response channel blocked", "id", resp.ID)
			}
		} else {
			c.logger.Warn("received response for unknown request", "id", resp.ID)
		}
		return
	}

	// Try to parse as notification
	var notif jsonrpcNotification
	if err := json.Unmarshal(data, &notif); err == nil && notif.Method != "" {
		// This is a notification
		c.notificationHandlersLock.RLock()
		handler, ok := c.notificationHandlers[notif.Method]
		c.notificationHandlersLock.RUnlock()

		if ok {
			// Marshal params back to JSON for handler
			paramsJSON, _ := json.Marshal(notif.Params)
			handler(paramsJSON)
		} else {
			c.logger.Debug("unhandled notification", "method", notif.Method)
		}
		return
	}

	c.logger.Warn("received unrecognized message", "data", string(data))
}

// stderrLoop reads stderr from the language server for logging purposes.
func (c *lspClient) stderrLoop() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		c.logger.Debug("lsp stderr", "line", line)
	}
}

// sendRequest sends a JSON-RPC request and waits for the response.
func (c *lspClient) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if c.shutdown.Load() {
		return nil, ErrLSPShutdown
	}

	// Allocate request ID
	id := c.nextID.Add(1)

	// Create response channel
	respChan := make(chan *jsonrpcResponse, 1)
	c.pendingLock.Lock()
	c.pending[id] = respChan
	c.pendingLock.Unlock()

	// Ensure cleanup on return
	defer func() {
		c.pendingLock.Lock()
		delete(c.pending, id)
		c.pendingLock.Unlock()
	}()

	// Construct request
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Marshal to JSON
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request with Content-Length header
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(reqJSON), reqJSON)
	if _, err := c.stdin.Write([]byte(msg)); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ErrLSPTimeout
	case <-c.readDone:
		return nil, errors.New("language server connection closed")
	}
}

// sendNotification sends a JSON-RPC notification (no response expected).
func (c *lspClient) sendNotification(method string, params interface{}) error {
	if c.shutdown.Load() {
		return ErrLSPShutdown
	}

	// Construct notification
	notif := jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	// Marshal to JSON
	notifJSON, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Send notification with Content-Length header
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(notifJSON), notifJSON)
	if _, err := c.stdin.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// onNotification registers a handler for a specific notification method.
func (c *lspClient) onNotification(method string, handler func(json.RawMessage)) {
	c.notificationHandlersLock.Lock()
	defer c.notificationHandlersLock.Unlock()
	c.notificationHandlers[method] = handler
}

// close shuts down the LSP client and kills the server process.
func (c *lspClient) close() error {
	if !c.shutdown.CompareAndSwap(false, true) {
		return nil // Already shut down
	}

	// Close stdin to signal EOF to the server
	c.stdin.Close()

	// Wait for read loop to finish (with timeout)
	select {
	case <-c.readDone:
	case <-make(chan struct{}): // Non-blocking check
	}

	// Kill the process if it's still running
	if c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			c.logger.Warn("failed to kill language server process", "error", err)
		}
	}

	// Wait for process to exit
	c.cmd.Wait()

	return nil
}
