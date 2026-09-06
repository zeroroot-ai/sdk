// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package registry_test

// coverage_test.go — proto-annotation coverage gate for the authz registry.
//
// This test asserts that every Gibson gRPC method in the SDK's proto descriptors
// carries a valid (gibson.auth.v1.authz) annotation. It is the proto-level half
// of Requirement 7.6: a developer who adds an RPC without annotating it sees this
// test fail on `go test ./auth/registry/...`.
//
// Design note (spec: private-authz-registry, Task 4, option a):
// The rendered registry artifacts (registry.go, registry.yaml, permissions.ts)
// are moving to the private core/gibson repo. This test no longer reads the
// committed registry map; instead it walks protoregistry.GlobalFiles directly,
// extracts the (gibson.auth.v1.authz) extension on each method, and validates
// annotation completeness. This keeps the SDK as the fast-feedback gate for SDK
// proto contributors without depending on the committed artifacts.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	// Force-import generated bindings so their init() registers
	// service descriptors with protoregistry.GlobalFiles. Without
	// these blank imports, GlobalFiles would be empty for a test
	// binary that doesn't otherwise reference them.
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/agent/v1"

	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/auth/v1"

	// gibson/authz/v1, gibson/admin/v1, gibson/usage/v1,
	// gibson/daemon/discovery/v1, and gibson/budget/v1 (BudgetService)
	// moved to platform-sdk under slices #108 and #106. Their authz
	// annotation coverage is now enforced in platform-sdk's own CI;
	// this OSS gate only covers customer-facing packages.
	// gibson/budget_status/v1 (BudgetExceeded + BudgetScope) is the
	// customer-visible value-type residue and has no annotated RPCs.
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"

	// NOTE(plugin-runtime Phase 6): gibson/plugin/v1 import removed — the old
	// PluginService proto was deleted by Spec 2 Phase 1. The new PluginInvokeService
	// (invoke.proto) will be added back here after Phase 6 regeneration.
	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/tool/v1"

	authv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/auth/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// objectDeriverRe is the allowlist regex for object_deriver values, mirroring
// the pattern enforced by authz-registry-gen at codegen time. Keeping both in
// sync ensures that the SDK test suite catches annotation regressions as fast
// as the codegen tool. Spec: zero-trust-hardening Requirement 10.1, 10.4.
var objectDeriverRe = regexp.MustCompile(
	`^(tenant_from_identity|system_tenant|from_field\('[a-zA-Z_]+'\)|tenant_and_field\('[a-zA-Z_]+'\))$`,
)

// isGibsonPackage returns true for packages owned by the Gibson platform that
// require authz annotations. Matches the same filter used by authz-registry-gen.
func isGibsonPackage(pkg string) bool {
	return strings.HasPrefix(pkg, "gibson.")
}

// daemonLocalPrefixes lists service full-names whose proto definitions live in
// the gibson daemon repo (zeroroot-ai/gibson), not the SDK. These services are
// forward-declared in the registry to satisfy the daemon's assertRegistryCoverage
// check at startup. Their descriptors are never registered in protoregistry.GlobalFiles
// from an SDK binary, so they are excluded from checks here.
var daemonLocalPrefixes = []string{
	"gibson.daemon.admin.v1.DaemonAdminService",
	"gibson.platform.v1.PlatformOperatorService",
	"gibson.tenant.v1.TenantAdminService",
	"gibson.user.v1.UserService",
}

func isDaemonLocal(fullMethod string) bool {
	for _, prefix := range daemonLocalPrefixes {
		if strings.HasPrefix(fullMethod, "/"+prefix+"/") {
			return true
		}
	}
	return false
}

