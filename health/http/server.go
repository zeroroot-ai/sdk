// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/zeroroot-ai/sdk/types"
)

// Server is an HTTP server that exposes health endpoints for Kubernetes probes.
// It provides /healthz (liveness) and /readyz (readiness) endpoints following
// Kubernetes best practices.
type Server struct {
	config           *Config
	httpServer       *http.Server
	livenessChecks   map[string]CheckFunc
	readinessChecks  map[string]CheckFunc
	mu               sync.RWMutex
	shutdownComplete chan struct{}
}

// NewServer creates a new health server with the given configuration.
// If cfg is nil, DefaultConfig() is used.
func NewServer(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	s := &Server{
		config:           cfg,
		livenessChecks:   make(map[string]CheckFunc),
		readinessChecks:  make(map[string]CheckFunc),
		shutdownComplete: make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)

	s.httpServer = &http.Server{
		Addr:         net.JoinHostPort(cfg.BindAddress, strconv.Itoa(cfg.Port)),
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	return s
}

// RegisterLivenessCheck registers a health check function for the /healthz endpoint.
// Liveness checks should be fast (< 100ms) and only verify the process is alive.
// If any liveness check fails, Kubernetes will restart the pod.
func (s *Server) RegisterLivenessCheck(name string, check CheckFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.livenessChecks[name] = check
}

// RegisterReadinessCheck registers a health check function for the /readyz endpoint.
// Readiness checks verify that the service is ready to handle traffic.
// If any readiness check fails, Kubernetes will remove the pod from service endpoints.
func (s *Server) RegisterReadinessCheck(name string, check CheckFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readinessChecks[name] = check
}

// Start starts the HTTP server in a goroutine (non-blocking).
// Returns an error if the server fails to start (e.g., port already in use).
// The server will continue running until Stop() is called.
func (s *Server) Start() error {
	// Test if we can bind to the port by creating a temporary listener
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", s.httpServer.Addr, err)
	}

	// Start server in goroutine
	go func() {
		defer close(s.shutdownComplete)
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Server failed unexpectedly - log but don't crash
			// In production, this would use a proper logger
			fmt.Printf("health server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server using the provided context.
// The context deadline determines how long to wait for in-flight requests to complete.
// If the context deadline is exceeded, the server is forcefully closed.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		// Force close if graceful shutdown fails
		if closeErr := s.httpServer.Close(); closeErr != nil {
			return fmt.Errorf("shutdown error: %w, close error: %w", err, closeErr)
		}
		return fmt.Errorf("forced shutdown after error: %w", err)
	}

	// Wait for server goroutine to complete
	<-s.shutdownComplete

	return nil
}

// handleHealthz handles the /healthz (liveness) endpoint.
// Returns 200 OK if the process is alive and healthy.
// Returns 503 Service Unavailable if any liveness check fails.
// If no checks are registered, returns 200 OK (default healthy).
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	checks := make(map[string]CheckFunc, len(s.livenessChecks))
	for name, check := range s.livenessChecks {
		checks[name] = check
	}
	s.mu.RUnlock()

	// If no checks registered, default to healthy
	if len(checks) == 0 {
		s.writeResponse(w, types.NewHealthyStatus(""), nil)
		return
	}

	// Run checks with short timeout for liveness
	ctx, cancel := context.WithTimeout(r.Context(), s.config.CheckTimeout)
	defer cancel()

	checkResults := s.runChecks(ctx, checks)

	// Determine overall status
	overallStatus := types.NewHealthyStatus("")
	for _, result := range checkResults {
		if !result.IsHealthy() {
			overallStatus = types.NewUnhealthyStatus("liveness check failed", nil)
			break
		}
	}

	s.writeResponse(w, overallStatus, nil)
}

