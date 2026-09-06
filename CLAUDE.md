# sdk — CLAUDE.md

> **Workflow rules:** see [`zeroroot-ai/.github` → `AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md) — canonical for branching / commits / PRs / releases / merging. Conventional Commits MANDATORY. Never push to main. Never force-push.

This file is the per-repo addendum. Workspace-wide concerns live in [`~/Code/zeroroot.ai/CLAUDE.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md); architectural decisions in ``docs/adr/`` (local docs → `adr`).

## TL;DR

The official Go SDK for building agents, tools, and plugins against the Gibson platform. Public OSS — every type and function is customer-visible. Entry point: `make check` (fmt + vet + test + check-coverage + check-no-gibson + check-buf-pinned + proto-breaking).

`make check` deliberately does **not** run golangci-lint: it type-checks the whole module (~3 GB resident, a full core for minutes) and the repos in this workspace share one machine. CI runs it directly via `make lint LINT_BASE=…`. Run `make lint` by hand when you want it locally.

This is a **library**, not a binary. `make examples` compiles the example binaries under `examples/`.

## Architecture

The SDK exposes two service stubs to customer code: `DaemonService` (agent/tool/plugin lifecycle, mission management, CUE authoring) in `api/proto/gibson/daemon/v1/` and `TenantService` (agent identity, provider config, drafts, usage) in `api/proto/gibson/tenant/v1/`. Internal platform services (`DaemonAdminService`, the old `TenantAdminService`) and all first-party infra clients (Vault, Redis, Neo4j, OpenFGA admin) live in `enterprise/platform/platform-clients` and `opensource/platform-sdk` — never here. The deny-list is enforced by `make check-no-gibson` (AST walker in `check_no_gibson_test.go`) and a transitive-module ceiling (≤30 modules).

Proto files live under `api/proto/`; generated bindings under `api/gen/` are **tracked source** (committed normally — Go consumers fetch this module by tag and never run protoc, so the bindings are the published artifact). A CI drift gate (`make generate && git diff --exit-code api/gen/`) keeps them in sync; codegen is deterministic (plugin versions pinned in the Makefile). Taxonomy types live under `taxonomy/core.yaml` — run `make taxonomy-gen` then `make proto` to propagate changes.

See `ADR-0025`, `ADR-0030`, and `ADR-0036`.

## Regen commands

```bash
make proto          # regenerate Go bindings from api/proto/ via buf
make generate       # full: taxonomy-gen → proto-clean → buf generate
make taxonomy-gen   # regenerate domain/validators/query/helpers from taxonomy/core.yaml
make proto-authz-registry-emit  # emit authz registry to tmp/authz/ (verify only)
```

## Gotchas

- **`api/gen/` is tracked source — commit it normally.** (Previously gitignored + force-added on tags, which silently dropped new files and shipped an incomplete v0.129.0; fixed per ADR-0039.) After changing a proto, run `make generate` and `git add api/gen/`; the CI `proto bindings drift gate` fails if you forget.
- **Proto generation uses an isolated toolchain** under `bin/tools/` to avoid version drift from the global `$GOBIN`. `make proto-deps` installs pinned versions; do not use a system-level `protoc-gen-go`.
- **`check-no-gibson`** runs both a `go.mod` grep and a typed AST test. A bare grep on import strings will miss aliased imports.
- **`api/gen/toolspb` is only present after external tool generation.** `graphrag`, `serve`, and `eval` packages exclude that import from `go vet` — this is intentional, tracked as a pre-existing issue.
- **`make check-coverage`** is the blocking coverage gate (QUALITY-BARS §4): a uniform **80% per-package floor** + **85% diff-coverage** on changed lines. The pre-existing sub-floor backlog is baselined in `scripts/coverage-baseline.json` (burndown sdk#388) — baselined packages may not regress and no new sub-floor package may land; diff-coverage holds the 85% line on every added/changed Go statement since the merge-base. After burning down coverage, regenerate the baseline with `make coverage-baseline` and commit it (the gate fails if a package graduates past 80% but is left in the baseline).
- **CUE binary** required for `make cue-defs`. Install: `go install cuelang.org/go/cmd/cue@v0.16.1`.

## Links

- Org-level workflow: [`AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md)
- Workspace map: workspace `CLAUDE.md`
- Per-repo ADRs: ``docs/repos/sdk/adr/`` (local docs → `repos/sdk/adr`)
- Domain glossary: ``docs/glossary.md`` (local docs → `glossary.md`)
- PR checklist: ``docs/agents/pr-checklist.md`` (local docs → `agents/pr-checklist.md`)
