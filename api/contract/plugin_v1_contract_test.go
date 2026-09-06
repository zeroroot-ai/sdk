// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	_ "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
)

func TestPluginV1_AllMessagesRoundTrip(t *testing.T) {
	roundTripPackage(t, "gibson.plugin.v1")
}
