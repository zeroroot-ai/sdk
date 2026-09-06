// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"log/slog"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// setDiscoveryField sets proto field 100 (DiscoveryResult) on a tool response
// message using protobuf reflection. This is the reverse of the SDK's
// graphrag.ExtractDiscoveryReflect() which reads field 100.
//
// Returns true if field 100 was successfully set, false if the message doesn't
// have a field 100 or the field type is incompatible.
func setDiscoveryField(msg proto.Message, discovery *graphragpb.DiscoveryResult) bool {
	if msg == nil || discovery == nil {
		return false
	}

	refl := msg.ProtoReflect()
	descriptor := refl.Descriptor()
	fields := descriptor.Fields()

	// Look for field 100 (the standard discovery field number).
	var discoveryField protoreflect.FieldDescriptor
	discoveryField = fields.ByNumber(100)
	if discoveryField == nil {
		// Try by name as fallback.
		discoveryField = fields.ByName("discovery")
	}

	if discoveryField == nil {
		slog.Debug("response proto has no field 100 or 'discovery' field",
			"message_type", string(descriptor.FullName()))
		return false
	}

	if discoveryField.Kind() != protoreflect.MessageKind {
		slog.Warn("field 100 exists but is not a message type",
			"message_type", string(descriptor.FullName()),
			"field_kind", discoveryField.Kind().String())
		return false
	}

	// Marshal the DiscoveryResult to bytes, then unmarshal into the target
	// field's message type. This handles both compiled and dynamic messages.
	discoveryBytes, err := proto.Marshal(discovery)
	if err != nil {
		slog.Warn("failed to marshal DiscoveryResult for field 100",
			"error", err)
		return false
	}

	// Get the target field's message and unmarshal into it.
	targetMsg := refl.Mutable(discoveryField).Message().Interface()
	if err := proto.Unmarshal(discoveryBytes, targetMsg); err != nil {
		slog.Warn("failed to unmarshal DiscoveryResult into field 100",
			"message_type", string(descriptor.FullName()),
			"error", err)
		return false
	}

	return true
}
