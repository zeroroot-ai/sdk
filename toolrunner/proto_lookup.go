// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolrunner

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// protoTypeFor returns the protoreflect.MessageType for a fully-qualified
// proto message type name. Returns a clear error if the type is not
// registered with the global proto type registry.
func protoTypeFor(name string) (protoreflect.MessageType, error) {
	if name == "" {
		return nil, errors.New("empty proto message type name")
	}
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		return nil, fmt.Errorf("proto type %q not registered: %w (ensure the generated proto package is imported)", name, err)
	}
	return mt, nil
}

// Compile-time guard: imports must result in proto.Marshaler being satisfied
// by anything we hand back. This is a no-op assertion that prevents the
// "imported and not used" foot-gun if the file is edited.
var _ proto.Message = (protoreflect.ProtoMessage)(nil)
