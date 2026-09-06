// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package protoresolver provides utilities for resolving and creating dynamic proto messages
// from FileDescriptorSet when compiled proto types aren't available in GlobalTypes.
package protoresolver

import (
	"encoding/base64"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// DynamicMessageFactory creates dynamicpb.Message instances from FileDescriptorSet
// when compiled proto types aren't available in GlobalTypes.
type DynamicMessageFactory interface {
	// CreateMessage creates a dynamic proto message from a base64-encoded FileDescriptorSet
	// and a fully-qualified type name (e.g., "gibson.tools.mytool-c.HttpxRequest").
	CreateMessage(fdsBase64 string, typeName string) (proto.Message, error)

	// CreateMessageFromFiles creates a dynamic proto message from an existing file registry
	// and a fully-qualified type name.
	CreateMessageFromFiles(files *protoregistry.Files, typeName string) (proto.Message, error)
}

// defaultDynamicFactory is the default implementation of DynamicMessageFactory.
type defaultDynamicFactory struct{}

// NewDynamicMessageFactory creates a new DynamicMessageFactory instance.
func NewDynamicMessageFactory() DynamicMessageFactory {
	return &defaultDynamicFactory{}
}

// CreateMessage creates a dynamic proto message from a base64-encoded FileDescriptorSet.
func (f *defaultDynamicFactory) CreateMessage(fdsBase64 string, typeName string) (proto.Message, error) {
	// Decode base64 string to bytes
	fdsBytes, err := base64.StdEncoding.DecodeString(fdsBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 file_descriptor_set: %w", err)
	}

	// Unmarshal bytes to FileDescriptorSet
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(fdsBytes, &fds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file_descriptor_set: %w", err)
	}

	// Create file registry from descriptor set
	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("failed to create file registry from descriptor set: %w", err)
	}

	// Use CreateMessageFromFiles to complete the creation
	return f.CreateMessageFromFiles(files, typeName)
}

// CreateMessageFromFiles creates a dynamic proto message from an existing file registry.
func (f *defaultDynamicFactory) CreateMessageFromFiles(files *protoregistry.Files, typeName string) (proto.Message, error) {
	// Find descriptor by name
	descriptor, err := files.FindDescriptorByName(protoreflect.FullName(typeName))
	if err != nil {
		return nil, fmt.Errorf("failed to find type %s in file descriptor set: %w", typeName, err)
	}

	// Assert it's a MessageDescriptor
	messageDescriptor, ok := descriptor.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("type %s is not a message descriptor (found: %T)", typeName, descriptor)
	}

	// Create and return dynamic message
	return dynamicpb.NewMessage(messageDescriptor), nil
}
