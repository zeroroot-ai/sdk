// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"bytes"
	"go/format"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	authv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/auth/v1"
)

// fixtureRequest builds a CodeGeneratorRequest containing one tiny
// service with two methods: an authenticated DoThing and an
// unauthenticated Ping. Used by every test in this file.
func fixtureRequest(t *testing.T, withAnnotations bool) *pluginpb.CodeGeneratorRequest {
	t.Helper()

	pkg := "gibson.example.v1"

	doThingOpts := &descriptorpb.MethodOptions{}
	pingOpts := &descriptorpb.MethodOptions{}

	if withAnnotations {
		proto.SetExtension(doThingOpts, authv1.E_Authz, &authv1.AuthOptions{
			Relation:          "member",
			ObjectType:        "tenant",
			ObjectDeriver:     "tenant_from_identity",
			AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER) | int32(authv1.IdentityClass_IDENTITY_CLASS_SERVICE),
		})
		proto.SetExtension(pingOpts, authv1.E_Authz, &authv1.AuthOptions{
			Unauthenticated: true,
		})
	}

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("ExampleService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			{
				Name:       proto.String("DoThing"),
				InputType:  proto.String(".gibson.example.v1.DoThingRequest"),
				OutputType: proto.String(".gibson.example.v1.DoThingResponse"),
				Options:    doThingOpts,
			},
			{
				Name:       proto.String("Ping"),
				InputType:  proto.String(".gibson.example.v1.PingRequest"),
				OutputType: proto.String(".gibson.example.v1.PingResponse"),
				Options:    pingOpts,
			},
		},
	}

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("gibson/example/v1/example.proto"),
		Package: proto.String(pkg),
		Service: []*descriptorpb.ServiceDescriptorProto{svc},
	}

	return &pluginpb.CodeGeneratorRequest{
		ProtoFile:      []*descriptorpb.FileDescriptorProto{file},
		FileToGenerate: []string{file.GetName()},
	}
}

func runFixture(t *testing.T, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	t.Helper()
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(bytes.NewReader(body), &out); err != nil {
		return nil, err
	}
	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(out.Bytes(), resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp, nil
}

func TestRun_HappyPath_ThreeArtifacts(t *testing.T) {
	resp, err := runFixture(t, fixtureRequest(t, true))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.File) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.File))
	}
	names := make([]string, len(resp.File))
	for i, f := range resp.File {
		names[i] = f.GetName()
	}
	wantSuffixes := []string{"registry.go", "registry.yaml", "permissions.ts"}
	for _, suf := range wantSuffixes {
		found := false
		for _, n := range names {
			if strings.HasSuffix(n, suf) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing artifact %s", suf)
		}
	}
}

func TestRun_GoOutputContent(t *testing.T) {
	resp, err := runFixture(t, fixtureRequest(t, true))
	if err != nil {
		t.Fatal(err)
	}
	var goFile string
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), "registry.go") {
			goFile = f.GetContent()
		}
	}
	if !strings.Contains(goFile, `"/gibson.example.v1.ExampleService/DoThing"`) {
		t.Errorf("Go output missing DoThing method:\n%s", goFile)
	}
	if !strings.Contains(goFile, "IdentityUser | IdentityService") {
		t.Errorf("Go output missing combined identities:\n%s", goFile)
	}
	if !strings.Contains(goFile, "Unauthenticated:   true") {
		t.Errorf("Go output missing Ping unauthenticated:\n%s", goFile)
	}
}

// TestRun_GoOutputIsGofmtStable guards against alignment drift in the
// generated registry.go (issue #414): the emitted source must already
// be gofmt-formatted so downstream regen (gibson `make authz-registry`)
// never produces formatting-only diffs.
func TestRun_GoOutputIsGofmtStable(t *testing.T) {
	resp, err := runFixture(t, fixtureRequest(t, true))
	if err != nil {
		t.Fatal(err)
	}
	var goFile string
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), "registry.go") {
			goFile = f.GetContent()
		}
	}
	formatted, err := format.Source([]byte(goFile))
	if err != nil {
		t.Fatalf("generated registry.go does not parse: %v", err)
	}
	if string(formatted) != goFile {
		t.Errorf("generated registry.go is not gofmt-stable; diff between emitted and gofmt'd output:\nemitted:\n%s\ngofmt:\n%s", goFile, formatted)
	}
}

