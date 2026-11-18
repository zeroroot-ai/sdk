// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"reflect"
	"testing"
)

func TestFinding_CiteToolCalls(t *testing.T) {
	tests := []struct {
		name string
		have []string
		add  []string
		want []string
	}{
		{"adds ids in order", nil, []string{"a", "b"}, []string{"a", "b"}},
		{"skips a repeated id", []string{"a"}, []string{"a", "b", "b"}, []string{"a", "b"}},
		{"skips an empty id", nil, []string{"", "a"}, []string{"a"}},
		{"adds nothing", nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Finding{ToolCallIDs: tt.have}
			f.CiteToolCalls(tt.add...)
			if !reflect.DeepEqual(f.ToolCallIDs, tt.want) {
				t.Errorf("ToolCallIDs = %v, want %v", f.ToolCallIDs, tt.want)
			}
		})
	}
}
