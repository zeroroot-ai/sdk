// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// SlotDefinition defines requirements for an LLM slot in the Gibson framework.
// Slots represent different LLM capabilities needed by an agent (e.g., "primary", "vision", "code").
type SlotDefinition struct {
	// Name is the unique identifier for this slot.
	Name string

	// Required indicates whether this slot must be filled for the agent to function.
	Required bool

	// MinContextWindow specifies the minimum context window size (in tokens) required.
	MinContextWindow int

	// RequiredFeatures lists capabilities that the LLM must support.
	// Examples: "vision", "function_calling", "json_mode", "streaming"
	RequiredFeatures []string

	// PreferredModels lists model identifiers that are preferred for this slot.
	// This is a hint to the deployment system, not a strict requirement.
	// Examples: "gpt-4-turbo", "claude-3-opus", "llama-3-70b"
	PreferredModels []string
}

// SlotRequirements specifies the capabilities needed for an LLM slot.
// This is used when requesting an LLM from the deployment system.
type SlotRequirements struct {
	// MinContextWindow specifies the minimum context window size (in tokens) required.
	MinContextWindow int

	// RequiredFeatures lists capabilities that the LLM must support.
	// Examples: "vision", "function_calling", "json_mode", "streaming"
	RequiredFeatures []string

	// PreferredModels lists model identifiers that are preferred.
	// The deployment system will try to use these models if available.
	PreferredModels []string
}
