// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	identityv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/identity/v1"
)

func TestIdentityV1_AllMessagesRoundTrip(t *testing.T) {
	roundTripPackage(t, "gibson.identity.v1")
}

// TestIdentityV1_PrincipalKindUser pins the wire number and name of the
// person kind. A reader that prints the kind from the enum name needs both.
func TestIdentityV1_PrincipalKindUser(t *testing.T) {
	if got := identityv1.PrincipalKind_PRINCIPAL_KIND_USER.String(); got != "PRINCIPAL_KIND_USER" {
		t.Fatalf("name = %q, want PRINCIPAL_KIND_USER", got)
	}
	if got := int32(identityv1.PrincipalKind_PRINCIPAL_KIND_USER); got != 4 {
		t.Fatalf("number = %d, want 4", got)
	}
}
