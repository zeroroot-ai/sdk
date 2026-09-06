// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResult_Fail(t *testing.T) {
	t.Run("populates both Error and ErrorInfo", func(t *testing.T) {
		result := Result{}
		testErr := errors.New("test error")

		result.Fail(testErr)

		assert.Equal(t, StatusFailed, result.Status)
		assert.Equal(t, testErr, result.Error)
		require.NotNil(t, result.ErrorInfo)
		assert.Equal(t, "UNKNOWN", result.ErrorInfo.Code)
		assert.Equal(t, "test error", result.ErrorInfo.Message)
	})

	t.Run("ErrorInfo is serialized to JSON", func(t *testing.T) {
		result := Result{}
		testErr := NewResultError("TEST_ERROR", "something went wrong")

		result.Fail(testErr)

		// Serialize to JSON
		jsonBytes, err := json.Marshal(result)
		require.NoError(t, err)

		// Deserialize back
		var decoded map[string]interface{}
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)

		// Verify error field is present and contains ErrorInfo
		errorField, ok := decoded["error"].(map[string]interface{})
		require.True(t, ok, "error field should be present in JSON")
		assert.Equal(t, "TEST_ERROR", errorField["code"])
		assert.Equal(t, "something went wrong", errorField["message"])
	})

	t.Run("Error field is not serialized to JSON", func(t *testing.T) {
		result := Result{}
		testErr := errors.New("test error")

		result.Fail(testErr)

		// Serialize to JSON
		jsonBytes, err := json.Marshal(result)
		require.NoError(t, err)

		// Check raw JSON string doesn't contain Error field key with capital E
		jsonStr := string(jsonBytes)
		assert.NotContains(t, jsonStr, `"Error"`, "Error field should not be in JSON (uses json:\"-\" tag)")
	})

	t.Run("works with ResultError", func(t *testing.T) {
		result := Result{}
		testErr := NewResultError("AGENT_TIMEOUT", "operation timed out").
			WithComponent("test-agent").
			WithRetryable(true)

		result.Fail(testErr)

		assert.Equal(t, StatusFailed, result.Status)
		assert.Equal(t, testErr, result.Error)
		assert.Equal(t, testErr, result.ErrorInfo)
		assert.True(t, result.ErrorInfo.Retryable)
		assert.Equal(t, "test-agent", result.ErrorInfo.Component)
	})

	t.Run("handles nil error", func(t *testing.T) {
		result := Result{}

		result.Fail(nil)

		assert.Equal(t, StatusFailed, result.Status)
		require.NoError(t, result.Error)
		assert.Nil(t, result.ErrorInfo)
	})
}

func TestNewFailedResult_PopulatesErrorInfo(t *testing.T) {
	t.Run("standard error", func(t *testing.T) {
		testErr := errors.New("test error")
		result := NewFailedResult(testErr)

		assert.Equal(t, StatusFailed, result.Status)
		assert.Equal(t, testErr, result.Error)
		require.NotNil(t, result.ErrorInfo)
		assert.Equal(t, "UNKNOWN", result.ErrorInfo.Code)
		assert.Equal(t, "test error", result.ErrorInfo.Message)
	})

	t.Run("ResultError", func(t *testing.T) {
		testErr := NewResultError("EXECUTION_FAILED", "failed to execute")
		result := NewFailedResult(testErr)

		assert.Equal(t, StatusFailed, result.Status)
		assert.Equal(t, testErr, result.Error)
		assert.Equal(t, testErr, result.ErrorInfo)
	})

	t.Run("nil error", func(t *testing.T) {
		result := NewFailedResult(nil)

		assert.Equal(t, StatusFailed, result.Status)
		require.NoError(t, result.Error)
		assert.Nil(t, result.ErrorInfo)
	})
}

func TestNewPartialResult_PopulatesErrorInfo(t *testing.T) {
	t.Run("standard error", func(t *testing.T) {
		testErr := errors.New("partial error")
		result := NewPartialResult("some output", testErr)

		assert.Equal(t, StatusPartial, result.Status)
		assert.Equal(t, "some output", result.Output)
		assert.Equal(t, testErr, result.Error)
		require.NotNil(t, result.ErrorInfo)
		assert.Equal(t, "UNKNOWN", result.ErrorInfo.Code)
		assert.Equal(t, "partial error", result.ErrorInfo.Message)
	})

	t.Run("ResultError", func(t *testing.T) {
		testErr := NewResultError("PARTIAL_FAILURE", "some steps failed")
		result := NewPartialResult(map[string]string{"key": "value"}, testErr)

		assert.Equal(t, StatusPartial, result.Status)
		assert.Equal(t, testErr, result.Error)
		assert.Equal(t, testErr, result.ErrorInfo)
	})

	t.Run("nil error", func(t *testing.T) {
		result := NewPartialResult("output", nil)

		assert.Equal(t, StatusPartial, result.Status)
		assert.Equal(t, "output", result.Output)
		require.NoError(t, result.Error)
		assert.Nil(t, result.ErrorInfo)
	})
}

func TestResult_BackwardsCompatibility(t *testing.T) {
	t.Run("existing code can still use Error field", func(t *testing.T) {
		testErr := errors.New("test error")
		result := NewFailedResult(testErr)

		// Old code pattern - directly checking result.Error
		if result.Error != nil {
			assert.Equal(t, "test error", result.Error.Error())
		}
	})

	t.Run("can construct Result manually", func(t *testing.T) {
		// Old code pattern - manual construction
		result := Result{
			Status: StatusFailed,
			Error:  errors.New("manual error"),
		}

		assert.Equal(t, StatusFailed, result.Status)
		assert.Error(t, result.Error)
		// ErrorInfo won't be populated in manual construction (backwards compat)
	})
}
