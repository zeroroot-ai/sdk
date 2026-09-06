# DiscoveryResult Proto Message Reference

## Overview

The `DiscoveryResult` message is a **standardized container** for declaring discovered entities during security assessments. Tools populate this message, and Gibson **automatically** persists the data to the GraphRAG knowledge graph - no graph database knowledge required.

## Key Concepts

- **Zero Graph Knowledge Required**: Tools just populate proto fields
- **Automatic Persistence**: Gibson handles all graph operations
- **Automatic Relationships**: Parent-child links inferred from foreign keys
- **Type Safety**: Proto3 schema validation
- **Extensible**: Support for custom nodes and relationships

## Location

- **Proto Definition**: `api/proto/graphrag.proto`
- **Generated Go Code**: `api/gen/graphragpb/graphrag.pb.go`
- **Documentation**: See `docs/TOOLS.md` for complete tool development guide

## Quick Start

### 1. Import Discovery Proto

```go
import graphragpb "github.com/zeroroot-ai/sdk/api/gen/graphragpb"
```

### 2. Add Discovery Field to Tool Response

```protobuf
syntax = "proto3";

package gibson.tools.mytool;

import "graphrag.proto";

message MyToolResponse {
    // Tool-specific fields (1-99)
    repeated Host hosts = 1;
    string raw_output = 2;

    // Discovery data (field 100 RESERVED across all tools)
    gibson.graphrag.DiscoveryResult discovery = 100;
}
```

### 3. Populate Discovery in Tool Code

```go
func (t *MyTool) ExecuteProto(ctx context.Context, input proto.Message) (proto.Message, error) {
    req := input.(*pb.MyToolRequest)

    // Run tool...
    results := executeTool(req)

    // Build discovery result
    discovery := &graphragpb.DiscoveryResult{
        Hosts: []*graphragpb.Host{
            {
                Ip:       "192.168.1.100",
                Hostname: "server01.example.com",
                State:    "up",
                Os:       "Linux 5.10",
            },
        },
        Ports: []*graphragpb.Port{
            {
                HostId:   "192.168.1.100",  // Links to parent host
                Number:   80,
                Protocol: "tcp",
                State:    "open",
            },
        },
    }

    // Return with discovery field populated
    return &pb.MyToolResponse{
        Hosts:     results.Hosts,  // Tool-specific format
        Discovery: discovery,       // Standard discovery format
    }, nil
}
```

That's it! Gibson automatically:
- Creates `Host:192.168.1.100` node
- Creates `Port:192.168.1.100:80:tcp` node
- Creates `(Port)-[:RUNS_ON]->(Host)` relationship

## Entity Categories

### Asset Discovery
- **Hosts**: IP addresses, hostnames, host state, OS
- **Ports**: Port numbers, protocols, states
- **Services**: Service names, versions, banners
- **Endpoints**: Web URLs, HTTP methods, status codes
- **Domains**: Root domains, registrar info
- **Subdomains**: Subdomains with parent relationships
- **Technologies**: Software, frameworks, libraries
- **Certificates**: TLS/SSL certificates with metadata

### Network Infrastructure
- **Networks**: Subnets, CIDR ranges, gateways
- **IPRanges**: IP address ranges with descriptions

### Cloud Resources
- **CloudAccounts**: AWS, GCP, Azure accounts
- **CloudAssets**: VPCs, instances, buckets, etc.
- **CloudRegions**: Cloud provider regions

### Kubernetes Resources
- **K8sClusters**: Kubernetes clusters
- **K8sNamespaces**: Namespaces with labels
- **K8sPods**: Pods with phase and IP
- **K8sContainers**: Containers within pods
- **K8sServices**: Services with ports
- **K8sIngresses**: Ingress rules with TLS

### Security Findings
- **Vulnerabilities**: CVEs with CVSS scores
- **Credentials**: Username/password pairs (hashed)
- **Secrets**: API keys, tokens (hashed)

## Advanced Features

### Explicit Relationships

When entities don't have a natural parent-child relationship, use **explicit relationships** to create custom links:

#### Use Cases
- Linking vulnerabilities to affected hosts
- Connecting credentials to discovered services
- Associating technologies with cloud assets
- Custom security relationships

#### Example: Vulnerability Affects Host

