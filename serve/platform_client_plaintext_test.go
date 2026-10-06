// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildDialOptions_PlaintextOnlyToALoopbackHost proves that a capability
// grant never travels in cleartext to a remote host: http and a bare target
// dial plaintext only for localhost, 127.0.0.0/8 or ::1.
func TestBuildDialOptions_PlaintextOnlyToALoopbackHost(t *testing.T) {
	for _, url := range []string{
		"https://platform.example:443",
		"http://localhost:50051",
		"http://127.0.0.1:50051",
		"http://127.8.9.10:50051",
		"http://[::1]:50051",
		"localhost:50051",
	} {
		_, err := (&PlatformClient{platformURL: url}).buildDialOptions()
		require.NoError(t, err, url)
	}
	for _, url := range []string{
		"http://platform.example:50051",
		"http://10.0.0.5:50051",
		"http://localhost.example:50051",
		"platform.example:50051",
		"http://[2001:db8::1]:50051",
	} {
		_, err := (&PlatformClient{platformURL: url}).buildDialOptions()
		require.ErrorContains(t, err, "loopback", url)
	}
}
