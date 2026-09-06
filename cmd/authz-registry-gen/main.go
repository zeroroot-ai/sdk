// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Command authz-registry-gen walks every gRPC method in a Gibson
// proto FileDescriptorSet, extracts the (gibson.auth.v1.authz)
// extension, and emits three artifacts:
//
//   - registry.go      — a typed Go map[string]Entry consumed by the
//     Gibson daemon's startup self-check
//   - registry.yaml    — a YAML rendering consumed by ext-authz at
//     startup
//   - permissions.ts   — a TypeScript constants table for the
//     dashboard to gate UI controls
//
// All three artifacts are written under the SDK's auth/registry/
// directory and are committed alongside the rest of api/gen so
// downstream consumers (gibson, ext-authz, dashboard, adk) pin the
// SDK at a known artifact set.
//
// The OpenFGA authorization model is hand-maintained at
// gibson/internal/authz/model.fga; this generator no longer emits an
// FGA coverage stub (the registry.yaml + audit.csv pair already
// expose every annotated rule).
//
// Invocation modes:
//
//  1. Make-driven (preferred): `make proto` runs `buf build` to
//     produce a FileDescriptorSet image and then invokes this tool
//     with -input pointing at the image. This avoids the buf plugin
//     protocol's per-file invocation pattern (which complicates
//     registry consolidation across files).
//
//  2. Buf plugin mode (legacy/optional): when invoked with no flags,
//     reads a CodeGeneratorRequest from stdin (the protoc plugin
//     protocol). Useful for buf-driven workflows; emits the same
//     artifacts. Not the recommended path because buf calls the
//     plugin once per input file, producing duplicate-file warnings.
//
// Spec: unified-identity-and-authorization Requirements 7.4, 7.5.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	authv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/auth/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// ObjectDeriverPattern is the allowlist regex for the object_deriver field.
// Valid derivers:
//   - tenant_from_identity
//   - system_tenant
//   - from_field('<name>')        where <name> matches [a-zA-Z_]+
//   - tenant_and_field('<name>')  where <name> matches [a-zA-Z_]+
//
// This pattern is shared with coverage_test.go to keep both in sync.
// Spec: zero-trust-hardening Requirement 10.1.
const ObjectDeriverPattern = `^(tenant_from_identity|system_tenant|from_field\('[a-zA-Z_]+'\)|tenant_and_field\('[a-zA-Z_]+'\))$`

var objectDeriverRe = regexp.MustCompile(ObjectDeriverPattern)

// ValidIdentityBitsMask is the bitmask covering all known IdentityClass bits.
// Bits outside this mask are unknown and must be rejected.
// Spec: zero-trust-hardening Requirement 10.2.
const ValidIdentityBitsMask = int32(0xF) // USER|SERVICE|COMPONENT|PLATFORM_OPERATOR = 1|2|4|8

// validateObjectDeriver rejects object_deriver strings that do not match the
// documented deriver grammar. Unknown derivers cause ext-authz to refuse to
// start; catching them at codegen time prevents a bad SDK release from bricking
// the gateway.
// Spec: zero-trust-hardening Requirement 10.1.
func validateObjectDeriver(s string) error {
	if !objectDeriverRe.MatchString(s) {
		return fmt.Errorf("object_deriver %q does not match allowlist pattern %s", s, ObjectDeriverPattern)
	}
	return nil
}

// validateIdentityBits rejects allowed_identities values that contain bits
// outside the known IdentityClass mask (0xF). Unknown bits indicate a typo or
// an unapproved extension to the identity model.
// Spec: zero-trust-hardening Requirement 10.2.
func validateIdentityBits(bits int32) error {
	if bits&^ValidIdentityBitsMask != 0 {
		return fmt.Errorf("allowed_identities 0x%x contains bits outside the valid mask 0x%x (USER|SERVICE|COMPONENT|PLATFORM_OPERATOR)", uint32(bits), uint32(ValidIdentityBitsMask))
	}
	return nil
}

