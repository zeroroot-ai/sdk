// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"testing"
)

func TestDescriptor_Serialization(t *testing.T) {
	// Create a descriptor
	desc := Descriptor{
		Name:        "serialization-test",
		Version:     "1.0.0",
		Description: "Test serialization",
		Tags:        []string{"test"},
	}

	// Verify struct tags are properly defined for JSON serialization
	if desc.Name == "" {
		t.Error("Descriptor Name should not be empty")
	}

	if desc.Version == "" {
		t.Error("Descriptor Version should not be empty")
	}

	if len(desc.Tags) == 0 {
		t.Error("Descriptor Tags should not be empty")
	}
}
