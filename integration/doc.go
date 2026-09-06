// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package integration provides comprehensive integration tests for the Gibson SDK.
//
// This package contains end-to-end tests that verify the SDK components work together
// correctly. Unlike unit tests that focus on individual components, these integration
// tests validate complete missions and interactions between packages.
//
// # Test Coverage
//
// The integration tests cover the following areas:
//
//  1. Agent Integration (agent_test.go)
//     - Agent creation using SDK entry points
//     - Agent lifecycle (Initialize → Execute → Shutdown)
//     - Agent execution with mock harness
//     - All agent capabilities (prompt injection, jailbreak, etc.)
//     - Health status reporting
//     - LLM slot configuration
//     - Multiple capabilities and target types
//
//  2. Tool Integration (tool_test.go)
//     - Tool creation using SDK entry points
//     - Tool execution with valid input
//     - Tool execution with invalid input (schema validation)
//     - Schema validation for various types (string, integer, boolean, array, object)
//     - Tool health endpoint
//     - Real-world tool scenarios (HTTP client, file reader, JSON processor)
//     - Error handling and context cancellation
//
//  3. Plugin Integration (plugin_test.go)
//     - Plugin creation using SDK entry points
//     - Plugin initialization and configuration
//     - Plugin method invocation
//     - Plugins with multiple methods
//     - Plugin shutdown and lifecycle management
//     - Plugin health status
//     - Real-world plugin scenarios (database, LLM provider)
//     - Input/output schema validation
//
//  4. Framework Integration (integration_test.go)
//     - Framework creation and lifecycle
//     - Registry operations (register, get, list, unregister)
//     - Mission creation and management
//     - Finding export in multiple formats (JSON, CSV, HTML, SARIF)
//     - End-to-end missions combining agents, tools, and plugins
//     - Concurrent registry access and thread safety
//     - Package import verification
//
// # Running the Tests
//
// To run all integration tests:
//
//	cd /path/to/sdk
//	go test ./integration/...
//
// To run with verbose output:
//
//	go test -v ./integration/...
//
// To run a specific test file:
//
//	go test -v ./integration/agent_test.go
//
// To run a specific test:
//
//	go test -v -run TestAgentCreation ./integration/
//
// # Test Organization
//
// Each test file focuses on a specific component:
//
//   - agent_test.go: Tests agent creation, execution, lifecycle, and capabilities
//   - tool_test.go: Tests tool creation, execution, and schema validation
//   - plugin_test.go: Tests plugin creation, methods, and lifecycle
//   - integration_test.go: Tests framework operations and cross-component missions
//
// # Best Practices
//
// When adding new integration tests:
//
//  1. Test real functionality, not just compilation
//  2. Include both positive and negative test cases
//  3. Verify error handling and edge cases
//  4. Use descriptive test names that explain what's being tested
//  5. Clean up resources (use defer for cleanup)
//  6. Test thread safety where applicable
//  7. Use subtests (t.Run) for logical grouping
//  8. Include realistic scenarios that mirror actual usage
//
// # Mock Components
//
// The integration tests include mock implementations for testing:
//
//   - mockHarness: A minimal implementation of agent.Harness for testing agents
//
// These mocks allow testing components in isolation while verifying they conform
// to the correct interfaces.
//
// # Dependencies
//
// These tests use the testify package for assertions:
//
//	github.com/stretchr/testify
//
// All other dependencies are from the Gibson SDK itself.
package integration
