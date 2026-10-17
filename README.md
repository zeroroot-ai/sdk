# Gibson SDK

> **Workflow rules:** see [`zeroroot-ai/.github` → `AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md) for branch / PR / commit / release / rebase rules. Releases are automated by release-please from Conventional Commit subjects; **never hand-tag the SDK** (the SDK fan-out workflow auto-bumps consumers when a new SDK tag appears). Repo-local rules below override only when explicitly noted.

The official open-source Go SDK for building agents, tools, and plugins for [zeroroot.ai](https://zeroroot.ai), the zero-trust agent factory. Kubernetes-native: you author components, and Gibson, the platform engine, runs them with identity, isolation, grants, memory, and audit.

## Installation

```bash
go get github.com/zeroroot-ai/sdk@latest
```

## Overview

This module is the component-development surface: the interfaces, types and
gRPC serving code you compile into your own agent, tool or plugin. It holds no
platform runtime.

The platform provides the runtime capabilities your component reaches through
the harness. Deploy Gibson with Helm, and it gives your component:

- LLM completions against the providers the platform is configured with
- Tool execution across the platform's distributed work queues
- A knowledge graph your component reads and writes through `Observe`
- Horizontal scaling with standard Kubernetes patterns

The SDK defines the wire contract for each of those. It does not contain an LLM
client, a queue or a graph database. See <https://docs.zeroroot.ai> for the
platform itself.

```bash
# Deploy Gibson to your cluster
helm install gibson gibson/gibson --namespace gibson-system --create-namespace

# Build and deploy your agent
go build -o myagent ./cmd/myagent
kubectl apply -f myagent-deployment.yaml
```

## Quick Example

Build an agent to troubleshoot Kubernetes clusters:

```go
package main

import (
    "context"
    "log"

    sdk "github.com/zeroroot-ai/sdk"
    "github.com/zeroroot-ai/sdk/agent"
    "github.com/zeroroot-ai/sdk/llm"
)

func main() {
    a, err := sdk.NewAgent(
        sdk.WithName("k8s-troubleshooter"),
        sdk.WithVersion("1.0.0"),
        sdk.WithDescription("Troubleshoots Kubernetes cluster issues"),
        sdk.WithTargetTypes("kubernetes"),
        sdk.WithExecuteFunc(execute),
    )
    if err != nil {
        log.Fatal(err)
    }
    if err := sdk.ServeAgent(a); err != nil {
        log.Fatal(err)
    }
}

func execute(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
    resp, err := h.Complete(ctx, "primary", []llm.Message{
        {Role: llm.RoleSystem, Content: "You are a Kubernetes expert. Diagnose cluster issues."},
        {Role: llm.RoleUser, Content: task.Goal},
    })
    if err != nil {
        return agent.Result{Status: agent.StatusFailed, Output: err.Error()}, nil
    }
    return agent.Result{Status: agent.StatusSuccess, Output: resp.Content}, nil
}
```

[`examples/minimal-agent`](examples/minimal-agent) is a complete agent that
also lists tools and records an observation.

Deploy as a Kubernetes workload. The agent dials the platform, so it opens no
port for work. It needs the platform URL and a bootstrap token:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-troubleshooter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: k8s-troubleshooter
  template:
    metadata:
      labels:
        app: k8s-troubleshooter
    spec:
      containers:
      - name: agent
        image: myorg/k8s-troubleshooter:latest
        env:
        - name: GIBSON_URL
          value: https://gibson.example.com
        - name: GIBSON_BOOTSTRAP_TOKEN
          valueFrom:
            secretKeyRef:
              name: gibson-bootstrap
              key: token
```

## Core Concepts

### Component Types

| Component | Purpose | State | I/O | Example |
|-----------|---------|-------|-----|---------|
| **Agent** | Autonomous LLM-driven task execution | Stateful | Harness API | k8s-troubleshooter, log-analyzer |
| **Tool** | Atomic operations (CLI wrappers, APIs) | Stateless | Protocol Buffers | kubectl, curl, terraform |
| **Plugin** | External service integrations | Stateful | JSON | slack, jira, pagerduty |

### The Harness

Every agent receives a `Harness` - your single interface to the platform. It is
composed of capability groups, declared in [`agent/harness.go`](agent/harness.go):

| Group | Methods |
|-------|---------|
| `LLMCaller` | `Complete`, `CompleteWithTools`, `Stream`, `CompleteStructured`, `CompleteStructuredAny` |
| `ToolCaller` | `CallToolProto`, `CallToolProtoStream`, `ListTools`, `QueueToolWork`, `ToolResults` |
| `PluginCaller` | `QueryPlugin`, `ListPlugins` |
| `Delegator` | `DelegateToAgent`, `ListAgents` |
| `WorldEmitter` | `SubmitFinding`, `Observe` |
| `WorldReader` | `WorldView` |
| `KnowledgeReader` | `QueryNodes`, `FindSimilarFindings`, `GetAttackChains`, and the other read-only graph queries |
| `Planner` | `PlanContext`, `ReportStepHints` |
| `WorkspaceAccess` | `Workspace`, `Workspaces` |
| `MissionManager` | `CreateMission`, `RunMission`, `GetMissionStatus`, `WaitForMission`, `ListMissions`, `CancelMission`, `GetMissionResults` |

`Harness` also carries `Authorize`, `Mission`, `Target`, `Logger`, `Tracer` and
`TokenUsage`.

### LLM Slots

An agent calls an LLM through a named slot. A slot is a name. The platform
resolves it to one of the tenant's providers and a model. It uses the tenant
default provider and model when that model meets the context floor of the
slot:

```go
resp, err := h.Complete(ctx, "primary", messages)
```

## Building an Agent

### 1. Configure the Agent

`sdk.NewAgent` builds an `agent.Agent` from options. The execute function is
the agent logic. The Quick Example above shows a complete agent.

```go
a, err := sdk.NewAgent(
    sdk.WithName("my-agent"),
    sdk.WithVersion("1.0.0"),
    sdk.WithDescription("My autonomous agent"),
    sdk.WithTargetTypes("llm_chat"),
    sdk.WithExecuteFunc(execute),
)
```

To implement `agent.Agent` yourself, read [`agent/agent.go`](agent/agent.go).

### 2. Serve the Agent

```go
if err := sdk.ServeAgent(a); err != nil {
    log.Fatal(err)
}
```

### 3. Deploy to Kubernetes

The health server listens on port 8080 (`GIBSON_HEALTH_PORT` changes it).
`/readyz` returns 503 until the agent is registered with the platform.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-agent
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-agent
  template:
    metadata:
      labels:
        app: my-agent
    spec:
      containers:
      - name: agent
        image: myorg/my-agent:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
        env:
        - name: GIBSON_URL
          value: https://gibson.example.com
        - name: GIBSON_BOOTSTRAP_TOKEN
          valueFrom:
            secretKeyRef:
              name: gibson-bootstrap
              key: token
```

## Building a Tool

Tools are stateless processes that implement the SDK's `tool.Tool` interface, define their own proto types, and register with gibson over gRPC. The platform handles dispatch, observability, and — critically — **automatic knowledge-graph population** based on what your tool returns.

### Mental model

Your tool runs as a separate Go binary. It does NOT live in this SDK repo. Both your tool and gibson import this SDK at compile time; at runtime they're separate processes talking gRPC. The SDK is the contract; you implement it.

The "auto-populate the graph" magic is one rule: **proto field 100 in your response message MUST be `gibson.graphrag.v1.DiscoveryResult`**. Gibson's `DiscoveryProcessor` introspects every tool response, finds field 100, and writes the entities + relationships into Neo4j. You never write to Neo4j directly.

### Worked example: an nmap tool that auto-populates host/port/service relationships

**Step 1 — Scaffold the tool**

```bash
go install github.com/zeroroot-ai/adk/gibson/cmd/gibson@latest
gibson component init my-nmap-tool --kind tool
cd my-nmap-tool
```

Set the name, the version and the description of the tool in `main.go`. A tool declares itself in code, and no manifest file exists.

**Step 2 — Define the proto (field 100 is the magic)**

The scaffold writes `api/proto/gibson/tools/mynmaptool/v1/mynmaptool.proto`.
Replace its two messages with these:

```proto
syntax = "proto3";

package gibson.tools.mynmaptool.v1;

import "gibson/graphrag/v1/graphrag.proto";  // ← from this SDK

option go_package = "github.com/zeroroot-ai/my-nmap-tool/api/gen/gibson/tools/mynmaptool/v1;mynmaptoolv1";

message NmapRequest {
  repeated string targets = 1;          // "10.0.0.0/24" or "example.com"
  string scan_type = 2;                 // "syn", "connect" or "udp"
  repeated string ports = 3;            // "1-1024" or "80,443,22"
  int32 timeout_seconds = 4;
}

message NmapResponse {
  // Tool-specific result fields surfaced to the agent.
  string raw_xml = 1;
  int32 hosts_scanned = 2;
  double scan_duration_seconds = 3;

  // ── Field 100 is reserved platform-wide for DiscoveryResult. ──
  // Gibson's DiscoveryProcessor reads it from every tool response.
  gibson.graphrag.v1.DiscoveryResult discovery = 100;
}
```

Run `make proto`. It runs `buf generate` and writes the Go bindings under
`api/gen/gibson/tools/mynmaptool/v1/`.

**Step 3 — Implement the tool**

```go
package main

import (
    "context"
    "encoding/xml"
    "fmt"
    "os/exec"
    "strings"
    "time"

    nmappb "github.com/zeroroot-ai/my-nmap-tool/api/gen/gibson/tools/mynmaptool/v1"
    graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
    "github.com/zeroroot-ai/sdk/serve"
    "google.golang.org/protobuf/proto"
)

type NmapTool struct{}

func (t *NmapTool) Name() string              { return "my-nmap-tool" }
func (t *NmapTool) Version() string           { return "v0.1.0" }
func (t *NmapTool) Description() string       { return "Runs nmap and reports hosts, ports and services" }
func (t *NmapTool) Tags() []string            { return []string{"network", "discovery"} }
func (t *NmapTool) InputMessageType() string  { return "gibson.tools.mynmaptool.v1.NmapRequest" }
func (t *NmapTool) OutputMessageType() string { return "gibson.tools.mynmaptool.v1.NmapResponse" }

var scanFlags = map[string]string{"syn": "-sS", "connect": "-sT", "udp": "-sU"}

// ExecuteProto is the entry point of the tool. The SDK calls it for each work item.
func (t *NmapTool) ExecuteProto(ctx context.Context, in proto.Message) (proto.Message, error) {
    req, ok := in.(*nmappb.NmapRequest)
    if !ok {
        return nil, fmt.Errorf("unexpected input %T", in)
    }
    if req.TimeoutSeconds > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
        defer cancel()
    }

    // 1. Run nmap with XML output on stdout.
    args := []string{"-sV", "-oX", "-"}
    if flag, ok := scanFlags[req.ScanType]; ok {
        args = append(args, flag)
    }
    if len(req.Ports) > 0 {
        args = append(args, "-p", strings.Join(req.Ports, ","))
    }
    args = append(args, req.Targets...)
    out, err := exec.CommandContext(ctx, "nmap", args...).Output()
    if err != nil {
        return nil, err
    }

    // 2. Parse the nmap XML report.
    var run nmapRun
    if err := xml.Unmarshal(out, &run); err != nil {
        return nil, fmt.Errorf("parse nmap output: %w", err)
    }

    // 3. Translate the report into a DiscoveryResult.
    //    THIS is the part that lights up the knowledge graph. gibson joins the
    //    lists by ID: Port.HostId names a Host.Id, Service.PortId a Port.Id.
    discovery := &graphragpb.DiscoveryResult{}
    for _, h := range run.Hosts {
        hostID := h.ip()
        if hostID == "" {
            continue
        }
        host := &graphragpb.Host{Id: ptr(hostID), Ip: hostID}
        if len(h.Hostnames) > 0 {
            host.Hostname = ptr(h.Hostnames[0].Name)
        }
        discovery.Hosts = append(discovery.Hosts, host)
        for _, p := range h.Ports {
            // gibson treats each reported port as open, so report open ports only.
            // nmap also lists closed and filtered ports when there are few.
            if p.State.State != "open" {
                continue
            }
            portID := fmt.Sprintf("%s/%s:%d", hostID, p.Protocol, p.PortID)
            discovery.Ports = append(discovery.Ports, &graphragpb.Port{
                Id:       ptr(portID),
                HostId:   hostID, // ← edge: Host HAS_PORT Port
                Number:   p.PortID,
                Protocol: p.Protocol,
                State:    ptr(p.State.State),
            })
            if p.Service.Name != "" {
                discovery.Services = append(discovery.Services, &graphragpb.Service{
                    PortId:  portID, // ← edge: Port RUNS_SERVICE Service
                    Name:    p.Service.Name,
                    Product: ptr(p.Service.Product),
                    Version: ptr(p.Service.Version),
                })
            }
        }
    }

    // 4. Return the response with discovery in field 100.
    return &nmappb.NmapResponse{
        RawXml:              string(out),
        HostsScanned:        int32(len(run.Hosts)),
        ScanDurationSeconds: run.RunStats.Finished.Elapsed,
        Discovery:           discovery, // ← the magic field
    }, nil
}

