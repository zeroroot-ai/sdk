// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

// Descriptor describes a tool's metadata.
// It provides a snapshot of a tool's configuration without the execution logic.
type Descriptor struct {
	// Name is the unique identifier for the tool.
	Name string `json:"name"`

	// Version is the semantic version of the tool.
	Version string `json:"version"`

	// Description is a human-readable description of what the tool does.
	Description string `json:"description"`

	// Tags are labels for categorizing and discovering the tool.
	Tags []string `json:"tags"`

	// InputMessageType is the fully-qualified proto message type name for input.
	InputMessageType string `json:"input_message_type"`

	// OutputMessageType is the fully-qualified proto message type name for output.
	OutputMessageType string `json:"output_message_type"`
}
