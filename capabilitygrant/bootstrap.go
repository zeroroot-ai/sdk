// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"fmt"
	"os"
	"strings"
)

// k8sSATokenPath is the standard path for the Kubernetes service account token.
const k8sSATokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// BootstrapCredential carries the one-time credential used for initial host
// registration. After registration succeeds, the host key takes over and
// the bootstrap credential is discarded — it is never stored.
type BootstrapCredential struct {

	// Token is the raw credential value (Bearer token, SA JWT, etc.).
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
//  3. Kubernetes service account token from the well-known path
//     /var/run/secrets/kubernetes.io/serviceaccount/token, type "k8s_sa".
//  4. Error — no bootstrap credential found.
//
// The numbered list used to omit step 2 while the code did it, so the doc and
// the code disagreed about the only path most readers use.
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

	data, err := os.ReadFile(k8sSATokenPath)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return &BootstrapCredential{
				Token: token,
			}, nil
		}
	}

	// Name every source that was tried, and name the variable. The previous
	// message said "provide an explicit token or run inside a Kubernetes pod",
	// which never mentioned GIBSON_BOOTSTRAP_TOKEN — the one thing a developer
	// following the documented path actually sets. An error that does not name
	// the knob is a statement that something is wrong rather than an
	// instruction (sdk#128 acceptance criterion 3).
	return nil, fmt.Errorf("capabilitygrant: no bootstrap credential found. Tried, in order: "+
		"an explicit token (none passed), the GIBSON_BOOTSTRAP_TOKEN environment variable (unset or empty), "+
		"and the in-cluster service account token at %s (absent or empty). "+
		"Set GIBSON_BOOTSTRAP_TOKEN to a one-time registration token from the dashboard, "+
		"or run in a pod with a service account. A bootstrap token is consumed once, so a "+
		"restart after a successful enrolment needs no token at all", k8sSATokenPath)
}
