// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

// field_100_contract_test.go — platform-wide proto invariant gate.
//
// Field 100 is reserved across all Gibson tool response messages for
// gibson.graphrag.v1.DiscoveryResult. See:
//   - workspace CLAUDE.md, "Cross-repo rules (CI-enforced; do not violate)"
//   - api/proto/DISCOVERY_RESULT.md, "Reserved Field Numbers"
//
// The runtime in graphrag.ExtractDiscoveryReflect (extract.go) depends on
// this contract. A violation would silently break automatic discovery
// extraction for the offending tool response.
//
// This test enforces the invariant at `go test ./...` time so the next
// proto author to slip a non-DiscoveryResult field into slot 100 sees a
// failing test before merge — not a silent regression at runtime.

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	toolv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/tool/v1"

	// Force-import every SDK proto package so its FileDescriptor registers
	// with protoregistry.GlobalFiles. Without these blank imports,
	// GlobalFiles would be empty for a test binary that does not otherwise
	// reference them. Mirrors the same pattern in auth/registry/coverage_test.go.
	// gibson/admin/v1, gibson/authz/v1, gibson/usage/v1,
	// gibson/daemon/discovery/v1, and gibson/budget/v1 moved to
	// platform-sdk under slices #108 and #106; their field-100
	// invariant is enforced by an equivalent test in platform-sdk
	// against its own descriptors. The customer-visible
	// gibson/budget_status/v1 (BudgetExceeded + BudgetScope) stays
	// in OSS.
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/agent/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/auth/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/budget_status/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/graph/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/identity/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/manifest/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/tool/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

const (
	// discoveryResultFullName is the fully-qualified name of the platform
	// standard discovery container documented in DISCOVERY_RESULT.md.
	discoveryResultFullName protoreflect.FullName = "gibson.graphrag.v1.DiscoveryResult"

	// discoveryFieldNumber is the platform-wide reserved field number.
	discoveryFieldNumber protoreflect.FieldNumber = 100
)

// checkField100Compliance walks msgs (and their nested messages, recursively)
// and returns a violation string for every message that defines field 100
// with a type other than gibson.graphrag.v1.DiscoveryResult. Empty result
// means compliant.
func checkField100Compliance(msgs protoreflect.MessageDescriptors) []string {
	var violations []string
	walkField100(msgs, &violations)
	return violations
}

func walkField100(msgs protoreflect.MessageDescriptors, violations *[]string) {
	for i := 0; i < msgs.Len(); i++ {
		msg := msgs.Get(i)
		if v := field100Violation(msg); v != "" {
			*violations = append(*violations, v)
		}
		walkField100(msg.Messages(), violations)
	}
}

func field100Violation(msg protoreflect.MessageDescriptor) string {
	f := msg.Fields().ByNumber(discoveryFieldNumber)
	if f == nil {
		return ""
	}
	if f.Kind() != protoreflect.MessageKind || f.Message() == nil {
		return fmt.Sprintf("%s: field 100 has kind %s; field 100 is reserved for %s",
			msg.FullName(), f.Kind(), discoveryResultFullName)
	}
	if f.Message().FullName() != discoveryResultFullName {
		return fmt.Sprintf("%s: field 100 is %s; field 100 is reserved for %s",
			msg.FullName(), f.Message().FullName(), discoveryResultFullName)
	}
	return ""
}

// ===== Walker unit tests =====

// TestField100Walker_CompliantFixture exercises the walker against
// gibson.test.v1.GenericDiscoveryResponse — the canonical fixture that
// preserves the field-100 contract — and asserts no violation is reported.
func TestField100Walker_CompliantFixture(t *testing.T) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath("gibson/test/v1/fixture.proto")
	if err != nil {
		t.Fatalf("fixture.proto not in registry (forgot a blank import?): %v", err)
	}
	if v := checkField100Compliance(fd.Messages()); len(v) != 0 {
		t.Fatalf("expected canonical fixture to be compliant, got violations: %v", v)
	}
}