```go
discovery := &graphragpb.DiscoveryResult{
    // Declare the entities
    Hosts: []*graphragpb.Host{
        {Ip: "192.168.1.100", Hostname: "web01"},
    },
    Vulnerabilities: []*graphragpb.Vulnerability{
        {
            Id:          "CVE-2024-1234",
            Name:        "OpenSSL Buffer Overflow",
            CvssScore:   9.8,
            Description: "Critical buffer overflow in OpenSSL < 3.0.8",
        },
    },

    // Create explicit relationship
    ExplicitRelationships: []*graphragpb.ExplicitRelationship{
        {
            FromType: graphragpb.NodeType_NODE_TYPE_VULNERABILITY,
            FromId:   map[string]string{"id": "CVE-2024-1234"},
            ToType:   graphragpb.NodeType_NODE_TYPE_HOST,
            ToId:     map[string]string{"ip": "192.168.1.100"},
            RelationshipType: graphragpb.RelationType_RELATION_TYPE_AFFECTS,
            Properties: map[string]string{
                "severity":       "critical",
                "exploitable":    "true",
                "cvss_score":     "9.8",
                "attack_vector":  "network",
                "patch_available": "true",
            },
        },
    },
}
```

Creates graph:
```
(Vulnerability:CVE-2024-1234) -[:AFFECTS {severity:"critical", cvss_score:"9.8"}]-> (Host:192.168.1.100)
```

#### Example: Credential Found on Service

```go
discovery := &graphragpb.DiscoveryResult{
    Services: []*graphragpb.Service{
        {
            PortId:  "192.168.1.100:22:tcp",
            Name:    "ssh",
            Version: "OpenSSH 8.2",
        },
    },
    Credentials: []*graphragpb.Credential{
        {
            Username: "admin",
            Password: "password123",  // Should be hashed in production
            Location: "ssh://192.168.1.100:22",
        },
    },

    ExplicitRelationships: []*graphragpb.ExplicitRelationship{
        {
            FromType: graphragpb.NodeType_NODE_TYPE_CREDENTIAL,
            FromId: map[string]string{
                "username": "admin",
                "location": "ssh://192.168.1.100:22",
            },
            ToType: graphragpb.NodeType_NODE_TYPE_SERVICE,
            ToId:   map[string]string{"port_id": "192.168.1.100:22:tcp", "name": "ssh"},
            RelationshipType: graphragpb.RelationType_RELATION_TYPE_AUTHENTICATES_TO,
            Properties: map[string]string{
                "method":         "password",
                "successful":     "true",
                "discovered_at":  "2024-01-23T10:30:00Z",
            },
        },
    },
}
```

#### Available Relationship Types

Common relationship types defined in `graphrag.proto`:

| Relationship Type | Description | Example |
|-------------------|-------------|---------|
| `RUNS_ON` | Service runs on port/host | `(Service)-[:RUNS_ON]->(Port)` |
| `AFFECTS` | Vulnerability affects asset | `(Vuln)-[:AFFECTS]->(Host)` |
| `BELONGS_TO` | Resource belongs to parent | `(Pod)-[:BELONGS_TO]->(Namespace)` |
| `EXPOSED_BY` | Endpoint exposed by service | `(Endpoint)-[:EXPOSED_BY]->(Service)` |
| `SUBDOMAIN_OF` | Subdomain of domain | `(Subdomain)-[:SUBDOMAIN_OF]->(Domain)` |
| `USES_TECHNOLOGY` | Asset uses technology | `(Host)-[:USES_TECHNOLOGY]->(Tech)` |
| `PROTECTED_BY_SG` | Protected by security group | `(Asset)-[:PROTECTED_BY_SG]->(SG)` |
| `AUTHENTICATES_TO` | Credential authenticates to service | `(Cred)-[:AUTHENTICATES_TO]->(Service)` |
| `ISSUED_BY` | Certificate issued by CA | `(Cert)-[:ISSUED_BY]->(CA)` |

### Custom Nodes

Create entity types **not in the standard taxonomy** using custom nodes:

#### Use Cases
- WAF rules, firewall rules
- Custom security policies
- Proprietary asset types
- Organization-specific entities

#### Example: WAF Rule

