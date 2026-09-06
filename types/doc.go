// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package types provides core type definitions for the Gibson AI Security Testing Framework.
//
// This package defines fundamental types used throughout the Gibson SDK for representing
// targets, techniques, missions, and health status. These types provide a consistent
// interface for agents and components to communicate and coordinate during security testing.
//
// # Health Types
//
// Health types represent the operational status of components:
//
//	status := types.NewHealthyStatus("all systems operational")
//	if status.IsHealthy() {
//	    // Component is fully operational
//	}
//
//	degraded := types.NewDegradedStatus("high latency", map[string]any{
//	    "latency_ms": 500,
//	})
//
// # Target Types
//
// Target types define the AI systems being tested:
//
//	target := &types.TargetInfo{
//	    ID:       "target-1",
//	    Name:     "Production ChatGPT",
//	    URL:      "https://api.openai.com/v1/chat/completions",
//	    Type:     types.TargetTypeLLMAPI,
//	    Provider: "openai",
//	}
//	target.SetHeader("Authorization", "Bearer "+apiKey)
//	target.SetMetadata("model", "gpt-4")
//
// Supported target types:
//   - TargetTypeLLMChat: Conversational LLM interfaces
//   - TargetTypeLLMAPI: Programmatic LLM API endpoints
//   - TargetTypeRAG: Retrieval-Augmented Generation systems
//   - TargetTypeAgent: Autonomous AI agent systems
//   - TargetTypeCopilot: AI coding assistants
//
// # Technique Types
//
// Technique types categorize security testing approaches:
//
//	technique := &types.TechniqueInfo{
//	    Type:        types.TechniquePromptInjection,
//	    Name:        "System Prompt Override",
//	    Description: "Attempts to override system instructions",
//	}
//	technique.AddTag("high-risk")
//	technique.SetMetadata("success_rate", 0.85)
//
// Supported technique types:
//   - TechniquePromptInjection: Malicious instruction injection
//   - TechniqueJailbreak: Safety guardrail bypass
//   - TechniqueDataExtraction: Training data or sensitive info extraction
//   - TechniqueModelManipulation: Behavior or output alteration
//   - TechniqueDOS: Denial-of-service attacks
//
// # Mission Types
//
// Mission types define the operational context for testing:
//
//	constraints := types.NewMissionConstraints().
//	    WithMaxDuration(2 * time.Hour).
//	    WithMaxFindings(50).
//	    WithSeverityThreshold("medium").
//	    WithRequireEvidence(true)
//
//	mission := types.NewMissionContext("mission-1", "Penetration Test")
//	mission.Constraints = constraints
//	mission.Phase = "reconnaissance"
//	mission.SetMetadata("start_time", time.Now())
//
//	// Check if mission should stop
//	if mission.ShouldStop(findingCount) {
//	    // Stop execution
//	}
//
// # Validation
//
// All major types support validation:
//
//	if err := target.Validate(); err != nil {
//	    log.Fatalf("Invalid target: %v", err)
//	}
//
//	if err := technique.Validate(); err != nil {
//	    log.Fatalf("Invalid technique: %v", err)
//	}
//
//	if err := mission.Validate(); err != nil {
//	    log.Fatalf("Invalid mission: %v", err)
//	}
//
// # JSON Serialization
//
// All types support JSON marshaling and unmarshaling:
//
//	data, err := json.Marshal(target)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	var loaded TargetInfo
//	if err := json.Unmarshal(data, &loaded); err != nil {
//	    log.Fatal(err)
//	}
package types
