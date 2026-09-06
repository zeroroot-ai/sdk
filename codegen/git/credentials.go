// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeroroot-ai/sdk/types"
)

// credentialProvider implements CredentialProvider for different credential types.
type credentialProvider struct {
	credential *types.Credential
}

// NewCredentialProvider creates a new credential provider from a types.Credential.
func NewCredentialProvider(cred *types.Credential) CredentialProvider {
	if cred == nil {
		return nil
	}
	return &credentialProvider{
		credential: cred,
	}
}

// ConfigureAuth configures Git authentication for the given repository path.
// Returns a cleanup function that must be called to remove temporary files.
func (c *credentialProvider) ConfigureAuth(ctx context.Context, repoPath string) (func(), error) {
	if c.credential == nil {
		return func() {}, nil
	}

	switch c.credential.Type {
	case types.CredentialTypeAPIKey, types.CredentialTypeBearer:
		return c.configureTokenAuth(ctx, repoPath)

	case types.CredentialTypeBasic:
		return c.configureBasicAuth(ctx, repoPath)

	case types.CredentialTypeCustom:
		// Assume SSH key for custom type
		return c.configureSSHAuth(ctx, repoPath)

	default:
		return nil, fmt.Errorf("unsupported credential type: %s", c.credential.Type)
	}
}

// configureTokenAuth configures HTTPS token authentication using a credential helper.
func (c *credentialProvider) configureTokenAuth(ctx context.Context, repoPath string) (func(), error) {
	// Use GIT_CONFIG_COUNT to set a local config for this operation
	// This approach doesn't modify the global or repository config

	// Create a credential helper script that provides the token
	helperScript := fmt.Sprintf(`#!/bin/sh
case "$1" in
get)
	echo "password=%s"
	;;
esac
`, c.credential.Secret)

	// Write helper script to a temporary file
	tempDir, err := os.MkdirTemp("", "git-cred-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	helperPath := filepath.Join(tempDir, "git-credential-helper.sh")
	if err := os.WriteFile(helperPath, []byte(helperScript), 0700); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write credential helper: %w", err)
	}

	// Configure git to use this credential helper via environment variables
	// We'll set GIT_CONFIG_KEY and GIT_CONFIG_VALUE environment variables
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Note: The actual configuration happens via git config in the git command
	// For now, we'll use a git credential helper approach
	gitConfigPath := filepath.Join(tempDir, "git-config")
	configContent := fmt.Sprintf("[credential]\n\thelper = \"%s\"\n", helperPath)
	if err := os.WriteFile(gitConfigPath, []byte(configContent), 0600); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to write git config: %w", err)
	}

	// Set GIT_CONFIG_GLOBAL to use our temporary config
	// This affects only git commands run in this process and its children
	os.Setenv("GIT_CONFIG_GLOBAL", gitConfigPath)

	return func() {
		os.Unsetenv("GIT_CONFIG_GLOBAL")
		cleanup()
	}, nil
}

// configureBasicAuth configures HTTPS basic authentication.
func (c *credentialProvider) configureBasicAuth(ctx context.Context, repoPath string) (func(), error) {
	// Similar to token auth, but include both username and password
	helperScript := fmt.Sprintf(`#!/bin/sh
case "$1" in
get)
	echo "username=%s"
	echo "password=%s"
	;;
esac
`, c.credential.Username, c.credential.Secret)

	tempDir, err := os.MkdirTemp("", "git-cred-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	helperPath := filepath.Join(tempDir, "git-credential-helper.sh")
	if err := os.WriteFile(helperPath, []byte(helperScript), 0700); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write credential helper: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	gitConfigPath := filepath.Join(tempDir, "git-config")
	configContent := fmt.Sprintf("[credential]\n\thelper = \"%s\"\n", helperPath)
	if err := os.WriteFile(gitConfigPath, []byte(configContent), 0600); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to write git config: %w", err)
	}

	os.Setenv("GIT_CONFIG_GLOBAL", gitConfigPath)

	return func() {
		os.Unsetenv("GIT_CONFIG_GLOBAL")
		cleanup()
	}, nil
}

