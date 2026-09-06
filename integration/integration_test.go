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
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/schema"
	"github.com/zeroroot-ai/sdk/types"
)

// TestSDKPackageImports verifies all SDK packages can be imported together.
func TestSDKPackageImports(t *testing.T) {
	// This test ensures all packages compile together without conflicts.
	// If this test compiles and runs, all imports are working correctly.

	t.Run("agent package", func(t *testing.T) {
		var _ agent.Agent
		var _ string = "prompt_injection"
		var _ agent.Result
		var _ agent.Task
	})

	t.Run("tool package", func(t *testing.T) {
		// Tool package types are accessible
		schema := schema.Object(map[string]schema.JSON{
			"test": schema.String(),
		})
		assert.NotNil(t, schema)
	})

	t.Run("plugin package", func(t *testing.T) {
		var _ plugin.Plugin
		var _ plugin.MethodDescriptor
	})

	t.Run("llm package", func(t *testing.T) {
		var _ llm.Message
		var _ llm.CompletionResponse
	})

	t.Run("types package", func(t *testing.T) {
		// Note: TargetType and TechniqueType removed - now plain strings
		var _ types.HealthStatus
	})

	t.Run("finding package", func(t *testing.T) {
		var _ finding.Finding
		var _ finding.Severity = finding.SeverityCritical
		var _ finding.Status = finding.StatusOpen
	})
}