func ptr[T any](v T) *T { return &v }

// nmapRun is the part of the nmap XML report (-oX) that the tool reads.
type nmapRun struct {
    Hosts    []nmapHost `xml:"host"`
    RunStats struct {
        Finished struct {
            Elapsed float64 `xml:"elapsed,attr"`
        } `xml:"finished"`
    } `xml:"runstats"`
}

type nmapHost struct {
    Addresses []struct {
        Addr     string `xml:"addr,attr"`
        AddrType string `xml:"addrtype,attr"`
    } `xml:"address"`
    Hostnames []struct {
        Name string `xml:"name,attr"`
    } `xml:"hostnames>hostname"`
    Ports []nmapPort `xml:"ports>port"`
}

// ip returns the IPv4 or IPv6 address of the host. nmap also lists the MAC
// address as an <address> element.
func (h nmapHost) ip() string {
    for _, a := range h.Addresses {
        if a.AddrType == "ipv4" || a.AddrType == "ipv6" {
            return a.Addr
        }
    }
    return ""
}

type nmapPort struct {
    Protocol string `xml:"protocol,attr"`
    PortID   int32  `xml:"portid,attr"`
    State    struct {
        State string `xml:"state,attr"`
    } `xml:"state"`
    Service struct {
        Name    string `xml:"name,attr"`
        Product string `xml:"product,attr"`
        Version string `xml:"version,attr"`
    } `xml:"service"`
}