// detectDuplicateMethodKeys reports an error if any two entries share the same
// method key (/<pkg>.<svc>/<method>). Duplicate keys arise when the same proto
// service is compiled into multiple files or when a rename collision occurs;
// they would silently overwrite each other in the generated Go map.
// Spec: zero-trust-hardening Requirement 10.3.
func detectDuplicateMethodKeys(entries []Entry) error {
	// entries is already sorted by Method; consecutive equal keys are duplicates.
	for i := 1; i < len(entries); i++ {
		if entries[i].Method == entries[i-1].Method {
			return fmt.Errorf("duplicate method key %q: appears in at least two source locations; check for copy-paste or rename collision", entries[i].Method)
		}
	}
	return nil
}

func main() {
	var (
		inputPath = flag.String("input", "", "FileDescriptorSet image (from `buf build -o`); when empty, read CodeGeneratorRequest from stdin")
		outputDir = flag.String("output", "auth/registry", "directory to write registry.go, registry.yaml, permissions.ts")
	)
	flag.Parse()

	if *inputPath != "" {
		if err := runImage(*inputPath, *outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "authz-registry-gen: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "authz-registry-gen: %v\n", err)
		os.Exit(1)
	}
}

// runImage is the Make-driven invocation: read a FileDescriptorSet
// from inputPath, generate artifacts, write to outputDir.
func runImage(inputPath, outputDir string) error {
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inputPath, err)
	}
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(raw, fds); err != nil {
		return fmt.Errorf("unmarshal FileDescriptorSet: %w", err)
	}
	// Synthesize a CodeGeneratorRequest so we can reuse collectEntries.
	req := &pluginpb.CodeGeneratorRequest{ProtoFile: fds.GetFile()}
	entries, missing, err := collectEntries(req)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("the following RPCs are missing the (gibson.auth.v1.authz) annotation:\n  - %s", strings.Join(missing, "\n  - "))
	}
	// Req 10.3: reject duplicate method keys.
	if err := detectDuplicateMethodKeys(entries); err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outputDir, err)
	}

	for _, f := range []*pluginpb.CodeGeneratorResponse_File{
		emitGo(entries),
		emitYAML(entries),
		emitTS(entries),
	} {
		// f.Name is "auth/registry/<base>"; we want only the base
		// when -output is supplied.
		base := filepath.Base(f.GetName())
		out := filepath.Join(outputDir, base)
		if err := os.WriteFile(out, []byte(f.GetContent()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
	}
	return nil
}

func run(in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(raw, req); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}

	entries, missing, err := collectEntries(req)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		// Fail-closed: every RPC MUST be annotated. The buf-lint
		// plugin (1.10) catches this earlier in CI; we re-enforce
		// here so the SDK's own `make proto` cannot regress the
		// invariant.
		sort.Strings(missing)
		return fmt.Errorf("the following RPCs are missing the (gibson.auth.v1.authz) annotation:\n  - %s", strings.Join(missing, "\n  - "))
	}
	// Req 10.3: reject duplicate method keys.
	if err := detectDuplicateMethodKeys(entries); err != nil {
		return err
	}

	files := []*pluginpb.CodeGeneratorResponse_File{
		emitGo(entries),
		emitYAML(entries),
		emitTS(entries),
	}

	resp := &pluginpb.CodeGeneratorResponse{
		File:              files,
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if _, err := out.Write(body); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}

// Entry is the plugin's normalized representation of one method's auth
// rule. It is the source from which all four output formats are
// generated.
type Entry struct {
	Method            string   // /<package>.<Service>/<Method>
	Service           string   // <package>.<Service>
	Relation          string   // empty if Unauthenticated or Self
	ObjectType        string   // empty if Unauthenticated or Self
	ObjectDeriver     string   // empty if Unauthenticated or Self
	AllowedIdentities []string // names from IdentityClass enum (without prefix)
	Unauthenticated   bool
	Self              bool // self-mode-authz: authenticated user reading own data, no FGA Check
}

