// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/types"
)

// sshKeyPathFromCommand returns the value of the -i flag in a GIT_SSH_COMMAND
// string. The value is single quoted by shellQuote.
func sshKeyPathFromCommand(sshCommand string) string {
	_, rest, ok := strings.Cut(sshCommand, "-i '")
	if !ok {
		return ""
	}
	keyPath, _, ok := strings.Cut(rest, "'")
	if !ok {
		return ""
	}
	return keyPath
}

// helperPathFromConfig returns the credential helper path named in the
// temporary git config that ConfigureAuth installs.
func helperPathFromConfig(t *testing.T) string {
	t.Helper()
	configPath := os.Getenv("GIT_CONFIG_GLOBAL")
	require.NotEmpty(t, configPath)
	content, err := os.ReadFile(configPath) //nolint:gosec // test reads the config it installed
	require.NoError(t, err)

	// The entry has the form: helper = "!sh '/path/to/script'"
	var helperPath string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "helper = \"!sh '")
		if !ok {
			continue
		}
		helperPath, _, _ = strings.Cut(rest, "'")
	}
	require.NotEmpty(t, helperPath, "config has no sh helper entry:\n%s", content)
	return helperPath
}

// runHelper runs the installed credential helper the way git does: as a
// shell script with the action as its argument and the request on stdin.
func runHelper(t *testing.T, helperPath, action string) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	cmd := exec.Command("sh", helperPath, action) //nolint:gosec // test runs the helper it just wrote
	cmd.Stdin = strings.NewReader("protocol=https\nhost=example.com\n\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), "helper failed: %s", stderr.String())
	return stdout.String()
}

// TestCredentialHelperSecretIsData shows that a secret full of shell
// metacharacters reaches git unchanged and is never run as shell code.
func TestCredentialHelperSecretIsData(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "")

	marker := filepath.Join(t.TempDir(), "pwned")
	secret := `pa"ss'w$(touch ` + marker + `)ord` + "`id`" + ` $HOME ; rm -rf / && echo *`

	tests := []struct {
		name       string
		credential *types.Credential
		want       string
	}{
		{
			name: "token",
			credential: &types.Credential{
				Type:   types.CredentialTypeBearer,
				Secret: secret,
			},
			want: "password=" + secret + "\n",
		},
		{
			name: "basic",
			credential: &types.Credential{
				Type:     types.CredentialTypeBasic,
				Username: `us"er$(touch ` + marker + `)`,
				Secret:   secret,
			},
			want: `username=us"er$(touch ` + marker + `)` + "\n" + "password=" + secret + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCredentialProvider(tt.credential)
			cleanup, err := provider.ConfigureAuth(context.Background(), t.TempDir())
			require.NoError(t, err)
			defer cleanup()

			helperPath := helperPathFromConfig(t)

			// The helper script itself never contains the secret
			script, err := os.ReadFile(helperPath) //nolint:gosec // test reads the helper it installed
			require.NoError(t, err)
			assert.NotContains(t, string(script), "touch")
			assert.NotContains(t, string(script), secret)

			// The helper and the secret files are owner-only
			for _, name := range []string{helperPath, filepath.Join(filepath.Dir(helperPath), "password")} {
				info, err := os.Stat(name)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), name)
			}

			// git asks with "get" and receives the exact value
			assert.Equal(t, tt.want, runHelper(t, helperPath, "get"))

			// The shell never ran the injected command
			_, err = os.Stat(marker)
			assert.True(t, os.IsNotExist(err), "secret was executed as shell code")

			// Other actions produce nothing
			assert.Empty(t, runHelper(t, helperPath, "store"))
		})
	}
}

// TestCredentialHelperRefusesMultiLineSecret shows that a value with a line
// break cannot be smuggled into the credential-helper protocol.
func TestCredentialHelperRefusesMultiLineSecret(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "")

	for _, cred := range []*types.Credential{
		{Type: types.CredentialTypeBearer, Secret: "tok\nurl=https://evil.example"},
		{Type: types.CredentialTypeBearer, Secret: "tok\rurl=https://evil.example"},
		{Type: types.CredentialTypeBasic, Username: "user\nquit=1", Secret: "pass"},
		{Type: types.CredentialTypeBasic, Username: "user", Secret: "pa\x00ss"},
	} {
		provider := NewCredentialProvider(cred)
		cleanup, err := provider.ConfigureAuth(context.Background(), t.TempDir())
		require.ErrorIs(t, err, ErrCredentialInvalid)
		assert.Nil(t, cleanup)
		assert.Empty(t, os.Getenv("GIT_CONFIG_GLOBAL"))
	}
}

// TestSSHHostKeyVerification shows that host keys are verified by default
// and that the relaxed mode is an explicit opt-in.
func TestSSHHostKeyVerification(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "")

	credential := &types.Credential{
		Type:   types.CredentialTypeCustom,
		Secret: "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
	}
	knownHosts := filepath.Join(t.TempDir(), "known hosts")

	tests := []struct {
		name        string
		opts        []Option
		wantStrict  string
		wantHosts   string
		notContains string
	}{
		{
			name:        "default verifies against the ssh default known_hosts",
			wantStrict:  "-o StrictHostKeyChecking=yes",
			notContains: "UserKnownHostsFile",
		},
		{
			name:       "caller supplied known_hosts",
			opts:       []Option{WithKnownHostsFile(knownHosts)},
			wantStrict: "-o StrictHostKeyChecking=yes",
			wantHosts:  "-o UserKnownHostsFile='" + knownHosts + "'",
		},
		{
			name:       "accept-new is an explicit opt-in",
			opts:       []Option{WithKnownHostsFile(knownHosts), WithAcceptNewHostKeys()},
			wantStrict: "-o StrictHostKeyChecking=accept-new",
			wantHosts:  "-o UserKnownHostsFile='" + knownHosts + "'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCredentialProvider(credential, tt.opts...)
			cleanup, err := provider.ConfigureAuth(context.Background(), t.TempDir())
			require.NoError(t, err)
			defer cleanup()

			sshCommand := os.Getenv("GIT_SSH_COMMAND")
			assert.Contains(t, sshCommand, tt.wantStrict)
			assert.Contains(t, sshCommand, "-o IdentitiesOnly=yes")
			assert.NotContains(t, sshCommand, "StrictHostKeyChecking=no")
			if tt.wantHosts != "" {
				assert.Contains(t, sshCommand, tt.wantHosts)
			}
			if tt.notContains != "" {
				assert.NotContains(t, sshCommand, tt.notContains)
			}

			// A known_hosts file inside a fresh temp dir would be empty on every
			// call and would make every host new. The command must not name one.
			keyPath := sshKeyPathFromCommand(sshCommand)
			require.NotEmpty(t, keyPath)
			assert.NotContains(t, sshCommand, filepath.Join(filepath.Dir(keyPath), "known_hosts"))
		})
	}
}

// TestShellQuote covers the quoting used for paths in GIT_SSH_COMMAND.
func TestShellQuote(t *testing.T) {
	assert.Equal(t, `'/tmp/a b'`, shellQuote("/tmp/a b"))
	assert.Equal(t, `'/tmp/it'\''s'`, shellQuote("/tmp/it's"))
	assert.Equal(t, `"/tmp/q\"b\\c"`, gitConfigQuote(`/tmp/q"b\c`))
}
