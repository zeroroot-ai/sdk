// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package extraction

import (
	"fmt"

	"github.com/google/uuid"
)

// Deterministic entity ID helpers. All IDs are UUID v5 (SHA-1 based) using
// a fixed namespace so that re-scanning the same target always produces the
// same entity IDs. This enables idempotent graph MERGE operations.

// gibsonNamespace is a fixed UUID namespace for all Gibson entity IDs.
var gibsonNamespace = uuid.NameSpaceOID

// HostID returns a deterministic UUID for a host identified by IP address.
func HostID(ip string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte("host:"+ip)).String()
}

// PortID returns a deterministic UUID for a port identified by parent host,
// port number, and protocol.
func PortID(hostID string, number int32, protocol string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte(fmt.Sprintf("port:%s:%d:%s", hostID, number, protocol))).String()
}

// ServiceID returns a deterministic UUID for a service identified by parent port.
func ServiceID(portID string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte("service:"+portID)).String()
}

// EndpointID returns a deterministic UUID for an endpoint identified by parent
// service, path, and HTTP method.
func EndpointID(serviceID string, path string, method string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte(fmt.Sprintf("endpoint:%s:%s:%s", serviceID, path, method))).String()
}

// DomainID returns a deterministic UUID for a domain.
func DomainID(domain string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte("domain:"+domain)).String()
}

// SubdomainID returns a deterministic UUID for a subdomain.
func SubdomainID(subdomain string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte("subdomain:"+subdomain)).String()
}

// FindingID returns a deterministic UUID for a finding identified by target
// entity and finding type/signature.
func FindingID(targetID string, findingType string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte(fmt.Sprintf("finding:%s:%s", targetID, findingType))).String()
}

// TechnologyID returns a deterministic UUID for a technology identified by
// name and version on a parent entity.
func TechnologyID(parentID string, name string, version string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte(fmt.Sprintf("technology:%s:%s:%s", parentID, name, version))).String()
}

// CertificateID returns a deterministic UUID for a certificate identified by
// its serial number or fingerprint.
func CertificateID(serialOrFingerprint string) string {
	return uuid.NewSHA1(gibsonNamespace, []byte("certificate:"+serialOrFingerprint)).String()
}

// StringPtr is a convenience helper to create a *string from a string value.
// Returns nil for empty strings, matching proto optional field semantics.
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
