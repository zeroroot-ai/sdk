// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package generator provides code generation from taxonomy YAML.
package generator

import "strings"

// toPascalCase converts a string to PascalCase.
// e.g., "mission_run" -> "MissionRun"
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
