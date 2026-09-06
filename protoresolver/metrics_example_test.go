// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver_test

import (
	"context"
	"fmt"

	"github.com/zeroroot-ai/sdk/protoresolver"
)

// ExampleDefaultProtoResolver_Metrics demonstrates how to access and use
// metrics from the ProtoResolver for monitoring and observability.
func ExampleDefaultProtoResolver_Metrics() {
	// Create a resolver with default config
	config := protoresolver.DefaultConfig()
	resolver := protoresolver.NewDefaultProtoResolver(config)

	// Simulate some resolution attempts
	ctx := context.Background()
	metadata := map[string]string{
		"tool_name": "example-tool",
		// Note: In real usage, you'd include file_descriptor_set here
	}

	// Try to resolve a type (this will fail without valid metadata, but still records metrics)
	_, _ = resolver.ResolveInputType(ctx, "example.Type", metadata)
	_, _ = resolver.ResolveInputType(ctx, "example.Type", metadata)
	_, _ = resolver.ResolveInputType(ctx, "example.Type", metadata)

	// Get metrics snapshot
	stats := resolver.Metrics()

	// Calculate cache hit rate
	totalCacheAccess := stats.CacheHits + stats.CacheMisses
	var hitRate float64
	if totalCacheAccess > 0 {
		hitRate = float64(stats.CacheHits) / float64(totalCacheAccess) * 100
	}

	fmt.Printf("Cache Hit Rate: %.1f%%\n", hitRate)
	fmt.Printf("Total Resolutions: %d\n",
		stats.GlobalTypesSuccess+stats.GlobalTypesError+
			stats.FDSSuccess+stats.FDSError+
			stats.FDSCachedSuccess+stats.FDSCachedError)

	// In production, you would export these metrics to your observability platform:
	// - Prometheus: Use OpenTelemetry metrics exporter
	// - DataDog: Use DogStatsD client
	// - CloudWatch: Use AWS SDK metrics
}

// ExampleProtoMetricsCollector_Stats shows how to interpret metrics statistics.
func ExampleProtoMetricsCollector_Stats() {
	collector := protoresolver.NewProtoMetricsCollector()

	// Simulate various resolution patterns
	// Fast global type resolutions
	for range 100 {
		collector.RecordResolution(
			protoresolver.StrategyGlobalTypes,
			protoresolver.StatusSuccess,
			0.5, // 0.5ms - very fast
		)
	}

	// Slower FileDescriptorSet resolutions
	for range 20 {
		collector.RecordResolution(
			protoresolver.StrategyFileDescriptorSet,
			protoresolver.StatusSuccess,
			5.0, // 5ms - slower due to parsing
		)
	}

	// Some cached resolutions
	for range 50 {
		collector.RecordResolution(
			protoresolver.StrategyFileDescriptorCached,
			protoresolver.StatusSuccess,
			0.8, // 0.8ms - fast due to cache
		)
		collector.RecordCacheHit()
	}

	stats := collector.Stats()

	fmt.Printf("Resolution Strategy Distribution:\n")
	fmt.Printf("  Global Types: %d\n", stats.GlobalTypesSuccess)
	fmt.Printf("  FDS (uncached): %d\n", stats.FDSSuccess)
	fmt.Printf("  FDS (cached): %d\n", stats.FDSCachedSuccess)
	fmt.Printf("\nCache Performance:\n")
	fmt.Printf("  Hits: %d\n", stats.CacheHits)
	fmt.Printf("  Misses: %d\n", stats.CacheMisses)
	fmt.Printf("\nLatency statistics are available including:\n")
	fmt.Printf("  Average, P50, P95, P99 percentiles\n")

	// Output:
	// Resolution Strategy Distribution:
	//   Global Types: 100
	//   FDS (uncached): 20
	//   FDS (cached): 50
	//
	// Cache Performance:
	//   Hits: 50
	//   Misses: 0
	//
	// Latency statistics are available including:
	//   Average, P50, P95, P99 percentiles
}

// Example showing how to integrate metrics with OpenTelemetry (conceptual)
func ExampleMetricsRecorder_integration() {
	// In a real application, you would create an OpenTelemetry metrics recorder
	// and pass it to components that need to record metrics.
	//
	// Example OpenTelemetry integration:
	//
	// import (
	//     "go.opentelemetry.io/otel/metric"
	//     sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	// )
	//
	// // Initialize OpenTelemetry metrics provider
	// provider := sdkmetric.NewMeterProvider(...)
	// meter := provider.Meter("gibson.protoresolver")
	//
	// // Create metrics instruments
	// resolutionCounter, _ := meter.Int64Counter(
	//     protoresolver.MetricProtoResolutionTotal,
	//     metric.WithDescription("Total proto type resolutions"),
	// )
	//
	// // Record metrics
	// resolutionCounter.Add(ctx, 1, metric.WithAttributes(
	//     attribute.String("strategy", protoresolver.StrategyGlobalTypes),
	//     attribute.String("status", protoresolver.StatusSuccess),
	// ))
	//
	// This would integrate with your existing observability stack
	// (Prometheus, Jaeger, Grafana, etc.)

	fmt.Println("See code comments for OpenTelemetry integration example")
	// Output: See code comments for OpenTelemetry integration example
}
