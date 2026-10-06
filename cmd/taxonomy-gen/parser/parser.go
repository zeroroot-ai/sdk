// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package parser provides YAML parsing functionality for taxonomy files.
package parser

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/schema"
)

// ParseYAML parses a taxonomy YAML file and returns the parsed taxonomy.
func ParseYAML(path string) (*schema.Taxonomy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return ParseYAMLBytes(data, path)
}

// ParseYAMLBytes parses taxonomy YAML from bytes.
func ParseYAMLBytes(data []byte, sourcePath string) (*schema.Taxonomy, error) {
	var taxonomy schema.Taxonomy

	if err := yaml.Unmarshal(data, &taxonomy); err != nil {
		return nil, &ParseError{
			Path:    sourcePath,
			Message: fmt.Sprintf("failed to parse YAML: %v", err),
		}
	}

	// Validate the parsed taxonomy
	if err := taxonomy.Validate(); err != nil {
		return nil, &ParseError{
			Path:    sourcePath,
			Message: err.Error(),
		}
	}

	return &taxonomy, nil
}

// ParseError represents a parsing error with file context.
type ParseError struct {
	Path    string
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}