```go
discovery := &graphragpb.DiscoveryResult{
    Hosts: []*graphragpb.Host{
        {Ip: "192.168.1.100", Hostname: "web01"},
    },

    CustomNodes: []*graphragpb.CustomNode{
        {
            NodeType: "WAFRule",
            IdProperties: map[string]string{
                "rule_id":  "12345",
                "waf_name": "cloudflare",
            },
            Properties: map[string]string{
                "pattern":     "^/admin/.*",
                "action":      "block",
                "enabled":     "true",
                "priority":    "high",
                "description": "Block admin panel access",
            },
            ParentType: graphragpb.NodeType_NODE_TYPE_HOST,
            ParentId:   map[string]string{"ip": "192.168.1.100"},
            RelationshipType: graphragpb.RelationType_RELATION_TYPE_PROTECTED_BY_SG,
        },
    },
}
```

Creates graph:
```
(WAFRule:12345:cloudflare) -[:PROTECTED_BY_SG]-> (Host:192.168.1.100)
```

#### Example: API Rate Limit Policy

```go
discovery := &graphragpb.DiscoveryResult{
    Endpoints: []*graphragpb.Endpoint{
        {
            Url:    "https://api.example.com/users",
            Method: "GET",
        },
    },

    CustomNodes: []*graphragpb.CustomNode{
        {
            NodeType: "RateLimitPolicy",
            IdProperties: map[string]string{
                "policy_id": "api-rate-limit-001",
            },
            Properties: map[string]string{
                "max_requests":   "1000",
                "time_window":    "60s",
                "burst_allowed":  "true",
                "burst_size":     "100",
            },
            // Link to endpoint via explicit relationship
            Relationships: []*graphragpb.ExplicitRelationship{
                {
                    FromType: graphragpb.NodeType_NODE_TYPE_CUSTOM,
                    FromId: map[string]string{
                        "node_type": "RateLimitPolicy",
                        "policy_id": "api-rate-limit-001",
                    },
                    ToType: graphragpb.NodeType_NODE_TYPE_ENDPOINT,
                    ToId: map[string]string{
                        "url":    "https://api.example.com/users",
                        "method": "GET",
                    },
                    RelationshipType: graphragpb.RelationType_RELATION_TYPE_PROTECTED_BY_SG,
                },
            },
        },
    },
}
```

#### Custom Node Best Practices

1. **Use descriptive node types**: `WAFRule`, `FirewallPolicy`, `ComplianceCheck`
2. **Choose unique ID properties**: Ensure `IdProperties` uniquely identify the node
3. **Link to standard entities**: Use `ParentType`/`ParentId` or explicit relationships
4. **Document custom types**: Add comments in code explaining custom node semantics

## Automatic Relationship Inference

Gibson creates relationships automatically based on foreign key fields:

### Parent-Child Relationships

| Child Entity | Parent Entity | Foreign Key Field | Relationship Type | Graph Example |
|--------------|---------------|-------------------|-------------------|---------------|
| Port | Host | `host_id` | `RUNS_ON` | `(Port)-[:RUNS_ON]->(Host)` |
| Service | Port | `port_id` | `RUNS_ON` | `(Service)-[:RUNS_ON]->(Port)` |
| Endpoint | Service | `service_id` | `EXPOSED_BY` | `(Endpoint)-[:EXPOSED_BY]->(Service)` |
| Subdomain | Domain | `domain_id` | `SUBDOMAIN_OF` | `(Subdomain)-[:SUBDOMAIN_OF]->(Domain)` |
| K8sNamespace | K8sCluster | `cluster_id` | `BELONGS_TO` | `(Namespace)-[:BELONGS_TO]->(Cluster)` |
| K8sPod | K8sNamespace | `namespace_id` | `BELONGS_TO` | `(Pod)-[:BELONGS_TO]->(Namespace)` |
| K8sContainer | K8sPod | `pod_id` | `RUNS_IN` | `(Container)-[:RUNS_IN]->(Pod)` |
| CloudAsset | CloudAccount | `account_id` | `BELONGS_TO` | `(Asset)-[:BELONGS_TO]->(Account)` |

### How It Works

When you populate entities with foreign keys, Gibson automatically creates the graph structure:

