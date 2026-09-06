// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
)

func TestDaemonV1_AllMessagesRoundTrip(t *testing.T) {
	roundTripPackage(t, "gibson.daemon.v1")
}