// TestEveryGibsonRPCHasAuthzAnnotation walks all Gibson service descriptors
// registered in protoregistry.GlobalFiles and asserts each method carries a
// valid (gibson.auth.v1.authz) extension.
//
// A valid annotation is one of:
//   - unauthenticated: true  (and no other fields set)
//   - relation + object_type both non-empty (and unauthenticated: false)
//
// This test catches a missing or zeroed annotation before regen is attempted.
func TestEveryGibsonRPCHasAuthzAnnotation(t *testing.T) {
	type result struct {
		method string
		err    string
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}

				opts := m.Options()
				if opts == nil {
					failures = append(failures, result{full, "method has no options — missing (gibson.auth.v1.authz) annotation"})
					continue
				}

				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil {
					failures = append(failures, result{full, "missing (gibson.auth.v1.authz) annotation"})
					continue
				}

				// Detect the all-zero options case (annotation present but empty).
				allZero := !ao.Unauthenticated &&
					!ao.Self &&
					ao.Relation == "" &&
					ao.ObjectType == "" &&
					ao.ObjectDeriver == "" &&
					ao.AllowedIdentities == 0
				if allZero {
					failures = append(failures, result{full, "annotation is all-zero; add relation/object_type, set unauthenticated: true, or set self: true (self-mode-authz)"})
					continue
				}

				// Validate mutually-exclusive fields.
				if ao.Unauthenticated {
					if ao.Relation != "" || ao.ObjectType != "" || ao.ObjectDeriver != "" || ao.AllowedIdentities != 0 {
						failures = append(failures, result{full, "unauthenticated: true cannot be combined with relation/object_type/identities"})
					}
					continue
				}

				// self-mode-authz: self: true requires allowed_identities and no rule fields.
				if ao.Self {
					if ao.AllowedIdentities == 0 {
						failures = append(failures, result{full, "self: true requires allowed_identities to be non-zero (self-mode-authz)"})
					}
					if ao.Relation != "" || ao.ObjectType != "" || ao.ObjectDeriver != "" {
						failures = append(failures, result{full, "self: true cannot be combined with relation/object_type/object_deriver (self-mode-authz)"})
					}
					continue
				}

				// Authenticated RPC (rule mode): relation and object_type are required.
				var missing []string
				if ao.Relation == "" {
					missing = append(missing, "relation")
				}
				if ao.ObjectType == "" {
					missing = append(missing, "object_type")
				}
				if len(missing) > 0 {
					failures = append(failures, result{full, "missing required fields: " + strings.Join(missing, ", ")})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d RPC(s) have invalid or missing (gibson.auth.v1.authz) annotations:\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: %s\n", f.method, f.err)
		}
		sb.WriteString("\nRun `make proto` in core/sdk to regenerate after adding/fixing annotations.")
		t.Fatal(sb.String())
	}
}

// ---------------------------------------------------------------------------
// Task 1.4: bitfield + deriver invariant tests
// Spec: zero-trust-hardening Requirements 10.4, 2.6
// ---------------------------------------------------------------------------

// TestObjectDeriverIsValidForEveryRPC asserts that every annotated RPC's
// object_deriver matches the codegen allowlist regex. A value outside the
// allowlist would pass the proto-level required-field check but cause ext-authz
// to refuse to start at runtime.
// Spec: zero-trust-hardening Requirement 10.4.
func TestObjectDeriverIsValidForEveryRPC(t *testing.T) {
	type result struct {
		method  string
		deriver string
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil || ao.Unauthenticated || ao.Self {
					continue
				}
				if ao.ObjectDeriver == "" {
					continue // caught by TestEveryGibsonRPCHasAuthzAnnotation
				}
				if !objectDeriverRe.MatchString(ao.ObjectDeriver) {
					failures = append(failures, result{full, ao.ObjectDeriver})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d RPC(s) have an object_deriver that does not match the allowlist regex:\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: object_deriver=%q\n", f.method, f.deriver)
		}
		sb.WriteString("\nAllowed values: tenant_from_identity | system_tenant | from_field('<name>') | tenant_and_field('<name>')")
		t.Fatal(sb.String())
	}
}

// TestIdentityBitsAreValidForEveryRPC asserts that every annotated RPC's
// allowed_identities is non-zero and contains only known bits (mask 0xF:
// USER|SERVICE|COMPONENT|PLATFORM_OPERATOR). Unknown bits indicate a typo
// or an unapproved extension to the identity model.
// Spec: zero-trust-hardening Requirement 10.4.
func TestIdentityBitsAreValidForEveryRPC(t *testing.T) {
	const validMask = int32(0xF)

	type result struct {
		method string
		bits   int32
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil || ao.Unauthenticated {
					continue
				}
				// self-mode entries must also carry valid identity bits.
				bits := ao.AllowedIdentities
				if bits == 0 || bits&^validMask != 0 {
					failures = append(failures, result{full, bits})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d RPC(s) have invalid allowed_identities (must be non-zero and a subset of 0xF):\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: allowed_identities=0x%x\n", f.method, uint32(f.bits))
		}
		t.Fatal(sb.String())
	}
}

