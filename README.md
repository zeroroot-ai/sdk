# Gibson SDK

> **Workflow rules:** see [`zeroroot-ai/.github` → `AGENTS.md`](https://github.com/zeroroot-ai/.github/blob/main/AGENTS.md) for branch / PR / commit / release / rebase rules. Releases are automated by release-please from Conventional Commit subjects; **never hand-tag the SDK** (the SDK fan-out workflow auto-bumps consumers when a new SDK tag appears). Repo-local rules below override only when explicitly noted.

The official open-source Go SDK for building agents, tools, and plugins for [zeroroot.ai](https://zeroroot.ai), the zero-trust agent factory. Kubernetes-native: you author components, and Gibson, the platform engine, runs them with identity, isolation, grants, memory, and audit.

## Installation

```bash
go get github.com/zeroroot-ai/sdk@latest
```

## Overview

Gibson is the **Kubernetes-native AI agent development framework**. Deploy Gibson with Helm, then use this SDK to build autonomous agents that:

- Reason with frontier LLMs (Claude, GPT, Gemini, Ollama)
- Execute tools via distributed Redis work queues
- Store knowledge in a Neo4j-backed graph database
- Scale horizontally with standard Kubernetes patterns

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
    "github.com/zeroroot-ai/sdk/agent"
    "github.com/zeroroot-ai/sdk/llm"
    "github.com/zeroroot-ai/sdk/serve"
)

type K8sTroubleshooter struct{}

func (a *K8sTroubleshooter) Name() string        { return "k8s-troubleshooter" }
func (a *K8sTroubleshooter) Version() string     { return "1.0.0" }
func (a *K8sTroubleshooter) Description() string { return "Troubleshoots Kubernetes cluster issues" }

func (a *K8sTroubleshooter) LLMSlots() []agent.SlotDefinition {
    return []agent.SlotDefinition{
        agent.NewSlotDefinition("primary", "Main reasoning LLM", true).
            WithConstraints(agent.SlotConstraints{
                MinContextWindow: 8000,
                RequiredFeatures: []string{agent.FeatureToolUse},
            }),
    }
}

func (a *K8sTroubleshooter) Execute(ctx context.Context, task agent.Task, h agent.Harness) (agent.Result, error) {
    // Use LLM to reason about the issue
    messages := []llm.Message{
        llm.NewSystemMessage("You are a Kubernetes expert. Diagnose cluster issues."),
        llm.NewUserMessage(task.Goal),
    }
    resp, err := h.Complete(ctx, "primary", messages)
    if err != nil {
        return agent.NewFailedResult(err), err
    }

    // Execute kubectl via tool
    output, err := h.ExecuteTool(ctx, "kubectl", &pb.KubectlRequest{
        Command: "get pods -A --field-selector=status.phase!=Running",
    })

    // Store findings in memory
    h.Memory().Mission().Set(ctx, "diagnosis", resp.Content)

    return agent.NewSuccessResult(map[string]any{
        "diagnosis": resp.Content,
        "pods":      output,
    }), nil
}

func main() {
    serve.Agent(&K8sTroubleshooter{}, serve.WithPort(50051))
}
```

Deploy as a Kubernetes workload:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-troubleshooter
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: agent
        image: myorg/k8s-troubleshooter:latest
        ports:
        - containerPort: 50051
        env:
        - name: REDIS_URL
          value: redis://gibson-redis:6379
```

## Core Concepts

### Component Types

| Component | Purpose | State | I/O | Example |
|-----------|---------|-------|-----|---------|
| **Agent** | Autonomous LLM-driven task execution | Stateful | Harness API | k8s-troubleshooter, log-analyzer |
| **Tool** | Atomic operations (CLI wrappers, APIs) | Stateless | Protocol Buffers | kubectl, curl, terraform |
| **Plugin** | External service integrations | Stateful | JSON | slack, jira, pagerduty |

### The Harness

Every agent receives a `Harness` - your single interface to all Gibson capabilities:

```go
type Harness interface {
    // LLM Access - reason with AI
    Complete(ctx, slot, messages, opts...) (*CompletionResponse, error)
    CompleteWithTools(ctx, slot, messages, tools) (*CompletionResponse, error)
    CompleteStructured(ctx, slot, schema, messages) (*StructuredResult, error)

    // Tool Execution - call tools via Redis queues
    ExecuteTool(ctx, name, input proto.Message) (proto.Message, error)
    CallToolProto(ctx, name, request, response proto.Message) error

    // Agent Delegation - spawn sub-agents
    DelegateToAgent(ctx, name, task) (Result, error)

    // Memory - three-tier persistence
    Memory() MemoryManager

    // Knowledge Graph - read-only; writes go through Observe
    QueryNodes(ctx, query) ([]*QueryResult, error)
    Observe(ctx, observation) error

    // Observability - built-in tracing and logging
    Logger() *slog.Logger
    Tracer() trace.Tracer
}
```

### LLM Slots

Agents declare abstract LLM requirements that are resolved at runtime:

```go
func (a *MyAgent) LLMSlots() []agent.SlotDefinition {
    return []agent.SlotDefinition{
        agent.NewSlotDefinition("primary", "Main reasoning LLM", true).
            WithConstraints(agent.SlotConstraints{
                MinContextWindow: 8000,
                RequiredFeatures: []string{agent.FeatureToolUse, agent.FeatureJSONMode},
            }),
        agent.NewSlotDefinition("fast", "Quick completions", false).
            WithConstraints(agent.SlotConstraints{
                MinContextWindow: 4000,
            }),
    }
}
```

**Slot Features:**
| Feature | Description |
|---------|-------------|
| `tool_use` | Function calling support |
| `vision` | Image analysis capability |
| `streaming` | Streaming response support |
| `json_mode` | Structured JSON output |

### Three-Tier Memory

```go
// Working Memory - ephemeral, task-scoped
h.Memory().Working().Set(ctx, "step", "diagnosing")
h.Memory().Working().Get(ctx, "step")

// Mission Memory - persistent, Redis-backed with full-text search
h.Memory().Mission().Set(ctx, "findings", data, metadata)
h.Memory().Mission().Search(ctx, "error timeout", 10)

// Long-Term Memory - vector embeddings for semantic search
h.Memory().LongTerm().Store(ctx, "Kubernetes OOM kills often caused by...", metadata)
h.Memory().LongTerm().Search(ctx, "pod memory issues", 5)
```

## Building an Agent

### 1. Implement the Agent Interface

```go
type MyAgent struct {
    config Config
}

// Identity
func (a *MyAgent) Name() string        { return "my-agent" }
func (a *MyAgent) Version() string     { return "1.0.0" }
func (a *MyAgent) Description() string { return "My autonomous agent" }

// Capabilities
func (a *MyAgent) Capabilities() []string {
    return []string{"troubleshooting", "analysis"}
}

// LLM Requirements
func (a *MyAgent) LLMSlots() []agent.SlotDefinition {
    return []agent.SlotDefinition{
        agent.NewSlotDefinition("primary", "Main LLM", true).
            WithConstraints(agent.SlotConstraints{
                MinContextWindow: 8000,
                RequiredFeatures: []string{agent.FeatureToolUse},
            }),
    }
}

// Lifecycle
func (a *MyAgent) Initialize(ctx context.Context, cfg agent.AgentConfig) error {
    return nil
}

func (a *MyAgent) Shutdown(ctx context.Context) error {
    return nil
}

func (a *MyAgent) Health(ctx context.Context) types.HealthStatus {
    return types.HealthStatus{Status: types.HealthStatusHealthy}
}

// Core Execution
func (a *MyAgent) Execute(ctx context.Context, task agent.Task, h agent.Harness) (agent.Result, error) {
    result := agent.NewResult(task.ID)
    result.Start()

    // Your agent logic here
    messages := []llm.Message{
        llm.NewSystemMessage("You are an expert assistant"),
        llm.NewUserMessage(task.Goal),
    }
    resp, err := h.Complete(ctx, "primary", messages)
    if err != nil {
        result.Fail(err)
        return result, err
    }

    result.Complete(map[string]any{"response": resp.Content})
    return result, nil
}
```

### 2. Serve the Agent

```go
func main() {
    agent := &myagent.MyAgent{}
    serve.Agent(agent,
        serve.WithPort(50051),
        serve.WithHealthPort(8080),  // K8s probes
    )
}
```

### 3. Deploy to Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-agent
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: agent
        image: myorg/my-agent:latest
        ports:
        - containerPort: 50051
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
        - name: REDIS_URL
          value: redis://gibson-redis:6379
```

## Building a Tool

Tools are stateless processes that implement the SDK's `tool.Tool` interface, define their own proto types, and register with gibson over gRPC. The platform handles dispatch, observability, and — critically — **automatic knowledge-graph population** based on what your tool returns.

### Mental model

Your tool runs as a separate Go binary. It does NOT live in this SDK repo. Both your tool and gibson import this SDK at compile time; at runtime they're separate processes talking gRPC. The SDK is the contract; you implement it.

The "auto-populate the graph" magic is one rule: **proto field 100 in your response message MUST be `gibson.graphrag.v1.DiscoveryResult`**. Gibson's `DiscoveryProcessor` introspects every tool response, finds field 100, and writes the entities + relationships into Neo4j. You never write to Neo4j directly.

### Worked example: an nmap tool that auto-populates host/port/service relationships

**Step 1 — Bootstrap from the template**

```bash
gh repo create my-nmap-tool --template zeroroot-ai/component-skeleton --public
cd my-nmap-tool
# You get: main.go, Dockerfile, Makefile, component.yaml, CLAUDE.md
```

Edit `component.yaml`: `name: my-nmap-tool`, `kind: tool`, version, description.

**Step 2 — Define the proto (field 100 is the magic)**

`api/proto/mytool/nmap/v1/nmap.proto`:

```proto
syntax = "proto3";

package mytool.nmap.v1;

option go_package = "github.com/you/my-nmap-tool/api/gen/nmap/v1;nmappb";

import "gibson/graphrag/v1/graphrag.proto";  // ← from this SDK

message NmapRequest {
  repeated string targets = 1;          // "10.0.0.0/24" or "example.com"
  string scan_type = 2;                 // "syn", "connect", "udp", etc.
  repeated string ports = 3;            // "1-1024" or "80,443,22"
  int32 timeout_seconds = 4;
}

message NmapResponse {
  // Tool-specific result fields surfaced to the agent.
  string raw_xml = 1;
  int32 hosts_scanned = 2;
  double scan_duration_seconds = 3;

  // ── Field 100 is reserved platform-wide for DiscoveryResult. ──
  // Gibson's DiscoveryProcessor extracts it from every tool response
  // and writes the entities + relationships into Neo4j automatically.
  gibson.graphrag.v1.DiscoveryResult discovery = 100;
}
```

Run `buf generate` (the skeleton's Makefile target). Output: `api/gen/nmap/v1/nmap.pb.go`.

**Step 3 — Implement the tool**

```go
package main

import (
    "context"
    "encoding/xml"
    "fmt"
    "os/exec"

    nmappb "github.com/you/my-nmap-tool/api/gen/nmap/v1"
    graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
    "github.com/zeroroot-ai/sdk/serve"
    "google.golang.org/protobuf/proto"
)

type NmapTool struct{}

func (t *NmapTool) Name() string              { return "my-nmap-tool" }
func (t *NmapTool) Version() string           { return "v0.1.0" }
func (t *NmapTool) InputMessageType() string  { return "mytool.nmap.v1.NmapRequest" }
func (t *NmapTool) OutputMessageType() string { return "mytool.nmap.v1.NmapResponse" }

// Execute is the tool's main entrypoint. SDK serves it over gRPC.
func (t *NmapTool) Execute(ctx context.Context, in proto.Message) (proto.Message, error) {
    req := in.(*nmappb.NmapRequest)

    // 1. Run nmap.
    args := []string{"-sV", "-oX", "-"}
    if req.ScanType != "" {
        args = append(args, "-s"+req.ScanType[:1])
    }
    args = append(args, req.Targets...)
    out, err := exec.CommandContext(ctx, "nmap", args...).Output()
    if err != nil {
        return nil, err
    }

    // 2. Parse nmap's XML output (use a parser library — omitted here).
    var parsed nmapXML
    xml.Unmarshal(out, &parsed)

    // 3. Translate parsed XML → DiscoveryResult.
    //    THIS is the part that lights up the knowledge graph.
    discovery := &graphragpb.DiscoveryResult{}
    for _, h := range parsed.Hosts {
        discovery.Hosts = append(discovery.Hosts, &graphragpb.Host{
            Ip:       h.Address,
            Hostname: ptr(h.Hostname),
        })
        for _, p := range h.Ports {
            discovery.Ports = append(discovery.Ports, &graphragpb.Port{
                Number:   int32(p.Number),
                Protocol: p.Protocol,
                State:    ptr(p.State),
                HostId:   h.Address, // ← edge: Port BELONGS_TO Host
            })
            if p.Service != "" {
                discovery.Services = append(discovery.Services, &graphragpb.Service{
                    Name:    p.Service,
                    PortId:  fmt.Sprintf("%s:%d", h.Address, p.Number), // ← edge: Service RUNS_ON Port
                    Version: p.Version,
                })
            }
        }
    }

    // 4. Return the response with discovery in field 100.
    return &nmappb.NmapResponse{
        RawXml:              string(out),
        HostsScanned:        int32(len(parsed.Hosts)),
        ScanDurationSeconds: parsed.Duration,
        Discovery:           discovery, // ← the magic field
    }, nil
}

func ptr[T any](v T) *T { return &v }

func main() {
    // serve.Tool handles all gRPC plumbing, FileDescriptorSet registration
    // with gibson, mode selection (local vs platform), health probes, OTel.
    if err := serve.Tool(&NmapTool{}); err != nil {
        panic(err)
    }
}
```

That's the entire tool — about 80 lines of your code.

**Step 4 — Deploy**

`make docker-build && make docker-push`, then either mode:

- **Local mode**: deploy as a Kubernetes service, expose port `50051`, install via `gibson component install <git-url>`. Gibson sends work to your tool's gRPC port.
- **Platform mode**: set `PLATFORM_URL=gibson:50002` env var. `serve.Tool()` switches to outbound — your tool polls gibson for work via `ComponentService.PollWork` instead of running its own server. Better for scaling: replicas can come and go without reconfiguring gibson.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nmap-tool
spec:
  replicas: 5  # Scale horizontally
  template:
    spec:
      containers:
      - name: tool
        image: ghcr.io/you/my-nmap-tool:v0.1.0
        env:
        - name: PLATFORM_URL
          value: gibson:50002
        - name: BOOTSTRAP_TOKEN
          valueFrom:
            secretKeyRef:
              name: gibson-bootstrap
              key: token
```

On first connection your tool sends its FileDescriptorSet (the schema for `NmapRequest`/`NmapResponse`) to gibson via `RegisterComponent`. Gibson caches it via `protoresolver`. From this point on, gibson knows how to call your tool by name and how to interpret its responses — without any compile-time dependency on your tool's Go types.

**Step 5 — How an agent uses it**

In an agent (a separate component):

```go
import (
    nmappb "github.com/you/my-nmap-tool/api/gen/nmap/v1"  // ← agent imports the tool's Go bindings
)

// Inside the agent's reasoning loop:
req := &nmappb.NmapRequest{
    Targets:  []string{"10.0.0.0/24"},
    ScanType: "syn",
    Ports:    []string{"1-1024"},
}
resp := &nmappb.NmapResponse{}
if err := h.CallToolProto(ctx, "my-nmap-tool", req, resp); err != nil {
    return err
}

slog.Info("nmap done", "hosts", resp.HostsScanned, "duration", resp.ScanDurationSeconds)

// The agent does NOT need to do anything for the graph — gibson already
// extracted resp.Discovery and wrote Hosts, Ports, Services + their
// relationships to Neo4j BEFORE returning the response to the agent.
```

### What gibson does automatically (the "free" part)

The instant your tool's response hits gibson's `HarnessCallbackService.CallToolProto` handler, the `DiscoveryProcessor`:

1. Uses `protoresolver` to find field 100 of the cached `NmapResponse` descriptor.
2. Reflects on `resp.Discovery` to extract every `Host`, `Port`, `Service`, etc.
3. For each one, calls the GraphRAG `loader` to write/upsert the corresponding Neo4j node — taxonomy-typed (`:Host {ip: "10.0.0.5"}`, `:Port {number: 443, protocol: "tcp"}`, `:Service {name: "https"}`).
4. Creates the relationships from `HostId` / `PortId` cross-references: `(:Port)-[:BELONGS_TO]->(:Host)`, `(:Service)-[:RUNS_ON]->(:Port)`.
5. Tags everything with mission/agent/run IDs so the orchestrator's cross-mission intelligence queries can later say "the orchestrator's prior knowledge for 10.0.0.5 includes these 12 ports from mission XYZ."

### What you DID NOT have to do

- Write any Neo4j Cypher.
- Touch this SDK's source.
- Tell gibson about your proto types ahead of time (auto-discovered via FileDescriptorSet at registration).
- Implement any "save to graph" logic — that's gibson's responsibility per the field-100 contract.
- Care about which mission, target, or agent invoked you — gibson tags the graph entries with that context automatically.

### One sentence

You implement `Execute(req) → resp`, you put your structured discoveries in field 100, and the platform handles persistence + cross-mission learning + intelligence queries on top of your tool's output without you writing a line of platform code.

## Building a Plugin

Plugins provide stateful service integrations:

```go
type SlackPlugin struct {
    client *slack.Client
}

func (p *SlackPlugin) Name() string    { return "slack" }
func (p *SlackPlugin) Version() string { return "1.0.0" }

func (p *SlackPlugin) Initialize(ctx context.Context, cfg plugin.PluginConfig) error {
    token := cfg.Settings["token"].(string)
    p.client = slack.New(token)
    return nil
}

func (p *SlackPlugin) Methods() []plugin.MethodDescriptor {
    return []plugin.MethodDescriptor{
        {
            Name:        "send_message",
            Description: "Send a message to a Slack channel",
            InputSchema: schema.JSON(`{
                "type": "object",
                "properties": {
                    "channel": {"type": "string"},
                    "message": {"type": "string"}
                },
                "required": ["channel", "message"]
            }`),
        },
    }
}

func (p *SlackPlugin) Query(ctx context.Context, method string, params map[string]any) (any, error) {
    switch method {
    case "send_message":
        channel := params["channel"].(string)
        message := params["message"].(string)
        _, _, err := p.client.PostMessage(channel, slack.MsgOptionText(message, false))
        return map[string]any{"sent": true}, err
    default:
        return nil, fmt.Errorf("unknown method: %s", method)
    }
}

func main() {
    serve.Plugin(&SlackPlugin{}, serve.WithPort(50053))
}
```

## GraphRAG Knowledge Graph

Every entity discovered by agents persists in Neo4j with full relationship mapping:

```go
// Create entities with automatic UUID assignment
host := graphrag.NewHost()
host.Ip = "192.168.1.100"
host.Hostname = "api-server"
host.Os = "Ubuntu 22.04"

// Child entities auto-wire parent relationships
port := graphrag.NewPort(host, 443, "tcp")
service := graphrag.NewService(port, "nginx")
endpoint := graphrag.NewEndpoint(service, "/api/v1/status")

// Emit into the World; the daemon projects it into the graph
h.Observe(ctx, agent.HostObservation{Address: "10.0.0.5"})

// Query knowledge
results, _ := h.QueryNodes(ctx, &graphragpb.GraphQuery{
    NodeType: "host",
    Filters: map[string]string{"os": "Ubuntu*"},
})
```

**Schema Highlights:**

```
Host ──[HAS_PORT]──▶ Port ──[RUNS_SERVICE]──▶ Service ──[HAS_ENDPOINT]──▶ Endpoint
Domain ──[HAS_SUBDOMAIN]──▶ Subdomain ──[RESOLVES_TO]──▶ Host
```

- UUID-based identity for mergeability
- CEL-based validation rules
- YAML-driven taxonomy (single source of truth)
- Cross-mission intelligence

## Package Structure

```
sdk/
├── agent/       # Agent interfaces and types
├── tool/        # Tool interfaces and worker system
├── plugin/      # Plugin interfaces
├── llm/         # LLM abstractions and message types
├── memory/      # Three-tier memory APIs
├── mission/     # Mission context types
├── result/      # Execution result types
├── health/      # Health check utilities
├── serve/       # gRPC serving utilities
├── graphrag/    # Knowledge graph integration
│   ├── domain/      # Generated domain types
│   ├── validation/  # CEL-based validators
│   └── id/          # Node ID generation
├── taxonomy/    # YAML-driven taxonomy
└── examples/    # Reference implementations
```

## gRPC Serving Options

```go
serve.Agent(agent,
    serve.WithPort(50051),
    serve.WithHealthPort(8080),  // K8s probes: /healthz, /readyz
    serve.WithTLS(certFile, keyFile),
    serve.WithLogger(logger),
)

serve.Tool(tool, serve.WithPort(50052))

serve.Plugin(plugin, serve.WithPort(50053))
```

## Documentation

| Guide | Description |
|-------|-------------|
| [Agent Development](docs/AGENTS.md) | Complete guide to building autonomous agents |
| [Tool Development](docs/TOOLS.md) | Building tools with automatic graph storage |
| [Plugin Development](docs/PLUGINS.md) | Creating stateful service integrations |

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
