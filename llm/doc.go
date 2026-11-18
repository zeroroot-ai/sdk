// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package llm holds the token counts of the model calls of an agent.
//
// An agent calls a model with the client of the openresponses package. The
// harness returns that client from Harness.Model. A slot is a name. The daemon
// resolves it to one exact model before the call.
//
// The harness adds the token counts of each response to a TokenTracker:
//
//	usage := h.TokenUsage()
//	total := usage.Total()
//	fmt.Printf("Total tokens used: %d\n", total.TotalTokens)
package llm
