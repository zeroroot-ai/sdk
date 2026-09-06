// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// SlotDefinition defines requirements for an LLM slot in the Gibson framework.
// Slots represent different LLM capabilities needed by an agent (e.g., "primary", "vision", "code").
type SlotDefinition struct {
	// Name is the unique identifier for this slot.
	Name string

	// Description explains what this slot is used for.
	Description string

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

// Validate checks if the slot definition is valid.
func (s *SlotDefinition) Validate() error {
	if s.Name == "" {
		return &ValidationError{Field: "Name", Message: "slot name cannot be empty"}
	}
	if s.MinContextWindow < 0 {
		return &ValidationError{Field: "MinContextWindow", Message: "context window cannot be negative"}
	}
	return nil
}

// ToRequirements converts a SlotDefinition to SlotRequirements.
func (s *SlotDefinition) ToRequirements() SlotRequirements {
	return SlotRequirements{
		MinContextWindow: s.MinContextWindow,
		RequiredFeatures: append([]string(nil), s.RequiredFeatures...),
		PreferredModels:  append([]string(nil), s.PreferredModels...),
	}
}

// HasFeature checks if a required feature is present.
func (s *SlotDefinition) HasFeature(feature string) bool {
	for _, f := range s.RequiredFeatures {
		if f == feature {
			return true
		}
	}
	return false
}

// PrefersModel checks if a model is in the preferred list.
func (s *SlotDefinition) PrefersModel(model string) bool {
	for _, m := range s.PreferredModels {
		if m == model {
			return true
		}
	}
	return false
}

// Satisfies checks if the given requirements satisfy this slot's requirements.
func (r *SlotRequirements) Satisfies(features []string, contextWindow int) bool {
	// Check context window
	if contextWindow < r.MinContextWindow {
		return false
	}

	// Check that all required features are present
	for _, required := range r.RequiredFeatures {
		found := false
		for _, available := range features {
			if available == required {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// HasFeature checks if a required feature is present.
func (r *SlotRequirements) HasFeature(feature string) bool {
	for _, f := range r.RequiredFeatures {
		if f == feature {
			return true
		}
	}
	return false
}

// ValidationError represents an error in slot validation.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
