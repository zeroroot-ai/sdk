// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// roundTrip exercises the proto-binary and protojson round-trip contracts
// on a single message instance.
//
// It asserts:
//
//  1. proto.Marshal → proto.Unmarshal preserves the message (proto.Equal).
//  2. protojson.Marshal → protojson.Unmarshal preserves the message (proto.Equal).
//
// If either round-trip diverges, that is a wire-format regression that will
// silently corrupt data crossing a network boundary (daemon ↔ SDK consumer,
// daemon → dashboard, etc.). Use this helper from every <pkg>_contract_test.go
// file in this package.
func roundTrip(t *testing.T, m proto.Message) {
	t.Helper()

	// proto-binary round-trip.
	binWire, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	binRoundTripped := m.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(binWire, binRoundTripped); err != nil {
		t.Fatalf("proto.Unmarshal: %v", err)
	}
	if !proto.Equal(m, binRoundTripped) {
		t.Errorf("proto-binary round-trip diverged\n  in:  %v\n  out: %v", m, binRoundTripped)
	}

	// protojson round-trip.
	jsonWire, err := protojson.Marshal(m)
	if err != nil {
		t.Fatalf("protojson.Marshal: %v", err)
	}
	jsonRoundTripped := m.ProtoReflect().New().Interface()
	if err := protojson.Unmarshal(jsonWire, jsonRoundTripped); err != nil {
		t.Fatalf("protojson.Unmarshal: %v\n  wire: %s", err, string(jsonWire))
	}
	if !proto.Equal(m, jsonRoundTripped) {
		t.Errorf("protojson round-trip diverged\n  in:   %v\n  out:  %v\n  wire: %s", m, jsonRoundTripped, string(jsonWire))
	}
}

// roundTripPackage round-trips every exported top-level message and every
// nested message within the named proto package (e.g. "gibson.admin.v1")
// across all .proto files belonging to that package. Each message gets a
// sub-test named after its fully-qualified proto name.
//
// Messages are constructed in zero form via protoregistry.GlobalTypes and
// fed through the same roundTrip helper that per-message populated fixtures
// use, so the assertions are identical.
//
// If a package defines no messages, the helper fails with a clear error —
// silently passing on "the import didn't pull anything in" would defeat
// the purpose.
func roundTripPackage(t *testing.T, pkg string) {
	t.Helper()

	var descriptors []protoreflect.MessageDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if string(fd.Package()) != pkg {
			return true
		}
		collectMessages(fd.Messages(), &descriptors)
		return true
	})

	if len(descriptors) == 0 {
		t.Fatalf("roundTripPackage(%q): no messages registered; missing blank import?", pkg)
	}

	for _, desc := range descriptors {
		// Sub-test name = message name with the package prefix stripped (the
		// package is already implicit in the parent test).
		name := strings.TrimPrefix(string(desc.FullName()), pkg+".")
		t.Run(name, func(t *testing.T) {
			mt, err := protoregistry.GlobalTypes.FindMessageByName(desc.FullName())
			if err != nil {
				t.Fatalf("FindMessageByName(%q): %v", desc.FullName(), err)
			}
			roundTrip(t, mt.New().Interface())
		})
	}
}

func collectMessages(msgs protoreflect.MessageDescriptors, out *[]protoreflect.MessageDescriptor) {
	for i := 0; i < msgs.Len(); i++ {
		msg := msgs.Get(i)
		// Skip MapEntry synthetic messages — protoreflect surfaces them as
		// nested messages but they aren't real wire-format containers.
		if msg.IsMapEntry() {
			continue
		}
		*out = append(*out, msg)
		collectMessages(msg.Messages(), out)
	}
}