// TestField100Walker_NoFieldAt100 asserts the walker is silent when no
// message declares field 100.
func TestField100Walker_NoFieldAt100(t *testing.T) {
	fd := buildSyntheticDescriptor(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("contract_no_field.proto"),
		Package: proto.String("contract_no_field"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("NoDiscovery"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("a"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	})
	if v := checkField100Compliance(fd.Messages()); len(v) != 0 {
		t.Fatalf("expected no violation for message without field 100, got: %v", v)
	}
}

// TestField100Walker_PrimitiveAt100 asserts the walker reports a violation
// when a message uses field 100 for a primitive (non-message) type.
func TestField100Walker_PrimitiveAt100(t *testing.T) {
	fd := buildSyntheticDescriptor(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("contract_primitive.proto"),
		Package: proto.String("contract_primitive"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("BadResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("wrong_field"),
						Number: proto.Int32(100),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	})
	violations := checkField100Compliance(fd.Messages())
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "BadResponse") {
		t.Errorf("violation should reference the offending message name: %s", violations[0])
	}
	if !strings.Contains(violations[0], string(discoveryResultFullName)) {
		t.Errorf("violation should reference the expected type: %s", violations[0])
	}
}

// TestField100Walker_WrongMessageAt100 asserts a violation is reported when
// field 100 IS a message but of the wrong type (not DiscoveryResult).
func TestField100Walker_WrongMessageAt100(t *testing.T) {
	fd := buildSyntheticDescriptor(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("contract_wrong_message.proto"),
		Package: proto.String("contract_wrong_message"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Imposter"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("x"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Response"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("not_real_discovery"),
						Number:   proto.Int32(100),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".contract_wrong_message.Imposter"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	})
	violations := checkField100Compliance(fd.Messages())
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "Response") {
		t.Errorf("violation should reference the offending message name: %s", violations[0])
	}
	if !strings.Contains(violations[0], "contract_wrong_message.Imposter") {
		t.Errorf("violation should reference the actual wrong type: %s", violations[0])
	}
}

// TestField100Walker_NestedMessage asserts the walker recurses into nested
// message definitions and flags violations inside them too.
func TestField100Walker_NestedMessage(t *testing.T) {
	fd := buildSyntheticDescriptor(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("contract_nested.proto"),
		Package: proto.String("contract_nested"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Outer"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("Inner"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("evil"),
								Number: proto.Int32(100),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							},
						},
					},
				},
			},
		},
	})
	violations := checkField100Compliance(fd.Messages())
	if len(violations) != 1 {
		t.Fatalf("expected nested violation to be detected, got %d violations: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "Outer.Inner") {
		t.Errorf("violation should reference the nested type Outer.Inner: %s", violations[0])
	}
}

func buildSyntheticDescriptor(t *testing.T, fdp *descriptorpb.FileDescriptorProto) protoreflect.FileDescriptor {
	t.Helper()
	fd, err := protodesc.NewFile(fdp, &protoregistry.Files{})
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd
}

// ===== Integration test against the live SDK proto registry =====

// TestField100Contract_SDKProtoRegistry enforces the platform-wide invariant
// across every gibson.* proto package registered with protoregistry.GlobalFiles
// (via the blank imports at the top of this file).
//
// If this test fails, a message in a Gibson proto package has declared field
// 100 with a type other than gibson.graphrag.v1.DiscoveryResult — fix the
// proto, do not loosen this test.
func TestField100Contract_SDKProtoRegistry(t *testing.T) {
	var violations []string
	seenPackages := make(map[string]bool)

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !strings.HasPrefix(pkg, "gibson.") {
			return true
		}
		seenPackages[pkg] = true
		violations = append(violations, checkField100Compliance(fd.Messages())...)
		return true
	})

	// Sanity check: if these well-known packages are not loaded, the test would
	// pass vacuously. Fail loud rather than silent.
	requiredPackages := []string{
		"gibson.tool.v1",
		"gibson.daemon.v1",
		"gibson.manifest.v1",
		"gibson.mission.v1",
		"gibson.test.v1",
		"gibson.graphrag.v1",
		"gibson.plugin.v1",
	}
	for _, p := range requiredPackages {
		if !seenPackages[p] {
			t.Fatalf("proto package %q is not loaded in protoregistry.GlobalFiles. Add a blank import for it in field_100_contract_test.go.", p)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("field 100 contract violated by %d message(s):\n  %s\n\nField 100 is reserved platform-wide for %s. See api/proto/DISCOVERY_RESULT.md.",
			len(violations), strings.Join(violations, "\n  "), discoveryResultFullName)
	}
}

