# a2a-inbound-client

A Go program that calls a published Gibson agent with A2A. It is the
first-party consumer of the package `a2ainbound` and of
`capabilitygrant.Client.SignPath`.

A standard A2A client cannot call Gibson. Each request needs a Capability Grant
JWT in the header `x-capability-grant`. The token is bound to the path of the
request, it lives 55 seconds, and the platform accepts it one time. The package
`a2ainbound` signs a new token for each request.

## Before you start

A tenant admin does these steps one time:

1. Publish a mission definition for inbound calls, with an agent name.
2. Create an agent identity with the ceiling `mission:originate`.
3. Grant that identity the right to use the published definition.
4. Give the bootstrap token of the identity to the caller.

## Run

```bash
export GIBSON_PLATFORM_URL=https://api.example.com
export GIBSON_INBOUND_AGENT=recon
export GIBSON_BOOTSTRAP_TOKEN=<the bootstrap token>   # first run only
go run ./examples/a2a-inbound-client "scan the staging hosts"
```

Set `GIBSON_INBOUND_TARGETS` to a list of target IDs, with commas between them,
to narrow the targets of the definition. The platform never reads a target from
the text of the message.

## The calls

```go
identity, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
    PlatformURL: "https://api.example.com", AgentName: "my-caller",
})
// handle err, then identity.Discover(ctx) and identity.Register(ctx)

client, err := a2ainbound.NewClient(a2ainbound.Config{
    BaseURL: "https://api.example.com", Signer: identity,
})
// handle err

card, err := client.AgentCard(ctx, "recon")
task, err := client.SendMessage(ctx, "recon", "scan the staging hosts", "target-id")
task, err = client.GetTask(ctx, "recon", task.ID)
task, err = client.CancelTask(ctx, "recon", task.ID)
```

A program with its own HTTP client signs the token itself:

```go
token, err := identity.SignPath("/a2a/inbound/recon")
// handle err
req.Header.Set(capabilitygrant.MetadataHeaderCapabilityGrant, token)
```

`main_test.go` runs the program against a fake platform. The unwired-code guard
(`make lint-unwired`) counts the reads in this package.
