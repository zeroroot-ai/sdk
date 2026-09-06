// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import "testing"

func TestCapabilities_IsArgBlocked(t *testing.T) {
	tests := []struct {
		name        string
		blockedArgs []string
		arg         string
		want        bool
	}{
		{
			name:        "blocked argument exists",
			blockedArgs: []string{"-sS", "-O", "-sU"},
			arg:         "-sS",
			want:        true,
		},
		{
			name:        "blocked argument in middle",
			blockedArgs: []string{"-sS", "-O", "-sU"},
			arg:         "-O",
			want:        true,
		},
		{
			name:        "blocked argument at end",
			blockedArgs: []string{"-sS", "-O", "-sU"},
			arg:         "-sU",
			want:        true,
		},
		{
			name:        "argument not blocked",
			blockedArgs: []string{"-sS", "-O", "-sU"},
			arg:         "-sT",
			want:        false,
		},
		{
			name:        "empty string not blocked",
			blockedArgs: []string{"-sS", "-O"},
			arg:         "",
			want:        false,
		},
		{
			name:        "similar but different argument",
			blockedArgs: []string{"-sS"},
			arg:         "-sS1",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				BlockedArgs: tt.blockedArgs,
			}
			if got := c.IsArgBlocked(tt.arg); got != tt.want {
				t.Errorf("IsArgBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapabilities_IsArgBlocked_Empty(t *testing.T) {
	tests := []struct {
		name        string
		blockedArgs []string
		arg         string
		want        bool
	}{
		{
			name:        "nil blocked args",
			blockedArgs: nil,
			arg:         "-sS",
			want:        false,
		},
		{
			name:        "empty blocked args slice",
			blockedArgs: []string{},
			arg:         "-sS",
			want:        false,
		},
		{
			name:        "empty arg with nil blocked args",
			blockedArgs: nil,
			arg:         "",
			want:        false,
		},
		{
			name:        "empty arg with empty blocked args",
			blockedArgs: []string{},
			arg:         "",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				BlockedArgs: tt.blockedArgs,
			}
			if got := c.IsArgBlocked(tt.arg); got != tt.want {
				t.Errorf("IsArgBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapabilities_GetAlternative(t *testing.T) {
	tests := []struct {
		name            string
		argAlternatives map[string]string
		arg             string
		wantAlt         string
		wantExists      bool
	}{
		{
			name: "alternative exists",
			argAlternatives: map[string]string{
				"-sS": "-sT",
				"-O":  "",
			},
			arg:        "-sS",
			wantAlt:    "-sT",
			wantExists: true,
		},
		{
			name: "alternative is empty string",
			argAlternatives: map[string]string{
				"-sS": "-sT",
				"-O":  "",
			},
			arg:        "-O",
			wantAlt:    "",
			wantExists: true,
		},
		{
			name: "alternative does not exist",
			argAlternatives: map[string]string{
				"-sS": "-sT",
			},
			arg:        "-sU",
			wantAlt:    "",
			wantExists: false,
		},
		{
			name: "empty arg lookup",
			argAlternatives: map[string]string{
				"":    "default",
				"-sS": "-sT",
			},
			arg:        "",
			wantAlt:    "default",
			wantExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				ArgAlternatives: tt.argAlternatives,
			}
			gotAlt, gotExists := c.GetAlternative(tt.arg)
			if gotAlt != tt.wantAlt {
				t.Errorf("GetAlternative() alt = %v, want %v", gotAlt, tt.wantAlt)
			}
			if gotExists != tt.wantExists {
				t.Errorf("GetAlternative() exists = %v, want %v", gotExists, tt.wantExists)
			}
		})
	}
}

func TestCapabilities_GetAlternative_NotFound(t *testing.T) {
	tests := []struct {
		name            string
		argAlternatives map[string]string
		arg             string
	}{
		{
			name: "key not in map",
			argAlternatives: map[string]string{
				"-sS": "-sT",
				"-O":  "",
			},
			arg: "-sU",
		},
		{
			name:            "empty map",
			argAlternatives: map[string]string{},
			arg:             "-sS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				ArgAlternatives: tt.argAlternatives,
			}
			gotAlt, gotExists := c.GetAlternative(tt.arg)
			if gotAlt != "" {
				t.Errorf("GetAlternative() alt = %v, want empty string", gotAlt)
			}
			if gotExists {
				t.Errorf("GetAlternative() exists = %v, want false", gotExists)
			}
		})
	}
}

func TestCapabilities_GetAlternative_NilMap(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{
			name: "nil map with valid arg",
			arg:  "-sS",
		},
		{
			name: "nil map with empty arg",
			arg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				ArgAlternatives: nil,
			}
			gotAlt, gotExists := c.GetAlternative(tt.arg)
			if gotAlt != "" {
				t.Errorf("GetAlternative() alt = %v, want empty string", gotAlt)
			}
			if gotExists {
				t.Errorf("GetAlternative() exists = %v, want false", gotExists)
			}
		})
	}
}

