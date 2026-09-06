// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"strings"
	"testing"
	"time"
)

// TestTimeoutConfig_Validate tests the internal consistency validation of TimeoutConfig.
func TestTimeoutConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  TimeoutConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields set",
			config: TimeoutConfig{
				Default: 5 * time.Minute,
				Max:     10 * time.Minute,
				Min:     1 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config with default equal to min",
			config: TimeoutConfig{
				Default: 1 * time.Minute,
				Max:     10 * time.Minute,
				Min:     1 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config with default equal to max",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
				Max:     10 * time.Minute,
				Min:     1 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config with only max set",
			config: TimeoutConfig{
				Max: 10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config with only min set",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config with only default set",
			config: TimeoutConfig{
				Default: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name:    "valid empty config (all zeros)",
			config:  TimeoutConfig{},
			wantErr: false,
		},
		{
			name: "invalid: min > max",
			config: TimeoutConfig{
				Min: 10 * time.Minute,
				Max: 5 * time.Minute,
			},
			wantErr: true,
			errMsg:  "min timeout 10m0s exceeds max timeout 5m0s",
		},
		{
			name: "invalid: default < min",
			config: TimeoutConfig{
				Default: 30 * time.Second,
				Min:     1 * time.Minute,
				Max:     10 * time.Minute,
			},
			wantErr: true,
			errMsg:  "default timeout 30s below min 1m0s",
		},
		{
			name: "invalid: default > max",
			config: TimeoutConfig{
				Default: 15 * time.Minute,
				Min:     1 * time.Minute,
				Max:     10 * time.Minute,
			},
			wantErr: true,
			errMsg:  "default timeout 15m0s exceeds max 10m0s",
		},
		{
			name: "invalid: default < min (no max)",
			config: TimeoutConfig{
				Default: 30 * time.Second,
				Min:     1 * time.Minute,
			},
			wantErr: true,
			errMsg:  "default timeout 30s below min 1m0s",
		},
		{
			name: "invalid: default > max (no min)",
			config: TimeoutConfig{
				Default: 15 * time.Minute,
				Max:     10 * time.Minute,
			},
			wantErr: true,
			errMsg:  "default timeout 15m0s exceeds max 10m0s",
		},
		{
			name: "valid: min and max equal",
			config: TimeoutConfig{
				Min: 5 * time.Minute,
				Max: 5 * time.Minute,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestTimeoutConfig_ValidateTimeout tests the bounds checking for requested timeouts.
func TestTimeoutConfig_ValidateTimeout(t *testing.T) {
	tests := []struct {
		name      string
		config    TimeoutConfig
		requested time.Duration
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid: within bounds",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 10 * time.Minute,
			},
			requested: 5 * time.Minute,
			wantErr:   false,
		},
		{
			name: "valid: equal to min",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 10 * time.Minute,
			},
			requested: 1 * time.Minute,
			wantErr:   false,
		},
		{
			name: "valid: equal to max",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 10 * time.Minute,
			},
			requested: 10 * time.Minute,
			wantErr:   false,
		},
		{
			name: "invalid: below min",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 10 * time.Minute,
			},
			requested: 30 * time.Second,
			wantErr:   true,
			errMsg:    "timeout 30s below minimum 1m0s",
		},
		{
			name: "invalid: above max",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 10 * time.Minute,
			},
			requested: 15 * time.Minute,
			wantErr:   true,
			errMsg:    "timeout 15m0s exceeds maximum 10m0s",
		},
		{
			name: "valid: no bounds set (zero config)",
			config: TimeoutConfig{
				Min: 0,
				Max: 0,
			},
			requested: 100 * time.Hour,
			wantErr:   false,
		},
		{
			name: "valid: only min set, above min",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
			},
			requested: 2 * time.Minute,
			wantErr:   false,
		},
		{
			name: "invalid: only min set, below min",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
			},
			requested: 30 * time.Second,
			wantErr:   true,
			errMsg:    "timeout 30s below minimum 1m0s",
		},
		{
			name: "valid: only max set, below max",
			config: TimeoutConfig{
				Max: 10 * time.Minute,
			},
			requested: 5 * time.Minute,
			wantErr:   false,
		},
		{
			name: "invalid: only max set, above max",
			config: TimeoutConfig{
				Max: 10 * time.Minute,
			},
			requested: 15 * time.Minute,
			wantErr:   true,
			errMsg:    "timeout 15m0s exceeds maximum 10m0s",
		},
		{
			name: "invalid: zero requested below min bound",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 10 * time.Minute,
			},
			requested: 0,
			wantErr:   true,
			errMsg:    "timeout 0s below minimum 1m0s",
		},
		{
			name: "edge case: very large timeout within bounds",
			config: TimeoutConfig{
				Max: 24 * time.Hour,
			},
			requested: 23 * time.Hour,
			wantErr:   false,
		},
		{
			name: "edge case: very small timeout within bounds",
			config: TimeoutConfig{
				Min: 1 * time.Millisecond,
			},
			requested: 1 * time.Second,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateTimeout(tt.requested)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateTimeout() expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateTimeout() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTimeout() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestTimeoutConfig_ResolveTimeout tests the precedence logic for timeout resolution.
func TestTimeoutConfig_ResolveTimeout(t *testing.T) {
	tests := []struct {
		name      string
		config    TimeoutConfig
		requested time.Duration
		want      time.Duration
	}{
		{
			name: "precedence: requested overrides default",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
			},
			requested: 2 * time.Minute,
			want:      2 * time.Minute,
		},
		{
			name: "precedence: requested overrides default and SDK default",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
			},
			requested: 1 * time.Hour,
			want:      1 * time.Hour,
		},
		{
			name: "precedence: config default overrides SDK default",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
			},
			requested: 0,
			want:      10 * time.Minute,
		},
		{
			name:      "precedence: SDK default when no config default",
			config:    TimeoutConfig{},
			requested: 0,
			want:      5 * time.Minute,
		},
		{
			name: "precedence: SDK default when config has only bounds",
			config: TimeoutConfig{
				Min: 1 * time.Minute,
				Max: 1 * time.Hour,
			},
			requested: 0,
			want:      5 * time.Minute,
		},
		{
			name: "requested: zero with default set",
			config: TimeoutConfig{
				Default: 3 * time.Minute,
			},
			requested: 0,
			want:      3 * time.Minute,
		},
		{
			name:      "requested: zero with no default",
			config:    TimeoutConfig{},
			requested: 0,
			want:      5 * time.Minute,
		},
		{
			name: "requested: very small value",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
			},
			requested: 1 * time.Millisecond,
			want:      1 * time.Millisecond,
		},
		{
			name: "requested: very large value",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
			},
			requested: 24 * time.Hour,
			want:      24 * time.Hour,
		},
		{
			name: "default: exactly 5 minutes (same as SDK default)",
			config: TimeoutConfig{
				Default: 5 * time.Minute,
			},
			requested: 0,
			want:      5 * time.Minute,
		},
		{
			name: "all three: requested wins",
			config: TimeoutConfig{
				Default: 10 * time.Minute,
				Min:     1 * time.Minute,
				Max:     1 * time.Hour,
			},
			requested: 30 * time.Minute,
			want:      30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ResolveTimeout(tt.requested)
			if got != tt.want {
				t.Errorf("ResolveTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTimeoutConfig_EdgeCases tests boundary conditions and special cases.
func TestTimeoutConfig_EdgeCases(t *testing.T) {
	t.Run("zero config is valid", func(t *testing.T) {
		var cfg TimeoutConfig
		if err := cfg.Validate(); err != nil {
			t.Errorf("zero config should be valid, got error: %v", err)
		}
	})

	t.Run("zero config accepts any timeout", func(t *testing.T) {
		var cfg TimeoutConfig
		timeouts := []time.Duration{
			1 * time.Nanosecond,
			1 * time.Second,
			1 * time.Hour,
			24 * time.Hour,
			1000 * time.Hour,
		}
		for _, timeout := range timeouts {
			if err := cfg.ValidateTimeout(timeout); err != nil {
				t.Errorf("zero config should accept %v, got error: %v", timeout, err)
			}
		}
	})

	t.Run("zero config resolves to SDK default", func(t *testing.T) {
		var cfg TimeoutConfig
		got := cfg.ResolveTimeout(0)
		want := 5 * time.Minute
		if got != want {
			t.Errorf("zero config with zero requested should return SDK default %v, got %v", want, got)
		}
	})

	t.Run("negative timeout requested", func(t *testing.T) {
		cfg := TimeoutConfig{
			Min: 1 * time.Minute,
			Max: 10 * time.Minute,
		}
		// Negative durations are less than min
		err := cfg.ValidateTimeout(-1 * time.Second)
		if err == nil {
			t.Error("negative timeout should fail validation")
		}
	})

	t.Run("max duration value", func(t *testing.T) {
		cfg := TimeoutConfig{
			Max: 1<<63 - 1, // max int64 nanoseconds
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("max duration value should be valid: %v", err)
		}
	})

	t.Run("identical min and max with matching default", func(t *testing.T) {
		cfg := TimeoutConfig{
			Default: 5 * time.Minute,
			Min:     5 * time.Minute,
			Max:     5 * time.Minute,
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("config with identical min/max/default should be valid: %v", err)
		}
		if err := cfg.ValidateTimeout(5 * time.Minute); err != nil {
			t.Errorf("timeout equal to identical min/max should be valid: %v", err)
		}
	})
}