```go
discovery := &graphragpb.DiscoveryResult{
    Hosts: []*graphragpb.Host{
        {Ip: "192.168.1.100", Hostname: "web01"},
    },
    Ports: []*graphragpb.Port{
        {
            HostId:   "192.168.1.100",  // Foreign key
            Number:   443,
            Protocol: "tcp",
            State:    "open",
        },
    },
    Services: []*graphragpb.Service{
        {
            PortId:  "192.168.1.100:443:tcp",  // Foreign key
            Name:    "https",
            Version: "nginx 1.18.0",
        },
    },
}
```

Creates this graph:
```
(Service:https) -[:RUNS_ON]-> (Port:443) -[:RUNS_ON]-> (Host:192.168.1.100)
```

### Field Numbering Convention

To ensure consistency across **all Gibson tools**:

| Field Range | Purpose | Usage | Example |
|-------------|---------|-------|---------|
| **1-99** | Tool-specific response fields | Any tool-specific data | `hosts`, `ports`, `raw_output`, `scan_stats` |
| **100** | **Discovery data (RESERVED)** | Gibson discovery entities | `gibson.graphrag.DiscoveryResult discovery = 100;` |

**CRITICAL**: Always use field number **100** for the discovery field. This is a framework-wide convention that cannot be changed.

## Composite Keys and Deduplication

Each entity type has **identifying properties** that form a unique composite key. Gibson uses these keys to deduplicate entities and merge data from multiple tools.

### Identifying Properties by Entity Type

| Entity Type | Identifying Properties | Example Key |
|-------------|------------------------|-------------|
| **Host** | `ip` | `192.168.1.100` |
| **Port** | `host_id`, `number`, `protocol` | `192.168.1.100:80:tcp` |
| **Service** | `port_id`, `name` | `192.168.1.100:80:tcp:http` |
| **Endpoint** | `url`, `method` | `https://api.example.com/users:GET` |
| **Domain** | `name` | `example.com` |
| **Subdomain** | `name` | `api.example.com` |
| **Certificate** | `serial_number` | `4A:3F:2D:...` |
| **Technology** | `name`, `version` | `nginx:1.18.0` |
| **Vulnerability** | `id` | `CVE-2024-1234` |
| **Credential** | `username`, `location` | `admin:192.168.1.100` |
| **Secret** | `key_hash` | `sha256:a4f3...` |
| **Network** | `cidr` | `192.168.1.0/24` |
| **IpRange** | `start_ip`, `end_ip` | `10.0.0.1-10.0.0.255` |
| **K8sCluster** | `cluster_id` | `prod-east-1` |
| **K8sNamespace** | `cluster_id`, `name` | `prod-east-1:default` |
| **K8sPod** | `namespace_id`, `name` | `default:webapp-7d8f...` |
| **K8sContainer** | `pod_id`, `name` | `webapp-7d8f...:nginx` |
| **K8sService** | `namespace_id`, `name` | `default:webapp-service` |
| **K8sIngress** | `namespace_id`, `name` | `default:webapp-ingress` |
| **CloudAccount** | `account_id`, `provider` | `123456789012:aws` |
| **CloudAsset** | `account_id`, `asset_id` | `123456789012:i-0abc123...` |
| **CloudRegion** | `provider`, `region` | `aws:us-east-1` |

### Deduplication Example

When multiple tools discover the same entity, Gibson merges them:

**Tool 1 (Mytool-a):**
```go
discovery := &graphragpb.DiscoveryResult{
    Hosts: []*graphragpb.Host{
        {
            Ip:    "192.168.1.100",
            State: "up",
        },
    },
}
```

**Tool 2 (Web Scanner):**
```go
discovery := &graphragpb.DiscoveryResult{
    Hosts: []*graphragpb.Host{
        {
            Ip:       "192.168.1.100",
            Hostname: "web01.example.com",
            Os:       "Linux 5.10",
        },
    },
}
```

**Merged Result in Graph:**
```cypher
CREATE (h:Host {
    ip: "192.168.1.100",
    state: "up",               // From Tool 1
    hostname: "web01.example.com",  // From Tool 2
    os: "Linux 5.10"           // From Tool 2
})
```

The GraphRAG system intelligently merges properties from multiple sources.

## Complete Example: Kubernetes Scanner

Here's a comprehensive example showing multiple entity types with relationships:

