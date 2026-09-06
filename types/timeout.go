// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"fmt"
	"time"
)

// TimeoutConfig defines timeout bounds for tool execution.
// It specifies default, minimum, and maximum timeout values that control
// how long a tool is allowed to execute.
type TimeoutConfig struct {
	// Default is the timeout to use if the caller doesn't specify one.
	// A zero value means use the SDK default (5 minutes).
	Default time.Duration

	// Max is the maximum allowed timeout for this tool.
	// A zero value means no upper bound is enforced.
	Max time.Duration

	// Min is the minimum allowed timeout for this tool.
	// A zero value means no lower bound is enforced.
	Min time.Duration
}

// Validate checks that the timeout configuration is internally consistent.
// It verifies that:
// - If both min and max are set, min <= max
// - If default is set, it falls within the min/max bounds
//
// Returns an error if the configuration is invalid.
func (c TimeoutConfig) Validate() error {
	// Check that min doesn't exceed max
	if c.Min > 0 && c.Max > 0 && c.Min > c.Max {
		return fmt.Errorf("min timeout %v exceeds max timeout %v", c.Min, c.Max)
	}

	// Check that default is within bounds
	if c.Default > 0 {
		if c.Min > 0 && c.Default < c.Min {
			return fmt.Errorf("default timeout %v below min %v", c.Default, c.Min)
		}
		if c.Max > 0 && c.Default > c.Max {
			return fmt.Errorf("default timeout %v exceeds max %v", c.Default, c.Max)
		}
	}

	return nil
}

// ValidateTimeout checks if a requested timeout is within the configured bounds.
// It verifies that:
// - If min is set, requested >= min
// - If max is set, requested <= max
//
// Returns an error if the timeout is outside the allowed range, nil otherwise.
func (c TimeoutConfig) ValidateTimeout(requested time.Duration) error {
	if c.Min > 0 && requested < c.Min {
		return fmt.Errorf("timeout %v below minimum %v", requested, c.Min)
	}
	if c.Max > 0 && requested > c.Max {
		return fmt.Errorf("timeout %v exceeds maximum %v", requested, c.Max)
	}
	return nil
}

// ResolveTimeout returns the effective timeout to use for tool execution.
// It implements the following precedence order:
// 1. If requested > 0, use the requested timeout
// 2. Else if config.Default > 0, use the config default
// 3. Else use the SDK default (5 minutes)
//
// Note: This method does not perform validation. Call ValidateTimeout first
// if the requested timeout needs to be checked against bounds.
func (c TimeoutConfig) ResolveTimeout(requested time.Duration) time.Duration {
	if requested > 0 {
		return requested
	}
	if c.Default > 0 {
		return c.Default
	}
	return 5 * time.Minute // SDK default
}
