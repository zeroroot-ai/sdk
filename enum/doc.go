// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package enum provides a global registry for tool enum value normalization.
//
// This package enables tools to register mappings from shorthand or user-friendly
// enum values to their corresponding protobuf enum names. For example, mapping
// "syn" to "SYN_SCAN" for an mytool-a scan type field.
//
// # Usage
//
// Register enum mappings for a tool:
//
//	enum.Register("mytool-a", "scan_type", map[string]string{
//	    "syn": "SYN_SCAN",
//	    "udp": "UDP_SCAN",
//	})
//
// Or register multiple fields at once:
//
//	enum.RegisterBatch("mytool-a", map[string]map[string]string{
//	    "scan_type": {
//	        "syn": "SYN_SCAN",
//	        "udp": "UDP_SCAN",
//	    },
//	    "timing": {
//	        "fast": "TIMING_FAST",
//	        "slow": "TIMING_SLOW",
//	    },
//	})
//
// Normalize JSON input before passing to a tool:
//
//	input := `{"scan_type": "syn", "target": "example.com"}`
//	normalized := enum.Normalize("mytool-a", input)
//	// Result: {"scan_type": "SYN_SCAN", "target": "example.com"}
//
// # Thread Safety
//
// All operations are thread-safe and can be called concurrently from multiple
// goroutines. The registry uses sync.RWMutex for efficient concurrent access.
//
// # Case Insensitivity
//
// Input values are matched case-insensitively, so "SYN", "syn", and "Syn" will
// all match the same registered mapping. The output always uses the exact proto
// enum name as registered.
//
// # Error Handling
//
// The Normalize function is designed to be fail-safe. If any error occurs during
// parsing or normalization (invalid JSON, type mismatches, etc.), it returns the
// original input unchanged rather than returning an error. This ensures that
// invalid input doesn't break tool execution.
package enum
