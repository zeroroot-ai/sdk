// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"testing"
	"time"
)

func TestProtoMetricsCollector_RecordResolution(t *testing.T) {
	collector := NewProtoMetricsCollector()

	// Record some successful global type resolutions
	collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, 1.5)
	collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, 2.0)
	collector.RecordResolution(StrategyGlobalTypes, StatusError, 0.5)

	// Record FDS resolutions
	collector.RecordResolution(StrategyFileDescriptorSet, StatusSuccess, 5.0)
	collector.RecordResolution(StrategyFileDescriptorSet, StatusError, 3.0)

	// Record cached FDS resolutions
	collector.RecordResolution(StrategyFileDescriptorCached, StatusSuccess, 0.8)

	stats := collector.Stats()

	// Verify counters
	if stats.GlobalTypesSuccess != 2 {
		t.Errorf("Expected 2 global types success, got %d", stats.GlobalTypesSuccess)
	}
	if stats.GlobalTypesError != 1 {
		t.Errorf("Expected 1 global types error, got %d", stats.GlobalTypesError)
	}
	if stats.FDSSuccess != 1 {
		t.Errorf("Expected 1 FDS success, got %d", stats.FDSSuccess)
	}
	if stats.FDSError != 1 {
		t.Errorf("Expected 1 FDS error, got %d", stats.FDSError)
	}
	if stats.FDSCachedSuccess != 1 {
		t.Errorf("Expected 1 FDS cached success, got %d", stats.FDSCachedSuccess)
	}

	// Verify duration stats exist
	if stats.AvgDurationMs <= 0 {
		t.Errorf("Expected positive average duration, got %f", stats.AvgDurationMs)
	}
	if stats.MinDurationMs <= 0 {
		t.Errorf("Expected positive min duration, got %f", stats.MinDurationMs)
	}
	if stats.MaxDurationMs <= 0 {
		t.Errorf("Expected positive max duration, got %f", stats.MaxDurationMs)
	}
}

func TestProtoMetricsCollector_CacheMetrics(t *testing.T) {
	collector := NewProtoMetricsCollector()

	// Record cache operations
	collector.RecordCacheHit()
	collector.RecordCacheHit()
	collector.RecordCacheHit()
	collector.RecordCacheMiss()
	collector.RecordCacheMiss()
	collector.RecordCacheEviction()

	stats := collector.Stats()

	if stats.CacheHits != 3 {
		t.Errorf("Expected 3 cache hits, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 2 {
		t.Errorf("Expected 2 cache misses, got %d", stats.CacheMisses)
	}
	if stats.CacheEvictions != 1 {
		t.Errorf("Expected 1 cache eviction, got %d", stats.CacheEvictions)
	}
}

func TestProtoMetricsCollector_DurationStats(t *testing.T) {
	collector := NewProtoMetricsCollector()

	// Record known durations for verification
	durations := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	for _, d := range durations {
		collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, d)
	}

	stats := collector.Stats()

	// Average should be 5.5
	expectedAvg := 5.5
	if stats.AvgDurationMs < expectedAvg-0.1 || stats.AvgDurationMs > expectedAvg+0.1 {
		t.Errorf("Expected average ~%f, got %f", expectedAvg, stats.AvgDurationMs)
	}

	// Min should be 1.0
	if stats.MinDurationMs < 0.9 || stats.MinDurationMs > 1.1 {
		t.Errorf("Expected min ~1.0, got %f", stats.MinDurationMs)
	}

	// Max should be 10.0
	if stats.MaxDurationMs < 9.9 || stats.MaxDurationMs > 10.1 {
		t.Errorf("Expected max ~10.0, got %f", stats.MaxDurationMs)
	}

	// P50 should be around 5.0-6.0
	if stats.P50DurationMs < 4.5 || stats.P50DurationMs > 6.5 {
		t.Errorf("Expected P50 ~5.0-6.0, got %f", stats.P50DurationMs)
	}

	// P95 should be around 9.5-10.0
	if stats.P95DurationMs < 9.0 || stats.P95DurationMs > 10.5 {
		t.Errorf("Expected P95 ~9.5-10.0, got %f", stats.P95DurationMs)
	}
}