func TestRun_YAMLOutputContent(t *testing.T) {
	resp, err := runFixture(t, fixtureRequest(t, true))
	if err != nil {
		t.Fatal(err)
	}
	var y string
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), "registry.yaml") {
			y = f.GetContent()
		}
	}
	if !strings.Contains(y, "/gibson.example.v1.ExampleService/DoThing") {
		t.Errorf("YAML missing method:\n%s", y)
	}
	if !strings.Contains(y, "unauthenticated: true") {
		t.Errorf("YAML missing unauthenticated marker:\n%s", y)
	}
	if !strings.Contains(y, "USER") || !strings.Contains(y, "SERVICE") {
		t.Errorf("YAML missing identity classes:\n%s", y)
	}
}

func TestRun_TSOutputContent(t *testing.T) {
	resp, err := runFixture(t, fixtureRequest(t, true))
	if err != nil {
		t.Fatal(err)
	}
	var ts string
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), "permissions.ts") {
			ts = f.GetContent()
		}
	}
	if !strings.Contains(ts, "export const AuthRegistry") {
		t.Errorf("TS missing AuthRegistry export:\n%s", ts)
	}
	if !strings.Contains(ts, "IdentityClass.USER | IdentityClass.SERVICE") {
		t.Errorf("TS missing combined identity expr:\n%s", ts)
	}
}

func TestRun_FailsWhenAnnotationMissing(t *testing.T) {
	_, err := runFixture(t, fixtureRequest(t, false))
	if err == nil {
		t.Fatal("expected fail-closed on missing annotations")
	}
	if !strings.Contains(err.Error(), "missing the (gibson.auth.v1.authz) annotation") {
		t.Fatalf("error wording unexpected: %v", err)
	}
}

func TestRun_RejectsConflictingFlags(t *testing.T) {
	// unauthenticated + relation set together is malformed.
	pkg := "gibson.example.v1"
	opts := &descriptorpb.MethodOptions{}
	proto.SetExtension(opts, authv1.E_Authz, &authv1.AuthOptions{
		Unauthenticated:   true,
		Relation:          "member",
		ObjectType:        "tenant",
		ObjectDeriver:     "tenant_from_identity",
		AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER),
	})
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("x.proto"),
		Package: proto.String(pkg),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("S"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("M"),
				InputType:  proto.String(".gibson.example.v1.X"),
				OutputType: proto.String(".gibson.example.v1.Y"),
				Options:    opts,
			}},
		}},
	}
	req := &pluginpb.CodeGeneratorRequest{ProtoFile: []*descriptorpb.FileDescriptorProto{file}, FileToGenerate: []string{"x.proto"}}
	_, err := runFixture(t, req)
	if err == nil {
		t.Fatal("expected error on conflicting flags")
	}
}

// ---------------------------------------------------------------------------
// Task 1.1: validator unit tests (zero-trust-hardening Req 10.1, 10.2, 10.3)
// ---------------------------------------------------------------------------

func TestValidateObjectDeriver_ValidValues(t *testing.T) {
	valids := []string{
		"tenant_from_identity",
		"system_tenant",
		"from_field('tenant_id')",
		"from_field('mission_id')",
		"from_field('_private')",
		"tenant_and_field('target_id')",
		"tenant_and_field('resource')",
	}
	for _, v := range valids {
		if err := validateObjectDeriver(v); err != nil {
			t.Errorf("validateObjectDeriver(%q) should be valid but got: %v", v, err)
		}
	}
}

