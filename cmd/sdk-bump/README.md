# sdk-bump

`sdk-bump` automates opening pull requests in every SDK consumer repo when a new SDK version is released.

## Usage

```
sdk-bump --to vX.Y.Z [--dry-run] [--repos repo1,repo2] [--workdir /tmp/sdk-bump] [--json] [--ci-delay 30s]
```

| Flag | Default | Description |
|---|---|---|
| `--to` | _(required)_ | SDK version tag, e.g. `v0.85.0` |
| `--dry-run` | false | Clone + bump but skip push and PR creation |
| `--repos` | _(all)_ | Comma-separated subset of consumer names |
| `--workdir` | `/tmp/sdk-bump` | Parent directory for clones |
| `--json` | false | Emit JSON summary to stdout |
| `--ci-delay` | 30s | Wait after PR open before polling `gh pr checks`; `0` disables |
| `--workspace` | `~/Code/zeroroot.ai` | Polyrepo root for local-path drift warnings |

## Prerequisites

- `git` with SSH access to `git@github.com:zeroroot-ai/*`
- `gh` CLI authenticated (`gh auth status`)
- `go` in PATH (for Go consumer repos)
- `pnpm` in PATH (for the dashboard consumer)

## Consumers

The hard-coded `CONSUMERS` slice in `consumer.go` is the source of truth.
Current entries (as of SDK v0.85.x):

| Name | Repo | Go module | Post-bump commands |
|---|---|---|---|
| gibson | zeroroot-ai/gibson | yes | go mod tidy, make proto, go build, go test -short |
| ext-authz | zeroroot-ai/ext-authz | yes | go mod tidy, go build, go test -short |
| adk | zeroroot-ai/adk | yes | go mod tidy, go build, go test -short |
| gibson-executor | zeroroot-ai/gibson-executor | yes | go mod tidy, go build, go test -short |
| tenant-operator | zeroroot-ai/tenant-operator | yes | go mod tidy, make manifests, go build, go test -short |
| dashboard | zeroroot-ai/dashboard | no | pnpm install --no-frozen-lockfile, npx buf generate, pnpm typecheck |

## Failure handling

- A failure in one consumer does not abort others. The final summary table shows pass/fail for each consumer and the tool exits non-zero if any failed.
- If push succeeds but `gh pr create` fails, the branch remains pushed. You can open the PR manually with `gh pr create` from the clone directory under `--workdir`.
- CI status (`gh pr checks`) is polled once per PR after `--ci-delay`. A pending result is shown as a warning and does not change the exit code.

## Examples

Dry run across all consumers:

```
sdk-bump --to v0.85.0 --dry-run
```

Bump only gibson and adk, output JSON:

```
sdk-bump --to v0.85.0 --repos gibson,adk --json
```

Bump everything with a longer CI wait:

```
sdk-bump --to v0.85.0 --ci-delay 2m
```