// TestNoDuplicateMethodKeysAcrossFiles asserts that no two methods produce the
// same /<pkg>.<svc>/<method> key in the global registry. Duplicate keys would
// silently overwrite each other in the generated Go map.
// Spec: zero-trust-hardening Requirement 10.4.
func TestNoDuplicateMethodKeysAcrossFiles(t *testing.T) {
	seen := map[string]int{}

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				seen[full]++
			}
		}
		return true
	})

	var dupes []string
	for key, count := range seen {
		if count > 1 {
			dupes = append(dupes, fmt.Sprintf("%s (seen %d times)", key, count))
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		t.Fatalf("duplicate method keys found in proto registry:\n  %s", strings.Join(dupes, "\n  "))
	}
}

// TestAgentToolServicesNoUserBit is a regression guard for task 1.2: no RPC
// under gibson.agent.v1 or gibson.tool.v1 may declare the USER bit (1).
// These services are component-to-component surfaces and must be COMPONENT-only.
// Spec: zero-trust-hardening Requirement 2.6.
func TestAgentToolServicesNoUserBit(t *testing.T) {
	const userBit = int32(1)

	type result struct {
		method string
		bits   int32
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if pkg != "gibson.agent.v1" && pkg != "gibson.tool.v1" {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil || ao.Unauthenticated {
					continue
				}
				if ao.AllowedIdentities&userBit != 0 {
					failures = append(failures, result{full, ao.AllowedIdentities})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d RPC(s) in gibson.agent.v1 or gibson.tool.v1 declare the USER bit — these are COMPONENT-only surfaces:\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: allowed_identities=0x%x (USER bit is 0x1)\n", f.method, uint32(f.bits))
		}
		sb.WriteString("\nFix: set allowed_identities: 4 (COMPONENT only) in the proto. See zero-trust-hardening Req 2.5.")
		t.Fatal(sb.String())
	}
}

// TestAnnotatedRPCMinimumCount is a sanity sentinel: if the annotated method
// count ever drops below a meaningful floor, the regression should be loud.
// The exact count fluctuates as RPCs are added; the floor is set conservatively.
// self-mode entries are counted alongside rule-mode ones (Req 2.5).
func TestAnnotatedRPCMinimumCount(t *testing.T) {
	ruleCount := 0
	selfCount := 0
	unauthCount := 0
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !isGibsonPackage(string(fd.Package())) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil {
					continue
				}
				switch {
				case ao.Unauthenticated:
					unauthCount++
				case ao.Self:
					selfCount++
				default:
					ruleCount++
				}
			}
		}
		return true
	})

	total := ruleCount + selfCount + unauthCount
	const minExpected = 50
	if total < minExpected {
		t.Fatalf("only %d annotated RPCs found (rule=%d self=%d unauth=%d); expected at least %d. Did a large batch of protos lose their authz annotations? (self-mode-authz Req 2.5)", total, ruleCount, selfCount, unauthCount, minExpected)
	}
}

// ---------------------------------------------------------------------------
// Task 1.3: self-mode coverage invariants
// Spec: self-mode-authz Requirements 2.4, 2.5
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Task 1.2: member/admin relation bitmask coherence guards
// Spec: discovery-bitfield-coherence Requirements 3.1, 3.2, 3.4
// ---------------------------------------------------------------------------

