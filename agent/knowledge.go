// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"errors"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/types"
)

// ErrKnowledgeUnavailable reports that a knowledge read could not be served,
// as distinct from a read that found nothing.
//
// The distinction is the whole reason this error exists. The daemon deliberately
// leaves the graphrag seam nil when it has no embedder resolver, and answers
// Unimplemented — "unavailable" is a designed state, not an outage. Without a
// matchable sentinel the natural implementation of a failed read is "log it and
// carry on with no results", and the agent then proceeds as though the graph
// said "nothing known" when it actually said "I could not look". For a security
// agent that is a silent false negative: a clean prior history reported for a
// target nobody checked.
//
// So: a KnowledgeReader method returns this (wrapped) when its seam is absent,
// and NEVER an empty result with a nil error. An empty result means the tenant
// genuinely knows nothing.
//
//	hits, err := h.QueryNodes(ctx, q)
//	switch {
//	case errors.Is(err, agent.ErrKnowledgeUnavailable):
//	    // say so; do not conclude anything about the target
//	case err != nil:
//	    return err
//	case len(hits) == 0:
//	    // the graph really has nothing
//	}
var ErrKnowledgeUnavailable = errors.New("agent: knowledge reads are not available on this harness")

// RunScope selects which mission runs GetRunFindings reads.
//
// A scope value rather than one method per scope: the scope is data, and
// modelling it as data is what lets a caller pass it through. The daemon-side
// harness historically carried GetPreviousRunFindings and GetAllRunFindings as
// separate methods, which forced every caller to branch before calling.
type RunScope int32

const (
	// RunScopeUnspecified is the zero value and is rejected by the daemon.
	RunScopeUnspecified RunScope = iota
	// RunScopePrevious reads the run immediately before the current one.
	RunScopePrevious
	// RunScopeAll reads every run of this mission.
	RunScopeAll
)

// String implements fmt.Stringer.
func (s RunScope) String() string {
	switch s {
	case RunScopePrevious:
		return "previous"
	case RunScopeAll:
		return "all"
	case RunScopeUnspecified:
		return "unspecified"
	default:
		return "unspecified"
	}
}

// KnowledgeReader is the read-only view of what earlier work established: the
// tenant knowledge graph, previously submitted findings, and mission run
// history.
//
// READ-ONLY BY CONSTRUCTION. There is no write half and there will not be one:
// the projector is the sole graph writer (ADR-0012), and an agent contributes to
// the graph by emitting — see WorldEmitter — not by writing to it.
//
// Every method derives its tenant from the call context. No method takes a
// tenant argument, so an agent cannot name another tenant's graph; that is
// unrepresentable rather than rejected.
//
// Types follow one rule: the SDK-native type where one exists (finding.Filter,
// finding.Finding, types.MissionRunSummary), and the proto type where it does
// not. Only the graph nodes fall in the second case — their Go originals live in
// gibson's internal packages, which the SDK cannot import.
type KnowledgeReader interface {
	// QueryNodes searches the knowledge graph with hybrid vector + graph scoring.
	QueryNodes(ctx context.Context, query *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error)

	// FindSimilarAttacks returns attack patterns semantically close to content.
	FindSimilarAttacks(ctx context.Context, content string, topK int) ([]*graphragpb.AttackPattern, error)

	// GetAttackChains returns multi-hop technique paths from a starting technique.
	GetAttackChains(ctx context.Context, techniqueID string, maxDepth int) ([]*graphragpb.AttackChain, error)

	// FindSimilarFindings returns findings semantically close to the given one.
	FindSimilarFindings(ctx context.Context, findingID string, topK int) ([]*graphragpb.FindingNode, error)

	// GetRelatedFindings returns findings reachable from the given one by graph
	// relationship rather than by similarity.
	GetRelatedFindings(ctx context.Context, findingID string) ([]*graphragpb.FindingNode, error)

	// GetFindings returns previously submitted findings matching filter.
	GetFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error)

	// GetRunFindings returns findings from earlier runs of this mission.
	// Pair it with GetMissionRunHistory: list the runs, then read one.
	GetRunFindings(ctx context.Context, scope RunScope, filter finding.Filter) ([]*finding.Finding, error)

	// GetMissionRunHistory returns every run of the caller's mission, oldest first.
	GetMissionRunHistory(ctx context.Context) ([]types.MissionRunSummary, error)

	// ApplicationFindings returns one Application's Findings with the lifecycle
	// context that decides how much each one matters: whether a Deployment of
	// this Application actually runs the code the Finding is in, and whether
	// that Deployment is exposed.
	//
	// statuses filters by lifecycle status ("open", "fixing", "fixed",
	// "verified"); nil or empty means every status. limit bounds the read; 0
	// takes the server default, and the server caps it.
	//
	// The other reads on this interface are search — text or embedding, a
	// node-type filter, top-k. This one is a traversal, and it exists because no
	// top-k over a node-type filter can answer "does anything run this". The
	// distinction matters because of what an agent does with an unanswerable
	// question: a triage rule reads "not reachable" as "nothing runs this" and
	// ranks the finding last, so an unavailable read would silently bury a live
	// backlog rather than surface as an error. Hence the standing rule holds
	// hardest here — unavailability returns ErrKnowledgeUnavailable, never an
	// empty slice with a nil error.
	ApplicationFindings(ctx context.Context, application string, statuses []string, limit int) ([]ApplicationFinding, error)
}

// ApplicationFinding is one Finding of an Application, with the lifecycle
// context that decides how much it matters.
type ApplicationFinding struct {
	// FindingID is the Finding's brain_id — the same identity every other
	// surface uses, so an agent can write back to the node it read.
	FindingID string
	// Status is the Finding's lifecycle status: open, fixing, fixed, verified.
	Status string
	// Severity is the severity recorded when the Finding was raised.
	Severity string
	// VulnerabilityID identifies the weakness this Finding instantiates (a CVE,
	// a GHSA, or a platform id). Empty when the Finding names no public
	// identifier — a source finding, say — which is a fact about the Finding,
	// not a failure of the read.
	VulnerabilityID string
	// PlaceLabel and PlaceKey are what the Finding affects: a Package in an
	// image, a Repository, or a Service on a host.
	PlaceLabel string
	PlaceKey   string
	// Reachable reports that the affected place is inside an Image that a
	// Deployment of this Application runs. False means nothing this Application
	// deploys contains it.
	Reachable bool
	// Exposed reports that the Deployment which reaches it also exposes a Host.
	// Only ever true when Reachable is.
	Exposed bool
	// DeploymentKey and ImageKey name the route that made it reachable, so an
	// agent can say WHY rather than assert it.
	DeploymentKey string
	ImageKey      string
	// Priority, PriorityRule and PriorityReason are what a previous triage pass
	// decided about this Finding, written back by the agent and returned here so
	// the next pass can read its own history (gibson#1684).
	//
	// Two behaviours are dark while they are empty. A triage rule that keeps the
	// previous priority when a scoring feed is unavailable — so an outage never
	// re-ranks a Finding on severity alone — has nothing to keep. And a pass that
	// explains only what changed sees every decision as a change, so model cost
	// scales with the size of the backlog rather than with what actually
	// happened.
	//
	// Empty means NO PASS HAS DECIDED YET. That is a fact about the Finding, not
	// a failure of the read, and a reader must never take it for "unimportant".
	// The three are written by different steps — a rule table decides the
	// priority, a model writes the reason — so an empty PriorityReason beside a
	// populated Priority is a model that went quiet, not a Finding nobody ranked.
	Priority       string
	PriorityRule   string
	PriorityReason string
}
