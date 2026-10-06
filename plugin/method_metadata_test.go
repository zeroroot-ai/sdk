// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"testing"
)

func TestBuildMethodMetadataEmpty(t *testing.T) {
	names, detailed := buildMethodMetadata(nil, nil, nil, nil)
	if len(names) != 0 || len(detailed) != 0 {
		t.Fatalf("empty inputs = (%v, %v), want empty", names, detailed)
	}
}
