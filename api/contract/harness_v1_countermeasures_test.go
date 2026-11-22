// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestHarnessV1_Technique_Validation: the technique of a countermeasure read
// and of a finding is an ATT&CK technique id, or empty.
func TestHarnessV1_Technique_Validation(t *testing.T) {
	v := newValidator(t)

	for _, ok := range []string{"", "T1190", "T1059.001"} {
		if err := v.Validate(&harnesspb.GetCountermeasuresRequest{Technique: ok}); err != nil {
			t.Errorf("request technique %q: unexpected error: %v", ok, err)
		}
		if err := v.Validate(&harnesspb.ApplicationFinding{Technique: ok}); err != nil {
			t.Errorf("finding technique %q: unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"t1190", "T119", "T11900", "T1059.01", "T1059.0011", "D3-FA", "T1190 ", "attack"} {
		if err := v.Validate(&harnesspb.GetCountermeasuresRequest{Technique: bad}); err == nil {
			t.Errorf("request technique %q accepted", bad)
		}
		if err := v.Validate(&harnesspb.ApplicationFinding{Technique: bad}); err == nil {
			t.Errorf("finding technique %q accepted", bad)
		}
	}
}

// TestHarnessV1_FixFiles_Validation: the head and the files of a fix merge
// request have bounds, so a value that the daemon fills cannot grow without
// limit or carry a malformed id.
func TestHarnessV1_FixFiles_Validation(t *testing.T) {
	v := newValidator(t)
	sha40, sha64 := strings.Repeat("a", 40), strings.Repeat("b", 64)
	good := func() *harnesspb.FixFile {
		return &harnesspb.FixFile{
			Path: "a.go", Change: harnesspb.FixFileChange_FIX_FILE_CHANGE_MODIFIED, BlobSha: sha40,
		}
	}
	renamed := func() *harnesspb.FixFile {
		f := good()
		f.Change, f.PreviousPath = harnesspb.FixFileChange_FIX_FILE_CHANGE_RENAMED, "old.go"
		return f
	}
	many := make([]*harnesspb.FixFile, 101)
	for i := range many {
		many[i] = good()
	}
	deleted := good()
	deleted.Change, deleted.BlobSha = harnesspb.FixFileChange_FIX_FILE_CHANGE_DELETED, ""

	for name, msg := range map[string]proto.Message{
		"a sha1 head":                &harnesspb.ApplicationFinding{FixHeadSha: sha40},
		"a sha256 head":              &harnesspb.ApplicationFinding{FixHeadSha: sha64},
		"no head":                    &harnesspb.ApplicationFinding{},
		"a modified file":            good(),
		"a renamed file":             renamed(),
		"a deleted file with no sha": deleted,
		"100 files":                  &harnesspb.ApplicationFinding{FixFiles: many[:100]},
	} {
		if err := v.Validate(msg); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
	}

	longPath := good()
	longPath.Path = strings.Repeat("p", 4097)
	longPrev := renamed()
	longPrev.PreviousPath = strings.Repeat("p", 4097)
	noPath := good()
	noPath.Path = ""
	noChange := good()
	noChange.Change = harnesspb.FixFileChange_FIX_FILE_CHANGE_UNSPECIFIED
	unknownChange := good()
	unknownChange.Change = harnesspb.FixFileChange(99)
	prevOnModify := good()
	prevOnModify.PreviousPath = "old.go"
	renameNoPrev := renamed()
	renameNoPrev.PreviousPath = ""
	badBlob := good()
	badBlob.BlobSha = "ABC"
	upperBlob := good()
	upperBlob.BlobSha = strings.Repeat("A", 40)

	for name, msg := range map[string]proto.Message{
		"a short head":                   &harnesspb.ApplicationFinding{FixHeadSha: "abc123"},
		"an upper case head":             &harnesspb.ApplicationFinding{FixHeadSha: strings.Repeat("A", 40)},
		"a head of 41 characters":        &harnesspb.ApplicationFinding{FixHeadSha: strings.Repeat("a", 41)},
		"101 files":                      &harnesspb.ApplicationFinding{FixFiles: many},
		"a path over 4096":               longPath,
		"a previous path over 4096":      longPrev,
		"no path":                        noPath,
		"no change":                      noChange,
		"an unknown change":              unknownChange,
		"a previous path on a modify":    prevOnModify,
		"a rename with no previous path": renameNoPrev,
		"a short blob sha":               badBlob,
		"an upper case blob sha":         upperBlob,
	} {
		if err := v.Validate(msg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
