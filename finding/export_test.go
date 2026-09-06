// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"testing"
	"time"
)

func TestExportFormat_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		format ExportFormat
		want   bool
	}{
		{"json is valid", FormatJSON, true},
		{"sarif is valid", FormatSARIF, true},
		{"csv is valid", FormatCSV, true},
		{"html is valid", FormatHTML, true},
		{"empty is invalid", ExportFormat(""), false},
		{"unknown is invalid", ExportFormat("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.IsValid(); got != tt.want {
				t.Errorf("ExportFormat.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportFormat_String(t *testing.T) {
	tests := []struct {
		name   string
		format ExportFormat
		want   string
	}{
		{"json", FormatJSON, "json"},
		{"sarif", FormatSARIF, "sarif"},
		{"csv", FormatCSV, "csv"},
		{"html", FormatHTML, "html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.String(); got != tt.want {
				t.Errorf("ExportFormat.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportFormat_FileExtension(t *testing.T) {
	tests := []struct {
		name   string
		format ExportFormat
		want   string
	}{
		{"json extension", FormatJSON, ".json"},
		{"sarif extension", FormatSARIF, ".sarif"},
		{"csv extension", FormatCSV, ".csv"},
		{"html extension", FormatHTML, ".html"},
		{"invalid extension", ExportFormat("invalid"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.FileExtension(); got != tt.want {
				t.Errorf("ExportFormat.FileExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportFormat_MimeType(t *testing.T) {
	tests := []struct {
		name   string
		format ExportFormat
		want   string
	}{
		{"json mime", FormatJSON, "application/json"},
		{"sarif mime", FormatSARIF, "application/sarif+json"},
		{"csv mime", FormatCSV, "text/csv"},
		{"html mime", FormatHTML, "text/html"},
		{"invalid mime", ExportFormat("invalid"), "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.MimeType(); got != tt.want {
				t.Errorf("ExportFormat.MimeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"open is valid", StatusOpen, true},
		{"confirmed is valid", StatusConfirmed, true},
		{"resolved is valid", StatusResolved, true},
		{"false_positive is valid", StatusFalsePositive, true},
		{"empty is invalid", Status(""), false},
		{"unknown is invalid", Status("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{"open", StatusOpen, "open"},
		{"confirmed", StatusConfirmed, "confirmed"},
		{"resolved", StatusResolved, "resolved"},
		{"false_positive", StatusFalsePositive, "false_positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("Status.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_DisplayName(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{"open", StatusOpen, "Open"},
		{"confirmed", StatusConfirmed, "Confirmed"},
		{"resolved", StatusResolved, "Resolved"},
		{"false_positive", StatusFalsePositive, "False Positive"},
		{"unknown", Status("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.DisplayName(); got != tt.want {
				t.Errorf("Status.DisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_Matches(t *testing.T) {
	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	baseFinding := Finding{
		ID:        "find-1",
		MissionID: "mission-1",
		AgentName: "agent-1",
		Title:     "Test Finding",
		Category:  string(CategoryJailbreak),
		Severity:  SeverityHigh,
		Status:    StatusOpen,
		Tags:      []string{"tag1", "tag2"},
		RiskScore: 7.5,
		CreatedAt: baseTime,
		UpdatedAt: baseTime,
	}

	tests := []struct {
		name    string
		filter  Filter
		finding Finding
		want    bool
	}{
		{
			name:    "empty filter matches all",
			filter:  Filter{},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "mission id matches",
			filter: Filter{
				MissionID: "mission-1",
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "mission id doesn't match",
			filter: Filter{
				MissionID: "mission-2",
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "agent name matches",
			filter: Filter{
				AgentName: "agent-1",
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "agent name doesn't match",
			filter: Filter{
				AgentName: "agent-2",
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "category matches",
			filter: Filter{
				Categories: []string{string(CategoryJailbreak), string(CategoryPromptInjection)},
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "category doesn't match",
			filter: Filter{
				Categories: []string{string(CategoryDOS)},
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "severity matches",
			filter: Filter{
				Severities: []Severity{SeverityHigh, SeverityCritical},
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "severity doesn't match",
			filter: Filter{
				Severities: []Severity{SeverityLow},
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "status matches",
			filter: Filter{
				Status: StatusOpen,
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "status doesn't match",
			filter: Filter{
				Status: StatusResolved,
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "tag matches",
			filter: Filter{
				Tags: []string{"tag1"},
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "tag doesn't match",
			filter: Filter{
				Tags: []string{"tag3"},
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "min score passes",
			filter: Filter{
				MinScore: 5.0,
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "min score fails",
			filter: Filter{
				MinScore: 8.0,
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "created after passes",
			filter: Filter{
				CreatedAfter: baseTime.Add(-1 * time.Hour),
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "created after fails",
			filter: Filter{
				CreatedAfter: baseTime.Add(1 * time.Hour),
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "created before passes",
			filter: Filter{
				CreatedBefore: baseTime.Add(1 * time.Hour),
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "created before fails",
			filter: Filter{
				CreatedBefore: baseTime.Add(-1 * time.Hour),
			},
			finding: baseFinding,
			want:    false,
		},
		{
			name: "multiple filters all match",
			filter: Filter{
				MissionID:  "mission-1",
				Categories: []string{string(CategoryJailbreak)},
				MinScore:   7.0,
			},
			finding: baseFinding,
			want:    true,
		},
		{
			name: "multiple filters one doesn't match",
			filter: Filter{
				MissionID:  "mission-1",
				Categories: []string{string(CategoryDOS)},
				MinScore:   7.0,
			},
			finding: baseFinding,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Matches(tt.finding); got != tt.want {
				t.Errorf("Filter.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_Validate(t *testing.T) {
	tests := []struct {
		name    string
		filter  Filter
		wantErr bool
	}{
		{
			name:    "valid empty filter",
			filter:  Filter{},
			wantErr: false,
		},
		{
			name: "valid complete filter",
			filter: Filter{
				MissionID:     "mission-1",
				Categories:    []string{string(CategoryJailbreak)},
				Severities:    []Severity{SeverityHigh},
				Status:        StatusOpen,
				MinScore:      5.0,
				CreatedAfter:  time.Now().Add(-24 * time.Hour),
				CreatedBefore: time.Now(),
				Limit:         10,
				Offset:        0,
			},
			wantErr: false,
		},
		{
			name: "empty category string",
			filter: Filter{
				Categories: []string{""},
			},
			wantErr: true,
		},
		{
			name: "invalid severity",
			filter: Filter{
				Severities: []Severity{Severity("invalid")},
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			filter: Filter{
				Status: Status("invalid"),
			},
			wantErr: true,
		},
		{
			name: "negative min score",
			filter: Filter{
				MinScore: -1.0,
			},
			wantErr: true,
		},
		{
			name: "negative limit",
			filter: Filter{
				Limit: -1,
			},
			wantErr: true,
		},
		{
			name: "negative offset",
			filter: Filter{
				Offset: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid time range",
			filter: Filter{
				CreatedAfter:  time.Now(),
				CreatedBefore: time.Now().Add(-24 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Filter.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseExportFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ExportFormat
		wantErr bool
	}{
		{"parse json", "json", FormatJSON, false},
		{"parse sarif", "sarif", FormatSARIF, false},
		{"parse csv", "csv", FormatCSV, false},
		{"parse html", "html", FormatHTML, false},
		{"invalid format", "invalid", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseExportFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExportFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseExportFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Status
		wantErr bool
	}{
		{"parse open", "open", StatusOpen, false},
		{"parse confirmed", "confirmed", StatusConfirmed, false},
		{"parse resolved", "resolved", StatusResolved, false},
		{"parse false_positive", "false_positive", StatusFalsePositive, false},
		{"invalid status", "invalid", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStatus(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllExportFormats(t *testing.T) {
	formats := AllExportFormats()
	if len(formats) != 4 {
		t.Errorf("AllExportFormats() returned %d formats, want 4", len(formats))
	}

	expected := []ExportFormat{FormatJSON, FormatSARIF, FormatCSV, FormatHTML}
	for i, format := range expected {
		if formats[i] != format {
			t.Errorf("AllExportFormats()[%d] = %v, want %v", i, formats[i], format)
		}
	}
}

func TestAllStatuses(t *testing.T) {
	statuses := AllStatuses()
	if len(statuses) != 4 {
		t.Errorf("AllStatuses() returned %d statuses, want 4", len(statuses))
	}

	expected := []Status{StatusOpen, StatusConfirmed, StatusResolved, StatusFalsePositive}
	for i, status := range expected {
		if statuses[i] != status {
			t.Errorf("AllStatuses()[%d] = %v, want %v", i, statuses[i], status)
		}
	}
}
