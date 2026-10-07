// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ResultError
		want string
	}{
		{
			name: "with component",
			err: &ResultError{
				Code:      "TIMEOUT",
				Message:   "operation timed out",
				Component: "sql-injector",
			},
			want: "sql-injector [TIMEOUT]: operation timed out",
		},
		{
			name: "without component",
			err: &ResultError{
				Code:    "NETWORK_ERROR",
				Message: "connection refused",
			},
			want: "[NETWORK_ERROR]: connection refused",
		},
		{
			name: "with cause",
			err: &ResultError{
				Code:    "TASK_FAILED",
				Message: "task execution failed",
				Cause: &ResultError{
					Code:    "TIMEOUT",
					Message: "operation timed out",
				},
			},
			want: "[TASK_FAILED]: task execution failed: [TIMEOUT]: operation timed out",
		},
		{
			name: "empty message",
			err: &ResultError{
				Code:      "UNKNOWN",
				Component: "test-agent",
			},
			want: "test-agent [UNKNOWN]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Equal(t, tt.want, got)
		})
	}
}