func collectEntries(req *pluginpb.CodeGeneratorRequest) ([]Entry, []string, error) {
	var entries []Entry
	var missing []string

	// We process every file in protos_files (the full transitive set),
	// not just files_to_generate, so that downstream consumers'
	// invocations that limit generation scope still produce a complete
	// registry.
	files := make([]*descriptorpb.FileDescriptorProto, 0, len(req.GetProtoFile()))
	files = append(files, req.GetProtoFile()...)

	for _, f := range files {
		pkg := f.GetPackage()
		// Only annotate Gibson-owned services; google/* and
		// taxonomy/* are excluded from the registry.
		if !strings.HasPrefix(pkg, "gibson.") && pkg != "intelligence.v1" {
			continue
		}
		for _, svc := range f.GetService() {
			svcFQN := pkg + "." + svc.GetName()
			for _, m := range svc.GetMethod() {
				method := "/" + svcFQN + "/" + m.GetName()
				opts := m.GetOptions()
				if opts == nil {
					missing = append(missing, method)
					continue
				}
				ext := proto.GetExtension(opts, authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil {
					missing = append(missing, method)
					continue
				}
				if !ao.Unauthenticated && !ao.Self && ao.Relation == "" && ao.ObjectType == "" && ao.ObjectDeriver == "" && ao.AllowedIdentities == 0 {
					// All-zero options is treated as missing.
					missing = append(missing, method)
					continue
				}
				if ao.Unauthenticated && (ao.Relation != "" || ao.ObjectType != "" || ao.ObjectDeriver != "" || ao.AllowedIdentities != 0) {
					return nil, nil, fmt.Errorf("%s: unauthenticated:true cannot be combined with relation/object/identities", method)
				}
				// self-mode-authz: mutual exclusivity and required-field validators.
				if ao.Self && ao.Unauthenticated {
					return nil, nil, fmt.Errorf("%s: self and unauthenticated are mutually exclusive (self-mode-authz)", method)
				}
				if ao.Self && (ao.Relation != "" || ao.ObjectType != "" || ao.ObjectDeriver != "") {
					return nil, nil, fmt.Errorf("%s: self mode is incompatible with relation/object_type/object_deriver (self-mode-authz)", method)
				}
				if ao.Self && ao.AllowedIdentities == 0 {
					return nil, nil, fmt.Errorf("%s: allowed_identities is required when self: true (self-mode-authz)", method)
				}
				if ao.Self {
					// Req 10.2: reject unknown identity bits on self-mode entries too.
					if err := validateIdentityBits(ao.AllowedIdentities); err != nil {
						return nil, nil, fmt.Errorf("%s: %w", method, err)
					}
					entries = append(entries, Entry{
						Method:            method,
						Service:           svcFQN,
						AllowedIdentities: identityNames(ao.AllowedIdentities),
						Self:              true,
					})
					continue
				}
				if !ao.Unauthenticated {
					if ao.Relation == "" || ao.ObjectType == "" || ao.ObjectDeriver == "" || ao.AllowedIdentities == 0 {
						return nil, nil, fmt.Errorf("%s: relation, object_type, object_deriver, allowed_identities all required when not unauthenticated", method)
					}
					// Req 10.1: reject unknown object_deriver values.
					if err := validateObjectDeriver(ao.ObjectDeriver); err != nil {
						return nil, nil, fmt.Errorf("%s: %w", method, err)
					}
					// Req 10.2: reject unknown identity bits.
					if err := validateIdentityBits(ao.AllowedIdentities); err != nil {
						return nil, nil, fmt.Errorf("%s: %w", method, err)
					}
				}
				entries = append(entries, Entry{
					Method:            method,
					Service:           svcFQN,
					Relation:          ao.Relation,
					ObjectType:        ao.ObjectType,
					ObjectDeriver:     ao.ObjectDeriver,
					AllowedIdentities: identityNames(ao.AllowedIdentities),
					Unauthenticated:   ao.Unauthenticated,
				})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Method < entries[j].Method })
	return entries, missing, nil
}

func identityNames(bits int32) []string {
	out := []string{}
	for _, c := range []struct {
		bit  int32
		name string
	}{
		{int32(authv1.IdentityClass_IDENTITY_CLASS_USER), "USER"},
		{int32(authv1.IdentityClass_IDENTITY_CLASS_SERVICE), "SERVICE"},
		{int32(authv1.IdentityClass_IDENTITY_CLASS_COMPONENT), "COMPONENT"},
		{int32(authv1.IdentityClass_IDENTITY_CLASS_PLATFORM_OPERATOR), "PLATFORM_OPERATOR"},
	} {
		if bits&c.bit != 0 {
			out = append(out, c.name)
		}
	}
	return out
}

// emitGo produces the typed Go map. Path is relative to the buf out
// directory (which the buf.gen.yaml entry sets to the SDK root, so
// the file lands at auth/registry/registry.go).
func emitGo(entries []Entry) *pluginpb.CodeGeneratorResponse_File {
	var b bytes.Buffer
	fmt.Fprint(&b, `// Code generated by authz-registry-gen. DO NOT EDIT.
//
// Source: gibson/<service>/<file>.proto (gibson.auth.v1.authz annotations)
// Spec: unified-identity-and-authorization Requirement 7.

package registry

// IdentityClass mirrors gibson.auth.v1.IdentityClass as a bitfield.
type IdentityClass uint8

const (
	IdentityUser             IdentityClass = 1
	IdentityService          IdentityClass = 2
	IdentityComponent        IdentityClass = 4
	IdentityPlatformOperator IdentityClass = 8
)

// Has reports whether c contains every bit set in want.
func (c IdentityClass) Has(want IdentityClass) bool { return c&want == want }

// Entry is the per-method authorization rule.
type Entry struct {
	Method            string
	Service           string
	Relation          string
	ObjectType        string
	ObjectDeriver     string
	AllowedIdentities IdentityClass
	Unauthenticated   bool
	Self              bool // self-mode-authz: authenticated user reading own data, no FGA Check
}

// Registry is the complete, sorted set of (method -> auth rule)
// mappings extracted from the SDK's proto annotations at codegen time.
var Registry = map[string]Entry{
`)
	for _, e := range entries {
		fmt.Fprintf(&b, "\t%q: {\n", e.Method)
		fmt.Fprintf(&b, "\t\tMethod:            %q,\n", e.Method)
		fmt.Fprintf(&b, "\t\tService:           %q,\n", e.Service)
		fmt.Fprintf(&b, "\t\tRelation:          %q,\n", e.Relation)
		fmt.Fprintf(&b, "\t\tObjectType:        %q,\n", e.ObjectType)
		fmt.Fprintf(&b, "\t\tObjectDeriver:     %q,\n", e.ObjectDeriver)
		fmt.Fprintf(&b, "\t\tAllowedIdentities: %s,\n", goIdentityExpr(e.AllowedIdentities))
		fmt.Fprintf(&b, "\t\tUnauthenticated:   %v,\n", e.Unauthenticated)
		fmt.Fprintf(&b, "\t\tSelf:              %v,\n", e.Self)
		fmt.Fprintf(&b, "\t},\n")
	}
	fmt.Fprintln(&b, "}")
	// Pipe the assembled source through go/format so the committed
	// artifact is always gofmt-stable, regardless of alignment in the
	// literals above. A failure here means the generator emitted
	// invalid Go — fail loudly rather than write a broken artifact.
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		panic(fmt.Sprintf("authz-registry-gen: generated registry.go does not parse: %v", err))
	}
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("auth/registry/registry.go"),
		Content: proto.String(string(formatted)),
	}
}

