// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// k8sSATokenPath is the standard path for the Kubernetes service account token.
const k8sSATokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// BootstrapCredential carries the one-time credential used for initial host
// registration. After registration succeeds, the host key takes over and
// the bootstrap credential is discarded — it is never stored.
type BootstrapCredential struct {
	// Type describes the kind of credential.
	// Possible values: "api_key", "k8s_sa".
	Type string

	// Token is the raw credential value (Bearer token, SA JWT, etc.).
	Token string
}

// ResolveBootstrap determines the bootstrap credential to use for initial host
// registration. Resolution priority:
//
//  1. explicitToken — if non-empty, used as-is with type "api_key".
//  2. Kubernetes service account token from the well-known path
//     /var/run/secrets/kubernetes.io/serviceaccount/token.
//  3. Error — no bootstrap credential found.
//
// Bootstrap credentials are consumed once. Callers must not store or log the
// Token value.
func ResolveBootstrap(explicitToken string) (*BootstrapCredential, error) {
	if explicitToken = strings.TrimSpace(explicitToken); explicitToken != "" {
		return &BootstrapCredential{
			Type:  "api_key",
			Token: explicitToken,
		}, nil
	}

	// Documented fallback: the GIBSON_BOOTSTRAP_TOKEN environment variable.
	// This is the contract the process-mode bridge runner relies on (and that
	// plugin.WithBootstrapToken's doc advertises); honor it before the
	// in-cluster Kubernetes service-account token.
	if envToken := strings.TrimSpace(os.Getenv("GIBSON_BOOTSTRAP_TOKEN")); envToken != "" {
		return &BootstrapCredential{
			Type:  "api_key",
			Token: envToken,
		}, nil
	}

	data, err := os.ReadFile(k8sSATokenPath)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return &BootstrapCredential{
				Type:  "k8s_sa",
				Token: token,
			}, nil
		}
	}

	return nil, errors.New("capabilitygrant: no bootstrap credential found: " +
		"provide an explicit token or run inside a Kubernetes pod with a service account")
}

// ResolveBootstrapFromSecret reads a bootstrap token from a Kubernetes
// Secret mounted as a volume. In Kubernetes, Secrets mounted via
// `secretKeyRef` or a `volumes.secret` entry appear as files under the
// mount directory, one file per data key.
//
// This is the preferred on-prem K8s pattern for customers running Gibson
// agents via the `gibson-tenant-operator`'s AgentEnrollment CRD:
//
//	Pod spec:
//	  volumes:
//	  - name: bootstrap
//	    secret:
//	      secretName: scanner-01-bootstrap
//	  containers:
//	  - name: agent
//	    volumeMounts:
//	    - name: bootstrap
//	      mountPath: /etc/gibson/bootstrap
//	      readOnly: true
//
//	Go code:
//	  cred := capabilitygrant.MustResolveBootstrapFromSecret("/etc/gibson/bootstrap", "token")
//
// No Kubernetes API access or RBAC is required — just volume mount
// permissions, which are namespace-scoped and far less privileged.
func ResolveBootstrapFromSecret(mountPath, dataKey string) (*BootstrapCredential, error) {
	path := filepath.Join(mountPath, dataKey)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: read bootstrap secret %q: %w", path, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return nil, fmt.Errorf("capabilitygrant: bootstrap secret %q is empty", path)
	}
	return &BootstrapCredential{
		Type:  "api_key",
		Token: token,
	}, nil
}

// MustResolveBootstrapFromSecret is like ResolveBootstrapFromSecret but
// panics on failure. Convenience wrapper for binaries that cannot sensibly
// continue without a bootstrap credential.
func MustResolveBootstrapFromSecret(mountPath, dataKey string) *BootstrapCredential {
	cred, err := ResolveBootstrapFromSecret(mountPath, dataKey)
	if err != nil {
		panic(err)
	}
	return cred
}
