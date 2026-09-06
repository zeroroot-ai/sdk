// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package http

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/zeroroot-ai/sdk/health"
	"github.com/zeroroot-ai/sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// RedisPinger defines the minimal interface required for Redis health checks.
// This interface uses a function-based approach to avoid type mismatches
// with specific Redis client implementations.
type RedisPinger interface {
	// Ping sends a PING command and returns the result and any error.
	Ping(ctx context.Context) (string, error)
}

// Neo4jDriver defines the minimal interface required for Neo4j health checks.
// This interface allows health checks to work with any Neo4j driver implementation
// without importing specific Neo4j libraries.
type Neo4jDriver interface {
	// VerifyConnectivity verifies that the driver can connect to the database.
	// It should respect the context deadline for timeout handling.
	VerifyConnectivity(ctx context.Context) error
}

// RedisCheck creates a CheckFunc that verifies Redis connectivity via PING.
// It returns healthy if a PONG response is received, unhealthy otherwise.
//
// Example using a wrapper:
//
//	type redisPingerWrapper struct { client redis.UniversalClient }
//	func (w *redisPingerWrapper) Ping(ctx context.Context) (string, error) {
//	    return w.client.Ping(ctx).Result()
//	}
//	server.RegisterReadinessCheck("redis", RedisCheck(&redisPingerWrapper{client}))
func RedisCheck(pinger RedisPinger) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if pinger == nil {
			return types.NewUnhealthyStatus("redis pinger is nil", nil)
		}

		result, err := pinger.Ping(ctx)
		if err != nil {
			return types.NewUnhealthyStatus(
				"redis ping failed",
				map[string]any{
					"error": err.Error(),
				},
			)
		}

		if result != "PONG" {
			return types.NewUnhealthyStatus(
				"unexpected redis ping response",
				map[string]any{
					"expected": "PONG",
					"received": result,
				},
			)
		}

		return types.NewHealthyStatus("redis is healthy (PONG received)")
	}
}

// RedisPingFunc creates a CheckFunc from a simple ping function.
// This is the easiest way to integrate with any Redis client.
//
// Example:
//
//	server.RegisterReadinessCheck("redis", RedisPingFunc(func(ctx context.Context) (string, error) {
//	    return redisClient.Ping(ctx).Result()
//	}))
func RedisPingFunc(ping func(ctx context.Context) (string, error)) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if ping == nil {
			return types.NewUnhealthyStatus("redis ping function is nil", nil)
		}

		result, err := ping(ctx)
		if err != nil {
			return types.NewUnhealthyStatus(
				"redis ping failed",
				map[string]any{
					"error": err.Error(),
				},
			)
		}

		if result != "PONG" {
			return types.NewUnhealthyStatus(
				"unexpected redis ping response",
				map[string]any{
					"expected": "PONG",
					"received": result,
				},
			)
		}

		return types.NewHealthyStatus("redis is healthy (PONG received)")
	}
}

// Neo4jCheck creates a CheckFunc that verifies Neo4j connectivity.
// It returns healthy if the driver can connect, unhealthy otherwise.
//
// Example:
//
//	driver, _ := neo4j.NewDriverWithContext("bolt://localhost:7687", neo4j.BasicAuth("user", "pass", ""))
//	server.RegisterReadinessCheck("neo4j", Neo4jCheck(driver))
func Neo4jCheck(driver Neo4jDriver) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if driver == nil {
			return types.NewUnhealthyStatus("neo4j driver is nil", nil)
		}

		if err := driver.VerifyConnectivity(ctx); err != nil {
			return types.NewUnhealthyStatus(
				"neo4j connection verification failed",
				map[string]any{
					"error": err.Error(),
				},
			)
		}

		return types.NewHealthyStatus("neo4j connection verified")
	}
}

// Neo4jConnectivityFunc creates a CheckFunc from a simple connectivity function.
// This is the easiest way to integrate with any Neo4j driver.
//
// Example:
//
//	server.RegisterReadinessCheck("neo4j", Neo4jConnectivityFunc(func(ctx context.Context) error {
//	    return neo4jDriver.VerifyConnectivity(ctx)
//	}))
func Neo4jConnectivityFunc(verify func(ctx context.Context) error) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if verify == nil {
			return types.NewUnhealthyStatus("neo4j verify function is nil", nil)
		}

		if err := verify(ctx); err != nil {
			return types.NewUnhealthyStatus(
				"neo4j connection verification failed",
				map[string]any{
					"error": err.Error(),
				},
			)
		}

		return types.NewHealthyStatus("neo4j connection verified")
	}
}

