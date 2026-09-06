// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package report renders per-consumer result tables.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/zeroroot-ai/sdk/cmd/sdk-bump/bumper"
)

// Summary wraps a slice of Results with formatting helpers.
type Summary struct {
	Version string
	Results []bumper.Result
	DryRun  bool
}

// HasFailures returns true if any result has a non-nil Err.
func (s Summary) HasFailures() bool {
	for _, r := range s.Results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// WriteText writes a human-readable table to w.
func (s Summary) WriteText(w io.Writer) {
	mode := "LIVE"
	if s.DryRun {
		mode = "DRY-RUN"
	}
	fmt.Fprintf(w, "\n=== sdk-bump %s summary (SDK %s) ===\n\n", mode, s.Version)

	// Header.
	fmt.Fprintf(w, "%-20s  %-10s  %-50s  %s\n", "CONSUMER", "STATUS", "PR / BRANCH", "NOTES")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 100))

	for _, r := range s.Results {
		status := "OK"
		notes := ""
		ref := r.PRURL
		if ref == "" {
			ref = r.Branch
		}
		if r.Err != nil {
			status = "FAILED"
			notes = truncate(r.Err.Error(), 40)
		} else if s.DryRun {
			status = "DRY-RUN"
		}
		fmt.Fprintf(w, "%-20s  %-10s  %-50s  %s\n", r.Consumer, status, ref, notes)
	}

	fmt.Fprintln(w)

	if s.HasFailures() {
		fmt.Fprintf(w, "RESULT: %d/%d consumers FAILED\n", s.failCount(), len(s.Results))
	} else {
		fmt.Fprintf(w, "RESULT: all %d consumers OK\n", len(s.Results))
	}
}

// WriteJSON writes a machine-readable JSON representation to w.
func (s Summary) WriteJSON(w io.Writer) error {
	type stepJSON struct {
		Name    string `json:"name"`
		Success bool   `json:"success"`
		Output  string `json:"output,omitempty"`
	}
	type resultJSON struct {
		Consumer string     `json:"consumer"`
		Branch   string     `json:"branch"`
		PRURL    string     `json:"pr_url,omitempty"`
		Success  bool       `json:"success"`
		Error    string     `json:"error,omitempty"`
		Steps    []stepJSON `json:"steps"`
	}
	type summaryJSON struct {
		Version string       `json:"version"`
		DryRun  bool         `json:"dry_run"`
		Results []resultJSON `json:"results"`
		AllOK   bool         `json:"all_ok"`
	}

	out := summaryJSON{
		Version: s.Version,
		DryRun:  s.DryRun,
		AllOK:   !s.HasFailures(),
	}
	for _, r := range s.Results {
		rj := resultJSON{
			Consumer: r.Consumer,
			Branch:   r.Branch,
			PRURL:    r.PRURL,
			Success:  r.Err == nil,
		}
		if r.Err != nil {
			rj.Error = r.Err.Error()
		}
		for _, st := range r.Steps {
			rj.Steps = append(rj.Steps, stepJSON{
				Name:    st.Name,
				Success: st.Success,
				Output:  st.Output,
			})
		}
		out.Results = append(out.Results, rj)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (s Summary) failCount() int {
	n := 0
	for _, r := range s.Results {
		if r.Err != nil {
			n++
		}
	}
	return n
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
