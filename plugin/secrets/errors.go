// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package secrets — plugin-side credential resolver.
//
// The sentinel errors below are the typed errors returned by Client.Resolve
// (or wrapping any callback error). Callers test with errors.Is.
//
// Historically these sentinels lived in github.com/zeroroot-ai/sdk/secrets
// alongside the daemon-side broker interface. The broker has moved to the
// private platform-clients module (see ADR-0025 / ADR-0030); this plugin
// client retains a minimal local copy of the sentinel names so customer
// plugins can compile without importing daemon-internal types.
//
// gRPC code mapping (when these sentinels round-trip across the
// HarnessCallbackService.GetCredential RPC, the daemon translates):
//
//	ErrNotFound         → codes.NotFound
//	ErrPermissionDenied → codes.PermissionDenied
//	ErrInvalidArgument  → codes.InvalidArgument
package secrets

import "errors"

// ErrNotFound is returned when the daemon's secrets backend has no secret
// with the requested name for this tenant. Maps to gRPC codes.NotFound on
// the daemon side; surfaced through GetCredentialFn errors here.
var ErrNotFound = errors.New("secrets: not found")

// ErrPermissionDenied is returned when the plugin's access to a secret has
// been revoked (via Client.MarkRevoked, typically driven by a
// secret_access_revoked event). It is also returned by Resolve immediately
// for any name the plugin has marked revoked, without invoking the
// callback RPC.
var ErrPermissionDenied = errors.New("secrets: permission denied")

// ErrInvalidArgument is returned by Resolve when the requested name is not
// declared in the plugin's manifest spec.secrets. The plugin author must
// declare every secret it consumes; this sentinel signals a manifest
// authoring error rather than a runtime data error.
var ErrInvalidArgument = errors.New("secrets: invalid argument")