func TestProtoMetricsCollector_Concurrent(t *testing.T) {
	collector := NewProtoMetricsCollector()

	// Test concurrent access
	done := make(chan bool)
	for range 10 {
		go func() {
			for range 100 {
				collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, 1.0)
				collector.RecordCacheHit()
				collector.RecordCacheMiss()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	stats := collector.Stats()

	// Should have 1000 total success resolutions
	if stats.GlobalTypesSuccess != 1000 {
		t.Errorf("Expected 1000 global types success, got %d", stats.GlobalTypesSuccess)
	}

	// Should have 1000 cache hits and 1000 cache misses
	if stats.CacheHits != 1000 {
		t.Errorf("Expected 1000 cache hits, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 1000 {
		t.Errorf("Expected 1000 cache misses, got %d", stats.CacheMisses)
	}
}

func TestProtoMetricsCollector_DurationBounding(t *testing.T) {
	collector := NewProtoMetricsCollector()

	// Record more than maxDurations entries
	for i := range 1500 {
		collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, float64(i))
	}

	stats := collector.Stats()

	// Should only keep last 1000 entries (maxDurations)
	// The collector keeps the latest entries, so we can verify stats are calculated
	if stats.AvgDurationMs <= 0 {
		t.Errorf("Expected positive average, got %f", stats.AvgDurationMs)
	}
	if stats.MaxDurationMs <= 0 {
		t.Errorf("Expected positive max, got %f", stats.MaxDurationMs)
	}
}

func TestNoOpMetricsRecorder(t *testing.T) {
	recorder := NewNoOpMetricsRecorder()

	// Should not panic
	recorder.RecordCounter("test", 1, map[string]string{"label": "value"})
	recorder.RecordGauge("test", 1.0, nil)
	recorder.RecordHistogram("test", 1.0, nil)
}

func TestCalculatePercentile(t *testing.T) {
	durations := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}

	// Test P50 (median)
	p50 := calculatePercentile(durations, 0.5)
	if p50 < 2.5 || p50 > 3.5 {
		t.Errorf("Expected P50 ~3.0, got %f", p50)
	}

	// Test P0 (min)
	p0 := calculatePercentile(durations, 0.0)
	if p0 < 0.5 || p0 > 1.5 {
		t.Errorf("Expected P0 ~1.0, got %f", p0)
	}

	// Test P100 (max)
	p100 := calculatePercentile(durations, 1.0)
	if p100 < 4.5 || p100 > 5.5 {
		t.Errorf("Expected P100 ~5.0, got %f", p100)
	}

	// Test empty slice
	emptyP50 := calculatePercentile([]time.Duration{}, 0.5)
	if emptyP50 != 0 {
		t.Errorf("Expected 0 for empty slice, got %f", emptyP50)
	}
}

func TestCalculateAverage(t *testing.T) {
	durations := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
	}

	avg := calculateAverage(durations)
	expected := 2.0
	if avg < expected-0.1 || avg > expected+0.1 {
		t.Errorf("Expected average ~%f, got %f", expected, avg)
	}

	// Test empty slice
	emptyAvg := calculateAverage([]time.Duration{})
	if emptyAvg != 0 {
		t.Errorf("Expected 0 for empty slice, got %f", emptyAvg)
	}
}

func TestCalculateMinMax(t *testing.T) {
	durations := []time.Duration{
		5 * time.Millisecond,
		1 * time.Millisecond,
		10 * time.Millisecond,
		3 * time.Millisecond,
	}

	min := calculateMin(durations)
	if min < 0.5 || min > 1.5 {
		t.Errorf("Expected min ~1.0, got %f", min)
	}

	max := calculateMax(durations)
	if max < 9.5 || max > 10.5 {
		t.Errorf("Expected max ~10.0, got %f", max)
	}

	// Test empty slice
	emptyMin := calculateMin([]time.Duration{})
	if emptyMin != 0 {
		t.Errorf("Expected 0 for empty slice, got %f", emptyMin)
	}

	emptyMax := calculateMax([]time.Duration{})
	if emptyMax != 0 {
		t.Errorf("Expected 0 for empty slice, got %f", emptyMax)
	}
}
