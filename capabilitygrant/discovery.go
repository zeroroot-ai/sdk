// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrPlatformURLNotHTTPS is returned when a platform URL does not use the
// https scheme. Every credential this client sends, the bootstrap token, the
// Kubernetes ServiceAccount token, or a JWT-SVID, travels in the Authorization
// header, so a cleartext platform URL is refused.
var ErrPlatformURLNotHTTPS = errors.New("capabilitygrant: platform URL must use https")

// ErrEndpointOrigin is returned when a discovery document names an endpoint
// whose scheme or host differs from the platform URL. The discovery document
// is unauthenticated, so an endpoint on another origin would receive the
// registration credential without any proof that it belongs to the platform.
var ErrEndpointOrigin = errors.New("capabilitygrant: discovery endpoint is not on the platform origin")

// MinProtocolVersion is the minimum Capability Grant Protocol version this client
// accepts from a discovery document.
const MinProtocolVersion = "1.0"

// wellKnownPath is the well-known URL path for the agent configuration document.
const wellKnownPath = "/.well-known/agent-configuration"

// DiscoveryDocument is the JSON document served at /.well-known/agent-configuration.
// It describes the platform's protocol version, supported agent modes, and the
// canonical endpoint URLs for all Agent Auth operations.
type DiscoveryDocument struct {
	// ProtocolVersion is the Capability Grant Protocol version implemented by the platform.
	// Must be >= MinProtocolVersion for this client to proceed.
	ProtocolVersion string `json:"protocol_version"`

	// ProviderName is the human-readable name of the platform provider.
	ProviderName string `json:"provider_name"`

	// Issuer is the canonical HTTPS URL of the platform. Used as the aud claim
	// target when signing host+jwt tokens during registration.
	Issuer string `json:"issuer"`

	// DefaultLocation is the default geographic region for new agent registrations.
	DefaultLocation string `json:"default_location"`

	// SupportedModes lists the agent execution modes the platform accepts.
	// Common values: "delegated", "autonomous".
	SupportedModes []string `json:"supported_modes"`

	// Endpoints contains the canonical URLs for each Agent Auth HTTP operation.
	Endpoints struct {
		// Register is the endpoint for host + agent registration (POST).
		Register string `json:"register"`
		// Execute is the endpoint for submitting agent execution results (POST).
		Execute string `json:"execute"`
		// List is the endpoint for listing registered agents (GET).
		List string `json:"list"`
		// Status is the endpoint for querying a specific agent's status (GET).
		Status string `json:"status"`
		// Revoke is the endpoint for revoking an agent registration (DELETE).
		Revoke string `json:"revoke"`
		// Introspect is the endpoint for introspecting a JWT token (POST).
		Introspect string `json:"introspect"`
	} `json:"endpoints"`
}

// Discover fetches and parses the /.well-known/agent-configuration document from
// baseURL. It returns an error if the request fails, the response is not valid
// JSON, or the document's protocol_version is less than MinProtocolVersion.
//
// If httpClient is nil, http.DefaultClient is used.
func Discover(ctx context.Context, baseURL string, httpClient *http.Client) (*DiscoveryDocument, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	base, err := ParsePlatformURL(baseURL)
	if err != nil {
		return nil, err
	}

	targetURL := strings.TrimRight(baseURL, "/") + wellKnownPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: build discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: discovery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("capabilitygrant: discovery returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: read discovery response: %w", err)
	}

	var doc DiscoveryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("capabilitygrant: parse discovery document: %w", err)
	}

	if err := checkProtocolVersion(doc.ProtocolVersion); err != nil {
		return nil, err
	}

	if err := checkEndpointOrigins(base, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// ParsePlatformURL parses a platform base URL and enforces the https scheme.
// It returns ErrPlatformURLNotHTTPS for any other scheme and an error for a
// URL with no host or with user information.
func ParsePlatformURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: parse platform URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return nil, fmt.Errorf("%w: %q", ErrPlatformURLNotHTTPS, raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("capabilitygrant: platform URL %q has no host", raw)
	}
	if u.User != nil {
		return nil, fmt.Errorf("capabilitygrant: platform URL %q must not carry user information", raw)
	}
	return u, nil
}

// checkEndpointOrigins rejects any endpoint in doc whose scheme or host
// differs from base. Every endpoint receives a credential, so each one must be
// on the origin the caller configured.
func checkEndpointOrigins(base *url.URL, doc *DiscoveryDocument) error {
	endpoints := []struct {
		name string
		raw  string
	}{
		{"register", doc.Endpoints.Register},
		{"execute", doc.Endpoints.Execute},
		{"list", doc.Endpoints.List},
		{"status", doc.Endpoints.Status},
		{"revoke", doc.Endpoints.Revoke},
		{"introspect", doc.Endpoints.Introspect},
	}
	for _, ep := range endpoints {
		if ep.raw == "" {
			continue
		}
		if err := checkEndpointOrigin(base, ep.name, ep.raw); err != nil {
			return err
		}
	}
	return nil
}

// checkEndpointOrigin rejects raw unless it is an absolute URL with the same
// scheme and host as base.
func checkEndpointOrigin(base *url.URL, name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %s endpoint %q: %w", ErrEndpointOrigin, name, raw, err)
	}
	if !strings.EqualFold(u.Scheme, base.Scheme) || !strings.EqualFold(u.Host, base.Host) {
		return fmt.Errorf("%w: %s endpoint %q, platform %s://%s", ErrEndpointOrigin, name, raw, base.Scheme, base.Host)
	}
	return nil
}

// checkProtocolVersion returns an error if version is below MinProtocolVersion.
// Versions are expected to be in "MAJOR.MINOR" format. An empty version string
// is treated as incompatible.
func checkProtocolVersion(version string) error {
	if version == "" {
		return errors.New("capabilitygrant: discovery document missing protocol_version")
	}

	// Parse major.minor for both the document and the minimum.
	docMajor, docMinor, err := parseVersion(version)
	if err != nil {
		return fmt.Errorf("capabilitygrant: invalid protocol_version %q: %w", version, err)
	}

	minMajor, minMinor, err := parseVersion(MinProtocolVersion)
	if err != nil {
		// This would be a programming error in this package itself.
		return fmt.Errorf("capabilitygrant: invalid MinProtocolVersion constant: %w", err)
	}

	if docMajor < minMajor || (docMajor == minMajor && docMinor < minMinor) {
		return fmt.Errorf("capabilitygrant: platform protocol_version %s is older than minimum required %s",
			version, MinProtocolVersion)
	}

	return nil
}

// parseVersion parses a "MAJOR.MINOR" version string and returns the components.
func parseVersion(v string) (major, minor int, err error) {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected MAJOR.MINOR format, got %q", v)
	}
	var maj, min int
	if _, err := fmt.Sscanf(parts[0], "%d", &maj); err != nil {
		return 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &min); err != nil {
		return 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}
	return maj, min, nil
}
