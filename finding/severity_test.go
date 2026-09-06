// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import "testing"

func TestSeverity_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{"critical is valid", SeverityCritical, true},
		{"high is valid", SeverityHigh, true},
		{"medium is valid", SeverityMedium, true},
		{"low is valid", SeverityLow, true},
		{"info is valid", SeverityInfo, true},
		{"empty is invalid", Severity(""), false},
		{"unknown is invalid", Severity("unknown"), false},
		{"invalid is invalid", Severity("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.IsValid(); got != tt.want {
				t.Errorf("Severity.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverity_Weight(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     float64
	}{
		{"critical weight", SeverityCritical, 10.0},
		{"high weight", SeverityHigh, 7.5},
		{"medium weight", SeverityMedium, 5.0},
		{"low weight", SeverityLow, 2.5},
		{"info weight", SeverityInfo, 1.0},
		{"invalid weight", Severity("invalid"), 0.0},
		{"empty weight", Severity(""), 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.Weight(); got != tt.want {
				t.Errorf("Severity.Weight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     string
	}{
		{"critical string", SeverityCritical, "critical"},
		{"high string", SeverityHigh, "high"},
		{"medium string", SeverityMedium, "medium"},
		{"low string", SeverityLow, "low"},
		{"info string", SeverityInfo, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("Severity.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Severity
		wantErr bool
	}{
		{"parse critical", "critical", SeverityCritical, false},
		{"parse high", "high", SeverityHigh, false},
		{"parse medium", "medium", SeverityMedium, false},
		{"parse low", "low", SeverityLow, false},
		{"parse info", "info", SeverityInfo, false},
		{"invalid severity", "invalid", "", true},
		{"empty string", "", "", true},
		{"unknown severity", "unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSeverity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSeverity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareSeverity(t *testing.T) {
	tests := []struct {
		name string
		s1   Severity
		s2   Severity
		want int
	}{
		{"critical > high", SeverityCritical, SeverityHigh, 1},
		{"high > medium", SeverityHigh, SeverityMedium, 1},
		{"medium > low", SeverityMedium, SeverityLow, 1},
		{"low > info", SeverityLow, SeverityInfo, 1},
		{"critical == critical", SeverityCritical, SeverityCritical, 0},
		{"high < critical", SeverityHigh, SeverityCritical, -1},
		{"info < low", SeverityInfo, SeverityLow, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSeverity(tt.s1, tt.s2)
			if (got < 0 && tt.want >= 0) || (got > 0 && tt.want <= 0) || (got == 0 && tt.want != 0) {
				t.Errorf("CompareSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllSeverities(t *testing.T) {
	severities := AllSeverities()
	if len(severities) != 5 {
		t.Errorf("AllSeverities() returned %d severities, want 5", len(severities))
	}

	expected := []Severity{
		SeverityCritical,
		SeverityHigh,
		SeverityMedium,
		SeverityLow,
		SeverityInfo,
	}

	for i, sev := range expected {
		if severities[i] != sev {
			t.Errorf("AllSeverities()[%d] = %v, want %v", i, severities[i], sev)
		}
	}
}
