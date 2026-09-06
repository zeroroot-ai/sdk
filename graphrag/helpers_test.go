// Code generated tests for helpers_generated.go
package graphrag

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
)

// ==================== ROOT TYPE TESTS ====================

func TestNewMission(t *testing.T) {
	mission := NewMission("test-mission", "https://target.com")

	require.NotNil(t, mission)
	assert.NotEmpty(t, mission.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(mission.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "test-mission", mission.Name)
	assert.Equal(t, "https://target.com", mission.Target)
}

func TestNewDomain(t *testing.T) {
	domain := NewDomain("example.com")

	require.NotNil(t, domain)
	assert.NotEmpty(t, domain.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(domain.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "example.com", domain.Name)
}

func TestNewHost(t *testing.T) {
	host := NewHost()

	require.NotNil(t, host)
	assert.NotEmpty(t, host.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(host.Id)
	assert.NoError(t, err, "Id should be a valid UUID")
}

func TestNewTechnology(t *testing.T) {
	tech := NewTechnology("nginx")

	require.NotNil(t, tech)
	assert.NotEmpty(t, tech.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(tech.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "nginx", tech.Name)
}

func TestNewCertificate(t *testing.T) {
	cert := NewCertificate()

	require.NotNil(t, cert)
	assert.NotEmpty(t, cert.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(cert.Id)
	assert.NoError(t, err, "Id should be a valid UUID")
}

func TestNewFinding(t *testing.T) {
	finding := NewFinding("SQL Injection", "critical")

	require.NotNil(t, finding)
	assert.NotEmpty(t, finding.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(finding.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "SQL Injection", finding.Title)
	assert.Equal(t, "critical", finding.Severity)
}

func TestNewTechnique(t *testing.T) {
	technique := NewTechnique("T1190", "Exploit Public-Facing Application")

	require.NotNil(t, technique)
	assert.NotEmpty(t, technique.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(technique.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "T1190", technique.TechniqueId)
	assert.Equal(t, "Exploit Public-Facing Application", technique.Name)
}

// ==================== CHILD TYPE TESTS ====================

func TestNewMissionRun(t *testing.T) {
	mission := NewMission("test-mission", "https://target.com")
	run := NewMissionRun(mission, 1, "actor-1", "tenant-a", "sha256:abc")

	require.NotNil(t, run)
	assert.NotEmpty(t, run.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(run.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, mission.Id, run.ParentMissionId)
	assert.Equal(t, int32(1), run.RunNumber)
	assert.Equal(t, "actor-1", run.ActorId)
	assert.Equal(t, "tenant-a", run.ActorTenantId)
	assert.Equal(t, "sha256:abc", run.MissionYamlDigest)
}

func TestNewMissionRun_PanicsOnEmptyParentId(t *testing.T) {
	mission := &taxonomypb.Mission{Name: "test-mission"} // No Id set

	assert.PanicsWithValue(t,
		"parent Mission must have Id set - use NewMission() or set Id manually",
		func() {
			NewMissionRun(mission, 1, "actor", "tenant", "digest")
		},
	)
}

func TestNewAgentRun(t *testing.T) {
	agentRun := NewAgentRun("network-recon", "actor-1", "tenant-a", "agent:network-recon", "1.0.0", false)

	require.NotNil(t, agentRun)
	assert.NotEmpty(t, agentRun.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(agentRun.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "network-recon", agentRun.AgentName)
	assert.Equal(t, "actor-1", agentRun.ActorId)
	assert.Equal(t, "tenant-a", agentRun.ActorTenantId)
	assert.Equal(t, "agent:network-recon", agentRun.ComponentName)
	assert.Equal(t, "1.0.0", agentRun.ComponentVersion)
	assert.False(t, agentRun.SystemOwned)
}

func TestNewToolExecution(t *testing.T) {
	agentRun := NewAgentRun("network-recon", "actor-1", "tenant-a", "agent:network-recon", "1.0.0", false)
	toolExec := NewToolExecution(agentRun, "mytool", "actor-1", "tenant-a", "tool:mytool", "7.94", false)

	require.NotNil(t, toolExec)
	assert.NotEmpty(t, toolExec.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(toolExec.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, agentRun.Id, toolExec.ParentAgentRunId)
	assert.Equal(t, "mytool", toolExec.ToolName)
	assert.Equal(t, "actor-1", toolExec.ActorId)
}

func TestNewToolExecution_PanicsOnEmptyParentId(t *testing.T) {
	agentRun := &taxonomypb.AgentRun{AgentName: "network-recon"} // No Id set

	assert.PanicsWithValue(t,
		"parent AgentRun must have Id set - use NewAgentRun() or set Id manually",
		func() {
			NewToolExecution(agentRun, "mytool", "actor", "tenant", "tool:mytool", "1.0", false)
		},
	)
}

func TestNewLlmCall(t *testing.T) {
	llmCall := NewLlmCall("claude-3-opus", "actor-1", "tenant-a", "agent:network-recon", "1.0.0", "claude-3-opus-20240229")

	require.NotNil(t, llmCall)
	assert.NotEmpty(t, llmCall.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(llmCall.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, "claude-3-opus", llmCall.Model)
	assert.Equal(t, "actor-1", llmCall.ActorId)
	assert.Equal(t, "claude-3-opus-20240229", llmCall.ModelId)
}

func TestNewSubdomain(t *testing.T) {
	domain := NewDomain("example.com")
	subdomain := NewSubdomain(domain, "www.example.com")

	require.NotNil(t, subdomain)
	assert.NotEmpty(t, subdomain.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(subdomain.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, domain.Id, subdomain.ParentDomainId)
	assert.Equal(t, "www.example.com", subdomain.Name)
}

func TestNewSubdomain_PanicsOnEmptyParentId(t *testing.T) {
	domain := &taxonomypb.Domain{Name: "example.com"} // No Id set

	assert.PanicsWithValue(t,
		"parent Domain must have Id set - use NewDomain() or set Id manually",
		func() {
			NewSubdomain(domain, "www.example.com")
		},
	)
}

func TestNewPort(t *testing.T) {
	host := NewHost()
	port := NewPort(host, 80, "tcp")

	require.NotNil(t, port)
	assert.NotEmpty(t, port.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(port.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, host.Id, port.ParentHostId)
	assert.Equal(t, int32(80), port.Number)
	assert.Equal(t, "tcp", port.Protocol)
}

func TestNewPort_PanicsOnEmptyParentId(t *testing.T) {
	host := &taxonomypb.Host{} // No Id set

	assert.PanicsWithValue(t,
		"parent Host must have Id set - use NewHost() or set Id manually",
		func() {
			NewPort(host, 80, "tcp")
		},
	)
}

func TestNewService(t *testing.T) {
	host := NewHost()
	port := NewPort(host, 80, "tcp")
	service := NewService(port, "http")

	require.NotNil(t, service)
	assert.NotEmpty(t, service.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(service.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, port.Id, service.ParentPortId)
	assert.Equal(t, "http", service.Name)
}

func TestNewService_PanicsOnEmptyParentId(t *testing.T) {
	port := &taxonomypb.Port{Number: 80, Protocol: "tcp"} // No Id set

	assert.PanicsWithValue(t,
		"parent Port must have Id set - use NewPort() or set Id manually",
		func() {
			NewService(port, "http")
		},
	)
}

func TestNewEndpoint(t *testing.T) {
	host := NewHost()
	port := NewPort(host, 443, "tcp")
	service := NewService(port, "https")
	endpoint := NewEndpoint(service, "https://api.example.com/v1")

	require.NotNil(t, endpoint)
	assert.NotEmpty(t, endpoint.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(endpoint.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, service.Id, endpoint.ParentServiceId)
	assert.Equal(t, "https://api.example.com/v1", endpoint.Url)
}

func TestNewEndpoint_PanicsOnEmptyParentId(t *testing.T) {
	service := &taxonomypb.Service{Name: "https"} // No Id set

	assert.PanicsWithValue(t,
		"parent Service must have Id set - use NewService() or set Id manually",
		func() {
			NewEndpoint(service, "https://api.example.com/v1")
		},
	)
}

func TestNewEvidence(t *testing.T) {
	finding := NewFinding("SQL Injection", "critical")
	evidence := NewEvidence(finding, "request")

	require.NotNil(t, evidence)
	assert.NotEmpty(t, evidence.Id)

	// Verify it's a valid UUID
	_, err := uuid.Parse(evidence.Id)
	assert.NoError(t, err, "Id should be a valid UUID")

	assert.Equal(t, finding.Id, evidence.ParentFindingId)
	assert.Equal(t, "request", evidence.Type)
}

func TestNewEvidence_PanicsOnEmptyParentId(t *testing.T) {
	finding := &taxonomypb.Finding{Title: "SQL Injection"} // No Id set

	assert.PanicsWithValue(t,
		"parent Finding must have Id set - use NewFinding() or set Id manually",
		func() {
			NewEvidence(finding, "request")
		},
	)
}

// ==================== INTEGRATION TESTS ====================

func TestHelpers_FullHierarchy(t *testing.T) {
	// Create a complete mission hierarchy using the new constructors.
	// NewAgentRun and NewLlmCall no longer take a parent parameter — the
	// parent relationship is established via SetMissionRunId / BelongsTo
	// on the domain types, not via the raw proto constructors.
	mission := NewMission("pentest-2024", "https://target.com")
	missionRun := NewMissionRun(mission, 1, "actor-1", "tenant-a", "sha256:abc")
	agentRun := NewAgentRun("network-recon", "actor-1", "tenant-a", "agent:network-recon", "1.0.0", false)
	toolExec := NewToolExecution(agentRun, "mytool", "actor-1", "tenant-a", "tool:mytool", "7.94", false)
	llmCall := NewLlmCall("claude-3-opus", "actor-1", "tenant-a", "agent:network-recon", "1.0.0", "claude-3-opus-20240229")

	// Verify all IDs are unique
	ids := []string{
		mission.Id,
		missionRun.Id,
		agentRun.Id,
		toolExec.Id,
		llmCall.Id,
	}

	uniqueIds := make(map[string]bool)
	for _, id := range ids {
		assert.NotEmpty(t, id)
		assert.False(t, uniqueIds[id], "Duplicate ID detected: %s", id)
		uniqueIds[id] = true

		// Verify each is a valid UUID
		_, err := uuid.Parse(id)
		assert.NoError(t, err, "Invalid UUID: %s", id)
	}

	// Verify parent relationships that are still set by constructors.
	assert.Equal(t, mission.Id, missionRun.ParentMissionId)
	assert.Equal(t, agentRun.Id, toolExec.ParentAgentRunId)

	// AgentRun and LlmCall no longer carry parent refs from their
	// constructors (they were moved to optional fields set via domain
	// type setters). Verify the identity fields instead.
	assert.Equal(t, "network-recon", agentRun.AgentName)
	assert.Equal(t, "claude-3-opus", llmCall.Model)
}

func TestHelpers_NetworkHierarchy(t *testing.T) {
	// Create a complete network hierarchy
	host := NewHost()
	port := NewPort(host, 443, "tcp")
	service := NewService(port, "https")
	endpoint := NewEndpoint(service, "https://api.example.com")

	// Verify all IDs are unique
	ids := []string{
		host.Id,
		port.Id,
		service.Id,
		endpoint.Id,
	}

	uniqueIds := make(map[string]bool)
	for _, id := range ids {
		assert.NotEmpty(t, id)
		assert.False(t, uniqueIds[id], "Duplicate ID detected: %s", id)
		uniqueIds[id] = true

		// Verify each is a valid UUID
		_, err := uuid.Parse(id)
		assert.NoError(t, err, "Invalid UUID: %s", id)
	}

	// Verify parent relationships
	assert.Equal(t, host.Id, port.ParentHostId)
	assert.Equal(t, port.Id, service.ParentPortId)
	assert.Equal(t, service.Id, endpoint.ParentServiceId)
}

func TestHelpers_FindingHierarchy(t *testing.T) {
	// Create finding with evidence
	finding := NewFinding("XSS Vulnerability", "high")
	evidence1 := NewEvidence(finding, "request")
	evidence2 := NewEvidence(finding, "response")

	// Verify all IDs are unique
	ids := []string{
		finding.Id,
		evidence1.Id,
		evidence2.Id,
	}

	uniqueIds := make(map[string]bool)
	for _, id := range ids {
		assert.NotEmpty(t, id)
		assert.False(t, uniqueIds[id], "Duplicate ID detected: %s", id)
		uniqueIds[id] = true

		// Verify each is a valid UUID
		_, err := uuid.Parse(id)
		assert.NoError(t, err, "Invalid UUID: %s", id)
	}

	// Verify parent relationships
	assert.Equal(t, finding.Id, evidence1.ParentFindingId)
	assert.Equal(t, finding.Id, evidence2.ParentFindingId)
}

func TestHelpers_DomainHierarchy(t *testing.T) {
	// Create domain with subdomains
	domain := NewDomain("example.com")
	sub1 := NewSubdomain(domain, "www.example.com")
	sub2 := NewSubdomain(domain, "api.example.com")

	// Verify all IDs are unique
	ids := []string{
		domain.Id,
		sub1.Id,
		sub2.Id,
	}

	uniqueIds := make(map[string]bool)
	for _, id := range ids {
		assert.NotEmpty(t, id)
		assert.False(t, uniqueIds[id], "Duplicate ID detected: %s", id)
		uniqueIds[id] = true

		// Verify each is a valid UUID
		_, err := uuid.Parse(id)
		assert.NoError(t, err, "Invalid UUID: %s", id)
	}

	// Verify parent relationships
	assert.Equal(t, domain.Id, sub1.ParentDomainId)
	assert.Equal(t, domain.Id, sub2.ParentDomainId)
}

// ==================== EDGE CASE TESTS ====================

func TestHelpers_EmptyStringParameters(t *testing.T) {
	// Test that helpers accept empty strings (validation is separate)
	mission := NewMission("", "")
	assert.NotEmpty(t, mission.Id)
	assert.Empty(t, mission.Name)
	assert.Empty(t, mission.Target)

	domain := NewDomain("")
	assert.NotEmpty(t, domain.Id)
	assert.Empty(t, domain.Name)
}

func TestHelpers_UniqueIdsOnMultipleCalls(t *testing.T) {
	// Verify that multiple calls generate different UUIDs
	mission1 := NewMission("test1", "https://target1.com")
	mission2 := NewMission("test2", "https://target2.com")

	assert.NotEqual(t, mission1.Id, mission2.Id,
		"Multiple calls should generate different UUIDs")
}

func TestHelpers_ParentIdImmutability(t *testing.T) {
	// Verify that child keeps reference to parent even if parent Id changes later
	host := NewHost()
	originalHostId := host.Id

	port := NewPort(host, 80, "tcp")
	assert.Equal(t, originalHostId, port.ParentHostId)

	// Change parent's Id (shouldn't affect child's reference)
	host.Id = uuid.New().String()
	assert.Equal(t, originalHostId, port.ParentHostId,
		"Child should keep original parent Id")
}

// ==================== BENCHMARK TESTS ====================

func BenchmarkNewMission(b *testing.B) {
	for range b.N {
		_ = NewMission("benchmark-mission", "https://target.com")
	}
}

func BenchmarkNewMissionRun(b *testing.B) {
	mission := NewMission("benchmark-mission", "https://target.com")
	b.ResetTimer()

	for range b.N {
		_ = NewMissionRun(mission, 1, "actor", "tenant", "digest")
	}
}

func BenchmarkNewFullHierarchy(b *testing.B) {
	for range b.N {
		mission := NewMission("benchmark-mission", "https://target.com")
		_ = NewMissionRun(mission, 1, "actor", "tenant", "digest")
		agentRun := NewAgentRun("network-recon", "actor", "tenant", "agent:recon", "1.0", false)
		_ = NewToolExecution(agentRun, "mytool", "actor", "tenant", "tool:mytool", "7.94", false)
		_ = NewLlmCall("claude-3-opus", "actor", "tenant", "agent:recon", "1.0", "claude-3-opus-id")
	}
}

func BenchmarkNewNetworkHierarchy(b *testing.B) {
	for range b.N {
		host := NewHost()
		port := NewPort(host, 443, "tcp")
		service := NewService(port, "https")
		_ = NewEndpoint(service, "https://api.example.com")
	}
}
