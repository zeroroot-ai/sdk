// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"net/http"

	"github.com/zeroroot-ai/sdk/plugin/secrets"
)

// Test seams. Production configures these fields from the environment and
// the defaults; tests set them directly through package-internal options.

func testSecretsClient(cl secrets.Client) Option {
	return func(c *config) { c.secretsClient = cl }
}

func testHealthAddr(addr string) Option {
	return func(c *config) { c.healthAddr = addr }
}

func testHTTPClient(hc *http.Client) Option {
	return func(c *config) { c.httpClient = hc }
}
