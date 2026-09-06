// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package sysconst is the home of the reserved SystemTenant constant. It lives
// in an internal subpackage of sdk/auth so that callers cannot construct a
// platform-operator-scoped TenantID by accident — only code that explicitly
// imports `github.com/zeroroot-ai/sdk/auth/internal/sysconst` can read the
// constant, and the SDK only re-exports it through `auth.SystemTenant` for
// platform-operator code paths that have a documented need.
//
// This indirection is the reason TenantID is a sealed struct: there is no
// path to a non-empty TenantID except through `auth.NewTenantID(s)` (which
// validates s) or through `auth.SystemTenant` (which is the single literal
// reserved string `_system`). The data-plane spec relies on this property —
// `pool.For(tenant)` cannot be called with the system tenant by random
// handler code, only by code under `gibson/internal/admin/`.
//
// Spec: unified-identity-and-authorization Requirement 6.4.
package sysconst

// Reserved is the literal string identifier for the platform-operator tenant.
// It is intentionally unexported as a string and only exposed through the
// auth package's SystemTenant value.
const Reserved = "_system"
