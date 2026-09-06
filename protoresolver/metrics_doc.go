// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

// Observability Metrics for ProtoResolver
//
// This file provides documentation for the metrics collection system used
// to monitor ProtoResolver performance in production environments.
//
// # Overview
//
// The metrics system tracks three main categories:
//
// 1. Resolution Strategy Distribution
//   - How often each strategy succeeds/fails
//   - Tracks: global_types, file_descriptor_set, file_descriptor_cached
//
// 2. Cache Performance
//   - Cache hit rate, miss rate, eviction rate
//   - Critical for optimizing memory usage and performance
//
// 3. Resolution Latency
//   - Duration distributions (avg, min, max, percentiles)
//   - Helps identify performance regressions
//
// # Metric Constants
//
// The following metric names are defined for consistency:
//
//	MetricProtoResolutionTotal    - Counter: Total resolutions by strategy/status
//	MetricProtoResolutionDuration - Histogram: Resolution duration in milliseconds
//	MetricProtoCacheHits          - Counter: Successful cache lookups
//	MetricProtoCacheMisses        - Counter: Failed cache lookups
//	MetricProtoCacheEvictions     - Counter: Cache entries evicted
//
// # Strategy Labels
//
//	StrategyGlobalTypes          - Type found in global registry (fastest)
//	StrategyFileDescriptorSet    - Type resolved from FDS (slower, first time)
//	StrategyFileDescriptorCached - Type resolved from cached FDS (fast)
//
// # Status Labels
//
//	StatusSuccess - Resolution succeeded
//	StatusError   - Resolution failed
//
// # Usage Examples
//
// ## Accessing Metrics from DefaultProtoResolver
//
//	resolver := protoresolver.NewDefaultProtoResolver(config)
//	// ... perform some resolutions ...
//	stats := resolver.Metrics()
//	fmt.Printf("Cache hit rate: %.2f%%\n",
//	    float64(stats.CacheHits) / float64(stats.CacheHits + stats.CacheMisses) * 100)
//
// ## Using ProtoMetricsCollector Directly
//
//	collector := protoresolver.NewProtoMetricsCollector()
//	collector.RecordResolution(StrategyGlobalTypes, StatusSuccess, 1.5)
//	collector.RecordCacheHit()
//	stats := collector.Stats()
//
// ## Integration with OpenTelemetry
//
// The MetricsRecorder interface allows integration with observability platforms:
//
//	// Create OpenTelemetry meter
//	meter := provider.Meter("gibson.protoresolver")
//
//	// Create counter
//	counter, _ := meter.Int64Counter(MetricProtoResolutionTotal)
//
//	// Record metric
//	counter.Add(ctx, 1, metric.WithAttributes(
//	    attribute.String("strategy", StrategyGlobalTypes),
//	    attribute.String("status", StatusSuccess),
//	))
//
// # Metrics Dashboard Recommendations
//
// ## Key Performance Indicators (KPIs)
//
// 1. Cache Hit Rate
//   - Formula: CacheHits / (CacheHits + CacheMisses)
//   - Target: >80% for optimal performance
//   - Alert if: <50% (indicates cache size too small or TTL too short)
//
// 2. Resolution Success Rate
//   - Formula: SuccessCount / TotalResolutions
//   - Target: >99%
//   - Alert if: <95% (indicates schema availability issues)
//
// 3. P95 Resolution Latency
//   - Target: <5ms for global types, <20ms for FDS
//   - Alert if: >50ms (indicates performance degradation)
//
// ## Grafana Dashboard Queries (Prometheus)
//
// Cache Hit Rate (Last 5m):
//
//	sum(rate(gibson_proto_cache_hits_total[5m]))
//	/
//	(sum(rate(gibson_proto_cache_hits_total[5m])) + sum(rate(gibson_proto_cache_misses_total[5m])))
//
// Resolution Rate by Strategy:
//
//	sum by(strategy) (rate(gibson_proto_resolution_total[5m]))
//
// P95 Resolution Latency:
//
//	histogram_quantile(0.95, rate(gibson_proto_resolution_duration_ms_bucket[5m]))
//
// # Performance Considerations
//
// The ProtoMetricsCollector uses atomic operations for counters, making it
// suitable for high-concurrency environments. Duration tracking uses a bounded
// circular buffer (default 1000 samples) to prevent unbounded memory growth.
//
// Memory overhead per collector:
// - ~48 bytes for atomic counters (6 counters * 8 bytes)
// - ~8KB for duration buffer (1000 samples * 8 bytes)
// - Total: ~8KB per resolver instance
//
// # Thread Safety
//
// All metrics operations are thread-safe:
// - Counters use atomic.Int64 operations
// - Duration buffer uses sync.RWMutex for concurrent access
// - Safe to call from multiple goroutines simultaneously
//
// # Production Deployment
//
// 1. Enable metrics in your resolver configuration
// 2. Export metrics to your observability platform (Prometheus, DataDog, etc.)
// 3. Create dashboards for monitoring (see examples above)
// 4. Set up alerts for KPI thresholds
// 5. Review metrics weekly to optimize cache configuration
//
// # Troubleshooting Guide
//
// High Cache Miss Rate:
//   - Increase cache size (CacheMaxEntries)
//   - Increase cache TTL (CacheTTL)
//   - Verify tools are using consistent naming
//
// High Resolution Latency:
//   - Check if most resolutions use file_descriptor_set (uncached)
//   - Increase cache hit rate to reduce parsing overhead
//   - Consider registering frequently-used types globally
//
// High Error Rate:
//   - Verify file_descriptor_set metadata is present
//   - Check for schema compatibility issues
//   - Review error logs for specific failure modes
