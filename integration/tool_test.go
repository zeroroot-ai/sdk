// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

//go:build integration_execute_legacy
// +build integration_execute_legacy

// Copyright (c) 2026 ZeroRoot
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/schema"
)

// TestToolCreation tests creating a tool using SDK entry points.
func TestToolCreation(t *testing.T) {
	t.Run("with basic configuration", func(t *testing.T) {
		tool, err := sdk.NewTool(
			sdk.WithToolName("http-request"),
			sdk.WithToolDescription("Makes HTTP requests"),
			sdk.WithToolVersion("1.0.0"),
			sdk.WithToolTags("http", "network"),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return map[string]any{"status": 200}, nil
			}),
		)

		require.NoError(t, err)
		require.NotNil(t, tool)

		assert.Equal(t, "http-request", tool.Name())
		assert.Equal(t, "1.0.0", tool.Version())
		assert.Equal(t, "Makes HTTP requests", tool.Description())
		assert.Contains(t, tool.Tags(), "http")
		assert.Contains(t, tool.Tags(), "network")
	})

	t.Run("with input and output schemas", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"url":    schema.String(),
			"method": schema.String(),
		}, "url")

		outputSchema := schema.Object(map[string]schema.JSON{
			"status": schema.Int(),
			"body":   schema.String(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("web-scraper"),
			sdk.WithToolDescription("Scrapes web pages"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithOutputSchema(outputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				url := input["url"].(string)
				return map[string]any{
					"status": 200,
					"body":   "Content from " + url,
				}, nil
			}),
		)

		require.NoError(t, err)
		assert.NotNil(t, tool.InputSchema())
		assert.NotNil(t, tool.OutputSchema())
	})

	t.Run("missing required name", func(t *testing.T) {
		tool, err := sdk.NewTool(
			sdk.WithToolDescription("No name provided"),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return map[string]any{}, nil
			}),
		)

		assert.Error(t, err)
		assert.Nil(t, tool)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("missing execute handler", func(t *testing.T) {
		tool, err := sdk.NewTool(
			sdk.WithToolName("no-handler"),
			sdk.WithToolDescription("Missing handler"),
		)

		assert.Error(t, err)
		assert.Nil(t, tool)
		assert.Contains(t, err.Error(), "execute")
	})
}