// TestMemberRelationIncludesUserBit asserts that every RPC whose
// (gibson.auth.v1.authz) annotation has relation == "member" or
// relation == "tenant_member", AND unauthenticated == false, AND self == false,
// MUST have bit 1 (USER) set in allowed_identities.
//
// Rationale: a "member" relation means "any tenant member can call this RPC".
// Tenant members are by definition USER, SERVICE, or COMPONENT callers — not
// PLATFORM_OPERATOR-only. A bitmask that excludes USER (e.g., 8 = PLATFORM_OPERATOR)
// is incoherent and causes every legitimate human caller to receive a 403 at the
// ext-authz identity-class gate before the FGA check even runs.
//
// Exception: gibson.agent.v1 and gibson.tool.v1 packages are intentionally
// COMPONENT-only (allowed_identities: 4). They use "member" for tenant-scoping
// but are exclusively component runtime surfaces — a separate, opposite-polarity
// guard (TestAgentToolServicesNoUserBit) enforces this. Skipping these packages
// here avoids the two guards contradicting each other.
//
// Failure messages name the offending RPC and reference this spec so contributors
// can grep for the originating guard. Spec: discovery-bitfield-coherence Req 3.1, 3.4.
func TestMemberRelationIncludesUserBit(t *testing.T) {
	const userBit = int32(1)

	// componentOnlyPackages lists packages that are intentionally COMPONENT-only
	// (allowed_identities: 4) for every RPC. They use "member" for tenant-scoping
	// but are exclusively agent/tool runtime surfaces. Their bitmask coherence is
	// enforced by the opposite-polarity guard TestAgentToolServicesNoUserBit, which
	// asserts they do NOT have USER bit set.
	componentOnlyPackages := map[string]bool{
		"gibson.agent.v1": true,
		"gibson.tool.v1":  true,
	}

	// componentOnlyIdentities is the bitmask value for a COMPONENT-only RPC
	// (USER=1, SERVICE=2, COMPONENT=4 → COMPONENT-only = 4). member-relation
	// RPCs with exactly this mask are intentionally agent-runtime-only surfaces;
	// they are not the incoherent PLATFORM_OPERATOR-only pattern this guard targets.
	const componentOnlyIdentities = int32(4)

	type result struct {
		method string
		bits   int32
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		// Skip packages with their own opposite-polarity guard (agent/tool services).
		if componentOnlyPackages[pkg] {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil {
					continue
				}
				// Skip unauthenticated and self-mode entries — those have their own rules.
				if ao.Unauthenticated || ao.Self {
					continue
				}
				// Only inspect member / tenant_member relations.
				rel := ao.Relation
				if rel != "member" && rel != "tenant_member" {
					continue
				}
				// COMPONENT-only (4) is a validated intentional pattern for agent-runtime
				// RPCs that use "member" for tenant-scoping but restrict callers to
				// components only (e.g., DaemonService.RenewCapabilityGrant). These are
				// not the incoherent PLATFORM_OPERATOR-only discovery-regression pattern.
				if ao.AllowedIdentities == componentOnlyIdentities {
					continue
				}
				// All other cases: USER bit (1) must be set.
				if ao.AllowedIdentities&userBit == 0 {
					failures = append(failures, result{full, ao.AllowedIdentities})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d RPC(s) have relation \"member\" or \"tenant_member\" but do NOT include the USER bit (1) in allowed_identities.\n", len(failures))
		fmt.Fprintf(&sb, "Guard: discovery-bitfield-coherence Requirement 3.1\n")
		fmt.Fprintf(&sb, "A member-relation RPC must allow USER callers (bit 1); a PLATFORM_OPERATOR-only mask (8) is incoherent.\n\n")
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: allowed_identities=0x%x (USER bit 0x1 not set) [discovery-bitfield-coherence]\n", f.method, uint32(f.bits))
		}
		sb.WriteString("\nFix: set allowed_identities to a value with bit 1 set (e.g. 7 = USER|SERVICE|COMPONENT).")
		sb.WriteString("\nSpec: discovery-bitfield-coherence")
		t.Fatal(sb.String())
	}
}

