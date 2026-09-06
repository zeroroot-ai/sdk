# auth.md — `zeroroot-ai/sdk`

Auth model from the SDK's perspective. AI-agent-facing.
Spec: `unified-identity-and-authorization` (the entire spec).

## What this provides

The SDK is the **identity-types library** every Gibson Go service imports.
It does **not** validate JWTs, talk to Zitadel, or talk to OpenFGA — those
live in Envoy and ext-authz upstream. The SDK owns:

| Symbol | File | Purpose |
|---|---|---|
| `auth.TenantID` | [`auth/tenantid.go:78`](../auth/tenantid.go) | Sealed, validated tenant identifier. |
| `auth.NewTenantID`, `auth.MustNewTenantID` | [`auth/tenantid.go:97`](../auth/tenantid.go), [`auth/tenantid.go:123`](../auth/tenantid.go) | The only constructors. Refuse empty / whitespace / oversize / bad chars. |
| `auth.SystemTenant` | [`auth/tenantid.go:140`](../auth/tenantid.go) | Reserved `_system` tenant. Single audit-grep handle. |
| `auth.Identity` | [`auth/identity.go:65`](../auth/identity.go) | Request-scoped identity carrier. |
| `auth.Issuer{Zitadel,CapabilityGrant}` | [`auth/identity.go:12`](../auth/identity.go) | Closed enum of recognised IdPs. |
| `auth.CredentialType{OIDCUser,ClientCredentials,CapabilityGrant}` | [`auth/identity.go:32`](../auth/identity.go) | Wire-credential class. |
| `auth.Header*` constants | [`auth/headers.go:24`](../auth/headers.go) | `x-gibson-identity-{subject,issuer,credential-type,tenant,issued-at}`. |
| `auth.IdentityFromMetadata` | [`auth/headers.go:72`](../auth/headers.go) | Reads headers, builds Identity. No HMAC verify. |
| `auth.WithIdentity`, `auth.IdentityFromContext`, `auth.TenantFromContext` | [`auth/context.go:35`](../auth/context.go), [`:50`](../auth/context.go), [`:89`](../auth/context.go) | Context plumbing. **No fallback to `_system` on miss.** |
| `auth.UnaryServerInterceptor`, `auth.StreamServerInterceptor` | [`auth/interceptor.go:34`](../auth/interceptor.go), [`:48`](../auth/interceptor.go) | The single gRPC interceptor every Gibson Go server installs. |
| `capabilitygrant.Claims`, `capabilitygrant.Verify` | [`capabilitygrant/claims.go`](../capabilitygrant/claims.go), [`capabilitygrant/verify.go:70`](../capabilitygrant/verify.go) | CG-JWT verifier (used by ext-authz; daemon does not verify its own grants). |
| `spiffe.DialOptions`, `spiffe.ExpectPeerSPIFFEID` | [`spiffe/dial.go`](../spiffe/dial.go) | Workload API auto-detect dial helper. |
| `agent.Connect` | [`agent/connect.go:1`](../agent/connect.go) | One-call agent dial: credentials → Zitadel JWT → SPIFFE-aware mTLS → `*grpc.ClientConn`. |

## TenantID is sealed

```go
// constructable only via NewTenantID / MustNewTenantID
type TenantID struct { s string }
```

`NewTenantID` enforces the `^[a-z][a-z0-9]*([-_][a-z0-9]+)*$` pattern and a
128-char ceiling ([`auth/tenantid.go:63`](../auth/tenantid.go)). `String()`
unwraps; `MarshalText` is implemented; `UnmarshalText` is **intentionally
not** implemented — JSON / YAML round-trips must call `NewTenantID`
explicitly so validation re-runs.

`SystemTenant` ([`auth/tenantid.go:140`](../auth/tenantid.go)) is the only
TenantID whose underlying string is `_system`. `NewTenantID("_system")`
refuses with `ErrInvalidTenant` so platform-operator code paths are easy
to grep.

Why sealed: prevents arbitrary tenant strings from untrusted code paths
(request bodies, env vars, log lines parsed back to structs) silently
becoming valid `TenantID` values. Audit findings C11/C12 closed.

## Identity from context

```go
id, err := auth.IdentityFromContext(ctx)        // full record
tenant, ok := auth.TenantFromContext(ctx)       // common case; ok=false on miss
```

The interceptor populates the context. Origin chain:

