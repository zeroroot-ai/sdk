// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"log/slog"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ExtractDiscovery attempts to extract a DiscoveryResult from a proto message.
// It handles three cases:
//  1. The message is a DiscoveryResult itself - return it directly
//  2. The message has a field named "discovery" - extract and return it (compiled types)
//  3. The message has field number 100 - extract and return it (dynamic messages)
//
// Returns nil if no discovery data is found.
//
// This function is used by the harness to automatically extract discovery data
// from tool responses and persist it to the knowledge graph.
func ExtractDiscovery(msg proto.Message) *graphragpb.DiscoveryResult {
	if msg == nil {
		return nil
	}

	// Case 1: Check if the message itself is a DiscoveryResult
	if discovery, ok := msg.(*graphragpb.DiscoveryResult); ok {
		return discovery
	}

	// Case 2: Try reflection-based extraction (handles both compiled and dynamic messages)
	return ExtractDiscoveryReflect(msg)
}

// ExtractDiscoveryReflect uses protobuf reflection to extract DiscoveryResult from field 100.
// This works with both compiled proto types and dynamicpb.Message instances.
//
// It tries two approaches:
//  1. Look for field by name "discovery" (compiled types with named fields)
//  2. Look for field by number 100 (standard discovery field number)
//
// Returns nil if no discovery data is found or if the field has the wrong type.
func ExtractDiscoveryReflect(msg proto.Message) *graphragpb.DiscoveryResult {
	if msg == nil {
		return nil
	}

	refl := msg.ProtoReflect()
	descriptor := refl.Descriptor()
	fields := descriptor.Fields()

	var discoveryField protoreflect.FieldDescriptor

	// First try to find by name (works for compiled types)
	discoveryField = fields.ByName("discovery")

	// If not found by name, try field number 100 (standard for all tool responses)
	if discoveryField == nil {
		discoveryField = fields.ByNumber(100)
	}

	// No discovery field found
	if discoveryField == nil {
		return nil
	}

	// Check if the field is set
	if !refl.Has(discoveryField) {
		return nil
	}

	// The field must be a message type
	if discoveryField.Kind() != protoreflect.MessageKind {
		slog.Warn("discovery field (100) exists but is not a message type",
			"message_type", string(descriptor.FullName()),
			"field_kind", discoveryField.Kind().String())
		return nil
	}

	// Get the field value
	fieldValue := refl.Get(discoveryField)

	// Extract the message interface
	discoveryMsg := fieldValue.Message().Interface()

	// Try type assertion to *graphragpb.DiscoveryResult (compiled type)
	if discovery, ok := discoveryMsg.(*graphragpb.DiscoveryResult); ok {
		slog.Debug("extracted discovery data from message",
			"message_type", string(descriptor.FullName()),
			"extraction_method", "compiled_type_assertion")
		return discovery
	}

	// For dynamic messages, we need to marshal/unmarshal to convert
	// from dynamicpb.Message to *graphragpb.DiscoveryResult
	discoveryBytes, err := proto.Marshal(discoveryMsg)
	if err != nil {
		slog.Warn("failed to marshal discovery field for conversion",
			"message_type", string(descriptor.FullName()),
			"error", err)
		return nil
	}

	var result graphragpb.DiscoveryResult
	if err := proto.Unmarshal(discoveryBytes, &result); err != nil {
		slog.Warn("failed to unmarshal discovery field to DiscoveryResult",
			"message_type", string(descriptor.FullName()),
			"error", err)
		return nil
	}

	slog.Debug("extracted discovery data from dynamic message",
		"message_type", string(descriptor.FullName()),
		"extraction_method", "marshal_unmarshal")

	return &result
}
