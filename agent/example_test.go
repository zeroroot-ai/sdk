// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent_test

import (
	"fmt"
)

// Example_capabilities demonstrates agent capability system.
func Example_capabilities() {
	// Show all available capabilities
	capabilities := []string{
		"prompt_injection",
		"jailbreak",
		"data_extraction",
		"model_manipulation",
		"dos",
	}

	// Note: Capabilities are now simple strings. Domain-specific capability
	// descriptions are maintained in Gibson's taxonomy system.
	fmt.Println("Available capabilities:")
	for _, cap := range capabilities {
		fmt.Printf("- %s\n", cap)
	}

	// Output:
	// Available capabilities:
	// - prompt_injection
	// - jailbreak
	// - data_extraction
	// - model_manipulation
	// - dos
}
