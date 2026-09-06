// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package graphrag provides the GraphRAG knowledge graph system.
//
// This file contains the go:generate directive for taxonomy code generation.
// Run `go generate ./...` from the SDK root to regenerate taxonomy code.
package graphrag

//go:generate go run github.com/zeroroot-ai/sdk/cmd/taxonomy-gen --base ../taxonomy/core.yaml --output-proto ../api/proto/taxonomy.proto --output-domain domain/domain_generated.go --output-validators validation/validators_generated.go --output-constants constants_generated.go --output-query query/query_generated.go --output-helpers helpers_generated.go --package graphrag
