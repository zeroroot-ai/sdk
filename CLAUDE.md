# sdk — CLAUDE.md

> **Workflow rules:** see [`zeroroot-ai/.github` → `AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md) — canonical for branching / commits / PRs / releases / merging. Conventional Commits MANDATORY. Never push to main. Never force-push.

This file is the per-repo addendum. Workspace-wide concerns live in the workspace `CLAUDE.md` and in [`AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md).

## TL;DR

The open-source Go SDK, Apache-2.0: the interfaces, types and gRPC serving code you compile into your own agent, tool or plugin. It is the **component-development surface only** and holds no platform runtime (ADR-0058). Start with `make test lint`.

## Architecture

One Go module, `github.com/zeroroot-ai/sdk`, rooted at the repo root. A component author imports the package for the shape they are building — `agent`, `tool`, `plugin`, `connector` — and the harness reaches the platform's runtime capabilities for them: identity, isolation, grants, memory, audit.

`api/gen/` holds the generated Go bindings. The protos are published to **BSR** at `buf.build/zeroroot-ai/sdk`, which is the contract every consumer reads: gibson, the ADK and `sdk-ts` all generate from the registry, never from a local path.

**The hard boundary: this module must not import `gibson`.** `make check-no-gibson` enforces it at the `go.mod` layer, and `check_no_gibson_test.go` at the import layer. The SDK is what a third party compiles against; a dependency on the closed engine would make it unbuildable outside this org.

## Commands

```bash
make test             # unit tests
make test-race        # the race detector
make test-integration # Git-backed codegen tests; LSP tests skip without language servers
make lint             # golangci-lint
make lint-deadcode    # the blocking unused gate
make check-no-gibson  # the import boundary
make check-coverage   # coverage floor + diff against the committed baseline
make bootstrap        # fetch the pinned tooling
```

## Gotchas

- **Never hand-tag, and never hand-bump a consumer.** release-please cuts the tag from Conventional Commit subjects, and the SDK fan-out opens the bump PRs on every consumer when a new tag appears. A hand-tag races the fan-out.
- **A rename makes the fan-out bump unmergeable.** If a release renames or removes an RPC, the mechanical bump PRs land on consumers that no longer compile. Expect to follow the fan-out with real call-site work, and say so in the release's own PR.
- **Proto changes go through BSR, never a local include path.** A consumer generating from a local checkout gets bindings nobody else has. Publish, then regenerate.
- **`sdk-ts` regeneration needs a live BSR token.** `buf registry login` with an expired token fails with "your Buf API token for buf.build is invalid", and no amount of local work gets past it.
- **Coverage has a floor and a committed baseline.** `make check-coverage` compares against `scripts/coverage-baseline.json`; regenerate it deliberately with `make coverage-baseline`, never to make a red run green.
- **Run `golangci-lint` only under the workspace memory cap**, and never alongside a kind cluster or a race build.

## Links

- Org-level workflow: [`AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md)
- The published protos: `buf.build/zeroroot-ai/sdk`
- The CLI built on this: [`zeroroot-ai/adk`](https://github.com/zeroroot-ai/adk)
- The TypeScript bindings: [`zeroroot-ai/sdk-ts`](https://github.com/zeroroot-ai/sdk-ts)
- Worked integrations: [`zeroroot-ai/integrations`](https://github.com/zeroroot-ai/integrations)