// TestAdminRelationIncludesUserOrServiceBit asserts that every RPC whose
// (gibson.auth.v1.authz) annotation has relation == "admin" or
// relation == "tenant_admin", AND unauthenticated == false, AND self == false,
// MUST have at least bit 1 (USER) OR bit 2 (SERVICE) set in allowed_identities.
//
// Rationale: admin/tenant_admin relations are used by human tenant admins (USER)
// and admin-acting service accounts (SERVICE). A bitmask that contains neither
// (e.g., PLATFORM_OPERATOR-only = 8, or COMPONENT-only = 4) is incoherent.
//
// Note: this test does NOT constrain can_use / can_execute / can_invoke /
// can_resolve / can_configure (COMPONENT-only by convention) or platform_operator
// relations — those have their own coherent bitmask conventions (Req 3.3).
//
// Failure messages name the offending RPC and reference this spec.
// Spec: discovery-bitfield-coherence Req 3.2, 3.4.
func TestAdminRelationIncludesUserOrServiceBit(t *testing.T) {
	const (
		userBit    = int32(1)
		serviceBit = int32(2)
	)

	type result struct {
		method string
		bits   int32
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil {
					continue
				}
				// Skip unauthenticated and self-mode entries.
				if ao.Unauthenticated || ao.Self {
					continue
				}
				// Only inspect admin / tenant_admin relations.
				rel := ao.Relation
				if rel != "admin" && rel != "tenant_admin" {
					continue
				}
				// At least USER (1) or SERVICE (2) must be set.
				if ao.AllowedIdentities&userBit == 0 && ao.AllowedIdentities&serviceBit == 0 {
					failures = append(failures, result{full, ao.AllowedIdentities})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d RPC(s) have relation \"admin\" or \"tenant_admin\" but do NOT include USER (bit 1) or SERVICE (bit 2) in allowed_identities.\n", len(failures))
		fmt.Fprintf(&sb, "Guard: discovery-bitfield-coherence Requirement 3.2\n")
		fmt.Fprintf(&sb, "An admin-relation RPC must allow human admins (USER, bit 1) or admin service accounts (SERVICE, bit 2).\n\n")
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: allowed_identities=0x%x (neither USER 0x1 nor SERVICE 0x2 is set) [discovery-bitfield-coherence]\n", f.method, uint32(f.bits))
		}
		sb.WriteString("\nFix: include at least USER (1) or SERVICE (2) in allowed_identities for admin-relation RPCs.")
		sb.WriteString("\nSpec: discovery-bitfield-coherence")
		t.Fatal(sb.String())
	}
}

// TestSelfModeIsValidForEveryRPC walks protoregistry.GlobalFiles and asserts that
// every RPC with self: true also has non-zero allowed_identities AND no rule fields
// set (relation/object_type/object_deriver must be empty).
//
// This is the proto-level enforcement that the self-mode shape is correct before
// SDK codegen is attempted. A contributor who sets self: true without allowed_identities
// will see this test fail immediately on `go test ./auth/registry/...`.
//
// Spec: self-mode-authz Requirement 2.4.
func TestSelfModeIsValidForEveryRPC(t *testing.T) {
	type result struct {
		method string
		err    string
	}
	var failures []result

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		pkg := string(fd.Package())
		if !isGibsonPackage(pkg) {
			return true
		}
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				full := "/" + string(svc.FullName()) + "/" + string(m.Name())
				if isDaemonLocal(full) {
					continue
				}
				opts := m.Options()
				if opts == nil {
					continue
				}
				ext := proto.GetExtension(opts.ProtoReflect().Interface(), authv1.E_Authz)
				ao, ok := ext.(*authv1.AuthOptions)
				if !ok || ao == nil || !ao.Self {
					continue
				}
				// self: true entries must have non-zero allowed_identities.
				if ao.AllowedIdentities == 0 {
					failures = append(failures, result{full, "self: true requires allowed_identities to be non-zero (self-mode-authz Req 2.4)"})
				}
				// self: true entries must not have rule fields set.
				if ao.Relation != "" {
					failures = append(failures, result{full, fmt.Sprintf("self: true cannot have relation=%q — mutually exclusive (self-mode-authz Req 2.4)", ao.Relation)})
				}
				if ao.ObjectType != "" {
					failures = append(failures, result{full, fmt.Sprintf("self: true cannot have object_type=%q — mutually exclusive (self-mode-authz Req 2.4)", ao.ObjectType)})
				}
				if ao.ObjectDeriver != "" {
					failures = append(failures, result{full, fmt.Sprintf("self: true cannot have object_deriver=%q — mutually exclusive (self-mode-authz Req 2.4)", ao.ObjectDeriver)})
				}
				// self: true must not also be unauthenticated.
				if ao.Unauthenticated {
					failures = append(failures, result{full, "self: true and unauthenticated: true are mutually exclusive (self-mode-authz Req 2.4)"})
				}
			}
		}
		return true
	})

	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool { return failures[i].method < failures[j].method })
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d self-mode RPC(s) have invalid annotations:\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(&sb, "  %s: %s\n", f.method, f.err)
		}
		sb.WriteString("\nSee spec self-mode-authz Requirement 2.4 for the correct annotation shape.")
		t.Fatal(sb.String())
	}
}
