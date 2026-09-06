# forbidden-patterns.md — `zeroroot-ai/sdk`

Companion to [`rules.yaml`](./rules.yaml). Wrong vs right code shapes for
the SDK's auth surface. Spec: `unified-identity-and-authorization`.

## SDK-AUTH-001: importing a consumer or provider SDK from `auth/`

The auth package is the spine. It is not allowed to know about a specific
identity provider, a specific authorization engine, or any consumer repo.

Wrong (would couple the SDK to ext-authz's choice of FGA SDK):

```go
package auth

import "github.com/openfga/go-sdk/client"   // forbidden in auth/

func newCheck(c *client.OpenFgaClient, ...) bool { /* ... */ }
```

Right — provider integration lives in the consumer:

```go
// in core/ext-authz/internal/fga/check.go
import openfga "github.com/openfga/go-sdk/client"
// ext-authz wires its own check; the SDK exposes only types.
```

## SDK-AUTH-002: implementing `UnmarshalText` on `TenantID`

Wrong — would let any text deserializer manufacture a TenantID without
running `NewTenantID`'s validation:

```go
func (t *TenantID) UnmarshalText(b []byte) error {   // forbidden
    t.s = string(b)
    return nil
}
```

Right — callers that need a TenantID from text MUST call the constructor
so validation runs:

```go
var raw struct { Tenant string `json:"tenant"` }
_ = json.Unmarshal(body, &raw)

tid, err := auth.NewTenantID(raw.Tenant)
if err != nil { return err }
```

## SDK-AUTH-003: defining an RPC without the authz annotation

Wrong (`api/proto/gibson/daemon/v1/daemon.proto`):

```proto
rpc DoThing(DoThingRequest) returns (DoThingResponse);   // no annotation
```

Right — every RPC declares its policy in-place:

```proto
rpc DoThing(DoThingRequest) returns (DoThingResponse) {
  option (gibson.auth.v1.authz) = {
    relation: "member"
    object_type: "tenant"
    object_deriver: "tenant_from_identity"
    allowed_identities: 3   // USER | SERVICE
  };
}
```

For explicit no-auth methods (`Ping`, health):

```proto
rpc Ping(PingRequest) returns (PingResponse) {
  option (gibson.auth.v1.authz) = { unauthenticated: true };
}
```

The `cmd/authz-registry-gen` tool fails CI if any RPC misses the
annotation; the daemon panics at startup if its registered methods do
not match the generated registry.

## SDK-AUTH-004: validating JWTs in the SDK auth interceptor

Wrong — duplicates Envoy's `jwt_authn` filter and creates a second
trust path the daemon would have to keep in sync:

```go
package auth

import "github.com/zitadel/oidc/v3/pkg/op"   // forbidden in auth/

func UnaryServerInterceptor(verifier *op.Verifier) grpc.UnaryServerInterceptor {
    return func(ctx, req, info, h) (any, error) {
        token := metadataAuth(ctx)
        if _, err := verifier.Verify(ctx, token); err != nil {
            return nil, status.Error(codes.Unauthenticated, "bad jwt")
        }
        // ...
    }
}
```

Right — the interceptor is structural only ([`auth/interceptor.go:34`](../auth/interceptor.go)):

```go
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        newCtx, err := identityIntoContext(ctx)   // reads x-gibson-identity-* headers
        if err != nil { return nil, err }
        return handler(newCtx, req)
    }
}
```

Channel security between Envoy and the daemon is SPIFFE mTLS — the
daemon's listener pins Envoy's SVID, so any caller is necessarily
ext-authz-validated by the time these headers arrive.

## SDK-AUTH-005: relaxing the EdDSA-only rule on CG-JWT verify

Wrong — opens the door to algorithm confusion (HMAC-signed token under a
public key):

```go
parsed, err := jwt.Parse(token, keyfunc)   // no WithValidMethods
```

Right ([`capabilitygrant/verify.go:80`](../capabilitygrant/verify.go)):

```go
parsed, err := jwt.Parse(token, keyfunc,
    jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
```

The daemon mints with EdDSA only ([`core/gibson/internal/capabilitygrant/mint.go`](../../gibson/internal/capabilitygrant/mint.go));
ext-authz must reject everything else.

## SDK-AUTH-006: skipping the registry codegen step

Wrong (`Makefile`):

```make
proto:
	npx --prefix ../enterprise/platform/dashboard buf generate
	# stops here — registry stays stale
```

Right:

```make
proto:
	npx --prefix ../enterprise/platform/dashboard buf generate
	go run ./cmd/authz-registry-gen
```

`auth/registry/coverage_test.go` will catch a desync (missing methods)
on next test run, but the canonical fix is keeping the codegen step in
`make proto` so it cannot drift between local and CI.

## SDK-AUTH-007: defaulting to `_system` on missing identity

Wrong (this exact pattern was the audit C12 finding before this spec):

```go
func TenantFromContext(ctx context.Context) TenantID {
    if id, err := IdentityFromContext(ctx); err == nil {
        return id.Tenant
    }
    return SystemTenant   // forbidden — silently widens privilege
}
```

Right ([`auth/context.go:89`](../auth/context.go)):

```go
func TenantFromContext(ctx context.Context) (TenantID, bool) {
    id, err := IdentityFromContext(ctx)
    if err != nil { return TenantID{}, false }
    if id.Tenant.IsZero() { return TenantID{}, false }
    return id.Tenant, true
}
```

Caller pattern in handlers:

```go
tenant, ok := auth.TenantFromContext(ctx)
if !ok {
    return nil, status.Error(codes.PermissionDenied, "no tenant on context")
}
```

`SystemTenant` is reachable only by naming `auth.SystemTenant`
explicitly. That keeps the audit trail of platform-operator code paths
trivially greppable.