func goIdentityExpr(names []string) string {
	if len(names) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(names))
	for _, n := range names {
		switch n {
		case "USER":
			parts = append(parts, "IdentityUser")
		case "SERVICE":
			parts = append(parts, "IdentityService")
		case "COMPONENT":
			parts = append(parts, "IdentityComponent")
		case "PLATFORM_OPERATOR":
			parts = append(parts, "IdentityPlatformOperator")
		}
	}
	return strings.Join(parts, " | ")
}

func emitYAML(entries []Entry) *pluginpb.CodeGeneratorResponse_File {
	var b bytes.Buffer
	fmt.Fprintln(&b, "# Code generated by authz-registry-gen. DO NOT EDIT.")
	fmt.Fprintln(&b, "# Spec: unified-identity-and-authorization Requirement 7.4.")
	fmt.Fprintln(&b, "entries:")
	for _, e := range entries {
		fmt.Fprintf(&b, "  %q:\n", e.Method)
		if e.Unauthenticated {
			fmt.Fprintln(&b, "    unauthenticated: true")
			continue
		}
		if e.Self {
			fmt.Fprintln(&b, "    self: true")
			fmt.Fprintf(&b, "    allowed_identities:\n")
			for _, id := range e.AllowedIdentities {
				fmt.Fprintf(&b, "      - %s\n", id)
			}
			continue
		}
		fmt.Fprintf(&b, "    relation: %q\n", e.Relation)
		fmt.Fprintf(&b, "    object_type: %q\n", e.ObjectType)
		fmt.Fprintf(&b, "    object_deriver: %q\n", e.ObjectDeriver)
		fmt.Fprintf(&b, "    allowed_identities:\n")
		for _, id := range e.AllowedIdentities {
			fmt.Fprintf(&b, "      - %s\n", id)
		}
	}
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("auth/registry/registry.yaml"),
		Content: proto.String(b.String()),
	}
}