// TestFrameworkCreationAndLifecycle tests creating and managing the framework.
func TestFrameworkCreationAndLifecycle(t *testing.T) {
	t.Run("create framework with defaults", func(t *testing.T) {
		fw, err := sdk.NewFramework()
		require.NoError(t, err)
		require.NotNil(t, fw)

		ctx := context.Background()

		// Start the framework
		err = fw.Start(ctx)
		require.NoError(t, err)

		// Verify registries are accessible
		assert.NotNil(t, fw.Agents())
		assert.NotNil(t, fw.Tools())
		assert.NotNil(t, fw.Plugins())

		// Shutdown the framework
		err = fw.Shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("create framework with options", func(t *testing.T) {
		fw, err := sdk.NewFramework(
			sdk.WithConfig("/tmp/test-config.yaml"),
		)
		require.NoError(t, err)
		require.NotNil(t, fw)

		ctx := context.Background()
		err = fw.Start(ctx)
		require.NoError(t, err)

		err = fw.Shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("double start error", func(t *testing.T) {
		fw, err := sdk.NewFramework()
		require.NoError(t, err)

		ctx := context.Background()

		err = fw.Start(ctx)
		require.NoError(t, err)

		// Second start should fail
		err = fw.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")

		fw.Shutdown(ctx)
	})
}

// TestRegistryOperations tests registry registration, retrieval, and listing.
func TestRegistryOperations(t *testing.T) {
	fw, err := sdk.NewFramework()
	require.NoError(t, err)

	ctx := context.Background()
	err = fw.Start(ctx)
	require.NoError(t, err)
	defer fw.Shutdown(ctx)

	t.Run("agent registry operations", func(t *testing.T) {
		// Create test agent
		testAgent, err := sdk.NewAgent(
			sdk.WithName("registry-test-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Agent for registry testing"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
		)
		require.NoError(t, err)

		// Register agent
		err = fw.Agents().Register(testAgent)
		require.NoError(t, err)

		// Get agent
		retrieved, err := fw.Agents().Get("registry-test-agent")
		require.NoError(t, err)
		assert.Equal(t, "registry-test-agent", retrieved.Name())
		assert.Equal(t, "1.0.0", retrieved.Version())

		// List agents
		agents := fw.Agents().List()
		assert.Len(t, agents, 1)
		assert.Equal(t, "registry-test-agent", agents[0].Name)

		// Duplicate registration should fail
		err = fw.Agents().Register(testAgent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")

		// Unregister agent
		err = fw.Agents().Unregister("registry-test-agent")
		require.NoError(t, err)

		// Get should fail after unregister
		_, err = fw.Agents().Get("registry-test-agent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("tool registry operations", func(t *testing.T) {
		// Create test tool
		testTool, err := sdk.NewTool(
			sdk.WithToolName("registry-test-tool"),
			sdk.WithToolDescription("Tool for registry testing"),
			sdk.WithToolVersion("2.0.0"),
			sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
				return map[string]any{"result": "success"}, nil
			}),
		)
		require.NoError(t, err)

		// Register tool
		err = fw.Tools().Register(testTool)
		require.NoError(t, err)

		// Get tool
		retrieved, err := fw.Tools().Get("registry-test-tool")
		require.NoError(t, err)
		assert.Equal(t, "registry-test-tool", retrieved.Name())
		assert.Equal(t, "2.0.0", retrieved.Version())

		// List tools
		tools := fw.Tools().List()
		assert.Len(t, tools, 1)
		assert.Equal(t, "registry-test-tool", tools[0].Name)

		// Unregister tool
		err = fw.Tools().Unregister("registry-test-tool")
		require.NoError(t, err)

		// Get should fail after unregister
		_, err = fw.Tools().Get("registry-test-tool")
		assert.Error(t, err)
	})

	t.Run("plugin registry operations", func(t *testing.T) {
		// Create test plugin
		cfg := plugin.NewConfig()
		cfg.SetName("registry-test-plugin")
		cfg.SetVersion("3.0.0")
		cfg.SetDescription("Plugin for registry testing")

		testPlugin, err := plugin.New(cfg)
		require.NoError(t, err)

		// Register plugin
		err = fw.Plugins().Register(testPlugin)
		require.NoError(t, err)

		// Get plugin
		retrieved, err := fw.Plugins().Get("registry-test-plugin")
		require.NoError(t, err)
		assert.Equal(t, "registry-test-plugin", retrieved.Name())
		assert.Equal(t, "3.0.0", retrieved.Version())

		// List plugins
		plugins := fw.Plugins().List()
		assert.Len(t, plugins, 1)
		assert.Equal(t, "registry-test-plugin", plugins[0].Name)

		// Unregister plugin
		err = fw.Plugins().Unregister("registry-test-plugin")
		require.NoError(t, err)
	})
}

// TestMissionManagement tests mission creation and lifecycle.
func TestMissionManagement(t *testing.T) {
	fw, err := sdk.NewFramework()
	require.NoError(t, err)

	ctx := context.Background()
	err = fw.Start(ctx)
	require.NoError(t, err)
	defer fw.Shutdown(ctx)

	t.Run("create mission", func(t *testing.T) {
		mission, err := fw.CreateMission(ctx,
			sdk.WithMissionName("test-mission"),
			sdk.WithMissionDescription("Integration test mission"),
			sdk.WithMissionTarget("target-123"),
			sdk.WithMissionAgents("agent-1", "agent-2"),
		)

		require.NoError(t, err)
		require.NotNil(t, mission)

		assert.NotEmpty(t, mission.ID)
		assert.Equal(t, "test-mission", mission.Name)
		assert.Equal(t, "Integration test mission", mission.Description)
		assert.Equal(t, "pending", mission.Status)
		assert.Equal(t, "target-123", mission.TargetID)
		assert.Contains(t, mission.AgentNames, "agent-1")
		assert.Contains(t, mission.AgentNames, "agent-2")
	})

	t.Run("start and stop mission", func(t *testing.T) {
		mission, err := fw.CreateMission(ctx,
			sdk.WithMissionName("lifecycle-mission"),
		)
		require.NoError(t, err)

		// Start mission
		err = fw.StartMission(ctx, mission.ID)
		require.NoError(t, err)

		// Get mission and verify status
		retrieved, err := fw.GetMission(ctx, mission.ID)
		require.NoError(t, err)
		assert.Equal(t, "running", retrieved.Status)
		assert.NotNil(t, retrieved.StartedAt)

		// Stop mission
		err = fw.StopMission(ctx, mission.ID)
		require.NoError(t, err)

		// Verify stopped status
		retrieved, err = fw.GetMission(ctx, mission.ID)
		require.NoError(t, err)
		assert.Equal(t, "stopped", retrieved.Status)
		assert.NotNil(t, retrieved.CompletedAt)
	})

	t.Run("list missions", func(t *testing.T) {
		// Create multiple missions
		mission1, err := fw.CreateMission(ctx,
			sdk.WithMissionName("mission-1"),
		)
		require.NoError(t, err)

		mission2, err := fw.CreateMission(ctx,
			sdk.WithMissionName("mission-2"),
		)
		require.NoError(t, err)

		// List all missions
		missions, err := fw.ListMissions(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(missions), 2)

		// Verify our missions are in the list
		missionIDs := make(map[string]bool)
		for _, m := range missions {
			missionIDs[m.ID] = true
		}
		assert.True(t, missionIDs[mission1.ID])
		assert.True(t, missionIDs[mission2.ID])

		// List with limit
		missions, err = fw.ListMissions(ctx, sdk.WithLimit(1))
		require.NoError(t, err)
		assert.Len(t, missions, 1)
	})

	t.Run("invalid mission operations", func(t *testing.T) {
		// Get non-existent mission
		_, err := fw.GetMission(ctx, "invalid-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Start non-existent mission
		err = fw.StartMission(ctx, "invalid-id")
		assert.Error(t, err)

		// Stop non-existent mission
		err = fw.StopMission(ctx, "invalid-id")
		assert.Error(t, err)
	})
}

// TestFindingExport tests finding export functionality.
func TestFindingExport(t *testing.T) {
	fw, err := sdk.NewFramework()
	require.NoError(t, err)

	ctx := context.Background()
	err = fw.Start(ctx)
	require.NoError(t, err)
	defer fw.Shutdown(ctx)

	t.Run("export to JSON", func(t *testing.T) {
		var buf bytes.Buffer
		err := fw.ExportFindings(ctx, finding.FormatJSON, &buf)
		require.NoError(t, err)

		// Should produce valid JSON (empty array for no findings)
		output := buf.String()
		assert.Contains(t, output, "[")
		assert.Contains(t, output, "]")
	})

	t.Run("export to CSV", func(t *testing.T) {
		var buf bytes.Buffer
		err := fw.ExportFindings(ctx, finding.FormatCSV, &buf)
		require.NoError(t, err)

		// Should have CSV header
		output := buf.String()
		assert.Contains(t, output, "ID,Title,Severity")
	})

	t.Run("export to HTML", func(t *testing.T) {
		var buf bytes.Buffer
		err := fw.ExportFindings(ctx, finding.FormatHTML, &buf)
		require.NoError(t, err)

		// Should have HTML structure
		output := buf.String()
		assert.Contains(t, output, "<html>")
		assert.Contains(t, output, "</html>")
		assert.Contains(t, output, "Security Findings Report")
	})

	t.Run("export to SARIF", func(t *testing.T) {
		var buf bytes.Buffer
		err := fw.ExportFindings(ctx, finding.FormatSARIF, &buf)
		require.NoError(t, err)

		// Should have SARIF structure
		output := buf.String()
		assert.Contains(t, output, "sarif")
		assert.Contains(t, output, "version")
	})

	t.Run("invalid export format", func(t *testing.T) {
		var buf bytes.Buffer
		err := fw.ExportFindings(ctx, finding.ExportFormat("invalid"), &buf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid export format")
	})
}

// TestEndToEndIntegration tests a complete end-to-end mission.
func TestEndToEndIntegration(t *testing.T) {
	// Create framework
	fw, err := sdk.NewFramework()
	require.NoError(t, err)

	ctx := context.Background()
	err = fw.Start(ctx)
	require.NoError(t, err)
	defer fw.Shutdown(ctx)

	// Create and register an agent
	testAgent, err := sdk.NewAgent(
		sdk.WithName("integration-agent"),
		sdk.WithVersion("1.0.0"),
		sdk.WithDescription("End-to-end integration test agent"),
		sdk.WithCapabilities("prompt_injection"),
		sdk.WithTargetTypes("llm_chat"),
		sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
			return agent.NewSuccessResult("integration test completed"), nil
		}),
	)
	require.NoError(t, err)

	err = fw.Agents().Register(testAgent)
	require.NoError(t, err)

	// Create and register a tool
	testTool, err := sdk.NewTool(
		sdk.WithToolName("integration-tool"),
		sdk.WithToolDescription("End-to-end integration test tool"),
		sdk.WithInputSchema(schema.Object(map[string]schema.JSON{
			"input": schema.String(),
		})),
		sdk.WithOutputSchema(schema.Object(map[string]schema.JSON{
			"output": schema.String(),
		})),
		sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
			return map[string]any{"output": "processed"}, nil
		}),
	)
	require.NoError(t, err)

	err = fw.Tools().Register(testTool)
	require.NoError(t, err)

	// Create and register a plugin
	pluginCfg := plugin.NewConfig()
	pluginCfg.SetName("integration-plugin")
	pluginCfg.SetVersion("1.0.0")
	pluginCfg.SetDescription("End-to-end integration test plugin")
	pluginCfg.AddMethod("test", func(ctx context.Context, params map[string]any) (any, error) {
		return "plugin method result", nil
	}, schema.Object(map[string]schema.JSON{}), schema.String())

	testPlugin, err := plugin.New(pluginCfg)
	require.NoError(t, err)

	err = fw.Plugins().Register(testPlugin)
	require.NoError(t, err)

	// Create a mission
	mission, err := fw.CreateMission(ctx,
		sdk.WithMissionName("integration-mission"),
		sdk.WithMissionDescription("End-to-end integration test"),
		sdk.WithMissionAgents("integration-agent"),
	)
	require.NoError(t, err)

	// Start the mission
	err = fw.StartMission(ctx, mission.ID)
	require.NoError(t, err)

	// Verify all components are accessible
	agents := fw.Agents().List()
	assert.Len(t, agents, 1)

	tools := fw.Tools().List()
	assert.Len(t, tools, 1)

	plugins := fw.Plugins().List()
	assert.Len(t, plugins, 1)

	// Stop the mission
	err = fw.StopMission(ctx, mission.ID)
	require.NoError(t, err)

	// Verify mission is stopped
	finalMission, err := fw.GetMission(ctx, mission.ID)
	require.NoError(t, err)
	assert.Equal(t, "stopped", finalMission.Status)

	t.Log("End-to-end integration test completed successfully")
}

// TestConcurrentRegistryAccess tests thread safety of registries.
func TestConcurrentRegistryAccess(t *testing.T) {
	fw, err := sdk.NewFramework()
	require.NoError(t, err)

	ctx := context.Background()
	err = fw.Start(ctx)
	require.NoError(t, err)
	defer fw.Shutdown(ctx)

	t.Run("concurrent agent registration", func(t *testing.T) {
		done := make(chan bool)

		// Register agents concurrently
		for i := 0; i < 10; i++ {
			go func(id int) {
				agentName := "concurrent-agent-" + string(rune('a'+id))
				a, err := sdk.NewAgent(
					sdk.WithName(agentName),
					sdk.WithVersion("1.0.0"),
					sdk.WithDescription("Concurrent test"),
					sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
						return agent.NewSuccessResult("done"), nil
					}),
				)
				if err == nil {
					fw.Agents().Register(a)
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify agents were registered
		agents := fw.Agents().List()
		assert.GreaterOrEqual(t, len(agents), 1)
	})
}