func main() {
    // serve.Tool connects to the platform at GIBSON_URL, registers the
    // FileDescriptorSet, polls for work, and serves the health probes.
    if err := serve.Tool(&NmapTool{}); err != nil {
        panic(err)
    }
}
```

**Step 4 — Deploy**

Build and push the image. The image must install `nmap`, and a SYN or UDP
scan (`-sS`, `-sU`) needs the `NET_RAW` capability. `serve.Tool()` dials the platform at `GIBSON_URL`
with `GIBSON_BOOTSTRAP_TOKEN` and polls for work through
`ComponentService.PollWork`. The tool runs no server of its own, so replicas
can come and go without a change to gibson.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nmap-tool
spec:
  replicas: 5  # Scale horizontally
  selector:
    matchLabels:
      app: my-nmap-tool
  template:
    metadata:
      labels:
        app: my-nmap-tool
    spec:
      containers:
      - name: tool
        image: ghcr.io/you/my-nmap-tool:v0.1.0
        env:
        - name: GIBSON_URL
          value: https://gibson.example.com
        - name: GIBSON_BOOTSTRAP_TOKEN
          valueFrom:
            secretKeyRef:
              name: gibson-bootstrap
              key: token
```

On first connection your tool sends its FileDescriptorSet (the schema for `NmapRequest`/`NmapResponse`) to gibson via `RegisterComponent`. Gibson caches it via `protoresolver`. From this point on, gibson knows how to call your tool by name and how to interpret its responses, without any compile-time dependency on your tool's Go types.