// GRPCHealthCheck creates a CheckFunc that verifies gRPC service health.
// It uses the standard gRPC health checking protocol to determine service status.
// It returns healthy if the service reports SERVING, unhealthy otherwise.
//
// Example:
//
//	conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
//	healthClient := grpc_health_v1.NewHealthClient(conn)
//	server.RegisterReadinessCheck("grpc-service", GRPCHealthCheck(healthClient))
func GRPCHealthCheck(client grpc_health_v1.HealthClient) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if client == nil {
			return types.NewUnhealthyStatus("grpc health client is nil", nil)
		}

		req := &grpc_health_v1.HealthCheckRequest{}
		resp, err := client.Check(ctx, req)
		if err != nil {
			return types.NewUnhealthyStatus(
				"grpc health check failed",
				map[string]any{
					"error": err.Error(),
				},
			)
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return types.NewUnhealthyStatus(
				"grpc service not serving",
				map[string]any{
					"status": resp.Status.String(),
				},
			)
		}

		return types.NewHealthyStatus("grpc service is serving")
	}
}

// GRPCConnCheck creates a CheckFunc that verifies gRPC connectivity using a connection.
// It creates a health client from the connection and performs a health check.
//
// Example:
//
//	conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
//	server.RegisterReadinessCheck("grpc-service", GRPCConnCheck(conn))
func GRPCConnCheck(conn *grpc.ClientConn) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if conn == nil {
			return types.NewUnhealthyStatus("grpc connection is nil", nil)
		}

		client := grpc_health_v1.NewHealthClient(conn)
		req := &grpc_health_v1.HealthCheckRequest{}
		resp, err := client.Check(ctx, req)
		if err != nil {
			return types.NewUnhealthyStatus(
				"grpc health check failed",
				map[string]any{
					"error": err.Error(),
				},
			)
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return types.NewUnhealthyStatus(
				"grpc service not serving",
				map[string]any{
					"status": resp.Status.String(),
				},
			)
		}

		return types.NewHealthyStatus("grpc service is serving")
	}
}

// BinaryCheck creates a CheckFunc that wraps the existing health.BinaryCheck.
// It verifies that a binary exists and is executable in the system PATH.
// This is useful for ensuring tool dependencies are available before starting work.
//
// Example:
//
//	server.RegisterReadinessCheck("mytool-a", BinaryCheck("mytool-a"))
//	server.RegisterReadinessCheck("mytool-b", BinaryCheck("mytool-b"))
func BinaryCheck(name string) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		// The existing BinaryCheck doesn't take a context, but it's fast enough
		// that timeout handling isn't critical. We call it directly.
		return health.BinaryCheck(name)
	}
}

// TCPCheck creates a CheckFunc that verifies TCP connectivity to an address.
// It attempts to establish a TCP connection within the specified timeout.
// It returns healthy if the connection succeeds, unhealthy otherwise.
//
// Example:
//
//	server.RegisterReadinessCheck("database", TCPCheck("postgres:5432", 2*time.Second))
//	server.RegisterReadinessCheck("cache", TCPCheck("redis:6379", 1*time.Second))
func TCPCheck(addr string, timeout time.Duration) CheckFunc {
	return func(ctx context.Context) types.HealthStatus {
		if addr == "" {
			return types.NewUnhealthyStatus("tcp address cannot be empty", nil)
		}

		// Create a context with the specified timeout
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Use DialContext to respect context cancellation
		var dialer net.Dialer
		conn, err := dialer.DialContext(checkCtx, "tcp", addr)
		if err != nil {
			return types.NewUnhealthyStatus(
				fmt.Sprintf("tcp connection to %s failed", addr),
				map[string]any{
					"address": addr,
					"timeout": timeout.String(),
					"error":   err.Error(),
				},
			)
		}

		// Close the connection immediately since we only need to verify connectivity
		conn.Close()

		return types.NewHealthyStatus(
			fmt.Sprintf("tcp connection to %s successful", addr),
		)
	}
}
