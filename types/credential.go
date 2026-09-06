// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

// CredentialType represents the type of credential.
type CredentialType string

const (
	// CredentialTypeAPIKey represents an API key credential.
	CredentialTypeAPIKey CredentialType = "api_key"

	// CredentialTypeBearer represents a bearer token credential.
	CredentialTypeBearer CredentialType = "bearer"

	// CredentialTypeBasic represents a basic auth credential (username/password).
	CredentialTypeBasic CredentialType = "basic"

	// CredentialTypeOAuth represents an OAuth token credential.
	CredentialTypeOAuth CredentialType = "oauth"

	// CredentialTypeCustom represents a custom credential type.
	CredentialTypeCustom CredentialType = "custom"
)

// Credential represents a stored credential retrieved from the credential store.
// Agents, plugins, and tools should use credential names to retrieve credentials
// at runtime, never accept raw secrets as parameters.
type Credential struct {
	// Name is the unique identifier for this credential.
	Name string

	// Type indicates the credential type (api_key, bearer, basic, oauth, custom).
	Type CredentialType

	// Secret is the decrypted secret value (API key, token, or password).
	Secret string

	// Username is the username for BASIC auth credentials (empty for other types).
	Username string

	// Metadata contains additional key-value pairs for the credential.
	Metadata map[string]any
}