func TestValidateObjectDeriver_InvalidValues(t *testing.T) {
	invalids := []struct {
		input   string
		wantMsg string
	}{
		{"", "does not match allowlist"},
		{"from_request_field", "does not match allowlist"},
		{"from_field(bad)", "does not match allowlist"},
		{"from_field('bad name')", "does not match allowlist"},
		{"from_field('')", "does not match allowlist"},
		{"tenant_from_request", "does not match allowlist"},
		{"TENANT_FROM_IDENTITY", "does not match allowlist"},
		{"from_field('a') extra", "does not match allowlist"},
	}
	for _, tc := range invalids {
		err := validateObjectDeriver(tc.input)
		if err == nil {
			t.Errorf("validateObjectDeriver(%q) expected error but got nil", tc.input)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantMsg) {
			t.Errorf("validateObjectDeriver(%q): want message containing %q, got %q", tc.input, tc.wantMsg, err.Error())
		}
	}
}

func TestValidateIdentityBits_ValidValues(t *testing.T) {
	// All valid combinations within 0xF.
	valids := []int32{
		1,   // USER
		2,   // SERVICE
		4,   // COMPONENT
		8,   // PLATFORM_OPERATOR
		3,   // USER | SERVICE
		5,   // USER | COMPONENT
		0xF, // all four
	}
	for _, v := range valids {
		if err := validateIdentityBits(v); err != nil {
			t.Errorf("validateIdentityBits(0x%x) should be valid but got: %v", v, err)
		}
	}
}

func TestValidateIdentityBits_InvalidValues(t *testing.T) {
	invalids := []struct {
		bits    int32
		wantMsg string
	}{
		{0x10, "contains bits outside the valid mask"},
		{0x11, "contains bits outside the valid mask"},
		{0xFF, "contains bits outside the valid mask"},
		{int32(^int32(0)), "contains bits outside the valid mask"}, // all bits set
	}
	for _, tc := range invalids {
		err := validateIdentityBits(tc.bits)
		if err == nil {
			t.Errorf("validateIdentityBits(0x%x) expected error but got nil", tc.bits)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantMsg) {
			t.Errorf("validateIdentityBits(0x%x): want message containing %q, got %q", tc.bits, tc.wantMsg, err.Error())
		}
	}
}

func TestDetectDuplicateMethodKeys_NoDuplicates(t *testing.T) {
	entries := []Entry{
		{Method: "/gibson.example.v1.S/A"},
		{Method: "/gibson.example.v1.S/B"},
		{Method: "/gibson.example.v1.S/C"},
	}
	if err := detectDuplicateMethodKeys(entries); err != nil {
		t.Errorf("detectDuplicateMethodKeys: unexpected error: %v", err)
	}
}

func TestDetectDuplicateMethodKeys_WithDuplicate(t *testing.T) {
	entries := []Entry{
		{Method: "/gibson.example.v1.S/A"},
		{Method: "/gibson.example.v1.S/B"},
		{Method: "/gibson.example.v1.S/B"}, // duplicate
		{Method: "/gibson.example.v1.S/C"},
	}
	err := detectDuplicateMethodKeys(entries)
	if err == nil {
		t.Fatal("detectDuplicateMethodKeys: expected error for duplicate key, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate method key") {
		t.Errorf("detectDuplicateMethodKeys: want 'duplicate method key' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "/gibson.example.v1.S/B") {
		t.Errorf("detectDuplicateMethodKeys: want method name in error, got: %v", err)
	}
}

// TestRun_RejectsInvalidObjectDeriver tests the full pipeline rejection of a
// typo'd object_deriver value at codegen time.
func TestRun_RejectsInvalidObjectDeriver(t *testing.T) {
	pkg := "gibson.example.v1"
	opts := &descriptorpb.MethodOptions{}
	proto.SetExtension(opts, authv1.E_Authz, &authv1.AuthOptions{
		Relation:          "member",
		ObjectType:        "tenant",
		ObjectDeriver:     "from_request_field", // invalid — not in allowlist
		AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER),
	})
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("gibson/example/v1/bad.proto"),
		Package: proto.String(pkg),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("BadService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("DoThing"),
				InputType:  proto.String(".gibson.example.v1.Req"),
				OutputType: proto.String(".gibson.example.v1.Resp"),
				Options:    opts,
			}},
		}},
	}
	req := &pluginpb.CodeGeneratorRequest{ProtoFile: []*descriptorpb.FileDescriptorProto{file}, FileToGenerate: []string{"gibson/example/v1/bad.proto"}}
	_, err := runFixture(t, req)
	if err == nil {
		t.Fatal("expected error for invalid object_deriver, got nil")
	}
	if !strings.Contains(err.Error(), "does not match allowlist") {
		t.Errorf("expected 'does not match allowlist' in error, got: %v", err)
	}
}

