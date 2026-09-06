// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadEvalSet loads an evaluation set from a file.
// The format is automatically detected by file extension (.json, .yaml, .yml).
// It validates that all samples have required fields and unique IDs.
func LoadEvalSet(path string) (*EvalSet, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("eval set file not found: %s", path)
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read eval set file: %w", err)
	}

	// Detect format by extension
	ext := filepath.Ext(path)
	var evalSet EvalSet

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &evalSet); err != nil {
			return nil, fmt.Errorf("failed to parse JSON eval set: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &evalSet); err != nil {
			return nil, fmt.Errorf("failed to parse YAML eval set: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported eval set format: %s (supported: .json, .yaml, .yml)", ext)
	}

	// Validate the loaded eval set
	if err := evalSet.Validate(); err != nil {
		return nil, fmt.Errorf("eval set validation failed: %w", err)
	}

	return &evalSet, nil
}

// Validate checks the eval set structure for correctness.
// It ensures all samples have required fields and unique IDs.
func (e *EvalSet) Validate() error {
	// Track sample IDs to detect duplicates
	seenIDs := make(map[string]bool)

	// Validate each sample
	for i, sample := range e.Samples {
		// Check required field: ID
		if sample.ID == "" {
			return fmt.Errorf("sample at index %d is missing required field 'id'", i)
		}

		// Check required field: Task.Context["objective"] or Task.ID
		if sample.Task.ID == "" {
			return fmt.Errorf("sample %s at index %d is missing required field 'task.id'", sample.ID, i)
		}

		// Check for duplicate IDs
		if seenIDs[sample.ID] {
			return fmt.Errorf("duplicate sample ID found: %s", sample.ID)
		}
		seenIDs[sample.ID] = true
	}

	return nil
}

// FilterByTags returns a new EvalSet containing only samples that have all specified tags.
// The original EvalSet is not modified.
// If tags is empty or nil, returns a copy of the entire EvalSet.
func (e *EvalSet) FilterByTags(tags []string) *EvalSet {
	// If no tags specified, return a copy of the entire set
	if len(tags) == 0 {
		return e.copy()
	}

	// Create a new EvalSet with the same metadata
	filtered := &EvalSet{
		Name:     e.Name,
		Version:  e.Version,
		Metadata: e.Metadata,
		Samples:  make([]Sample, 0),
	}

	// Filter samples by tags
	for _, sample := range e.Samples {
		if hasAllTags(sample.Tags, tags) {
			filtered.Samples = append(filtered.Samples, sample)
		}
	}

	return filtered
}

// copy creates a shallow copy of the EvalSet.
// This is used when no filtering is needed but a new instance is expected.
func (e *EvalSet) copy() *EvalSet {
	return &EvalSet{
		Name:     e.Name,
		Version:  e.Version,
		Metadata: e.Metadata,
		Samples:  append([]Sample{}, e.Samples...),
	}
}

// hasAllTags checks if sampleTags contains all of the required tags.
// Returns true if all required tags are present, false otherwise.
func hasAllTags(sampleTags, requiredTags []string) bool {
	// Create a map of sample tags for O(1) lookup
	tagMap := make(map[string]bool, len(sampleTags))
	for _, tag := range sampleTags {
		tagMap[tag] = true
	}

	// Check if all required tags are present
	for _, requiredTag := range requiredTags {
		if !tagMap[requiredTag] {
			return false
		}
	}

	return true
}
