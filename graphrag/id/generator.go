// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package id

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// Generator creates deterministic IDs for graph nodes.
// IDs are content-addressable and derived from node type and identifying properties.
type Generator interface {
	// Generate creates a deterministic ID from node type and properties.
	// The ID format is: {nodeType}:{base64url(sha256(canonical_properties)[:12])}
	//
	// Returns an error if:
	//   - The node type is not registered
	//   - Required identifying properties are missing
	//
	// The same node type and properties will always produce the same ID.
	//
	// Example:
	//   id, err := gen.Generate("host", map[string]any{"ip": "10.0.0.1"})
	//   // id = "host:ABC123xyz789"
	Generate(nodeType string, properties map[string]any) (string, error)
}

// DeterministicGenerator implements Generator using SHA-256 hashing.
// It produces stable, content-addressable IDs based on node type and identifying properties.
//
// ID Generation Algorithm:
//  1. Get identifying properties from registry for the node type
//  2. Validate all identifying properties are present
//  3. Build canonical string: nodeType:prop1=val1|prop2=val2 (sorted keys)
//  4. Normalize values: strings to lowercase/trimmed, ints as sprintf, complex as JSON
//  5. SHA-256 hash the canonical string
//  6. Base64url encode first 12 bytes (no padding)
//  7. Return {nodeType}:{encoded}
//
// This ensures:
//   - Same input always produces same output (deterministic)
//   - Different inputs produce different outputs (collision-resistant)
//   - IDs are stable across agent runs and missions
//   - IDs are human-readable (contain node type prefix)
type DeterministicGenerator struct {
	registry graphrag.NodeTypeRegistry
}

// NewGenerator creates a new DeterministicGenerator with the given registry.
// The registry is used to determine which properties identify each node type.
//
// Example:
//
//	registry := graphrag.NewDefaultNodeTypeRegistry()
//	gen := id.NewGenerator(registry)
func NewGenerator(registry graphrag.NodeTypeRegistry) *DeterministicGenerator {
	return &DeterministicGenerator{
		registry: registry,
	}
}

// Generate creates a deterministic ID from node type and properties.
func (g *DeterministicGenerator) Generate(nodeType string, properties map[string]any) (string, error) {
	// Step 1: Get identifying properties from registry
	identifyingProps, err := g.registry.GetIdentifyingProperties(nodeType)
	if err != nil {
		return "", fmt.Errorf("failed to get identifying properties for node type %q: %w", nodeType, err)
	}

	// Step 2: Validate all identifying properties are present
	missing, err := g.registry.ValidateProperties(nodeType, properties)
	if err != nil {
		return "", fmt.Errorf("validation failed for node type %q: %w (missing: %v)", nodeType, err, missing)
	}

	// Step 3: Build canonical string with sorted keys
	canonical, err := g.buildCanonicalString(nodeType, identifyingProps, properties)
	if err != nil {
		return "", fmt.Errorf("failed to build canonical string for node type %q: %w", nodeType, err)
	}

	// Step 4: SHA-256 hash the canonical string
	hash := sha256.Sum256([]byte(canonical))

	// Step 5: Base64url encode first 12 bytes (96 bits)
	encoded := base64.RawURLEncoding.EncodeToString(hash[:12])

	// Step 6: Return formatted ID
	return fmt.Sprintf("%s:%s", nodeType, encoded), nil
}

// buildCanonicalString creates a canonical string representation of the identifying properties.
// Format: nodeType:prop1=val1|prop2=val2|... (properties sorted by key)
func (g *DeterministicGenerator) buildCanonicalString(nodeType string, identifyingProps []string, properties map[string]any) (string, error) {
	// Sort property names for consistent ordering
	sortedProps := make([]string, len(identifyingProps))
	copy(sortedProps, identifyingProps)
	sort.Strings(sortedProps)

	// Build property pairs
	pairs := make([]string, 0, len(sortedProps))
	for _, prop := range sortedProps {
		val := properties[prop]

		// Normalize the value
		normalized, err := g.normalizeValue(val)
		if err != nil {
			return "", fmt.Errorf("failed to normalize property %q with value %v: %w", prop, val, err)
		}

		pairs = append(pairs, fmt.Sprintf("%s=%s", prop, normalized))
	}

	// Join with pipe separator
	return fmt.Sprintf("%s:%s", nodeType, strings.Join(pairs, "|")), nil
}

// normalizeValue converts a property value to its canonical string representation.
// Normalization rules:
//   - string: lowercase and trimmed
//   - int/int64/int32/int16/int8: sprintf "%d"
//   - uint/uint64/uint32/uint16/uint8: sprintf "%d"
//   - float64/float32: sprintf "%.6f" (6 decimal places)
//   - bool: "true" or "false"
//   - nil: "null"
//   - complex types (maps, slices, structs): JSON marshal
func (g *DeterministicGenerator) normalizeValue(val any) (string, error) {
	if val == nil {
		return "null", nil
	}

	switch v := val.(type) {
	case string:
		// String: lowercase and trim whitespace
		return strings.ToLower(strings.TrimSpace(v)), nil

	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.Itoa(int(v)), nil
	case int16:
		return strconv.Itoa(int(v)), nil
	case int32:
		return strconv.Itoa(int(v)), nil
	case int64:
		return strconv.FormatInt(v, 10), nil

	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil

	case float32:
		return fmt.Sprintf("%.6f", v), nil
	case float64:
		return fmt.Sprintf("%.6f", v), nil

	case bool:
		if v {
			return "true", nil
		}
		return "false", nil

	default:
		// Complex types: marshal to JSON for stable representation
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal complex value to JSON: %w", err)
		}
		return string(jsonBytes), nil
	}
}
