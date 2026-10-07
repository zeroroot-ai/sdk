// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

// Reasoner performs ontology-aware graph reasoning over a loaded
// OntologyExtension. The SDK declares this interface so the daemon and any
// SDK-level tooling can share a contract without the implementation living
// here.
//
// The implementation lives in the gibson daemon, in its own
// internal/graphrag package. The SDK only publishes the contract.
//
// All IRI arguments use prefix:localname form (e.g. "soc2:CC6.1",
// "mitre:T1190.001"). Methods return nil/empty slices rather than errors for
// unknown IRIs — callers should treat an empty result as "not found in this
// ontology", not as a hard failure.
type Reasoner interface {
}
