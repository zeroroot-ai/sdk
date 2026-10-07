// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"errors"
	"os"
	"strings"
)

// BootstrapCredential carries the one-time credential used for initial host
// registration. After registration succeeds, the host key takes over and
// the bootstrap credential is discarded — it is never stored.
type BootstrapCredential struct {

	// Token is the raw credential value (Bearer token, one-time registration token).
	Token string
}

// ResolveBootstrap determines the bootstrap credential to use for initial host
// registration. Resolution priority:
//
//  1. explicitToken — if non-empty, used as-is with type "api_key".
//  2. The GIBSON_BOOTSTRAP_TOKEN environment variable, type "api_key". This is
//     the path a developer running their own component takes (sdk#128: two
//     environment variables and a binary, no CLI and no file), and it is what
//     the process-mode bridge runner relies on.
//  3. Error — no bootstrap credential found.
//
// The function never reads the Kubernetes service account token. That token
// is valid against the cluster API, so the client must not send it to a
// platform endpoint.
//
// Bootstrap credentials are consumed once. Callers must not store or log the
// Token value.
func ResolveBootstrap(explicitToken string) (*BootstrapCredential, error) {
	if explicitToken = strings.TrimSpace(explicitToken); explicitToken != "" {
		return &BootstrapCredential{
			Token: explicitToken,
		}, nil
	}

	// Documented fallback: the GIBSON_BOOTSTRAP_TOKEN environment variable.
	// This is the contract the process-mode bridge runner relies on (and that
	// plugin.WithBootstrapToken's doc advertises); honor it before the
	// in-cluster Kubernetes service-account token.
	if envToken := strings.TrimSpace(os.Getenv("GIBSON_BOOTSTRAP_TOKEN")); envToken != "" {
		return &BootstrapCredential{
			Token: envToken,
		}, nil
	}

	// Name every source that was tried, and name the variable. The previous
	// message said "provide an explicit token or run inside a Kubernetes pod",
	// which never mentioned GIBSON_BOOTSTRAP_TOKEN — the one thing a developer
	// following the documented path actually sets. An error that does not name
	// the knob is a statement that something is wrong rather than an
	// instruction (sdk#128 acceptance criterion 3).
	return nil, errors.New("capabilitygrant: no bootstrap credential found. Tried, in order: " +
		"an explicit token (none passed) and the GIBSON_BOOTSTRAP_TOKEN environment variable (unset or empty). " +
		"Set GIBSON_BOOTSTRAP_TOKEN to a one-time registration token from the dashboard. " +
		"A bootstrap token is consumed once, so a " +
		"restart after a successful enrolment needs no token at all")
}
