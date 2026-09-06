// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithHealthEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	opt := WithHealthEndpoint("/healthz")
	opt(cfg)

	assert.Equal(t, "/healthz", cfg.HealthEndpoint)
}

func TestWithGracefulShutdown(t *testing.T) {
	cfg := DefaultConfig()
	opt := WithGracefulShutdown(60 * time.Second)
	opt(cfg)

	assert.Equal(t, 60*time.Second, cfg.GracefulTimeout)
}

func TestMultipleOptions(t *testing.T) {
	cfg := DefaultConfig()

	opts := []Option{
		WithHealthEndpoint("/ready"),
		WithGracefulShutdown(45 * time.Second),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	assert.Equal(t, "/ready", cfg.HealthEndpoint)
	assert.Equal(t, 45*time.Second, cfg.GracefulTimeout)
}