func emitTS(entries []Entry) *pluginpb.CodeGeneratorResponse_File {
	var b bytes.Buffer
	fmt.Fprintln(&b, "// Code generated by authz-registry-gen. DO NOT EDIT.")
	fmt.Fprintln(&b, "// Spec: unified-identity-and-authorization Requirement 7.4.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "export const IdentityClass = {")
	fmt.Fprintln(&b, "  USER: 1,")
	fmt.Fprintln(&b, "  SERVICE: 2,")
	fmt.Fprintln(&b, "  COMPONENT: 4,")
	fmt.Fprintln(&b, "  PLATFORM_OPERATOR: 8,")
	fmt.Fprintln(&b, "} as const")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "export interface AuthEntry {")
	fmt.Fprintln(&b, "  method: string")
	fmt.Fprintln(&b, "  service: string")
	fmt.Fprintln(&b, "  relation: string")
	fmt.Fprintln(&b, "  objectType: string")
	fmt.Fprintln(&b, "  objectDeriver: string")
	fmt.Fprintln(&b, "  allowedIdentities: number")
	fmt.Fprintln(&b, "  unauthenticated: boolean")
	fmt.Fprintln(&b, "  self: boolean // self-mode-authz: authenticated user reading own data, no FGA Check")
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "export const AuthRegistry: Record<string, AuthEntry> = {")
	for _, e := range entries {
		fmt.Fprintf(&b, "  %q: {\n", e.Method)
		fmt.Fprintf(&b, "    method: %q,\n", e.Method)
		fmt.Fprintf(&b, "    service: %q,\n", e.Service)
		fmt.Fprintf(&b, "    relation: %q,\n", e.Relation)
		fmt.Fprintf(&b, "    objectType: %q,\n", e.ObjectType)
		fmt.Fprintf(&b, "    objectDeriver: %q,\n", e.ObjectDeriver)
		fmt.Fprintf(&b, "    allowedIdentities: %s,\n", tsIdentityExpr(e.AllowedIdentities))
		fmt.Fprintf(&b, "    unauthenticated: %v,\n", e.Unauthenticated)
		fmt.Fprintf(&b, "    self: %v,\n", e.Self)
		fmt.Fprintln(&b, "  },")
	}
	fmt.Fprintln(&b, "}")
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("auth/registry/permissions.ts"),
		Content: proto.String(b.String()),
	}
}

func tsIdentityExpr(names []string) string {
	if len(names) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, "IdentityClass."+n)
	}
	return strings.Join(parts, " | ")
}

// Ensure protoreflect import is retained — used by future editors who
// extend collectEntries to use higher-level reflection. Removing it
// requires a code change that re-validates the descriptor walk.
var _ protoreflect.MessageDescriptor = nil