```go
package k8sscanner

import (
    "context"

    graphragpb "github.com/zeroroot-ai/sdk/api/gen/graphragpb"
    pb "github.com/myorg/k8sscanner/proto"
    "google.golang.org/protobuf/proto"
)

func (t *K8sScanner) ExecuteProto(ctx context.Context, input proto.Message) (proto.Message, error) {
    req := input.(*pb.ScanRequest)

    // Scan Kubernetes cluster
    clusterInfo := scanCluster(req.ClusterEndpoint)

    // Build comprehensive discovery result
    discovery := &graphragpb.DiscoveryResult{
        // Cluster
        K8sClusters: []*graphragpb.K8sCluster{
            {
                ClusterId: "prod-east-1",
                Name:      "Production East",
                Version:   "1.28.0",
                Provider:  "aws",
                Region:    "us-east-1",
            },
        },

        // Namespaces
        K8sNamespaces: []*graphragpb.K8sNamespace{
            {
                ClusterId: "prod-east-1",  // Links to cluster
                Name:      "default",
                Labels: map[string]string{
                    "env": "production",
                },
            },
        },

        // Pods
        K8sPods: []*graphragpb.K8sPod{
            {
                NamespaceId: "prod-east-1:default",  // Links to namespace
                Name:        "webapp-7d8f9c6b-x5z2k",
                Phase:       "Running",
                Ip:          "10.244.1.15",
                Node:        "worker-node-01",
            },
        },

        // Containers
        K8sContainers: []*graphragpb.K8sContainer{
            {
                PodId: "default:webapp-7d8f9c6b-x5z2k",  // Links to pod
                Name:  "nginx",
                Image: "nginx:1.18.0",
            },
        },

        // Services
        K8sServices: []*graphragpb.K8sService{
            {
                NamespaceId: "prod-east-1:default",
                Name:        "webapp-service",
                Type:        "LoadBalancer",
                Ports: []*graphragpb.K8sServicePort{
                    {Number: 80, Protocol: "TCP"},
                },
            },
        },

        // Discovered vulnerabilities
        Vulnerabilities: []*graphragpb.Vulnerability{
            {
                Id:          "CVE-2024-1234",
                Name:        "Nginx Vulnerability",
                CvssScore:   7.5,
                Description: "Buffer overflow in nginx < 1.19.0",
            },
        },

        // Link vulnerability to container
        ExplicitRelationships: []*graphragpb.ExplicitRelationship{
            {
                FromType: graphragpb.NodeType_NODE_TYPE_VULNERABILITY,
                FromId:   map[string]string{"id": "CVE-2024-1234"},
                ToType:   graphragpb.NodeType_NODE_TYPE_K8S_CONTAINER,
                ToId: map[string]string{
                    "pod_id": "default:webapp-7d8f9c6b-x5z2k",
                    "name":   "nginx",
                },
                RelationshipType: graphragpb.RelationType_RELATION_TYPE_AFFECTS,
                Properties: map[string]string{
                    "severity": "high",
                    "cvss":     "7.5",
                },
            },
        },

        // Custom security policy
        CustomNodes: []*graphragpb.CustomNode{
            {
                NodeType: "NetworkPolicy",
                IdProperties: map[string]string{
                    "policy_name": "deny-all-ingress",
                    "namespace":   "default",
                },
                Properties: map[string]string{
                    "policy_type": "Ingress",
                    "action":      "Deny",
                    "enabled":     "true",
                },
                ParentType: graphragpb.NodeType_NODE_TYPE_K8S_NAMESPACE,
                ParentId:   map[string]string{"cluster_id": "prod-east-1", "name": "default"},
                RelationshipType: graphragpb.RelationType_RELATION_TYPE_PROTECTED_BY_SG,
            },
        },
    }

    return &pb.ScanResponse{
        ClusterInfo: clusterInfo,
        Discovery:   discovery,
    }, nil
}
```

This creates a comprehensive graph:

```
(Cluster:prod-east-1)
  ├─[:BELONGS_TO]─ (Namespace:default)
  │                  ├─[:BELONGS_TO]─ (Pod:webapp-7d8f9c6b-x5z2k)
  │                  │                  └─[:RUNS_IN]─ (Container:nginx)
  │                  │                                  ↑
  │                  │                                  └─[:AFFECTS]─ (Vulnerability:CVE-2024-1234)
  │                  ├─[:BELONGS_TO]─ (Service:webapp-service)
  │                  └─[:PROTECTED_BY_SG]─ (NetworkPolicy:deny-all-ingress)
```