// handleReadyz handles the /readyz (readiness) endpoint.
// Returns 200 OK if all readiness checks pass.
// Returns 503 Service Unavailable if any readiness check fails.
// All checks are run concurrently for optimal performance.
// Panics in check functions are recovered and treated as unhealthy.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	checks := make(map[string]CheckFunc, len(s.readinessChecks))
	for name, check := range s.readinessChecks {
		checks[name] = check
	}
	s.mu.RUnlock()

	// If no checks registered, default to healthy
	if len(checks) == 0 {
		s.writeResponse(w, types.NewHealthyStatus(""), nil)
		return
	}

	// Run all checks concurrently with timeout
	ctx, cancel := context.WithTimeout(r.Context(), s.config.CheckTimeout)
	defer cancel()

	checkResults := s.runChecks(ctx, checks)

	// Determine overall status
	overallStatus := s.aggregateStatus(checkResults)

	s.writeResponse(w, overallStatus, checkResults)
}

// runChecks executes all check functions concurrently and returns their results.
// Each check runs in its own goroutine with panic recovery.
// Checks that timeout or panic are marked as unhealthy.
// errgroup.WithContext is used so that cancellation of the parent ctx propagates
// into individual checks via the group context (gctx). The mutex still protects
// concurrent map writes, which is orthogonal to cancellation coordination.
func (s *Server) runChecks(ctx context.Context, checks map[string]CheckFunc) map[string]types.HealthStatus {
	results := make(map[string]types.HealthStatus, len(checks))
	var mu sync.Mutex

	eg, gctx := errgroup.WithContext(ctx)

	for name, checkFunc := range checks {
		name, checkFunc := name, checkFunc
		eg.Go(func() error {
			// Recover from panics in check functions
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					results[name] = types.NewUnhealthyStatus(
						"check panicked",
						map[string]any{
							"panic": fmt.Sprintf("%v", r),
						},
					)
					mu.Unlock()
				}
			}()

			// Run check with the group context so cancellation propagates.
			status := checkFunc(gctx)

			mu.Lock()
			results[name] = status
			mu.Unlock()

			// Health checks return Status, not error; errgroup is used purely for
			// cancellation propagation, not error aggregation.
			return nil
		})
	}

	// Ignore error: we never return non-nil from the goroutines above.
	_ = eg.Wait()
	return results
}

// aggregateStatus determines the overall health status from individual check results.
// If any check is unhealthy, the overall status is unhealthy.
// If any check is degraded (but none unhealthy), the overall status is degraded.
// Otherwise, the overall status is healthy.
func (s *Server) aggregateStatus(checkResults map[string]types.HealthStatus) types.HealthStatus {
	unhealthyCount := 0
	degradedCount := 0
	totalCount := len(checkResults)

	for _, result := range checkResults {
		if result.IsUnhealthy() {
			unhealthyCount++
		} else if result.IsDegraded() {
			degradedCount++
		}
	}

	if unhealthyCount > 0 {
		message := fmt.Sprintf("%d of %d checks failed", unhealthyCount, totalCount)
		return types.NewUnhealthyStatus(message, nil)
	}

	if degradedCount > 0 {
		message := fmt.Sprintf("%d of %d checks degraded", degradedCount, totalCount)
		return types.NewDegradedStatus(message, nil)
	}

	return types.NewHealthyStatus("")
}

// writeResponse writes the health check response as JSON with appropriate status code.
// Status code is 200 for healthy, 503 for unhealthy or degraded.
func (s *Server) writeResponse(w http.ResponseWriter, status types.HealthStatus, checkResults map[string]types.HealthStatus) {
	// Build response
	resp := Response{
		Status:    status.Status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Message:   status.Message,
	}

	// Include individual check results if provided
	if len(checkResults) > 0 {
		resp.Checks = make(map[string]CheckResult)
		for name, result := range checkResults {
			resp.Checks[name] = CheckResult{
				Status:  result.Status,
				Message: result.Message,
				Details: result.Details,
			}
		}
	}

	// Determine HTTP status code
	statusCode := http.StatusOK
	if !status.IsHealthy() {
		statusCode = http.StatusServiceUnavailable
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If JSON encoding fails, we've already written the status code
		// so we can't change it. Just log the error.
		fmt.Printf("failed to encode health response: %v\n", err)
	}
}
