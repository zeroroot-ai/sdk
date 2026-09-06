// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Command mission-docs-gen reads a Buf FileDescriptorSet and emits
// MDX + JSON documentation for the mission DSL surface:
//
//	verbs.mdx        — every decision action (proto-derived where annotated)
//	nouns.mdx        — every NodeType enum value with its config message
//	schema-ref.mdx   — exhaustive field reference for every mission proto message
//	templates.mdx    — placeholder index of templates shipped from the ADK
//	glossary.json    — controlled-vocabulary term → definition map
//
// Verbs are sourced from the Decision-shaped action enum in the
// daemon, but since the generator only sees the SDK proto tree we
// emit a static placeholder list driven by the spec's locked verb
// table. The active surface is the orchestrator's act.go switch;
// keep this list in sync with mission-verb-noun-registry's
// conformance gate.
//
// Spec: mission-authoring-cue Requirement 8.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	missionPkg     = "gibson.mission.v1"
	missionRoot    = "gibson.mission.v1.MissionDefinition"
	missionNode    = "gibson.mission.v1.MissionNode"
	missionEnumNT  = "gibson.mission.v1.NodeType"
	mergeStrategyT = "gibson.mission.v1.MergeStrategy"
)

// verbCatalog mirrors the locked verb table from
// docs/mission-platform-audit.md. Each entry: action name, status,
// short summary, payload notes. Source: spec
// mission-verb-noun-registry's audit table.
type verbEntry struct {
	Action  string
	Status  string
	Summary string
	Payload string
}

var verbCatalog = []verbEntry{
	{"execute_agent", "LIVE", "Run an agent component against the configured task.", "TargetNodeID."},
	{"skip_agent", "LIVE", "Mark an agent node skipped without running it.", "TargetNodeID."},
	{"modify_params", "LIVE", "Override task parameters before delegating to execute_agent.", "TargetNodeID + Modifications map."},
	{"retry", "LIVE", "Retry a failed agent node under its retry policy.", "TargetNodeID."},
	{"spawn_agent", "LIVE", "Dynamically spawn a new agent node and link it into the DAG.", "SpawnConfig (AgentName, Description, TaskConfig, DependsOn)."},
	{"complete", "LIVE", "Mark the mission complete and stop the orchestration loop.", "StopReason."},
	{"request_approval", "LIVE", "Pause the mission for human approval; resume on response.", "ApprovalContext, ApprovalTimeout, TimeoutAction."},
	{"abort", "LIVE", "Mark the mission aborted with a documented severity and cleanup hint.", "AbortReason, AbortSeverity, CleanupRequired."},
	{"escalate", "LIVE", "Escalate to a human, senior agent, or specialist.", "EscalationLevel, EscalationUrgency, EscalationContext."},
	{"rollback", "LIVE", "Restore mission state from an explicit checkpoint or pre-node snapshot.", "CheckpointID or RollbackToNode."},
	{"reflect", "LIVE", "Run a reflection pass over the mission's decisions or a specific node.", "ReflectionScope, ReflectionPrompt."},
	{"recall", "LIVE", "Query mission/long-term memory and optionally inject results into context.", "RecallQuery, RecallMemoryTier, RecallFilters, InjectIntoContext."},
}

func main() {
	inputPath := flag.String("input", "", "Buf FileDescriptorSet path")
	outputDir := flag.String("output", "", "Directory to write MDX + glossary outputs")
	flag.Parse()
	if *inputPath == "" || *outputDir == "" {
		log.Fatal("both -input and -output are required")
	}
	if err := run(*inputPath, *outputDir); err != nil {
		log.Fatalf("mission-docs-gen: %v", err)
	}
}

