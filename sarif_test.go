// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"time"

	"github.com/zeroroot-ai/sdk/finding"
)

// newFinding is a convenience helper that returns a minimal Finding.
func newFinding(id, title, desc, category string, sev finding.Severity, targetID string) finding.Finding {
	now := time.Now()
	return finding.Finding{
		ID:          id,
		MissionID:   "mission-1",
		AgentName:   "test-agent",
		Title:       title,
		Description: desc,
		Category:    category,
		Severity:    sev,
		Status:      finding.StatusOpen,
		TargetID:    targetID,
		Confidence:  1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
