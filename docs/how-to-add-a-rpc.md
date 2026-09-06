# how-to-add-a-rpc.md — `zeroroot-ai/sdk`

Step-by-step for adding a new RPC to a SDK-owned proto. Worked example:
**"add `ListMyDrafts` to `gibson.daemon.v1.DaemonService` returning the
caller's mission drafts."**

Spec: `unified-identity-and-authorization`. Read [`auth.md`](./auth.md)
first if you have not.

---

## End-to-end checklist

This is a cross-repo **epic** — it touches the SDK, the daemon, and (if the RPC is
called from the UI) the dashboard. Open a Project v2 board titled `Epic: <id>` and
add every PR to it as you go. See the org [`AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md) §7 for epic conventions.

### Step 1 — Add the proto annotation (in `core/sdk/`)

Edit the target `.proto` under `core/sdk/api/proto/gibson/<pkg>/<version>/`:

```proto
// core/sdk/api/proto/gibson/daemon/v1/daemon.proto
rpc ListMyDrafts(ListMyDraftsRequest) returns (ListMyDraftsResponse) {
  option (gibson.auth.v1.authz) = {
    relation: "member",
    object_type: "tenant",
    object_deriver: "tenant_from_identity",
    allowed_identities: 1  // USER
  };
}
```

If the file has no other annotated RPCs yet, add the import:

```proto
import "gibson/auth/v1/options.proto";
```

The annotation is **not** optional. The `authz-required` buf lint plugin
and `authz-registry-gen` both fail CI on any RPC that omits it.

See the [Pagination convention](#pagination-convention) section before
defining the request message for any `List*` RPC.

### Step 2 — Open a PR; release-please cuts the SDK tag

> **Releases are automated. Do NOT `git tag && git push --tags`.**

```bash
# In core/sdk/, on a feature branch:
git checkout -b epic/<id>-add-listmydrafts-rpc   # or feat/list-my-drafts if single-repo
git add api/proto/...
git commit -m "feat(daemon): add ListMyDrafts RPC

Co-Authored-By: ..."
git push -u origin HEAD
gh pr create --draft --title "feat(daemon): add ListMyDrafts RPC" --body "..."
```

What happens after merge:

1. release-please runs on `main` push, sees the `feat:` commit, opens an auto-generated **release PR** titled `chore(main): release X.Y.(Z+1)` (minor bump for `feat:`, patch for `fix:`).
2. The release PR includes the bumped `.release-please-manifest.json` and a CHANGELOG entry derived from your PR title.
3. **Merging the release PR** creates the git tag (`vX.Y.Z`) automatically. No human ever runs `git tag`.
4. The new tag triggers `core/sdk/.github/workflows/fan-out.yml`, which opens `chore(deps): bump sdk to vX.Y.Z` PRs in all 6 Go consumers (`gibson`, `ext-authz`, `tenant-operator`, `adk`, `gibson-tool-runner`, `debug-plugin`). For `gibson`, the fan-out also runs `make authz-registry` and includes the regenerated `internal/authz/registry/` artifacts in the bump PR.
5. Each consumer PR auto-merges if its CI passes. Otherwise it sits awaiting human attention.

**You do not** hand-bump consumers, hand-tag the SDK, or hand-edit `.release-please-manifest.json`.

If your change is a breaking change, use `feat(daemon)!:` plus a `BREAKING CHANGE:` footer in the commit; release-please bumps the major version.

### Step 3 — Regenerate dashboard TypeScript bindings

```bash
# In enterprise/platform/dashboard/:
pnpm proto:generate          # regenerates src/gen/** from bumped SDK
pnpm prebuild                # runs authz-registry freshness check + lints
git add src/gen/
git commit -m "chore: regen proto bindings for sdk vX.Y.Z"
```

### Step 4 — Implement the handler in the daemon

In `core/gibson/` (the daemon repo):

```go
// internal/daemon/api/server.go (or a server_<area>.go split file)

func (s *Server) ListMyDrafts(ctx context.Context, req *daemonpb.ListMyDraftsRequest) (*daemonpb.ListMyDraftsResponse, error) {
    tenant, ok := auth.TenantFromContext(ctx)
    if !ok {
        return nil, status.Error(codes.PermissionDenied, "no tenant on context")
    }

    conn, err := s.pool.For(ctx, tenant)   // data-plane spec
    if err != nil { /* ... */ }
    defer conn.Release()

    drafts, next, err := conn.MissionDrafts().List(ctx, req.PageSize, req.PageToken)
    if err != nil { return nil, status.Errorf(codes.Internal, "list drafts: %v", err) }

    return &daemonpb.ListMyDraftsResponse{Drafts: drafts, NextToken: next}, nil
}
```

Patterns to enforce (see `core/gibson/docs/forbidden-patterns.md`):

- `auth.IdentityFromContext` first if you need subject; otherwise
  `auth.TenantFromContext` is enough.
- **Never** read `req.Tenant` or `req.TenantID`. The
  `gibsoncheck:tenantfromcontext` analyzer flags it.
- `pool.For(ctx, tenant)` for tenant-scoped storage; `pool.Admin(ctx)`
  for cross-tenant only, and only from `internal/admin/`.

### Step 5 — FGA tuple data (if needed)

If your RPC introduces a new relation or object type not covered by the
existing FGA model (`core/gibson/internal/authz/model.fga`), you must:

1. Add the relation/type to `model.fga` under the correct object type.
2. Seed any static tuples required (e.g. platform_operator entries) via
   the fga-init Helm Job's ConfigMap. See the `fga-smoke-test.yml` CI
   workflow for integration test patterns.
3. Verify `make check-authz` passes in `core/gibson/`.

For most tenant-scoped RPCs using `tenant_from_identity`, no new FGA
tuples are needed — the model already covers `tenant.admin` and
`tenant.member`.

---

## Pagination convention

**New `List*` RPCs MUST use AIP-158 pagination.**

Request message fields:

```proto
message ListMyDraftsRequest {
  // Maximum number of items to return. Server may return fewer.
  int32 page_size  = 1;
  // Page token from a previous response; empty string for the first page.
  string page_token = 2;
}
```

Response message field:

```proto
message ListMyDraftsResponse {
  repeated MyDraft items        = 1;
  // Token for the next page. Empty when there are no more pages.
  string           next_page_token = 2;
}
```

Do **not** use `offset: int32` + `limit: int32`. The `lint-pagination.mjs`
script (see below) fails CI on any new `List*` method that uses those
fields.

### Exceptions (grandfathered)

The following existing RPCs keep `offset`/`limit` for wire-compatibility.
**Do not add new RPCs here.** Adding to this list requires a PR that edits
`core/sdk/scripts/lint-pagination.mjs` and `core/gibson/scripts/lint-pagination.mjs`
and justifies the exception in the PR description.

| File | Method |
|---|---|
| `gibson/admin/v1/secrets.proto` | `ListSecrets` |
| `gibson/admin/v1/plugins.proto` | `ListPluginInstalls` |
| `gibson/admin/v1/grants.proto` | `ListActiveGrants` |

Migrating these to `page_size`/`page_token` is out of scope (wire
compatibility cost > benefit). Spec: cross-repo-cohesion-fixes design.md
"Out of scope".

### Lint enforcement

`core/sdk/scripts/lint-pagination.mjs` and
`core/gibson/scripts/lint-pagination.mjs` run as part of `make lint` in
each repo. Each script:

1. Builds a FileDescriptorSet from the local proto root via `buf build`.
2. Walks every service method whose name starts with `List`.
3. Inspects the request message for fields named `limit` or `offset`.
4. Fails (exit 1) when any such method is NOT in the grandfathered list
   embedded in the script.

Spec: cross-repo-cohesion-fixes Requirement 4.3, 4.4.

---

## Reference: pick the right proto file

SDK protos live under `api/proto/gibson/<package>/<version>/<file>.proto`.
The local `daemon_admin.proto` (privileged ops) is owned by the daemon
repo; everything else is the SDK's.

If your RPC belongs in a service that does not exist yet, create
`<file>.proto` next to the existing ones; do not split a service across
files.

## Reference: `object_deriver` choices

Supported by ext-authz today
([`core/ext-authz/internal/fga/check.go`](../../ext-authz/internal/fga/check.go)):

| Deriver | Object string emitted | Use when |
|---|---|---|
| `tenant_from_identity` | `tenant:<callerTenant>` | The RPC reads or writes data the tenant owns. |
| `system_tenant` | `system_tenant:_system` | Platform-operator-only RPC. |
| `from_field('<name>')` | `<object_type>:<req.<name>>` | The RPC names a specific object (e.g. `mission_id`). |
| `tenant_and_field('<name>')` | `<object_type>:<callerTenant>/<req.<name>>` | The RPC names a tenant-scoped object whose ID is reused across tenants. |

For `unauthenticated: true` (`Ping`, health), set ONLY that field — it
is mutually exclusive with `relation/object_type/object_deriver/allowed_identities`.

## Reference: run the build guards locally

```bash
make check                                         # SDK side
( cd ../gibson && make check )                     # daemon side
( cd ../ext-authz && make test )                   # registry-load smoke
( cd ../../enterprise/platform/dashboard && pnpm prebuild )
```

`make check` chains include `gibsoncheck` (forbidden imports, no
TrustLocalhost, tenant-from-context), the registry coverage test, the
ext-authz registry-load smoke test, and the dashboard's full
`prebuild` policy chain.

If any guard fires, fix the code — do not allowlist or `// nolint` the
check.