func TestCapabilities_HasFeature(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]bool
		feature  string
		want     bool
	}{
		{
			name: "feature enabled",
			features: map[string]bool{
				"stealth_scan": true,
				"os_detection": false,
			},
			feature: "stealth_scan",
			want:    true,
		},
		{
			name: "feature disabled",
			features: map[string]bool{
				"stealth_scan": true,
				"os_detection": false,
			},
			feature: "os_detection",
			want:    false,
		},
		{
			name: "feature not present",
			features: map[string]bool{
				"stealth_scan": true,
			},
			feature: "port_scan",
			want:    false,
		},
		{
			name:     "nil features map",
			features: nil,
			feature:  "any_feature",
			want:     false,
		},
		{
			name:     "empty features map",
			features: map[string]bool{},
			feature:  "any_feature",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				Features: tt.features,
			}
			if got := c.HasFeature(tt.feature); got != tt.want {
				t.Errorf("HasFeature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapabilities_HasPrivilegedAccess(t *testing.T) {
	tests := []struct {
		name         string
		hasRoot      bool
		hasSudo      bool
		canRawSocket bool
		want         bool
	}{
		{
			name:         "has root only",
			hasRoot:      true,
			hasSudo:      false,
			canRawSocket: false,
			want:         true,
		},
		{
			name:         "has sudo only",
			hasRoot:      false,
			hasSudo:      true,
			canRawSocket: false,
			want:         true,
		},
		{
			name:         "has raw socket only",
			hasRoot:      false,
			hasSudo:      false,
			canRawSocket: true,
			want:         true,
		},
		{
			name:         "has all privileges",
			hasRoot:      true,
			hasSudo:      true,
			canRawSocket: true,
			want:         true,
		},
		{
			name:         "has root and sudo",
			hasRoot:      true,
			hasSudo:      true,
			canRawSocket: false,
			want:         true,
		},
		{
			name:         "has root and raw socket",
			hasRoot:      true,
			hasSudo:      false,
			canRawSocket: true,
			want:         true,
		},
		{
			name:         "has sudo and raw socket",
			hasRoot:      false,
			hasSudo:      true,
			canRawSocket: true,
			want:         true,
		},
		{
			name:         "no privileges",
			hasRoot:      false,
			hasSudo:      false,
			canRawSocket: false,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capabilities{
				HasRoot:      tt.hasRoot,
				HasSudo:      tt.hasSudo,
				CanRawSocket: tt.canRawSocket,
			}
			if got := c.HasPrivilegedAccess(); got != tt.want {
				t.Errorf("HasPrivilegedAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCapabilities(t *testing.T) {
	caps := NewCapabilities()

	if caps == nil {
		t.Fatal("NewCapabilities() returned nil")
	}

	// Verify all collections are initialized
	if caps.Features == nil {
		t.Error("NewCapabilities() Features map is nil")
	}

	if caps.BlockedArgs == nil {
		t.Error("NewCapabilities() BlockedArgs slice is nil")
	}

	if caps.ArgAlternatives == nil {
		t.Error("NewCapabilities() ArgAlternatives map is nil")
	}

	// Verify collections are empty but not nil
	if len(caps.Features) != 0 {
		t.Errorf("NewCapabilities() Features length = %v, want 0", len(caps.Features))
	}

	if len(caps.BlockedArgs) != 0 {
		t.Errorf("NewCapabilities() BlockedArgs length = %v, want 0", len(caps.BlockedArgs))
	}

	if len(caps.ArgAlternatives) != 0 {
		t.Errorf("NewCapabilities() ArgAlternatives length = %v, want 0", len(caps.ArgAlternatives))
	}

	// Verify privilege fields default to false
	if caps.HasRoot {
		t.Error("NewCapabilities() HasRoot should be false")
	}

	if caps.HasSudo {
		t.Error("NewCapabilities() HasSudo should be false")
	}

	if caps.CanRawSocket {
		t.Error("NewCapabilities() CanRawSocket should be false")
	}
}

func TestNewCapabilities_SafeToModify(t *testing.T) {
	caps := NewCapabilities()

	// Verify we can add to collections without panics
	caps.Features["test"] = true
	caps.BlockedArgs = append(caps.BlockedArgs, "-test")
	caps.ArgAlternatives["-test"] = "-safe"

	// Verify additions work
	if !caps.HasFeature("test") {
		t.Error("Failed to add feature")
	}

	if !caps.IsArgBlocked("-test") {
		t.Error("Failed to add blocked arg")
	}

	alt, exists := caps.GetAlternative("-test")
	if !exists || alt != "-safe" {
		t.Error("Failed to add alternative")
	}
}