## How Gibson Processes Discovery

The harness automatically handles all graph operations:

### 1. Detection

Gibson uses reflection to find the `discovery` field in tool responses:

```go
// Gibson internal code (you don't write this)
func extractDiscovery(response proto.Message) *graphragpb.DiscoveryResult {
    val := reflect.ValueOf(response).Elem()
    field := val.FieldByName("Discovery")
    if field.IsValid() {
        return field.Interface().(*graphragpb.DiscoveryResult)
    }
    return nil
}
```

### 2. Entity Creation

For each entity type, Gibson:
- Extracts identifying properties
- Creates or merges graph nodes
- Adds entity properties as node attributes

```go
// Gibson internal code (you don't write this)
for _, host := range discovery.Hosts {
    nodeID := host.Ip  // Identifying property
    properties := map[string]any{
        "ip":       host.Ip,
        "hostname": host.Hostname,
        "state":    host.State,
        "os":       host.Os,
    }
    graph.CreateOrMergeNode("Host", nodeID, properties)
}
```

### 3. Relationship Inference

Gibson automatically creates relationships based on foreign keys:

```go
// Gibson internal code (you don't write this)
for _, port := range discovery.Ports {
    portID := fmt.Sprintf("%s:%d:%s", port.HostId, port.Number, port.Protocol)

    // Create port node
    graph.CreateOrMergeNode("Port", portID, portProperties)

    // Create relationship to parent host
    if port.HostId != "" {
        graph.CreateRelationship(portID, port.HostId, "RUNS_ON")
    }
}
```

### 4. Custom Processing

Explicit relationships and custom nodes are processed after standard entities:

```go
// Gibson internal code (you don't write this)
for _, rel := range discovery.ExplicitRelationships {
    fromID := buildCompositeKey(rel.FromType, rel.FromId)
    toID := buildCompositeKey(rel.ToType, rel.ToId)
    graph.CreateRelationship(fromID, toID, rel.RelationshipType, rel.Properties)
}
```

## Proto Generation

After modifying `graphrag.proto`, regenerate Go code:

```bash
cd /home/anthony/Code/zeroroot.ai/opensource/sdk
make proto
```

This generates:
- `api/gen/graphragpb/graphrag.pb.go` - All message types
- `api/gen/graphragpb/graphrag_grpc.pb.go` - gRPC service stubs (if applicable)

## Testing Discovery Integration

### Unit Test Example

```go
func TestMyTool_Discovery(t *testing.T) {
    tool := New()

    req := &pb.ScanRequest{
        Target: "192.168.1.0/24",
    }

    resp, err := tool.ExecuteProto(context.Background(), req)
    require.NoError(t, err)

    scanResp := resp.(*pb.ScanResponse)

    // Verify discovery populated
    require.NotNil(t, scanResp.Discovery)
    assert.NotEmpty(t, scanResp.Discovery.Hosts)
    assert.NotEmpty(t, scanResp.Discovery.Ports)

    // Verify host details
    host := scanResp.Discovery.Hosts[0]
    assert.NotEmpty(t, host.Ip)
    assert.Equal(t, "up", host.State)

    // Verify port relationships
    port := scanResp.Discovery.Ports[0]
    assert.Equal(t, host.Ip, port.HostId)  // Foreign key set correctly
    assert.Greater(t, port.Number, int32(0))
}
```

### Integration Test with Neo4j

```go
// +build integration

func TestMyTool_GraphIntegration(t *testing.T) {
    // Start test Neo4j instance
    neo4jContainer := startTestNeo4j(t)
    defer neo4jContainer.Terminate()

    // Create harness with test graph
    harness := createTestHarness(neo4jContainer.URI())

    // Execute tool
    tool := New()
    req := &pb.ScanRequest{Target: "192.168.1.100"}
    resp, err := tool.ExecuteProto(context.Background(), req)
    require.NoError(t, err)

    // Submit to harness (triggers graph storage)
    harness.ProcessToolResponse(resp)

    // Query graph to verify
    query := `
        MATCH (h:Host {ip: "192.168.1.100"})-[:RUNS_ON]-(p:Port)
        RETURN h, p
    `
    result := neo4jContainer.Query(query)
    assert.NotEmpty(t, result)
}
```

