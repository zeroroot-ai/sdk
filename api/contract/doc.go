// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package contract holds SDK-wide proto contract tests that span multiple
// proto packages — wire-format round-trip, field-numbering stability,
// platform invariants — rather than tests of any one package.
//
// # Pattern
//
// For each proto package P (e.g. gibson.mission.v1), drop a file
// `<P>_contract_test.go` here containing:
//
//  1. A blank import of the corresponding api/gen/.../v1 package so its
//     File and Type descriptors register with protoregistry.GlobalFiles
//     and protoregistry.GlobalTypes.
//  2. A Test<Pkg>_AllMessagesRoundTrip function that calls
//     roundTripPackage(t, "gibson.<pkg>.v1") — defined in
//     contract_helpers_test.go. The helper walks every top-level and
//     nested message in the named package and round-trips it through
//     proto-binary and protojson in zero form.
//  3. Optional per-message richer fixtures (Test<Message>_RoundTripPopulated)
//     that exercise a populated instance through the same roundTrip kernel.
//     Add these incrementally as messages start carrying load-bearing
//     data; see mission_v1_contract_test.go for the reference shape.
//
// # Why a separate package
//
// Tests under api/gen/ would be wiped by `make proto-clean`. Keeping the
// contract tests in api/contract/ makes them survive proto regeneration and
// keeps them grouped under api/ where proto consumers will naturally look.
//
// # Adding a new proto package
//
// When a new gibson.<pkg>.v1 is introduced, drop a matching
// <pkg>_v1_contract_test.go alongside the existing files. The mechanical
// boilerplate is short (one blank import + one one-line test function);
// the heavy lifting is in roundTripPackage.
package contract
