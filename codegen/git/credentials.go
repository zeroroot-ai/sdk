// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeroroot-ai/sdk/types"
)

// ErrCredentialInvalid indicates that a credential value cannot be passed to git.
// The credential-helper protocol is line based, so a username or secret that
// contains a newline, a carriage return, or a NUL byte is refused.
var ErrCredentialInvalid = errors.New("credential value is not a single line")

// credentialHelperScript is the git credential helper the provider installs.
// git runs it as "sh <script> get" and reads username= and password= lines
// from its stdout, the credential-helper protocol.
// The text is fixed. It holds no secret and takes no interpolated value.
// The username and password live in files next to the script, readable by
// the owner only. The script reads them at request time and prints them
// with printf, so shell metacharacters in a secret are data, never code.
const credentialHelperScript = `# Git credential helper installed by the Zero Root SDK.
# This file holds no secret. The secret is in files next to it.
[ "$1" = "get" ] || exit 0
dir=$(dirname -- "$0")
if [ -f "$dir/username" ]; then
	printf 'username=%s\n' "$(cat -- "$dir/username")"
fi
printf 'password=%s\n' "$(cat -- "$dir/password")"
`

// credentialProvider implements CredentialProvider for different credential types.
type credentialProvider struct {
	credential *types.Credential

	// knownHostsFile is the persistent known_hosts file ssh verifies host keys
	// against. Empty means the ssh default, ~/.ssh/known_hosts.
	knownHostsFile string

	// acceptNewHostKeys opts in to StrictHostKeyChecking=accept-new. The
	// default is StrictHostKeyChecking=yes, so an unknown host is refused.
	acceptNewHostKeys bool
}

// Option configures a credential provider.
type Option func(*credentialProvider)

// WithKnownHostsFile points ssh at a persistent known_hosts file.
// The file must already contain the host key of every git host the
// provider connects to, unless WithAcceptNewHostKeys is also set.
func WithKnownHostsFile(path string) Option {
	return func(c *credentialProvider) {
		c.knownHostsFile = path
	}
}

// WithAcceptNewHostKeys relaxes host key verification to
// StrictHostKeyChecking=accept-new. The first connection to a host records
// its key in the known_hosts file, and later connections verify against it.
// A changed key is still refused. This is an explicit opt-in. Use it only
// for automation that cannot seed known_hosts ahead of time.
func WithAcceptNewHostKeys() Option {
	return func(c *credentialProvider) {
		c.acceptNewHostKeys = true
	}
}

// NewCredentialProvider creates a new credential provider from a types.Credential.
func NewCredentialProvider(cred *types.Credential, opts ...Option) CredentialProvider {
	if cred == nil {
		return nil
	}
	c := &credentialProvider{
		credential: cred,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ConfigureAuth configures Git authentication for the given repository path.
// Returns a cleanup function that must be called to remove temporary files.
func (c *credentialProvider) ConfigureAuth(ctx context.Context, repoPath string) (func(), error) {
	if c.credential == nil {
		return func() {}, nil
	}

	switch c.credential.Type {
	case types.CredentialTypeAPIKey, types.CredentialTypeBearer:
		return c.configureHelperAuth(ctx, "", c.credential.Secret)

	case types.CredentialTypeBasic:
		return c.configureHelperAuth(ctx, c.credential.Username, c.credential.Secret)

	case types.CredentialTypeCustom:
		// Assume SSH key for custom type
		return c.configureSSHAuth(ctx, repoPath)

	default:
		return nil, fmt.Errorf("unsupported credential type: %s", c.credential.Type)
	}
}

// configureHelperAuth installs a git credential helper for HTTPS token and
// basic authentication. The helper script is a fixed text. The username and
// password are written to owner-only files that the script reads at request
// time, so no credential value ever enters shell source.
func (c *credentialProvider) configureHelperAuth(_ context.Context, username, password string) (func(), error) {
	if err := validateCredentialLine(password); err != nil {
		return nil, fmt.Errorf("password: %w", err)
	}
	if err := validateCredentialLine(username); err != nil {
		return nil, fmt.Errorf("username: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "git-cred-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// The script is not executable. git runs it through sh, see the config
	// entry below, so the file stays owner-only like the secret files.
	helperPath := filepath.Join(tempDir, "git-credential-helper.sh")
	if err := os.WriteFile(helperPath, []byte(credentialHelperScript), 0600); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to write credential helper: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "password"), []byte(password), 0600); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to write credential file: %w", err)
	}

	if username != "" {
		if err := os.WriteFile(filepath.Join(tempDir, "username"), []byte(username), 0600); err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to write credential file: %w", err)
		}
	}

	// The first empty helper entry clears any helper inherited from the
	// system config, so only this helper answers for the operation.
	gitConfigPath := filepath.Join(tempDir, "git-config")
	helperCommand := "!sh " + shellQuote(helperPath)
	configContent := fmt.Sprintf("[credential]\n\thelper = \n\thelper = %s\n", gitConfigQuote(helperCommand))
	if err := os.WriteFile(gitConfigPath, []byte(configContent), 0600); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to write git config: %w", err)
	}

	// GIT_CONFIG_GLOBAL points git at the temporary config for this process
	// and its children.
	os.Setenv("GIT_CONFIG_GLOBAL", gitConfigPath)

	return func() {
		os.Unsetenv("GIT_CONFIG_GLOBAL")
		cleanup()
	}, nil
}

// configureSSHAuth configures SSH key authentication.
func (c *credentialProvider) configureSSHAuth(_ context.Context, _ string) (func(), error) {
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

	// Host keys are verified against a persistent known_hosts file.
	// The default refuses an unknown host. WithAcceptNewHostKeys records
	// the key of an unknown host on first contact and verifies it after.
	strictHostKeyChecking := "yes"
	if c.acceptNewHostKeys {
		strictHostKeyChecking = "accept-new"
	}

	sshCommand := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=%s",
		shellQuote(keyPath), strictHostKeyChecking)
	if c.knownHostsFile != "" {
		sshCommand += " -o UserKnownHostsFile=" + shellQuote(c.knownHostsFile)
	}

	os.Setenv("GIT_SSH_COMMAND", sshCommand)

	cleanup := func() {
		os.Unsetenv("GIT_SSH_COMMAND")
		// Securely remove the SSH key
		os.Remove(keyPath)    // Remove file first
		os.RemoveAll(tempDir) // Then remove directory
	}

	return cleanup, nil
}

// validateCredentialLine refuses a value that cannot travel as one line of
// the git credential-helper protocol.
func validateCredentialLine(value string) error {
	if strings.ContainsAny(value, "\n\r\x00") {
		return ErrCredentialInvalid
	}
	return nil
}

// shellQuote wraps a value in single quotes for a POSIX shell command line.
// A single quote inside the value is closed, escaped, and reopened.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// gitConfigQuote wraps a value in double quotes for a git config file.
// Backslashes and double quotes inside the value are escaped.
func gitConfigQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
