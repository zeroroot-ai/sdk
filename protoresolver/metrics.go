// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package protoresolver provides observability metrics for proto type resolution.
//
// This file implements metrics collection for monitoring ProtoResolver performance
// and behavior in production. It tracks resolution strategies, cache performance,
// and resolution durations to enable monitoring and alerting.
package protoresolver

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metric name constants for proto resolution observability.
// These constants provide a centralized definition of all metric names
// to ensure consistency across the codebase and prevent typos.
const (

	// Strategy labels
	StrategyGlobalTypes          = "global_types"
	StrategyFileDescriptorSet    = "file_descriptor_set"
	StrategyFileDescriptorCached = "file_descriptor_cached"

	// Status labels
	StatusSuccess = "success"
	StatusError   = "error"
)

// ProtoMetricsCollector implements metrics collection for ProtoResolver operations.
// It uses atomic operations for thread-safe concurrent access without locks.
//
// The collector tracks:
// - Resolution attempts by strategy (global_types, file_descriptor_set)
// - Success/error counts
// - Cache hits, misses, and evictions
// - Resolution duration histograms
//
// Example usage:
//
//	collector := NewProtoMetricsCollector()
//	collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, 1.5)
//	stats := collector.Stats()
type ProtoMetricsCollector struct {
	// Resolution counters by strategy
	globalTypesSuccess atomic.Int64
	globalTypesError   atomic.Int64
	fdsSuccess         atomic.Int64
	fdsError           atomic.Int64
	fdsCachedSuccess   atomic.Int64
	fdsCachedError     atomic.Int64

	// Cache performance counters
	cacheHits      atomic.Int64
	cacheMisses    atomic.Int64
	cacheEvictions atomic.Int64

	// Duration tracking (for histogram simulation)
	mu                  sync.RWMutex
	resolutionDurations []time.Duration
	maxDurations        int // Maximum number of durations to keep
}

// NewProtoMetricsCollector creates a new metrics collector for ProtoResolver.
// The collector is safe for concurrent use.
func NewProtoMetricsCollector() *ProtoMetricsCollector {
	return &ProtoMetricsCollector{
		resolutionDurations: make([]time.Duration, 0, 1000),
		maxDurations:        1000, // Keep last 1000 samples for percentile calculation
	}
}

// RecordResolution records a proto type resolution attempt.
// This tracks which strategy was used and whether it succeeded.
//
// Parameters:
//   - strategy: The resolution strategy used (StrategyGlobalTypes, StrategyFileDescriptorSet, etc.)
//   - status: The resolution status (StatusSuccess or StatusError)
//   - durationMs: The resolution duration in milliseconds
//
// Example:
//
//	collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, 1.5)
func (c *ProtoMetricsCollector) RecordResolution(strategy, status string, durationMs float64) {
	// Record counter by strategy and status
	switch strategy {
	case StrategyGlobalTypes:
		if status == StatusSuccess {
			c.globalTypesSuccess.Add(1)
		} else {
			c.globalTypesError.Add(1)
		}
	case StrategyFileDescriptorSet:
		if status == StatusSuccess {
			c.fdsSuccess.Add(1)
		} else {
			c.fdsError.Add(1)
		}
	case StrategyFileDescriptorCached:
		if status == StatusSuccess {
			c.fdsCachedSuccess.Add(1)
		} else {
			c.fdsCachedError.Add(1)
		}
	}

	// Record duration
	if durationMs >= 0 {
		duration := time.Duration(durationMs * float64(time.Millisecond))
		c.recordDuration(duration)
	}
}

// RecordCacheHit records a successful cache lookup.
func (c *ProtoMetricsCollector) RecordCacheHit() {
	c.cacheHits.Add(1)
}

// RecordCacheMiss records a cache miss.
func (c *ProtoMetricsCollector) RecordCacheMiss() {
	c.cacheMisses.Add(1)
}

// RecordCacheEviction records a cache entry eviction.
func (c *ProtoMetricsCollector) RecordCacheEviction() {
	c.cacheEvictions.Add(1)
}

// recordDuration stores a resolution duration for histogram calculation.
// Uses a circular buffer approach to keep memory bounded.
func (c *ProtoMetricsCollector) recordDuration(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.resolutionDurations) >= c.maxDurations {
		// Shift left to make room (simple FIFO)
		c.resolutionDurations = c.resolutionDurations[1:]
	}
	c.resolutionDurations = append(c.resolutionDurations, duration)
}

// Ensure NoOpMetricsRecorder implements MetricsRecorder at compile time
