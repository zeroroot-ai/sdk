// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

// mission_v1_contract_test.go — wire-format round-trip contract for
// gibson.mission.v1. This is the first application of the per-package
// contract-test pattern documented in doc.go.
//
// What this file does:
//
//   - TestMissionV1_AllMessagesRoundTrip walks the file descriptor for
//     gibson/mission/v1/mission_definition.proto and round-trips every
//     top-level message in its zero form.
//
//   - TestMissionDefinition_RoundTripPopulated round-trips a richly-
//     populated MissionDefinition. Treat this as the reference for adding
//     per-message populated fixtures in future contract-test files.
//
// What this file deliberately does NOT do (follow-up issues):
//
//   - Exercise buf.validate runtime constraints. mission/v1 carries
//     buf.validate annotations on a handful of fields; verifying that a
//     known-bad input is rejected by protovalidate-go (and a known-good
//     accepted) requires adding the bufbuild/protovalidate-go dependency
//     and is out of scope for the pattern-establishment PR.
//
//   - Field-numbering stability (golden snapshots). `buf breaking` already
//     covers this at proto-level; adding a Go-side gate is a future
//     extension.

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/timestamppb"

	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

// TestMissionV1_AllMessagesRoundTrip walks every top-level message defined
// in gibson/mission/v1/mission_definition.proto and asserts the zero-value
// instance round-trips through proto-binary and protojson without loss.
//
// Sub-tests are named after the message so failures point directly at the
// regressing type.
func TestMissionV1_AllMessagesRoundTrip(t *testing.T) {
	const path = "gibson/mission/v1/mission_definition.proto"
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		t.Fatalf("could not find %s in protoregistry.GlobalFiles: %v", path, err)
	}

	msgs := fd.Messages()
	if msgs.Len() == 0 {
		t.Fatalf("expected gibson.mission.v1 to define at least one top-level message")
	}

	for i := 0; i < msgs.Len(); i++ {
		desc := msgs.Get(i)
		t.Run(string(desc.Name()), func(t *testing.T) {
			roundTrip(t, newZeroMessage(t, desc))
		})
	}
}

// newZeroMessage returns a fresh zero-value instance of the given message
// type. It uses protoregistry.GlobalTypes so the test exercises the actual
// generated Go type, not a dynamic message.
func newZeroMessage(t *testing.T, desc protoreflect.MessageDescriptor) protoreflect.ProtoMessage {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(desc.FullName())
	if err != nil {
		t.Fatalf("FindMessageByName(%q): %v", desc.FullName(), err)
	}
	return mt.New().Interface()
}

// TestMissionDefinition_RoundTripPopulated round-trips a richly-populated
// MissionDefinition. This serves as the reference fixture for adding per-
// message populated round-trips to other <pkg>_contract_test.go files.
//
// The fixture intentionally exercises:
//
//   - Scalars (id, name, description, version, target_ref, source)
//   - Repeated strings (entry_points, exit_points)
//   - Maps (nodes by id, metadata)
//   - Nested messages (MissionDependencies, MissionNode + AgentNodeConfig oneof,
//     WorkspaceConfig + RepositoryConfig)
//   - Well-known types (google.protobuf.Timestamp)
//   - A MissionEdge (proves the edge sub-message round-trips inside the parent)
//
// If this test fails after a generated-code regen, the regen has broken the
// wire format for one of the listed pieces — fix the proto change, do not
// loosen this test.
func TestMissionDefinition_RoundTripPopulated(t *testing.T) {
	m := &missionpb.MissionDefinition{
		Id:          "mission-def-01",
		Name:        "network-scan",
		Description: "Scan a /24 for open ports and known vulns.",
		Version:     "1.2.3",
		TargetRef:   "target-12345",
		Source:      "git@github.com:example/missions.git",
		Nodes: map[string]*missionpb.MissionNode{
			"agent-1": {
				Id:   "agent-1",
				Type: missionpb.NodeType_NODE_TYPE_AGENT,
				Config: &missionpb.MissionNode_AgentConfig{
					AgentConfig: &missionpb.AgentNodeConfig{
						AgentName:        "scanner",
						MaxTokensPerCall: int32Ptr(8192),
					},
				},
			},
		},
		Edges: []*missionpb.MissionEdge{
			{From: "agent-1", To: "agent-2"},
		},
		EntryPoints: []string{"agent-1"},
		ExitPoints:  []string{"agent-2"},
		Metadata: map[string]string{
			"created-by": "contract-test",
			"category":   "network",
		},
		Dependencies: &missionpb.MissionDependencies{
			Agents:  []string{"scanner@1.0"},
			Tools:   []string{"nmap@7.x"},
			Plugins: []string{"trivy"},
		},
		// Fixed timestamps so the test is deterministic; instant is irrelevant
		// — round-trip preservation is what matters.
		InstalledAt: &timestamppb.Timestamp{Seconds: 1778889600},
		CreatedAt:   &timestamppb.Timestamp{Seconds: 1778889600},
		Workspace: &missionpb.WorkspaceConfig{
			Repositories: []*missionpb.RepositoryConfig{
				{
					Name:   "example",
					Url:    "git@github.com:example/repo.git",
					Branch: "main",
				},
			},
		},
	}
	roundTrip(t, m)
}

func int32Ptr(v int32) *int32 { return &v }
