# Cohesive method clusters on Harness are embedded sub-interfaces

`agent.Harness` carried 27 flat methods whose grouping existed only as section
comments, and those comments had drifted: `DelegateToAgent`, `ListAgents` and
`SubmitFinding` sat under `// Plugin Access Methods`, `Observe` and `WorldView`
under `// Context Access`, and the five LLM methods had no header at all. So
cohesive clusters become **embedded sub-interfaces** — `LLMCaller`, `ToolCaller`,
`PluginCaller`, `Delegator`, `WorldEmitter`, `WorldReader`,
`KnowledgeReader`, `Planner`, `WorkspaceAccess`, alongside the `MissionManager`
that already existed.

Comments do not compile. `MissionManager` was the one group expressed as a type,
and the one that had not rotted; that is the whole argument.

## Consequences

- **Implementer cost is unchanged.** Embedding an interface inside an interface
  does not alter what a concrete type must implement. The grouping is therefore
  free, which is why "smaller diff" was not a reason to skip it.
- **A caller can ask for exactly the capability it needs.** A function taking a
  `WorldReader` cannot emit. That is the point of splitting `WorldEmitter` from
  `WorldReader`: ADR-0012 makes the projector the sole graph writer, and a split
  gives that constraint a type instead of a comment. `Emitter` rather than
  `Writer` because `SubmitFinding` and `Observe` are World emissions the
  projector later consumes, not graph writes.
- **Groups are named identically on gibson's `harness.AgentHarness`**, which is a
  second interface for the same concept that has drifted from this one (30
  methods against 27, with no compiler comparing them). Shared names make that
  drift a diff of named types rather than an eyeball comparison of 57
  signatures. **Membership is deliberately not converged**: some divergence is a
  real boundary — `Metrics()` and `MissionExecutionContext()` are daemon-internal
  and do not belong on a public component-dev surface — and some is rot. Naming
  the groups turns each difference into a small reviewable question instead of
  one large semantic argument nobody reads.
- **A group with no members on one side is simply not embedded there.** No empty
  marker interfaces.
- Single-purpose accessors that belong to no cluster stay flat: `Authorize`,
  `Mission`, `Target`, `Tracer`, `Logger`, `TokenUsage`.

## The rule this exists to hold

A new method on `Harness` joins an existing group, or comes with a new one. It
does not get added flat. Without that rule the next addition restarts the drift,
and the section comments this ADR replaced are the evidence that it will.