**Step 5 — How an agent uses it**

In an agent (a separate component), inside its execute function:

```go
import (
    nmappb "github.com/zeroroot-ai/my-nmap-tool/api/gen/gibson/tools/mynmaptool/v1" // ← the tool's Go bindings
)

// Inside the agent's reasoning loop:
req := &nmappb.NmapRequest{
    Targets:  []string{"10.0.0.0/24"},
    ScanType: "syn",
    Ports:    []string{"1-1024"},
}
resp := &nmappb.NmapResponse{}
if err := h.CallToolProto(ctx, "my-nmap-tool", req, resp); err != nil {
    return agent.Result{Status: agent.StatusFailed, Output: err.Error()}, nil
}

h.Logger().Info("nmap done", "hosts", resp.HostsScanned, "duration", resp.ScanDurationSeconds)

// The agent does nothing for the graph. gibson reads resp.Discovery and
// writes the hosts, ports and services after it answers the call.
```

### What gibson does automatically (the "free" part)

When your tool's response reaches gibson, the `DiscoveryProcessor`:

1. Uses `protoresolver` to find field 100 of the cached `NmapResponse` descriptor.
2. Reads `resp.Discovery` and extracts every `Host`, `Port`, `Service`, etc.
3. Folds them into the World as observations. The graph projector then writes the nodes (`:Host {address: "10.0.0.5"}`, `:Port {number: 443, open: true}`, `:Service {name: "https"}`).
4. Writes the relationships that the ID fields join: `(:Host)-[:HAS_PORT]->(:Port)` and `(:Port)-[:RUNS_SERVICE]->(:Service)`.

