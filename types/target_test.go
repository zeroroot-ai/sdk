// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"testing"
)

// Note: TargetType tests removed as target types are now plain strings.
// Domain-specific target type constants moved to Gibson's taxonomy.

func TestTargetInfo_URL_Method(t *testing.T) {
	tests := []struct {
		name   string
		target TargetInfo
		want   string
	}{
		{
			name: "URL from Connection map",
			target: TargetInfo{
				Connection: map[string]any{
					"url": "https://api.example.com",
				},
			},
			want: "https://api.example.com",
		},
		{
			name:   "empty when neither provided",
			target: TargetInfo{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.target.URL(); got != tt.want {
				t.Errorf("URL() = %v, want %v", got, tt.want)
			}
		})
	}
}
