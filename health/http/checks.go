// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package http

import (
	"context"

	"github.com/zeroroot-ai/sdk/types"
)

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
