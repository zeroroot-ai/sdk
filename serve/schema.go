// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/zeroroot-ai/sdk/tool"
)

// SchemaProvider is an optional interface that tools can implement to provide
// prototype instances of their input and output messages. This enables reliable
// schema extraction without depending on the global proto registry.
type SchemaProvider interface {
	// InputProto returns a prototype instance of the input message.
	InputProto() proto.Message

	// OutputProto returns a prototype instance of the output message.
	OutputProto() proto.Message
}

// ExtractFileDescriptorSet attempts to extract a FileDescriptorSet containing
// the proto definitions for a tool's input and output message types.
//
// This function uses multiple strategies to find the proto schema:
// 1. If the tool implements SchemaProvider, use the provided proto instances
// 2. Try to find message types in protoregistry.GlobalTypes (best effort)
// 3. Try to find message types by scanning protoregistry.GlobalFiles (best effort)
//
// This is an optional feature that enables schema introspection and validation
// in the daemon.
//
// Returns:
//   - ([]byte, nil): Successfully extracted and marshaled FileDescriptorSet
//   - (nil, nil): Tool doesn't implement SchemaProvider and fallback strategies failed (normal case)
//   - (nil, error): Extraction failed unexpectedly (e.g., SchemaProvider returned invalid protos, marshaling failed)
//
// Example usage:
//
//	fdsBytes, err := ExtractFileDescriptorSet(myTool)
//	if err != nil {
//	    log.Warn("schema extraction failed", "error", err)
//	}
//	if fdsBytes != nil {
//	    metadata["file_descriptor_set"] = base64.StdEncoding.EncodeToString(fdsBytes)
//	}
func ExtractFileDescriptorSet(t tool.Tool) ([]byte, error) {
	inputType := t.InputMessageType()
	outputType := t.OutputMessageType()

	if inputType == "" || outputType == "" {
		// Tool doesn't specify message types, can't extract schema
		return nil, nil
	}

	// Track unique file descriptors to avoid duplicates
	fileDescriptors := make(map[string]*descriptorpb.FileDescriptorProto)

	// Strategy 1: If tool implements SchemaProvider, use the provided protos
	if sp, ok := t.(SchemaProvider); ok {
		// Extract input proto schema
		inputProto := sp.InputProto()
		if inputProto == nil {
			return nil, fmt.Errorf("SchemaProvider.InputProto() returned nil for tool %q", t.Name())
		}
		if err := extractFileDescriptorFromMessage(inputProto, fileDescriptors); err != nil {
			return nil, fmt.Errorf("failed to extract input schema from SchemaProvider for tool %q: %w", t.Name(), err)
		}

		// Extract output proto schema
		outputProto := sp.OutputProto()
		if outputProto == nil {
			return nil, fmt.Errorf("SchemaProvider.OutputProto() returned nil for tool %q", t.Name())
		}
		if err := extractFileDescriptorFromMessage(outputProto, fileDescriptors); err != nil {
			return nil, fmt.Errorf("failed to extract output schema from SchemaProvider for tool %q: %w", t.Name(), err)
		}

		// Build and marshal the FileDescriptorSet
		fds := buildFileDescriptorSet(fileDescriptors)
		if fds == nil {
			return nil, fmt.Errorf("SchemaProvider extraction succeeded but buildFileDescriptorSet returned nil for tool %q", t.Name())
		}

		fdsBytes, err := proto.Marshal(fds)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal FileDescriptorSet for tool %q: %w", t.Name(), err)
		}

		return fdsBytes, nil
	}

	// Tool doesn't implement SchemaProvider - this is normal/expected
	// The GlobalTypes/GlobalFiles fallbacks below are best-effort only
	// If they fail, we return (nil, nil) because it's not an error condition

	// Strategy 2: Try GlobalTypes registry (best effort)
	if err := extractFileDescriptorFromGlobalTypes(inputType, fileDescriptors); err == nil {
		if inputType != outputType {
			if err := extractFileDescriptorFromGlobalTypes(outputType, fileDescriptors); err == nil {
				fds := buildFileDescriptorSet(fileDescriptors)
				if fds != nil {
					if fdsBytes, err := proto.Marshal(fds); err == nil {
						return fdsBytes, nil
					}
				}
			}
		} else {
			fds := buildFileDescriptorSet(fileDescriptors)
			if fds != nil {
				if fdsBytes, err := proto.Marshal(fds); err == nil {
					return fdsBytes, nil
				}
			}
		}
	}

	// Reset for strategy 3
	fileDescriptors = make(map[string]*descriptorpb.FileDescriptorProto)

	// Strategy 3: Scan GlobalFiles registry for the message types (best effort)
	if err := extractFileDescriptorFromGlobalFiles(inputType, fileDescriptors); err == nil {
		if inputType != outputType {
			if err := extractFileDescriptorFromGlobalFiles(outputType, fileDescriptors); err == nil {
				fds := buildFileDescriptorSet(fileDescriptors)
				if fds != nil {
					if fdsBytes, err := proto.Marshal(fds); err == nil {
						return fdsBytes, nil
					}
				}
			}
		} else {
			fds := buildFileDescriptorSet(fileDescriptors)
			if fds != nil {
				if fdsBytes, err := proto.Marshal(fds); err == nil {
					return fdsBytes, nil
				}
			}
		}
	}

	// No schema available through any strategy - this is normal, not an error
	return nil, nil
}

