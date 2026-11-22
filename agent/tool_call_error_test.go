// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"errors"
	"fmt"
	"testing"
)

func TestToolCallError(t *testing.T) {
	err := fmt.Errorf("invoke: %w", &ToolCallError{Code: ToolErrUnavailable, Message: "merge not granted yet"})
	tce, ok := IsToolCallError(err)
	if !ok || tce.Code != ToolErrUnavailable || !tce.Retryable() {
		t.Fatalf("got %+v %v, want a retryable unavailable error", tce, ok)
	}
	if got := tce.Error(); got != "CallToolProto error: merge not granted yet" {
		t.Errorf("Error() = %q, want the form of earlier versions", got)
	}
	denied := &ToolCallError{Code: ToolErrPermissionDenied, Message: "not authorized to invoke tool"}
	if denied.Retryable() {
		t.Error("a permission denial is final")
	}
	if _, ok := IsToolCallError(errors.New("CallToolProto callback failed")); ok {
		t.Error("a transport error is not a ToolCallError")
	}
}