// TestRun_RejectsUnknownIdentityBits tests the full pipeline rejection of an
// unknown identity bit value.
func TestRun_RejectsUnknownIdentityBits(t *testing.T) {
	pkg := "gibson.example.v1"
	opts := &descriptorpb.MethodOptions{}
	proto.SetExtension(opts, authv1.E_Authz, &authv1.AuthOptions{
		Relation:          "member",
		ObjectType:        "tenant",
		ObjectDeriver:     "tenant_from_identity",
		AllowedIdentities: 0x10, // bit 4 (0x10) is outside the valid mask
	})
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("gibson/example/v1/bad2.proto"),
		Package: proto.String(pkg),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("BadService2"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("DoThing"),
				InputType:  proto.String(".gibson.example.v1.Req"),
				OutputType: proto.String(".gibson.example.v1.Resp"),
				Options:    opts,
			}},
		}},
	}
	req := &pluginpb.CodeGeneratorRequest{ProtoFile: []*descriptorpb.FileDescriptorProto{file}, FileToGenerate: []string{"gibson/example/v1/bad2.proto"}}
	_, err := runFixture(t, req)
	if err == nil {
		t.Fatal("expected error for unknown identity bits, got nil")
	}
	if !strings.Contains(err.Error(), "contains bits outside the valid mask") {
		t.Errorf("expected 'contains bits outside the valid mask' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Task 1.2: self-mode validators (self-mode-authz Req 2.1, 2.2, 2.3)
// ---------------------------------------------------------------------------

// makeSelfRequest builds a CodeGeneratorRequest with a single self-mode method
// plus the specified annotation. Used by self-mode test cases below.
func makeSelfRequest(t *testing.T, ao *authv1.AuthOptions) *pluginpb.CodeGeneratorRequest {
	t.Helper()
	opts := &descriptorpb.MethodOptions{}
	proto.SetExtension(opts, authv1.E_Authz, ao)
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("gibson/example/v1/self.proto"),
		Package: proto.String("gibson.example.v1"),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("SelfService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("GetMyData"),
				InputType:  proto.String(".gibson.example.v1.GetMyDataRequest"),
				OutputType: proto.String(".gibson.example.v1.GetMyDataResponse"),
				Options:    opts,
			}},
		}},
	}
	return &pluginpb.CodeGeneratorRequest{
		ProtoFile:      []*descriptorpb.FileDescriptorProto{file},
		FileToGenerate: []string{file.GetName()},
	}
}