// buildFileDescriptorSet creates a FileDescriptorSet from collected file descriptors.
func buildFileDescriptorSet(fileDescriptors map[string]*descriptorpb.FileDescriptorProto) *descriptorpb.FileDescriptorSet {
	if len(fileDescriptors) == 0 {
		return nil
	}

	fds := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(fileDescriptors)),
	}
	for _, fd := range fileDescriptors {
		fds.File = append(fds.File, fd)
	}

	return fds
}

// extractFileDescriptorFromMessage extracts file descriptors from a proto message instance.
func extractFileDescriptorFromMessage(msg proto.Message, collected map[string]*descriptorpb.FileDescriptorProto) error {
	// Get the message descriptor through reflection
	msgDesc := msg.ProtoReflect().Descriptor()
	if msgDesc == nil {
		return errors.New("no descriptor for message")
	}

	// Get the file descriptor
	fileDesc := msgDesc.ParentFile()
	if fileDesc == nil {
		return errors.New("no file descriptor for message")
	}

	return collectFileDescriptor(fileDesc, collected)
}

// extractFileDescriptorFromGlobalTypes extracts file descriptors using GlobalTypes registry.
func extractFileDescriptorFromGlobalTypes(messageType string, collected map[string]*descriptorpb.FileDescriptorProto) error {
	// Find the message type in the global types registry
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(messageType))
	if err != nil {
		return fmt.Errorf("message type %q not found in GlobalTypes: %w", messageType, err)
	}

	// Get the file descriptor for this message
	fileDesc := msgType.Descriptor().ParentFile()
	if fileDesc == nil {
		return fmt.Errorf("no file descriptor for message type %q", messageType)
	}

	return collectFileDescriptor(fileDesc, collected)
}

// extractFileDescriptorFromGlobalFiles extracts file descriptors by scanning GlobalFiles registry.
func extractFileDescriptorFromGlobalFiles(messageType string, collected map[string]*descriptorpb.FileDescriptorProto) error {
	fullName := protoreflect.FullName(messageType)

	// Extract package name from message type (e.g., "gibson.tools.mytool-c.HttpxRequest" -> "gibson.tools.mytool-c")
	packageName := fullName.Parent()
	if packageName == "" {
		return fmt.Errorf("cannot determine package from message type %q", messageType)
	}

	// Try to find the file by package name patterns
	var foundFileDesc protoreflect.FileDescriptor

	// Iterate through all registered files to find one containing this message
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		// Check if the file's package matches
		if fd.Package() == packageName {
			// Check if this file contains our message type
			messages := fd.Messages()
			msgName := fullName.Name()
			for i := 0; i < messages.Len(); i++ {
				if messages.Get(i).Name() == msgName {
					foundFileDesc = fd
					return false // Stop iteration
				}
			}
		}
		return true // Continue iteration
	})

	if foundFileDesc == nil {
		// Try alternative: look for file by guessing common naming patterns
		// e.g., "gibson.tools.mytool-c" -> "mytool-c.proto" or "gibson/tools/mytool-c/mytool-c.proto"
		possiblePaths := []string{
			strings.ReplaceAll(string(packageName), ".", "/") + ".proto",
			string(fullName.Name()) + ".proto",
		}

		for _, path := range possiblePaths {
			if fd, err := protoregistry.GlobalFiles.FindFileByPath(path); err == nil {
				foundFileDesc = fd
				break
			}
		}

		// Last resort: try common tool proto naming pattern
		parts := strings.Split(string(packageName), ".")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if fd, err := protoregistry.GlobalFiles.FindFileByPath(lastPart + ".proto"); err == nil {
				foundFileDesc = fd
			}
		}
	}

	if foundFileDesc == nil {
		return fmt.Errorf("file descriptor for message type %q not found in GlobalFiles", messageType)
	}

	return collectFileDescriptor(foundFileDesc, collected)
}

// collectFileDescriptor adds a file descriptor and its dependencies to the collection.
func collectFileDescriptor(fileDesc protoreflect.FileDescriptor, collected map[string]*descriptorpb.FileDescriptorProto) error {
	// Get the file path (unique identifier for this file)
	filePath := fileDesc.Path()

	// Skip if already collected
	if _, exists := collected[filePath]; exists {
		return nil
	}

	// Convert FileDescriptor to FileDescriptorProto
	fdProto := protodesc.ToFileDescriptorProto(fileDesc)
	collected[filePath] = fdProto

	// Recursively collect dependencies
	imports := fileDesc.Imports()
	for i := 0; i < imports.Len(); i++ {
		importDesc := imports.Get(i).FileDescriptor
		if importDesc != nil {
			importPath := importDesc.Path()
			if _, exists := collected[importPath]; !exists {
				importProto := protodesc.ToFileDescriptorProto(importDesc)
				collected[importPath] = importProto
			}
		}
	}

	return nil
}