// TestToolExecution tests tool execution with valid and invalid inputs.
func TestToolExecution(t *testing.T) {
	t.Run("successful execution with valid input", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"x": schema.Int(),
			"y": schema.Int(),
		}, "x", "y")

		outputSchema := schema.Object(map[string]schema.JSON{
			"sum": schema.Int(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("calculator"),
			sdk.WithToolDescription("Adds two numbers"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithOutputSchema(outputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				x := input["x"].(float64)
				y := input["y"].(float64)
				return map[string]any{"sum": x + y}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		output, err := tool.Execute(ctx, map[string]any{
			"x": 5.0,
			"y": 3.0,
		})

		require.NoError(t, err)
		assert.Equal(t, 8.0, output["sum"])
	})

	t.Run("execution with invalid input schema", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"required_field": schema.String(),
		}, "required_field")

		tool, err := sdk.NewTool(
			sdk.WithToolName("validator"),
			sdk.WithToolDescription("Validates input"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return map[string]any{"valid": true}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		output, err := tool.Execute(ctx, map[string]any{
			"wrong_field": "value",
		})

		assert.Error(t, err)
		assert.Nil(t, output)
	})

	t.Run("execution with handler error", func(t *testing.T) {
		expectedErr := errors.New("processing failed")

		tool, err := sdk.NewTool(
			sdk.WithToolName("failing-tool"),
			sdk.WithToolDescription("Always fails"),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return nil, expectedErr
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		output, err := tool.Execute(ctx, map[string]any{})

		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("execution with context cancellation", func(t *testing.T) {
		tool, err := sdk.NewTool(
			sdk.WithToolName("long-running"),
			sdk.WithToolDescription("Long running operation"),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					return map[string]any{"done": true}, nil
				}
			}),
		)

		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		output, err := tool.Execute(ctx, map[string]any{})

		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Equal(t, context.Canceled, err)
	})
}

// TestToolSchemaValidation tests schema validation for various types.
func TestToolSchemaValidation(t *testing.T) {
	t.Run("string schema", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"message": schema.String(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("echo"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return input, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()

		// Valid string
		output, err := tool.Execute(ctx, map[string]any{"message": "hello"})
		require.NoError(t, err)
		assert.Equal(t, "hello", output["message"])

		// Invalid type (should fail validation)
		output, err = tool.Execute(ctx, map[string]any{"message": 123})
		assert.Error(t, err)
	})

	t.Run("integer schema", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"count": schema.Int(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("counter"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return input, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()

		// Valid integer
		output, err := tool.Execute(ctx, map[string]any{"count": 42.0})
		require.NoError(t, err)
		assert.Equal(t, 42.0, output["count"])
	})

	t.Run("boolean schema", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"enabled": schema.Bool(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("toggle"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return input, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()

		// Valid boolean
		output, err := tool.Execute(ctx, map[string]any{"enabled": true})
		require.NoError(t, err)
		assert.Equal(t, true, output["enabled"])
	})

	t.Run("array schema", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"items": schema.Array(schema.String()),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("list-processor"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				items := input["items"].([]any)
				return map[string]any{"count": float64(len(items))}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()

		// Valid array
		output, err := tool.Execute(ctx, map[string]any{
			"items": []any{"a", "b", "c"},
		})
		require.NoError(t, err)
		assert.Equal(t, 3.0, output["count"])
	})

	t.Run("nested object schema", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"user": schema.Object(map[string]schema.JSON{
				"name":  schema.String(),
				"email": schema.String(),
				"age":   schema.Int(),
			}, "name", "email"),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("user-processor"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				user := input["user"].(map[string]any)
				return map[string]any{"processed": user["name"]}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()

		// Valid nested object
		output, err := tool.Execute(ctx, map[string]any{
			"user": map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30.0,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Alice", output["processed"])

		// Missing required field
		output, err = tool.Execute(ctx, map[string]any{
			"user": map[string]any{
				"name": "Bob",
				// missing email
			},
		})
		assert.Error(t, err)
	})
}

// TestToolHealth tests tool health endpoint.
func TestToolHealth(t *testing.T) {
	t.Run("default healthy status", func(t *testing.T) {
		tool, err := sdk.NewTool(
			sdk.WithToolName("healthy-tool"),
			sdk.WithToolDescription("Always healthy"),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return map[string]any{}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		health := tool.Health(ctx)

		assert.True(t, health.IsHealthy())
		assert.NotEmpty(t, health.Message)
	})
}

// TestToolWithRealWorldScenarios tests realistic tool scenarios.
func TestToolWithRealWorldScenarios(t *testing.T) {
	t.Run("HTTP request tool", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"url":     schema.String(),
			"method":  schema.String(),
			"headers": schema.Object(map[string]schema.JSON{}),
			"body":    schema.String(),
		}, "url", "method")

		outputSchema := schema.Object(map[string]schema.JSON{
			"status_code": schema.Int(),
			"headers":     schema.Object(map[string]schema.JSON{}),
			"body":        schema.String(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("http-client"),
			sdk.WithToolDescription("Makes HTTP requests"),
			sdk.WithToolTags("http", "network", "web"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithOutputSchema(outputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				// Simulate HTTP request
				url := input["url"].(string)
				method := input["method"].(string)

				// Mock response
				return map[string]any{
					"status_code": 200.0,
					"headers":     map[string]any{"Content-Type": "application/json"},
					"body":        `{"result": "success", "url": "` + url + `", "method": "` + method + `"}`,
				}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		output, err := tool.Execute(ctx, map[string]any{
			"url":     "https://api.example.com/data",
			"method":  "GET",
			"headers": map[string]any{},
		})

		require.NoError(t, err)
		assert.Equal(t, 200.0, output["status_code"])
		assert.Contains(t, output["body"], "success")
	})

	t.Run("file reader tool", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"path": schema.String(),
		}, "path")

		outputSchema := schema.Object(map[string]schema.JSON{
			"content": schema.String(),
			"size":    schema.Int(),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("file-reader"),
			sdk.WithToolDescription("Reads file contents"),
			sdk.WithToolTags("file", "io"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithOutputSchema(outputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				path := input["path"].(string)

				// Mock file reading
				mockContent := "File content from " + path
				return map[string]any{
					"content": mockContent,
					"size":    float64(len(mockContent)),
				}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		output, err := tool.Execute(ctx, map[string]any{
			"path": "/tmp/test.txt",
		})

		require.NoError(t, err)
		assert.Contains(t, output["content"], "/tmp/test.txt")
		assert.Greater(t, output["size"], 0.0)
	})

	t.Run("JSON processor tool", func(t *testing.T) {
		inputSchema := schema.Object(map[string]schema.JSON{
			"data":      schema.Object(map[string]schema.JSON{}),
			"transform": schema.String(),
		}, "data")

		outputSchema := schema.Object(map[string]schema.JSON{
			"result": schema.Object(map[string]schema.JSON{}),
		})

		tool, err := sdk.NewTool(
			sdk.WithToolName("json-processor"),
			sdk.WithToolDescription("Processes JSON data"),
			sdk.WithToolTags("json", "data", "transform"),
			sdk.WithInputSchema(inputSchema),
			sdk.WithOutputSchema(outputSchema),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				data := input["data"].(map[string]any)

				// Add a processed flag
				data["processed"] = true
				data["timestamp"] = "2024-01-01T00:00:00Z"

				return map[string]any{
					"result": data,
				}, nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		output, err := tool.Execute(ctx, map[string]any{
			"data": map[string]any{
				"id":   "123",
				"name": "test",
			},
		})

		require.NoError(t, err)
		result := output["result"].(map[string]any)
		assert.Equal(t, true, result["processed"])
		assert.Equal(t, "123", result["id"])
	})
}
