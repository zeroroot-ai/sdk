// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package extraction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostID_Deterministic(t *testing.T) {
	id1 := HostID("10.0.0.1")
	id2 := HostID("10.0.0.1")
	assert.Equal(t, id1, id2, "same IP must produce same ID")
}

func TestHostID_DifferentInputs(t *testing.T) {
	id1 := HostID("10.0.0.1")
	id2 := HostID("10.0.0.2")
	assert.NotEqual(t, id1, id2, "different IPs must produce different IDs")
}

func TestPortID_Deterministic(t *testing.T) {
	hostID := HostID("10.0.0.1")
	id1 := PortID(hostID, 443, "tcp")
	id2 := PortID(hostID, 443, "tcp")
	assert.Equal(t, id1, id2)
}

func TestPortID_DifferentPort(t *testing.T) {
	hostID := HostID("10.0.0.1")
	id1 := PortID(hostID, 443, "tcp")
	id2 := PortID(hostID, 80, "tcp")
	assert.NotEqual(t, id1, id2)
}

func TestPortID_DifferentProtocol(t *testing.T) {
	hostID := HostID("10.0.0.1")
	id1 := PortID(hostID, 53, "tcp")
	id2 := PortID(hostID, 53, "udp")
	assert.NotEqual(t, id1, id2)
}

func TestServiceID_Deterministic(t *testing.T) {
	portID := PortID(HostID("10.0.0.1"), 443, "tcp")
	id1 := ServiceID(portID)
	id2 := ServiceID(portID)
	assert.Equal(t, id1, id2)
}

func TestEndpointID_Deterministic(t *testing.T) {
	svcID := ServiceID(PortID(HostID("10.0.0.1"), 443, "tcp"))
	id1 := EndpointID(svcID, "/api/v1/users", "GET")
	id2 := EndpointID(svcID, "/api/v1/users", "GET")
	assert.Equal(t, id1, id2)
}

func TestEndpointID_DifferentMethod(t *testing.T) {
	svcID := ServiceID(PortID(HostID("10.0.0.1"), 443, "tcp"))
	id1 := EndpointID(svcID, "/api/v1/users", "GET")
	id2 := EndpointID(svcID, "/api/v1/users", "POST")
	assert.NotEqual(t, id1, id2)
}

func TestDomainID_Deterministic(t *testing.T) {
	id1 := DomainID("example.com")
	id2 := DomainID("example.com")
	assert.Equal(t, id1, id2)
}

func TestSubdomainID_Deterministic(t *testing.T) {
	id1 := SubdomainID("api.example.com")
	id2 := SubdomainID("api.example.com")
	assert.Equal(t, id1, id2)
}

func TestFindingID_Deterministic(t *testing.T) {
	hostID := HostID("10.0.0.1")
	id1 := FindingID(hostID, "CVE-2024-1234")
	id2 := FindingID(hostID, "CVE-2024-1234")
	assert.Equal(t, id1, id2)
}

func TestFindingID_DifferentType(t *testing.T) {
	hostID := HostID("10.0.0.1")
	id1 := FindingID(hostID, "CVE-2024-1234")
	id2 := FindingID(hostID, "CVE-2024-5678")
	assert.NotEqual(t, id1, id2)
}

func TestTechnologyID_Deterministic(t *testing.T) {
	hostID := HostID("10.0.0.1")
	id1 := TechnologyID(hostID, "nginx", "1.24")
	id2 := TechnologyID(hostID, "nginx", "1.24")
	assert.Equal(t, id1, id2)
}

func TestCertificateID_Deterministic(t *testing.T) {
	id1 := CertificateID("ABC123DEF456")
	id2 := CertificateID("ABC123DEF456")
	assert.Equal(t, id1, id2)
}

func TestStringPtr(t *testing.T) {
	t.Run("non-empty", func(t *testing.T) {
		p := StringPtr("hello")
		assert.NotNil(t, p)
		assert.Equal(t, "hello", *p)
	})

	t.Run("empty returns nil", func(t *testing.T) {
		p := StringPtr("")
		assert.Nil(t, p)
	})
}

func TestAllIDsAreValidUUIDs(t *testing.T) {
	// All ID functions should return 36-char UUID strings (8-4-4-4-12)
	ids := []string{
		HostID("10.0.0.1"),
		PortID("host-id", 443, "tcp"),
		ServiceID("port-id"),
		EndpointID("svc-id", "/path", "GET"),
		DomainID("example.com"),
		SubdomainID("api.example.com"),
		FindingID("target-id", "CVE-2024-1234"),
		TechnologyID("parent-id", "nginx", "1.24"),
		CertificateID("fingerprint"),
	}

	for _, id := range ids {
		assert.Len(t, id, 36, "UUID should be 36 chars: %s", id)
		assert.Contains(t, id, "-", "UUID should contain dashes: %s", id)
	}
}