// isToolResponse reports whether the given message carries the
// (gibson.tool.v1.is_tool_response) option set to true.
func isToolResponse(msg protoreflect.MessageDescriptor) bool {
	opts, _ := msg.Options().(*descriptorpb.MessageOptions)
	if opts == nil {
		return false
	}
	if !proto.HasExtension(opts, toolv1.E_IsToolResponse) {
		return false
	}
	v, _ := proto.GetExtension(opts, toolv1.E_IsToolResponse).(bool)
	return v
}

// TestToolResponse_HasDiscoveryResultAtField100 tightens the field-100
// invariant for messages that have explicitly opted in via the
// (gibson.tool.v1.is_tool_response) annotation. Where the broad
// TestField100Contract_SDKProtoRegistry check above ensures no message
// accidentally puts something else at field 100, this test ensures every
// annotated tool response actually HAS DiscoveryResult at field 100 —
// not absent and not some other type.
//
// This catches the failure mode "I added a new tool response and forgot
// to wire in the discovery slot," which the broad check (focused on
// preventing accidental use) does not.
func TestToolResponse_HasDiscoveryResultAtField100(t *testing.T) {
	var (
		violations  []string
		annotated   int
		seenFixture bool
	)

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "gibson.") {
			return true
		}
		walkToolResponses(fd.Messages(), &violations, &annotated, &seenFixture)
		return true
	})

	// Sanity: the canonical fixture (gibson.test.v1.GenericDiscoveryResponse)
	// is annotated. If we don't see it, the registry isn't loaded — fail
	// loudly so the test never passes vacuously.
	if !seenFixture {
		t.Fatalf("gibson.test.v1.GenericDiscoveryResponse not observed; the test/v1 package isn't loaded in protoregistry.GlobalFiles")
	}

	if annotated == 0 {
		t.Fatalf("no messages carry the (gibson.tool.v1.is_tool_response) annotation; the fixture should at minimum")
	}

	if len(violations) > 0 {
		t.Fatalf("%d tool-response message(s) missing DiscoveryResult at field 100:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

func walkToolResponses(msgs protoreflect.MessageDescriptors, violations *[]string, annotated *int, seenFixture *bool) {
	for i := 0; i < msgs.Len(); i++ {
		msg := msgs.Get(i)
		if isToolResponse(msg) {
			*annotated++
			if msg.FullName() == "gibson.test.v1.GenericDiscoveryResponse" {
				*seenFixture = true
			}
			f := msg.Fields().ByNumber(discoveryFieldNumber)
			switch {
			case f == nil:
				*violations = append(*violations, fmt.Sprintf("%s: marked is_tool_response but has no field at %d",
					msg.FullName(), discoveryFieldNumber))
			case f.Kind() != protoreflect.MessageKind || f.Message() == nil:
				*violations = append(*violations, fmt.Sprintf("%s: field %d is kind %s, expected message %s",
					msg.FullName(), discoveryFieldNumber, f.Kind(), discoveryResultFullName))
			case f.Message().FullName() != discoveryResultFullName:
				*violations = append(*violations, fmt.Sprintf("%s: field %d is %s, expected %s",
					msg.FullName(), discoveryFieldNumber, f.Message().FullName(), discoveryResultFullName))
			}
		}
		walkToolResponses(msg.Messages(), violations, annotated, seenFixture)
	}
}
