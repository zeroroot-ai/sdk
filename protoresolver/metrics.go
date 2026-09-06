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
	// Resolution strategy metrics
	MetricProtoResolutionTotal    = "gibson.proto.resolution.total"
	MetricProtoResolutionDuration = "gibson.proto.resolution.duration_ms"

	// Cache performance metrics
	MetricProtoCacheHits      = "gibson.proto.cache.hits"
	MetricProtoCacheMisses    = "gibson.proto.cache.misses"
	MetricProtoCacheEvictions = "gibson.proto.cache.evictions"

	// Strategy labels
	StrategyGlobalTypes          = "global_types"
	StrategyFileDescriptorSet    = "file_descriptor_set"
	StrategyFileDescriptorCached = "file_descriptor_cached"

	// Status labels
	StatusSuccess = "success"
	StatusError   = "error"
)

// MetricsRecorder defines the interface for recording proto resolution metrics.
// Implementations should be thread-safe for concurrent use.
//
// This interface allows decoupling from specific metrics implementations
// (e.g., Prometheus, OpenTelemetry, in-memory counters) for testing and flexibility.
type MetricsRecorder interface {
	// RecordCounter increments a counter metric by the given value.
	// Counters are cumulative metrics that only increase.
	RecordCounter(name string, value int64, labels map[string]string)

	// RecordGauge sets a gauge metric to the given value.
	// Gauges represent point-in-time measurements that can go up or down.
	RecordGauge(name string, value float64, labels map[string]string)

	// RecordHistogram records a value in a histogram metric.
	// Histograms track distributions of values over time.
	RecordHistogram(name string, value float64, labels map[string]string)
}

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

// Stats returns current metrics statistics.
// This provides a snapshot of all collected metrics.
type ProtoMetricsStats struct {
	// Resolution counts by strategy and status
	GlobalTypesSuccess int64
	GlobalTypesError   int64
	FDSSuccess         int64
	FDSError           int64
	FDSCachedSuccess   int64
	FDSCachedError     int64

	// Cache performance
	CacheHits      int64
	CacheMisses    int64
	CacheEvictions int64

	// Duration statistics
	AvgDurationMs float64
	MinDurationMs float64
	MaxDurationMs float64
	P50DurationMs float64
	P95DurationMs float64
	P99DurationMs float64
}

// Stats returns a snapshot of all collected metrics.
// This method is thread-safe and can be called concurrently with RecordResolution.
//
// Example:
//
//	stats := collector.Stats()
//	fmt.Printf("Cache hit rate: %.2f%%\n",
//	    float64(stats.CacheHits) / float64(stats.CacheHits + stats.CacheMisses) * 100)
func (c *ProtoMetricsCollector) Stats() ProtoMetricsStats {
	stats := ProtoMetricsStats{
		GlobalTypesSuccess: c.globalTypesSuccess.Load(),
		GlobalTypesError:   c.globalTypesError.Load(),
		FDSSuccess:         c.fdsSuccess.Load(),
		FDSError:           c.fdsError.Load(),
		FDSCachedSuccess:   c.fdsCachedSuccess.Load(),
		FDSCachedError:     c.fdsCachedError.Load(),
		CacheHits:          c.cacheHits.Load(),
		CacheMisses:        c.cacheMisses.Load(),
		CacheEvictions:     c.cacheEvictions.Load(),
	}

	// Calculate duration statistics
	c.mu.RLock()
	durations := make([]time.Duration, len(c.resolutionDurations))
	copy(durations, c.resolutionDurations)
	c.mu.RUnlock()

	if len(durations) > 0 {
		stats.AvgDurationMs = calculateAverage(durations)
		stats.MinDurationMs = calculateMin(durations)
		stats.MaxDurationMs = calculateMax(durations)
		stats.P50DurationMs = calculatePercentile(durations, 0.50)
		stats.P95DurationMs = calculatePercentile(durations, 0.95)
		stats.P99DurationMs = calculatePercentile(durations, 0.99)
	}

	return stats
}

// calculateAverage computes the average duration in milliseconds.
func calculateAverage(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}

	return float64(sum.Microseconds()) / float64(len(durations)) / 1000.0
}

// calculateMin finds the minimum duration in milliseconds.
func calculateMin(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}

	return float64(min.Microseconds()) / 1000.0
}

// calculateMax finds the maximum duration in milliseconds.
func calculateMax(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}

	return float64(max.Microseconds()) / 1000.0
}

// calculatePercentile computes the specified percentile.
// Uses linear interpolation for percentiles between samples.
func calculatePercentile(durations []time.Duration, percentile float64) float64 {
	if len(durations) == 0 {
		return 0
	}

	// Create a sorted copy
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	// Simple insertion sort for small datasets
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	// Calculate percentile index
	index := percentile * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return float64(sorted[len(sorted)-1].Microseconds()) / 1000.0
	}

	// Linear interpolation between samples
	weight := index - float64(lower)
	lowerVal := float64(sorted[lower].Microseconds()) / 1000.0
	upperVal := float64(sorted[upper].Microseconds()) / 1000.0

	return lowerVal + weight*(upperVal-lowerVal)
}

// NoOpMetricsRecorder is a no-operation implementation of MetricsRecorder.
// It discards all metrics and is useful for testing or when metrics are disabled.
type NoOpMetricsRecorder struct{}

// NewNoOpMetricsRecorder creates a new no-op metrics recorder.
func NewNoOpMetricsRecorder() *NoOpMetricsRecorder {
	return &NoOpMetricsRecorder{}
}

// RecordCounter is a no-op implementation.
func (n *NoOpMetricsRecorder) RecordCounter(name string, value int64, labels map[string]string) {
	// No-op: metrics are discarded
}

// RecordGauge is a no-op implementation.
func (n *NoOpMetricsRecorder) RecordGauge(name string, value float64, labels map[string]string) {
	// No-op: metrics are discarded
}

// RecordHistogram is a no-op implementation.
func (n *NoOpMetricsRecorder) RecordHistogram(name string, value float64, labels map[string]string) {
	// No-op: metrics are discarded
}

// Ensure NoOpMetricsRecorder implements MetricsRecorder at compile time
var _ MetricsRecorder = (*NoOpMetricsRecorder)(nil)
