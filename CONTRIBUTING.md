# Contributing to `sdk`

This is the component-development surface: the protos and Go types you write Gibson agents, tools, plugins and missions against. It is deliberately the permissive layer — what you build with it is yours.

If anything here is unclear, open an issue rather than guessing — an unclear
contributing guide is a bug in this file.

## Prerequisites

- Go 1.26+
- `buf` for proto changes
- `make`

## Build and test

```sh
make test            # unit tests
make generate        # regenerate from protos
make check-no-gibson # the boundary guard
```

## The merge gate

`make check-no-gibson` is the guard that defines this repo: **the SDK must never
import `gibson`.** It is the component-dev surface only (ADR-0058), and a
dependency edge in that direction would drag the whole platform into anything
that builds a component.

Proto changes go through `buf`, and breaking changes are caught before publish.

Every pull request runs it. A red gate is a real signal: **do not** disable a
guard to get a PR through. If a guard is wrong, fix the guard in the same PR
and say why — a guard that needs re-pinning after an unrelated edit is a defect
in the guard.

## Pull requests

- **Conventional Commits in the PR title** — `feat:`, `fix:`, `chore:`,
  `docs:`, `ci:`, `test:`, `refactor:`. The subject must start lowercase;
  `pr-title-lint` enforces both.
- **One root cause per PR.** Two unrelated fixes are two pull requests.
- **Rebase, never merge.** `git fetch origin && git rebase origin/main`
- Releases are automatic via release-please. Never hand-tag, never hand-edit a
  version.

## Reporting a security issue

Do not open a public issue. See [SECURITY.md](SECURITY.md).

## License

Apache-2.0 — see [LICENSE](LICENSE). What you build with this is yours, with no obligation back to us.
