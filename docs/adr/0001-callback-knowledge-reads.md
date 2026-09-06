# HarnessCallbackService carries the knowledge-graph reads

A dispatched agent authenticates with the **task-scoped** grant gibson mints per
dispatch (`callback_endpoint` + `callback_token`), so everything it needs must be
reachable on `HarnessCallbackService`. The knowledge reads existed only on
`ComponentService`, which meant a dispatched run had to keep a component-scoped
grant alive purely to call `recall` — so we mirrored the six read RPCs onto
`HarnessCallbackService` rather than accept that. Duplication on the wire buys a
run that holds exactly the authority its dispatch granted, and nothing more.

## Considered options

**Drop the capability.** Dispatched runs lose `recall` and the ambient-knowledge
block; the graph becomes write-only for them. Rejected: reading prior findings
before acting is the thing that makes an agent sharpen across runs, and it is
what the product is *for*. A clean authority story that guts the feature is not
a win.

**Split the grants** — harness operations on the task grant, knowledge reads on
the component grant. Rejected, and it is the tempting one: it looks incremental
but the child still holds the component grant, so the authority gap is not
closed at all. It buys the complexity of two clients and none of the benefit.

## Consequences

- The six reads (`QueryNodes`, `FindSimilarAttacks`, `GetAttackChains`,
  `FindSimilarFindings`, `GetRelatedFindings`, `GetFindings`) now exist on both
  services and must stay behaviourally identical.

  **How that is achieved was stated wrongly when this ADR was accepted.** It
  claimed both services "delegate to the same graphrag querier on the gibson
  side, so there is one implementation, not two." They do not, and at the time
  they could not: `ComponentServiceServer` holds a `GraphRAGQuerier`, but
  `HarnessCallbackService` has no such field, `DefaultAgentHarness` has no
  graphrag dependency at all, and the SDK's `agent.Harness` exposes none of these
  reads. There was no shared path to delegate to.

  The single implementation is something to *build*, not something that already
  held: the reads become a `KnowledgeReader` group on the harness, the daemon
  wires its existing `PoolGraphRAGQuerier` into `DefaultAgentHarness`, and both
  services reach it — `ComponentService` directly, `HarnessCallbackService`
  through the `getHarness(ctx, req.Context)` resolution every other callback
  handler already uses. Until that lands, this ADR describes an intent, not a
  mechanism.

  Recorded rather than quietly corrected, because the gap between "these two
  surfaces agree" and "these two surfaces are the same code" is exactly the class
  of drift this ADR exists to close, and an ADR that mis-states its own mechanism
  reproduces it.
- Only the **read** half is mirrored. The projector is the sole graph writer
  (ADR-0012) and sdk#451 already removed the generic graph-write RPC from
  `ComponentService`; mirroring writes here would reintroduce what that removed.
- The callback variants carry `ContextInfo` instead of a bare `work_id`. That is
  this service's existing convention, and it also names `mission_run_id`, which
  mission-scoped GraphRAG reads need and a `work_id` cannot express.
- Adding RPCs is additive under the module's `WIRE` breaking policy, so no
  consumer breaks. Removing them later would not be — this is the hard-to-reverse
  part.

Related: zerocool-plugins ADR-0006 (the dispatched `kind=agent` shape that
surfaced the gap), zerocool-plugins#33.
