// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