This runs after gibson answers the tool call, so a graph read right after the call may not see the new nodes yet.

### What you DID NOT have to do

- Write any Neo4j Cypher.
- Touch this SDK's source.
- Tell gibson about your proto types ahead of time (auto-discovered via FileDescriptorSet at registration).
- Implement any "save to graph" logic. That is gibson's job under the field-100 contract.

### One sentence

You implement `ExecuteProto(req) → resp`, you put your structured discoveries in field 100, and the platform writes them to the knowledge graph without you writing a line of platform code.

## Building a Plugin

A plugin is a stateful service integration, and the only component that may
read a secret. It declares its methods with `plugin.WithHandler` and serves
them with `plugin.Serve`:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/zeroroot-ai/sdk/plugin"
)

type SendRequest struct {
    Channel string `json:"channel"`
    Message string `json:"message"`
}

type SendResponse struct {
    Sent bool `json:"sent"`
}

func send(ctx context.Context, req SendRequest) (SendResponse, error) {
    token, err := plugin.ResolveSecret(ctx, "cred:slack_token")
    if err != nil {
        return SendResponse{}, fmt.Errorf("resolve slack_token: %w", err)
    }
    _ = token // call the Slack API with the token here
    return SendResponse{Sent: true}, nil
}

func main() {
    if err := plugin.Serve(context.Background(),
        plugin.WithName("slack"),
        plugin.WithVersion("1.0.0"),
        plugin.WithHandler("send_message", "Send a message to a Slack channel", send),
    ); err != nil {
        log.Fatal(err)
    }
}
```

## GraphRAG Knowledge Graph

An agent emits what it saw into the World, and the daemon projects it into the
knowledge graph. An agent reads the graph with `QueryNodes`. Both calls need
the callback harness: the agent dials `GIBSON_CALLBACK_ENDPOINT` and builds it
with `serve.NewCallbackHarness`, as [`examples/agent-harness`](examples/agent-harness)
does. The harness of an agent served with `sdk.ServeAgent` returns an error for
`Observe` and `agent.ErrKnowledgeUnavailable` for `QueryNodes`.

```go
// Emit into the World; the daemon projects it into the graph.
if err := h.Observe(ctx, agent.HostObservation{
    Address: "10.0.0.5",
    Ports:   []agent.PortObservation{{Number: 443, Protocol: "tcp", Service: "https"}},
}); err != nil {
    return agent.Result{Status: agent.StatusFailed, Output: err.Error()}, nil
}