func run(inputPath, outputDir string) error {
	fdsBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read FDS: %w", err)
	}
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(fdsBytes, fds); err != nil {
		return fmt.Errorf("unmarshal FDS: %w", err)
	}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return fmt.Errorf("build descriptor index: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	if err := emitVerbs(filepath.Join(outputDir, "verbs.mdx")); err != nil {
		return fmt.Errorf("verbs: %w", err)
	}
	if err := emitNouns(files, filepath.Join(outputDir, "nouns.mdx")); err != nil {
		return fmt.Errorf("nouns: %w", err)
	}
	if err := emitSchemaRef(files, filepath.Join(outputDir, "schema-ref.mdx")); err != nil {
		return fmt.Errorf("schema-ref: %w", err)
	}
	if err := emitTemplates(filepath.Join(outputDir, "templates.mdx")); err != nil {
		return fmt.Errorf("templates: %w", err)
	}
	if err := emitGlossary(files, filepath.Join(outputDir, "glossary.json")); err != nil {
		return fmt.Errorf("glossary: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// verbs.mdx
// ---------------------------------------------------------------------------

func emitVerbs(path string) error {
	var sb strings.Builder
	sb.WriteString("---\ntitle: Mission Verbs\ndescription: Decision actions the orchestrator can take.\n---\n\n")
	sb.WriteString("# Mission Verbs\n\n")
	sb.WriteString("Every iteration of the orchestrator's THINK→ACT cycle ends in one ")
	sb.WriteString("of these decision actions. The set is closed; new verbs land via the ")
	sb.WriteString("controlled-extension contract documented in `mission-verb-noun-registry`.\n\n")

	sb.WriteString("| Action | Status | Summary | Payload |\n")
	sb.WriteString("|---|---|---|---|\n")
	for _, e := range verbCatalog {
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", e.Action, e.Status, e.Summary, e.Payload))
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// ---------------------------------------------------------------------------
// nouns.mdx
// ---------------------------------------------------------------------------

func emitNouns(files *protoregistry.Files, path string) error {
	var sb strings.Builder
	sb.WriteString("---\ntitle: Mission Nouns\ndescription: Node types in the mission DAG.\n---\n\n")
	sb.WriteString("# Mission Nouns (`NodeType`)\n\n")
	sb.WriteString("Each mission node carries a `NodeType` enum value selecting its ")
	sb.WriteString("execution semantics. The matching `*NodeConfig` variant in the ")
	sb.WriteString("`MissionNode.config` oneof carries the per-noun parameters.\n\n")

	ntDesc, err := files.FindDescriptorByName(protoreflect.FullName(missionEnumNT))
	if err != nil {
		return err
	}
	nt, ok := ntDesc.(protoreflect.EnumDescriptor)
	if !ok {
		return fmt.Errorf("%s is not an enum", missionEnumNT)
	}

	for i := 0; i < nt.Values().Len(); i++ {
		v := nt.Values().Get(i)
		name := string(v.Name())
		if strings.HasSuffix(name, "_UNSPECIFIED") {
			continue
		}
		comment := commentFor(v)

		sb.WriteString(fmt.Sprintf("## `%s`\n\n", name))
		if comment != "" {
			sb.WriteString(comment + "\n\n")
		}
		sb.WriteString(fmt.Sprintf("**Enum value:** `%d` &middot; ", v.Number()))
		sb.WriteString(fmt.Sprintf("**Config message:** `%s`\n\n", configMessageNameFor(name)))
	}

	// MergeStrategy is paired with JOIN. Document inline.
	if msDesc, err := files.FindDescriptorByName(protoreflect.FullName(mergeStrategyT)); err == nil {
		if ms, ok := msDesc.(protoreflect.EnumDescriptor); ok {
			sb.WriteString("## MergeStrategy (JOIN)\n\n")
			if c := commentFor(ms); c != "" {
				sb.WriteString(c + "\n\n")
			}
			sb.WriteString("| Value | Number | Description |\n|---|---|---|\n")
			for i := 0; i < ms.Values().Len(); i++ {
				v := ms.Values().Get(i)
				sb.WriteString(fmt.Sprintf("| `%s` | `%d` | %s |\n", v.Name(), v.Number(), commentFor(v)))
			}
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func configMessageNameFor(nodeType string) string {
	// NODE_TYPE_AGENT → AgentNodeConfig
	suffix := strings.TrimPrefix(nodeType, "NODE_TYPE_")
	parts := strings.Split(strings.ToLower(suffix), "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "") + "NodeConfig"
}

// ---------------------------------------------------------------------------
// schema-ref.mdx
// ---------------------------------------------------------------------------

func emitSchemaRef(files *protoregistry.Files, path string) error {
	var sb strings.Builder
	sb.WriteString("---\ntitle: Schema Reference\ndescription: Every field of every mission-related message.\n---\n\n")
	sb.WriteString("# Mission Schema Reference\n\n")
	sb.WriteString("Generated from the canonical proto at ")
	sb.WriteString("`gibson/mission/v1/mission_definition.proto` and ")
	sb.WriteString("`gibson/daemon/v1/daemon.proto`.\n\n")

	type msgEntry struct {
		full string
		md   protoreflect.MessageDescriptor
	}
	var msgs []msgEntry

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if pkg != missionPkg && pkg != "gibson.daemon.v1" {
			return true
		}
		for i := 0; i < fd.Messages().Len(); i++ {
			md := fd.Messages().Get(i)
			full := string(md.FullName())
			if pkg == "gibson.daemon.v1" && full != "gibson.daemon.v1.MissionConstraints" {
				continue
			}
			msgs = append(msgs, msgEntry{full, md})
		}
		return true
	})

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].full < msgs[j].full })

	for _, m := range msgs {
		sb.WriteString(fmt.Sprintf("## `%s`\n\n", m.full))
		if c := commentFor(m.md); c != "" {
			sb.WriteString(c + "\n\n")
		}
		sb.WriteString("| Field | Type | Tag | Description |\n|---|---|---|---|\n")
		for i := 0; i < m.md.Fields().Len(); i++ {
			f := m.md.Fields().Get(i)
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %d | %s |\n",
				f.Name(),
				fieldTypeString(f),
				f.Number(),
				oneLine(commentFor(f)),
			))
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func fieldTypeString(f protoreflect.FieldDescriptor) string {
	prefix := ""
	if f.IsList() {
		prefix = "repeated "
	}
	if f.IsMap() {
		k := scalarKindName(f.MapKey().Kind())
		v := scalarOrMessageKindName(f.MapValue())
		return fmt.Sprintf("map<%s, %s>", k, v)
	}
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return prefix + string(f.Message().FullName())
	case protoreflect.EnumKind:
		return prefix + string(f.Enum().FullName())
	default:
		return prefix + scalarKindName(f.Kind())
	}
}

func scalarKindName(k protoreflect.Kind) string {
	switch k {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "bytes"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float"
	case protoreflect.DoubleKind:
		return "double"
	}
	return k.String()
}

func scalarOrMessageKindName(f protoreflect.FieldDescriptor) string {
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return string(f.Message().FullName())
	case protoreflect.EnumKind:
		return string(f.Enum().FullName())
	default:
		return scalarKindName(f.Kind())
	}
}

// ---------------------------------------------------------------------------
// templates.mdx
// ---------------------------------------------------------------------------

func emitTemplates(path string) error {
	body := `---
title: Mission Templates
description: Reusable mission templates shipped by the ADK.
---

# Mission Templates

Templates live under ` + "`opensource/adk/templates/`" + `. Each template
ships as three artifacts:

- ` + "`template.cue`" + ` — authoring source (consumed by ` + "`gibson mission new`" + `).
- ` + "`template.json`" + ` — proto-shaped JSON export (consumed by the dashboard).
- ` + "`template.mdx`" + ` — handwritten description (rendered here).

Catalog index is generated at bundle-build time and committed to the
ADK repo. This page is regenerated whenever the bundle is rebuilt.

The v1 template set covers ` + "`recon`, `webapp-scan`, `secrets-audit`, " +
		"`compliance-check`" + ` — see the per-template detail pages once those
templates land.
`
	return os.WriteFile(path, []byte(body), 0o644)
}

// ---------------------------------------------------------------------------
// glossary.json
// ---------------------------------------------------------------------------

func emitGlossary(files *protoregistry.Files, path string) error {
	out := map[string]string{}

	// Verb names.
	for _, v := range verbCatalog {
		out["verb."+v.Action] = v.Summary
	}

	// Enum values + message names from the mission package.
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if string(fd.Package()) != missionPkg && string(fd.Package()) != "gibson.daemon.v1" {
			return true
		}
		for i := 0; i < fd.Enums().Len(); i++ {
			ed := fd.Enums().Get(i)
			for j := 0; j < ed.Values().Len(); j++ {
				v := ed.Values().Get(j)
				key := string(v.Name())
				if c := commentFor(v); c != "" {
					out[key] = c
				}
			}
		}
		for i := 0; i < fd.Messages().Len(); i++ {
			md := fd.Messages().Get(i)
			key := string(md.Name())
			if c := commentFor(md); c != "" {
				out[key] = c
			}
		}
		return true
	})

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func commentFor(d protoreflect.Descriptor) string {
	loc := d.ParentFile().SourceLocations().ByDescriptor(d)
	c := strings.TrimSpace(loc.LeadingComments)
	if c == "" {
		return ""
	}
	lines := strings.Split(c, "\n")
	for i, l := range lines {
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(l, "//")
		l = strings.TrimSpace(l)
		lines[i] = l
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func oneLine(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}