## Migration Checklist

Migrating an existing tool to use automatic graph storage:

- [ ] Import discovery proto: `import graphragpb "github.com/zeroroot-ai/sdk/api/gen/graphragpb"`
- [ ] Add discovery field to response proto (field 100)
- [ ] Run `make proto` to regenerate Go code
- [ ] Create discovery result in tool code
- [ ] Populate entities with proper identifying properties
- [ ] Set foreign key fields for parent relationships
- [ ] Remove manual graph creation code (if any)
- [ ] Write unit tests for discovery population
- [ ] Verify graph creation in integration tests
- [ ] Update tool documentation

## Quick Reference Card

### Basic Discovery Pattern

```go
// 1. Import
import graphragpb "github.com/zeroroot-ai/sdk/api/gen/graphragpb"

// 2. Create discovery result
discovery := &graphragpb.DiscoveryResult{
    Hosts: []*graphragpb.Host{
        {Ip: "192.168.1.100", Hostname: "web01", State: "up"},
    },
    Ports: []*graphragpb.Port{
        {HostId: "192.168.1.100", Number: 80, Protocol: "tcp"},
    },
}

// 3. Return in response (field 100)
return &pb.MyToolResponse{
    Discovery: discovery,
}
```

### Common Entity Patterns

```go
// Host
{Ip: "192.168.1.100", Hostname: "server01", State: "up", Os: "Linux"}

// Port (links to host via host_id)
{HostId: "192.168.1.100", Number: 443, Protocol: "tcp", State: "open"}

// Service (links to port via port_id)
{PortId: "192.168.1.100:443:tcp", Name: "https", Version: "nginx 1.18"}

// Endpoint
{Url: "https://api.example.com/users", Method: "GET", StatusCode: 200}

// Vulnerability
{Id: "CVE-2024-1234", Name: "Buffer Overflow", CvssScore: 9.8}

// K8s Pod
{NamespaceId: "prod:default", Name: "webapp-abc123", Phase: "Running"}
```

### Foreign Key Patterns

| Entity | Foreign Key Field | Points To |
|--------|-------------------|-----------|
| Port | `host_id` | Host's `ip` |
| Service | `port_id` | Port's composite key |
| Endpoint | `service_id` | Service's composite key |
| K8sNamespace | `cluster_id` | Cluster's `cluster_id` |
| K8sPod | `namespace_id` | Namespace's composite key |
| CloudAsset | `account_id` | CloudAccount's composite key |

### Reserved Field Numbers

```protobuf
message ToolResponse {
    // Tool-specific fields (1-99)
    repeated Result results = 1;
    string raw_output = 2;

    // Discovery (ALWAYS field 100)
    gibson.graphrag.DiscoveryResult discovery = 100;
}
```

### Explicit Relationship Template

```go
ExplicitRelationships: []*graphragpb.ExplicitRelationship{
    {
        FromType: graphragpb.NodeType_NODE_TYPE_<TYPE>,
        FromId:   map[string]string{"<key>": "<value>"},
        ToType:   graphragpb.NodeType_NODE_TYPE_<TYPE>,
        ToId:     map[string]string{"<key>": "<value>"},
        RelationshipType: graphragpb.RelationType_RELATION_TYPE_<TYPE>,
        Properties: map[string]string{"key": "value"},
    },
}
```

### Custom Node Template

```go
CustomNodes: []*graphragpb.CustomNode{
    {
        NodeType: "MyCustomType",
        IdProperties: map[string]string{"id": "unique-id"},
        Properties: map[string]string{"key": "value"},
        ParentType: graphragpb.NodeType_NODE_TYPE_<TYPE>,
        ParentId:   map[string]string{"<key>": "<value>"},
        RelationshipType: graphragpb.RelationType_RELATION_TYPE_<TYPE>,
    },
}
```

## See Also

- **[Tool Development Guide](../../docs/TOOLS.md)** - Complete guide to building Gibson tools
- **[Proto Definitions](./graphrag.proto)** - Complete proto schema reference
- **[SDK Examples](../../examples/)** - Example tool implementations