// Query what the tenant already knows.
hits, err := h.QueryNodes(ctx, &graphragpb.GraphQuery{
    Text:      "nginx hosts",
    NodeTypes: []string{"host"},
    TopK:      10,
})
switch {
case errors.Is(err, agent.ErrKnowledgeUnavailable):
    // The graph could not be read. Conclude nothing about the target.
case err != nil:
    return agent.Result{Status: agent.StatusFailed, Output: err.Error()}, nil
default:
    h.Logger().Info("prior knowledge", "hits", len(hits))
}
```

**Schema Highlights:**

```
Host ──[HAS_PORT]──▶ Port ──[RUNS_SERVICE]──▶ Service
Domain ──[HAS_SUBDOMAIN]──▶ Subdomain ──[RESOLVES_TO]──▶ Host
```

- YAML-driven taxonomy (single source of truth)
- Cross-mission intelligence

## Package Structure

```
sdk/
├── agent/          # Agent interface, harness, task and result types
├── tool/           # Tool interface and builders
├── plugin/         # Plugin interface and serving
├── llm/            # LLM message and completion types
├── mission/        # Mission context and sub-mission types
├── finding/        # Structured security findings
├── graphrag/       # Knowledge graph types and discovery extraction
│   └── id/             # Node ID generation
├── auth/           # SDK-side identity and authorization
├── capabilitygrant/ # Capability Grant Protocol client
├── serve/          # gRPC serving utilities
├── health/         # Health check helpers
├── schema/         # JSON Schema types and validation
├── taxonomy/       # Compliance rule catalog loader
├── toolrunner/     # Tool worker runtime
├── codegen/        # Code generation, editing and workspace primitives
└── examples/       # Reference implementations
```

`go list ./...` is the full list. The module has no `memory` package and no
`result` package.

## gRPC Serving

`sdk.ServeAgent(a)` serves an agent, and `serve.Tool(t)` serves a tool. Each
call returns an error when serving fails.

## Documentation

| Guide | Description |
|-------|-------------|
| [Feature reference](FEATURES.md) | The SDK surface: what each package gives a component author |
| [Identity and authorization](docs/auth.md) | Tenant identity, capability grants and the gRPC interceptor |
| [Add an RPC](docs/how-to-add-a-rpc.md) | The steps that add a new RPC end to end |
| [Forbidden patterns](docs/forbidden-patterns.md) | Wrong and right code shapes, side by side |
| [Examples](examples/) | Runnable agent, tool and tool-runner programs |

## Use Cases

Gibson agents can automate any domain:

| Domain | Example Agents |
|--------|----------------|
| **DevOps** | K8s troubleshooter, log analyzer, incident responder |
| **Platform Engineering** | Drift detector, cost optimizer, compliance auditor |
| **Security** | Vulnerability scanner, pentester, threat hunter |
| **Data Engineering** | Pipeline monitor, schema validator, ETL orchestrator |
| **Custom Workflows** | Any domain where autonomous agents add value |

## Authz registry

The SDK protos carry `(gibson.auth.v1.authz)` annotations describing the per-RPC
authorization rules. The SDK codegen tool (`cmd/authz-registry-gen`) is the
canonical source for rendering those annotations into runtime artifacts.

The rendered output (`registry.go`, `registry.yaml`, `permissions.ts`) is **no
longer published from this repo**. It lives in the private `zeroroot-ai/gibson`
repository at `internal/authz/registry/` and is published as a private OCI
artifact (`ghcr.io/zeroroot-ai/internal-authz-registry`) on every gibson
release.

- **Platform engineers**: run `make authz-registry` in `zeroroot-ai/gibson` to regenerate.
- **SDK contributors**: run `make proto-authz-registry-emit` in this repo to verify
  annotation changes locally (output goes to `tmp/authz/`, not committed).

Spec: `private-authz-registry`.

## License

Apache-2.0. See [LICENSE](LICENSE).

## License and history

Apache License 2.0. See [LICENSE](LICENSE). Copyright Zero Root AI.

Issue and pull request numbers cited in comments and documents dated before 2026-09-05 refer to the tracker before the history reset, archived offline. They do not resolve on GitHub.