// TestRun_SelfMode_Success verifies that a valid self: true + allowed_identities
// annotation produces all three artifacts with self: true in YAML and Go output.
// Spec: self-mode-authz Req 2.3.
func TestRun_SelfMode_Success(t *testing.T) {
	req := makeSelfRequest(t, &authv1.AuthOptions{
		Self:              true,
		AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER),
	})
	resp, err := runFixture(t, req)
	if err != nil {
		t.Fatalf("unexpected error for valid self-mode annotation: %v", err)
	}
	if len(resp.File) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.File))
	}
	for _, f := range resp.File {
		switch {
		case strings.HasSuffix(f.GetName(), "registry.go"):
			if !strings.Contains(f.GetContent(), "Self:              true") {
				t.Errorf("Go output missing Self: true:\n%s", f.GetContent())
			}
			if strings.Contains(f.GetContent(), "Unauthenticated:   true") {
				t.Errorf("Go output must not set Unauthenticated on self-mode entry:\n%s", f.GetContent())
			}
		case strings.HasSuffix(f.GetName(), "registry.yaml"):
			if !strings.Contains(f.GetContent(), "self: true") {
				t.Errorf("YAML output missing self: true:\n%s", f.GetContent())
			}
			if strings.Contains(f.GetContent(), "unauthenticated: true") {
				t.Errorf("YAML output must not set unauthenticated on self-mode entry:\n%s", f.GetContent())
			}
			if !strings.Contains(f.GetContent(), "USER") {
				t.Errorf("YAML output missing allowed_identities USER:\n%s", f.GetContent())
			}
		case strings.HasSuffix(f.GetName(), "permissions.ts"):
			if !strings.Contains(f.GetContent(), "self: true") {
				t.Errorf("TS output missing self: true:\n%s", f.GetContent())
			}
		}
	}
}

// TestRun_SelfMode_RejectsMutexWithUnauthenticated verifies that self: true
// combined with unauthenticated: true fails with a spec-named error.
// Spec: self-mode-authz Req 2.1.
func TestRun_SelfMode_RejectsMutexWithUnauthenticated(t *testing.T) {
	req := makeSelfRequest(t, &authv1.AuthOptions{
		Self:            true,
		Unauthenticated: true,
	})
	_, err := runFixture(t, req)
	if err == nil {
		t.Fatal("expected error for self + unauthenticated, got nil")
	}
	if !strings.Contains(err.Error(), "self and unauthenticated are mutually exclusive") {
		t.Errorf("expected mutex error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "self-mode-authz") {
		t.Errorf("expected spec name 'self-mode-authz' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "/gibson.example.v1.SelfService/GetMyData") {
		t.Errorf("expected RPC method name in error, got: %v", err)
	}
}

// TestRun_SelfMode_RejectsMutexWithRuleFields verifies that self: true combined
// with rule fields fails with a spec-named error.
// Spec: self-mode-authz Req 2.1.
func TestRun_SelfMode_RejectsMutexWithRuleFields(t *testing.T) {
	cases := []struct {
		name string
		ao   *authv1.AuthOptions
	}{
		{
			name: "self+relation",
			ao: &authv1.AuthOptions{
				Self:              true,
				Relation:          "member",
				AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER),
			},
		},
		{
			name: "self+object_type",
			ao: &authv1.AuthOptions{
				Self:              true,
				ObjectType:        "tenant",
				AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER),
			},
		},
		{
			name: "self+object_deriver",
			ao: &authv1.AuthOptions{
				Self:              true,
				ObjectDeriver:     "tenant_from_identity",
				AllowedIdentities: int32(authv1.IdentityClass_IDENTITY_CLASS_USER),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := makeSelfRequest(t, tc.ao)
			_, err := runFixture(t, req)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "self mode is incompatible with relation/object_type/object_deriver") {
				t.Errorf("expected incompatibility error, got: %v", err)
			}
			if !strings.Contains(err.Error(), "self-mode-authz") {
				t.Errorf("expected spec name 'self-mode-authz' in error, got: %v", err)
			}
		})
	}
}

// TestRun_SelfMode_RejectsMissingIdentities verifies that self: true without
// allowed_identities fails codegen. Spec: self-mode-authz Req 2.2.
func TestRun_SelfMode_RejectsMissingIdentities(t *testing.T) {
	req := makeSelfRequest(t, &authv1.AuthOptions{
		Self:              true,
		AllowedIdentities: 0, // missing
	})
	_, err := runFixture(t, req)
	if err == nil {
		t.Fatal("expected error for self without allowed_identities, got nil")
	}
	if !strings.Contains(err.Error(), "allowed_identities is required when self: true") {
		t.Errorf("expected required-identities error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "self-mode-authz") {
		t.Errorf("expected spec name 'self-mode-authz' in error, got: %v", err)
	}
}
