# Gibson SDK — the component author's surface

This module is the Apache-2.0 SDK. You compile it into an agent, a tool or a
plugin, and the Gibson platform runs that component. The SDK holds interfaces,
types, gRPC serving code and client stubs. It holds no platform runtime, and
per ADR-0058 it never imports `zeroroot-ai/gibson`.

**The platform is documented elsewhere.** The daemon, orchestration, mission
management, the registry, the attack framework, guardrails and the CLI are not
in this repository. Read <https://docs.zeroroot.ai> for those. This page
describes only what `go get github.com/zeroroot-ai/sdk` gives you.

## Installation

```bash
go get github.com/zeroroot-ai/sdk@latest
```

## Component authoring

| Package | What it gives you |
|---------|-------------------|
| `agent` | The `Agent` interface, the `Harness` your agent is handed, task and result types, LLM slot declarations, capability declarations and target schemas |
| `tool` | The `Tool` interface and builders for executable components with protobuf inputs and outputs |
| `plugin` | The plugin SDK: the `Plugin` interface, manifest handling and `plugin.Serve` |
| `toolrunner` | The worker runtime that pulls tool work and returns results |
| `startup` | Pre-registration validation: parses `component.yaml` and checks that required binaries are on `PATH` |
| `examples` | Runnable agent, tool and tool-runner programs |

## Reasoning and data

| Package | What it gives you |
|---------|-------------------|
| `llm` | Provider-agnostic message, completion, streaming and token-tracking types. There is no provider client here — the platform holds the credentials and makes the call |
| `graphrag` | The knowledge-graph client: nodes, relationships, query builders, CEL validators, generated domain types and node-ID generation |
| `extraction` | The `EntityExtractor` framework that turns a tool's proto response into a standard `DiscoveryResult` |
| `finding` | Structured security findings: categories, severities, evidence and SARIF-shaped output |
| `taxonomy` | The compliance rule catalog loader that maps graph signals to SOC 2, NIST AI RMF and MITRE control IDs |
| `mission` | Mission context types, and the types an agent uses to create and monitor sub-missions |
| `planning` | Planning-aware interfaces: where the agent sits in mission execution, and the hints it reports back |
| `types` | Targets, techniques, mission context and health status shared across the SDK |

## Identity and transport

| Package | What it gives you |
|---------|-------------------|
| `auth` | The SDK-side identity and authorization surface, including the tenant type |
| `capabilitygrant` | The client half of the Capability Grant Protocol: enrollment, the host keypair store, grant verification |
| `spiffe` | Thin helpers over the SPIFFE Workload API |
| `serve` | gRPC servers for agents, tools and plugins, with Kubernetes health probes and graceful shutdown |
| `daemonclient` | The client stub SDK consumers use to reach a running daemon |
| `api` | The generated protobuf and gRPC types the wire contract is made of |

## Supporting packages

| Package | What it gives you |
|---------|-------------------|
| `schema` | JSON Schema Draft 7 types, builders and validation |
| `health` | Reusable dependency and system-state checks for tools |
| `enum` | A registry that normalizes shorthand enum values to their protobuf names |
| `toolerr` | Structured, typed tool errors |
| `protoresolver` | An LRU cache for parsed protobuf `FileDescriptorSet`s |
| `eval` | A `testing`-integrated evaluation framework with scorers for security agents |
| `codegen` | Primitives for generating, editing and validating code inside a workspace |

## What this module does not contain

- The Gibson daemon, the orchestrator and the mission engine
- Any LLM provider client or API key handling
- A queue, a cache or a graph database
- The rendered authz registry — it is built in `zeroroot-ai/gibson` and
  published as a private OCI artifact

## Where to go next

| Guide | Description |
|-------|-------------|
| [README](README.md) | Install, the quick example, and the package map |
| [Identity and authorization](docs/auth.md) | Tenant identity, capability grants and the gRPC interceptor |
| [Add an RPC](docs/how-to-add-a-rpc.md) | The steps that add a new RPC end to end |
| [Forbidden patterns](docs/forbidden-patterns.md) | Wrong and right code shapes, side by side |
| [Platform documentation](https://docs.zeroroot.ai) | The daemon, deployment and operations |