```
caller --(Zitadel JWT)--> Envoy
                              |
                              | jwt_authn validates
                              | ext_authz callout
                              v
                          ext-authz
                              |  emits headers:
                              |    x-gibson-identity-subject
                              |    x-gibson-identity-issuer
                              |    x-gibson-identity-credential-type
                              |    x-gibson-identity-tenant
                              |    x-gibson-identity-issued-at
                              v
            Envoy --mTLS (SPIFFE pin)--> daemon
                                          |
                                          | sdk auth interceptor:
                                          |   IdentityFromMetadata(md)
                                          |   WithIdentity(ctx, id)
                                          v
                                       handler(ctx, req)
                                          |
                                          | TenantFromContext(ctx)
                                          v
                                       pool.For(ctx, tenant)
```

Headers are **not** HMAC-signed. Channel security is the SPIFFE-pinned
mTLS hop between Envoy and the daemon — the daemon's TLS listener accepts
only Envoy's SVID. See `core/gibson/docs/auth.md` for the daemon-side
listener pin.

`TenantFromContext` returns `(zero, false)` when no Identity is on the
context **and never falls back to `SystemTenant`**. Handler code MUST
treat `ok == false` as PermissionDenied. The interceptor itself rejects
identity-less requests with PermissionDenied before they reach a handler;
`TenantFromContext` is the layered backstop.

## Serve auth model

Every agent and tool that connects to the Gibson platform uses a two-layer auth model:
Capability Grant provides the identity (always required); SPIFFE provides the transport
channel security (optional in-cluster upgrade).

### Capability Grant (required)

Every component performs Ed25519 host registration on first run. The host key is
persisted to `~/.gibson/host_key.json` (override via `GIBSON_HOST_KEY_PATH`) and
reused on every subsequent run without re-registration.

| Step | What happens |
|---|---|
| Bootstrap | `capabilitygrant.Authenticate` exchanges `GIBSON_AGENT_BOOTSTRAP_TOKEN` for the host registration record. Token is consumed on the first `Register` call; subsequent runs reuse the persisted key. |
| Per-RPC credential | Each daemon RPC carries a CG-JWT (`Authorization: Bearer`): Ed25519-signed, ephemeral agent key, 55-second expiry. |

Required env vars:

| Env var | Required | Purpose |
|---|---|---|
| `GIBSON_PLATFORM_URL` | Always | Platform HTTPS base URL |
| `GIBSON_AGENT_BOOTSTRAP_TOKEN` | First run only | One-time registration token; not needed after host key persists |
| `GIBSON_HOST_KEY_PATH` | Optional | Override path for the Ed25519 host key file |

### SPIFFE transport upgrade (optional)

When `SPIFFE_ENDPOINT_SOCKET` is set and the socket file exists at runtime, the serve
loop opens an `X509Source` via the SPIRE Workload API and dials the daemon over mTLS
using the component's X509-SVID. The CG-JWT is still presented as the application-level
credential — SPIFFE upgrades the **transport** only, it does not replace Capability Grant.

| Env var | Default | Purpose |
|---|---|---|
| `SPIFFE_ENDPOINT_SOCKET` | `/run/spire/sockets/agent.sock` | SPIRE Workload API socket path |
| `GIBSON_DAEMON_ADDRESS` | `gibson.gibson.svc.cluster.local:50002` | Daemon gRPC address |

### Recommended option call

`WithPlatformFromEnv()` reads both sets of env vars in one call and is the recommended
option for all deployment topologies:

```go
err := serve.Agent(myAgent, serve.WithPlatformFromEnv())
```

For external deployments (no SPIRE), set `GIBSON_PLATFORM_URL` and
`GIBSON_AGENT_BOOTSTRAP_TOKEN`; `SPIFFE_ENDPOINT_SOCKET` need not be set.
For in-cluster deployments (SPIRE co-located), set all five env vars; the SPIFFE
transport upgrade is applied automatically.

See [ADR-0036](../../docs/adr/0036-capability-grant-first-agent-identity.md) for the
normative decision record.

## Capability-grant verify

Agents and tools verify CG-JWTs using `capabilitygrant.Verify`
([`capabilitygrant/verify.go:70`](../capabilitygrant/verify.go)). The
**daemon mints**; the SDK only verifies. Mint lives in the daemon at
[`core/gibson/internal/capabilitygrant/mint.go`](../../gibson/internal/capabilitygrant/mint.go).

