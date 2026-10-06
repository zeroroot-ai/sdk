// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package agent — supplemental capability grant tests to cover remaining
// branches missed by the main test suite.
package agent

// -----------------------------------------------------------------------
// mintGrantCustom — extends grantOpts with fields for empty-claim injection
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Tests for empty claim branches in ValidateCapabilityGrant
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// jwkToECDSA — bad Y coordinate base64
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// refreshJWKS — non-EC / non-P256 key is silently skipped
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// refreshJWKS — non-JSON body
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// lookupKeyLocked — staleExpired branch when keys exist
// This covers the path where jwksFetched is non-zero but staleAt has passed.
// The stale-expired case is already covered by TestCapabilityGrantJWKSStaleWindowExceeded
// but the "no key empty set" path isn't. Verify with empty-key server.
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// lookupKeyLocked — empty kid lookup when cache has keys (returns any key)
// This covers the "kid=="" and loop-over-map" path in lookupKeyLocked.
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// lookupKeyLocked: verify the "wrong algorithm" rejection path
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Issuer match when WithIssuer matches correctly
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Background stale refresh (goroutine path in resolveKey)
// Serve stale while background refresh fires.
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// generateTestKeys with a different key produces signature failure
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Inline JWKS HTTP status != 200
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Prometheus counter is initialized (smoke test for init())
// -----------------------------------------------------------------------

type countingCounter struct{ inc func() }

func (c *countingCounter) Inc() { c.inc() }

// -----------------------------------------------------------------------
// Validate iat missing path
// -----------------------------------------------------------------------
