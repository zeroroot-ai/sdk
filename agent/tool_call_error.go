// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import "errors"

// The codes of a ToolCallError that an agent acts on. The value is the name of
// the daemon error code (gibson.common.v1.ErrorCode).
const (
	// ToolErrUnavailable means that the daemon did not run the call now and a
	// later call can succeed. Examples are a merge slot that is not granted, a
	// call that waits for approval, and an audit store that is down.
	ToolErrUnavailable = "ERROR_CODE_UNAVAILABLE"
	// ToolErrPermissionDenied means that the daemon refused the call and the
	// refusal is final. Examples are a failed can_execute check and a tool that
	// the mission policy blocks.
	ToolErrPermissionDenied = "ERROR_CODE_PERMISSION_DENIED"
)

// ToolCallError is the error of a tool call that the daemon refused or that
// failed. It carries the code of the daemon, so an agent tells a refusal that
// can pass from a final refusal without a match on words (gibson#1575).
//
// Use errors.As to read it. A call that did not reach the daemon returns
// another error type.
type ToolCallError struct {
	// Code is the name of the daemon error code, for example
	// ToolErrUnavailable.
	Code string
	// Message is the message of the daemon.
	Message string
}

// Error returns the message of the daemon in the form that earlier versions
// returned.
func (e *ToolCallError) Error() string { return "CallToolProto error: " + e.Message }

// Retryable reports whether a later call can succeed.
func (e *ToolCallError) Retryable() bool { return e.Code == ToolErrUnavailable }

// IsToolCallError returns the ToolCallError in the chain of err, if any.
func IsToolCallError(err error) (*ToolCallError, bool) {
	var tce *ToolCallError
	if errors.As(err, &tce) {
		return tce, true
	}
	return nil, false
}
