// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package health provides reusable health check functions for Gibson tools.
// It offers standardized ways to verify dependencies, connectivity, and system state.
package health

import (
	"fmt"
	"os/exec"

	"github.com/zeroroot-ai/sdk/types"
)

// BinaryCheck verifies that a binary exists and is executable in the system PATH.
// It returns a healthy status if the binary is found, unhealthy otherwise.
//
// Example:
//
//	status := health.BinaryCheck("mytool-a")
//	if status.IsUnhealthy() {
//	    log.Fatal("mytool-a is required but not installed")
//	}
func BinaryCheck(name string) types.HealthStatus {
	if name == "" {
		return types.NewUnhealthyStatus("binary name cannot be empty", nil)
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return types.NewUnhealthyStatus(
			fmt.Sprintf("binary '%s' not found in PATH", name),
			map[string]any{
				"binary": name,
				"error":  err.Error(),
			},
		)
	}

	return types.NewHealthyStatus(
		fmt.Sprintf("binary '%s' found at %s", name, path),
	)
}
