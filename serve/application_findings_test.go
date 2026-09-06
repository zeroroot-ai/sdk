// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestApplicationFindingsFromProto_MapsEveryField: a field dropped here is not a
// cosmetic loss. reachable and exposed decide where a triage rule ranks the
// finding, and deployment_key / image_key are how an agent says WHY rather than
// asserting it — so each one is checked by name rather than by struct equality,
// which would pass on a zero value silently mapped from the wrong field.
func TestApplicationFindingsFromProto_MapsEveryField(t *testing.T) {
	out := applicationFindingsFromProto([]*harnesspb.ApplicationFinding{{
		FindingId:       "brain-1",
		Status:          "open",
		Severity:        "critical",
		VulnerabilityId: "CVE-2025-1234",
		PlaceLabel:      "Package",
		PlaceKey:        "pkg:npm/lodash@4.17.20",
		Reachable:       true,
		Exposed:         true,
		DeploymentKey:   "examplebank/customer-portal",
		ImageKey:        "sha256:abc",
		Priority:        "P1",
		PriorityRule:    "R01",
		PriorityReason:  "listed in CISA KEV",
	}})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	f := out[0]
	for _, c := range []struct{ name, got, want string }{
		{"FindingID", f.FindingID, "brain-1"},
		{"Status", f.Status, "open"},
		{"Severity", f.Severity, "critical"},
		{"VulnerabilityID", f.VulnerabilityID, "CVE-2025-1234"},
		{"PlaceLabel", f.PlaceLabel, "Package"},
		{"PlaceKey", f.PlaceKey, "pkg:npm/lodash@4.17.20"},
		{"DeploymentKey", f.DeploymentKey, "examplebank/customer-portal"},
		{"ImageKey", f.ImageKey, "sha256:abc"},
		{"Priority", f.Priority, "P1"},
		{"PriorityRule", f.PriorityRule, "R01"},
		{"PriorityReason", f.PriorityReason, "listed in CISA KEV"},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
	if !f.Reachable || !f.Exposed {
		t.Errorf("reachable/exposed not mapped: %+v", f)
	}
}

// TestApplicationFindingsFromProto_UnreachableStaysFalse: the default must be
// the honest one. A mapping that flipped these would rank a buried finding as
// live; one that dropped them would rank a live finding as buried.
func TestApplicationFindingsFromProto_UnreachableStaysFalse(t *testing.T) {
	out := applicationFindingsFromProto([]*harnesspb.ApplicationFinding{{
		FindingId: "brain-2", Status: "open", Reachable: false, Exposed: false,
	}})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	if out[0].Reachable || out[0].Exposed {
		t.Fatalf("expected unreachable and unexposed, got %+v", out[0])
	}
}

// TestApplicationFindingsFromProto_NilEntryIsSkipped: a nil element on the wire
// must not become a zero-valued finding, which would read as a real, unreachable
// finding with no identity.
func TestApplicationFindingsFromProto_NilEntryIsSkipped(t *testing.T) {
	out := applicationFindingsFromProto([]*harnesspb.ApplicationFinding{
		nil,
		{FindingId: "brain-3"},
		nil,
	})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(out), out)
	}
	if out[0].FindingID != "brain-3" {
		t.Fatalf("wrong finding survived: %+v", out[0])
	}
}

// TestApplicationFindingsFromProto_EmptyIsEmpty: an Application with nothing
// open returns an empty slice and no error. That is the one case where empty is
// the truth — every failure path returns an error instead, which is why the
// caller can trust an empty result to mean "nothing matched".
func TestApplicationFindingsFromProto_EmptyIsEmpty(t *testing.T) {
	if out := applicationFindingsFromProto(nil); len(out) != 0 {
		t.Fatalf("expected empty, got %+v", out)
	}
}

// TestApplicationFindingsFromProto_UntriagedInventsNothing: a Finding no pass has
// ranked arrives with the three priority fields empty, and the mapping must leave
// them empty rather than substituting a default.
//
// A default here would be indistinguishable from a decision. "P4" invented for an
// unranked Finding reads to the next pass as "a previous pass looked at this and
// ranked it last", so the rule that keeps a previous priority through a scoring
// outage would keep a value nobody ever decided — and the Finding would never be
// triaged, quietly, forever.
func TestApplicationFindingsFromProto_UntriagedInventsNothing(t *testing.T) {
	out := applicationFindingsFromProto([]*harnesspb.ApplicationFinding{{
		FindingId: "brain-4", Status: "open", Severity: "high",
	}})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	f := out[0]
	if f.Priority != "" || f.PriorityRule != "" || f.PriorityReason != "" {
		t.Fatalf("untriaged finding must carry no priority, got %+v", f)
	}
}

// TestApplicationFindingsFromProto_QuietModelKeepsTheDecision: the priority and
// the reason are written by different steps — a rule table decides, a model
// explains — so a Finding can legitimately arrive ranked but unexplained.
//
// The mapping must carry the decision through on its own. Treating the missing
// reason as "incomplete" and dropping the priority with it would let a model
// outage erase rankings that were computed deterministically and never needed the
// model at all.
func TestApplicationFindingsFromProto_QuietModelKeepsTheDecision(t *testing.T) {
	out := applicationFindingsFromProto([]*harnesspb.ApplicationFinding{{
		FindingId: "brain-5", Status: "open",
		Priority: "P1", PriorityRule: "R01", PriorityReason: "",
	}})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	f := out[0]
	if f.Priority != "P1" || f.PriorityRule != "R01" {
		t.Fatalf("a missing reason took the decision with it: %+v", f)
	}
	if f.PriorityReason != "" {
		t.Fatalf("expected no reason, got %q", f.PriorityReason)
	}
}
