// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"encoding/base64"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestNewDynamicMessageFactory(t *testing.T) {
	factory := NewDynamicMessageFactory()
	if factory == nil {
		t.Fatal("NewDynamicMessageFactory returned nil")
	}
}

func TestCreateMessage_InvalidBase64(t *testing.T) {
	factory := NewDynamicMessageFactory()
	_, err := factory.CreateMessage("invalid-base64!!!", "some.Type")
	if err == nil {
		t.Fatal("Expected error for invalid base64, got nil")
	}
	// Just verify we get a base64 decode error
	expectedPrefix := "failed to decode base64 file_descriptor_set:"
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Expected error starting with %q, got: %v", expectedPrefix, err)
	}
}

func TestCreateMessage_InvalidProto(t *testing.T) {
	factory := NewDynamicMessageFactory()
	invalidProto := base64.StdEncoding.EncodeToString([]byte("not a valid proto"))
	_, err := factory.CreateMessage(invalidProto, "some.Type")
	if err == nil {
		t.Fatal("Expected error for invalid proto, got nil")
	}
}

func TestCreateMessage_EmptyFileDescriptorSet(t *testing.T) {
	factory := NewDynamicMessageFactory()

	// Create an empty FileDescriptorSet
	emptyFds := &descriptorpb.FileDescriptorSet{}
	fdsBytes, err := proto.Marshal(emptyFds)
	if err != nil {
		t.Fatalf("Failed to marshal empty FileDescriptorSet: %v", err)
	}

	fdsBase64 := base64.StdEncoding.EncodeToString(fdsBytes)

	_, err = factory.CreateMessage(fdsBase64, "nonexistent.Type")
	if err == nil {
		t.Fatal("Expected error for type not found, got nil")
	}
}

func TestCreateMessageFromFiles_TypeNotFound(t *testing.T) {
	factory := NewDynamicMessageFactory()

	// Create an empty FileDescriptorSet and convert to Files
	emptyFds := &descriptorpb.FileDescriptorSet{}
	fdsBytes, err := proto.Marshal(emptyFds)
	if err != nil {
		t.Fatalf("Failed to marshal empty FileDescriptorSet: %v", err)
	}

	fdsBase64 := base64.StdEncoding.EncodeToString(fdsBytes)

	_, err = factory.CreateMessage(fdsBase64, "does.not.Exist")
	if err == nil {
		t.Fatal("Expected error when type not found, got nil")
	}
}