// configureSSHAuth configures SSH key authentication.
func (c *credentialProvider) configureSSHAuth(ctx context.Context, repoPath string) (func(), error) {
	// Write SSH private key to a temporary file with secure permissions
	tempDir, err := os.MkdirTemp("", "git-ssh-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	keyPath := filepath.Join(tempDir, "id_rsa")

	// Write the SSH key with 0600 permissions (read/write for owner only)
	// This is critical for SSH to accept the key
	if err := os.WriteFile(keyPath, []byte(c.credential.Secret), 0600); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write SSH key: %w", err)
	}

	// Verify permissions are correct (defense in depth)
	info, err := os.Stat(keyPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to stat SSH key: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("SSH key permissions incorrect: got %o, expected 0600", info.Mode().Perm())
	}

	// Configure GIT_SSH_COMMAND to use this key
	// We also disable strict host key checking for automation scenarios
	// and add the host key automatically (you may want to make this configurable)
	sshCommand := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=%s",
		keyPath, filepath.Join(tempDir, "known_hosts"))

	os.Setenv("GIT_SSH_COMMAND", sshCommand)

	cleanup := func() {
		os.Unsetenv("GIT_SSH_COMMAND")
		// Securely remove the SSH key
		os.Remove(keyPath)    // Remove file first
		os.RemoveAll(tempDir) // Then remove directory
	}

	return cleanup, nil
}

// InlineCredentialProvider creates a credential provider that embeds credentials
// directly in the Git URL. This is less secure but works when credential helpers
// are not available. USE WITH CAUTION.
type InlineCredentialProvider struct {
	username string
	password string
}

// NewInlineCredentialProvider creates a provider that embeds credentials in URLs.
// This should only be used when credential helpers are not available.
func NewInlineCredentialProvider(username, password string) *InlineCredentialProvider {
	return &InlineCredentialProvider{
		username: username,
		password: password,
	}
}

// ConfigureAuth for inline credentials is a no-op since credentials are in the URL.
func (i *InlineCredentialProvider) ConfigureAuth(ctx context.Context, repoPath string) (func(), error) {
	// No configuration needed - credentials are embedded in URL
	return func() {}, nil
}

// TransformURL adds credentials to an HTTPS URL.
// Input:  https://github.com/org/repo.git
// Output: https://username:password@github.com/org/repo.git
func (i *InlineCredentialProvider) TransformURL(url string) (string, error) {
	if !strings.HasPrefix(url, "https://") {
		return url, nil // Don't transform non-HTTPS URLs
	}

	// Parse and inject credentials
	url = strings.TrimPrefix(url, "https://")

	// Check if credentials are already in URL
	if strings.Contains(url, "@") {
		return "https://" + url, nil
	}

	// Inject credentials
	return fmt.Sprintf("https://%s:%s@%s", i.username, i.password, url), nil
}

// sanitizeGitURL removes credentials from a Git URL for safe logging.
// This should be used whenever logging URLs to prevent credential leakage.
func sanitizeGitURL(url string) string {
	// Remove credentials from HTTPS URLs
	// https://user:pass@host/path -> https://***:***@host/path
	if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")

		if atIndex := strings.Index(url, "@"); atIndex != -1 {
			// Found credentials in URL
			host := url[atIndex:]
			return "https://***:***" + host
		}

		return "https://" + url
	}

	// SSH URLs don't typically contain credentials
	return url
}

// CloneWithInlineCredentials is a helper function that clones with inline credentials.
// This is useful when credential helpers cannot be used.
func CloneWithInlineCredentials(ctx context.Context, url, destPath, username, password string, opts CloneOptions) error {
	provider := NewInlineCredentialProvider(username, password)
	transformedURL, err := provider.TransformURL(url)
	if err != nil {
		return err
	}

	// Use the transformed URL with embedded credentials
	args := []string{"clone"}

	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}

	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}

	if opts.SingleBranch {
		args = append(args, "--single-branch")
	}

	args = append(args, transformedURL, destPath)

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Sanitize URL in error message
		sanitizedURL := sanitizeGitURL(url)
		return fmt.Errorf("git clone failed for %s: %w (output: %s)",
			sanitizedURL, err, strings.TrimSpace(string(output)))
	}

	return nil
}