```go
claims, err := capabilitygrant.Verify(ctx, jwksFetcher, token, capabilitygrant.VerifyOptions{
    ExpectedIssuer:   "https://api.zeroroot.ai/gibson",
    ExpectedAudience: "gibson-daemon",
})
```

EdDSA / Ed25519 only. Any other `alg` header is rejected up-front
([`verify.go:80`](../capabilitygrant/verify.go)) to prevent algorithm
confusion. The JWKS fetcher is an interface; ext-authz wires an
HTTP-cached implementation polling the daemon's `/.well-known/jwks.json`
through Envoy.

Claims (see [`capabilitygrant/claims.go`](../capabilitygrant/claims.go)):

| Claim | Purpose |
|---|---|
| `iss`, `aud` | Validated against `VerifyOptions`. |
| `sub` | Agent service-account ID. |
| `tenant` | Required; constructed via `auth.NewTenantID`. |
| `mission_id`, `task_id` | Bind grant to one task. |
| `allowed_rpcs` | List of fully-qualified gRPC methods this grant authorises. |
| `iat`, `exp` | `exp - iat <= 30m` enforced by mint; verify rejects `now > exp`. |
| `jti` | Unique per grant; reserved for a future revocation list. |

## SPIFFE dial helper

`spiffe.DialOptions(ctx)` ([`spiffe/dial.go`](../spiffe/dial.go))
auto-detects the SPIFFE Workload API socket via `SPIFFE_ENDPOINT_SOCKET`.
If present → mTLS DialOption with `tlsconfig.MTLSClientConfig` against
the X509 source. If absent → server-side TLS only with system CAs (the
ADK / customer-network agent path).

`spiffe.ExpectPeerSPIFFEID(spiffeID)` returns a server option that
rejects any peer SVID that does not match — used by the daemon to pin
its inbound connections to Envoy's SPIFFE ID.

## What lives elsewhere

| Concern | Owner | File |
|---|---|---|
| JWT signature / iss / aud / exp validation | Envoy `jwt_authn` filter | `enterprise/deploy/helm/gibson/files/envoy/envoy.yaml` |
| FGA decision + cache | ext-authz | `core/ext-authz/internal/fga/` |
| CG-JWT short-circuit and identity header emission | ext-authz | `core/ext-authz/internal/server/envoy_extauthz.go:113` |
| CG-JWT minting (Ed25519, KMS-derived) and JWKS publication | gibson daemon | `core/gibson/internal/capabilitygrant/{mint,jwks}.go` |
| Browser session (Auth.js + Zitadel OIDC) | dashboard | `enterprise/platform/dashboard/auth.ts` |
| Service-account token cache (client_credentials) | dashboard | `enterprise/platform/dashboard/src/lib/auth/service-token.ts` |
| Per-tenant data plane | data-plane spec | see [`data-plane.md`](./data-plane.md) |
| Tenant lifecycle (Zitadel org create/delete) | tenant-operator | `enterprise/platform/tenant-operator/internal/saga/flows/` |

## Annotations: every RPC declares its policy

`api/proto/gibson/auth/v1/options.proto` defines
`(gibson.auth.v1.authz)` carried on every RPC method. Either
`unauthenticated: true` (e.g. `Ping`) or a
`{relation, object_type, object_deriver, allowed_identities}` tuple.

`make proto` runs `cmd/authz-registry-gen` which walks the
FileDescriptorSet and emits three artifacts under `auth/registry/`:

- `registry.go` — Go map consumed by daemon startup self-check.
- `registry.yaml` — consumed by ext-authz at startup (mounted via Helm).
- `permissions.ts` — TS constants for dashboard UI gating.

The OpenFGA model itself is hand-maintained at
`core/gibson/internal/authz/model.fga`; the codegen no longer emits a
coverage stub.

`auth/registry/coverage_test.go` verifies every method in every
registered service descriptor has an entry. Adding an RPC without the
annotation fails CI.

## Cross-link

- Adding a new RPC: [`how-to-add-a-rpc.md`](./how-to-add-a-rpc.md).
- Wrong vs right code shapes: [`forbidden-patterns.md`](./forbidden-patterns.md).
- Machine-readable rules: [`rules.yaml`](./rules.yaml).
- Daemon-side auth model: `core/gibson/docs/auth.md`.
- ext-authz internals: `core/ext-authz/docs/auth.md`.
- Helm wiring (Envoy, SPIRE, Vault): `enterprise/deploy/docs/auth.md`.
- Data-plane half (per-tenant Conn): [`data-plane.md`](./data-plane.md).
