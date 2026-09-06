// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package health provides reusable health check functions for Gibson tools.
//
// This package offers standardized ways to verify dependencies and system state.
// It is designed to help tools implement consistent health checking patterns.
//
// # Health Check Functions
//
// The package provides the following health check function:
//
//   - BinaryCheck: Verify a binary exists in PATH
//
// # Usage Example
//
//	import (
//	    "github.com/zeroroot-ai/sdk/health"
//	)
//
//	// Check individual dependencies
//	mytoolStatus := health.BinaryCheck("mytool")
//	if mytoolStatus.IsUnhealthy() {
//	    log.Fatal("mytool is required but not installed")
//	}
package health
