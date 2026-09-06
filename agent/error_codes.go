// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// Error codes for agent execution failures.
//
// These codes provide standardized error classification for observability,
// debugging, and automatic retry logic. Each code represents a specific
// failure mode that agents may encounter during execution.

// Agent lifecycle and execution errors.
const (
	// ErrCodeAgentTimeout indicates the agent exceeded its execution time limit.
	// This occurs when an agent fails to complete its task within the configured
	// timeout duration. The timeout may be set at the mission level, mission level,
	// or as a default in the agent's configuration.
	//
	// Common causes:
	// - Agent is stuck in an infinite loop
	// - Agent is waiting on a blocked resource
	// - Task complexity exceeds expected duration
	// - Network or I/O operations are taking too long
	//
	// This error is retryable with increased timeout or task decomposition.
	ErrCodeAgentTimeout = "AGENT_TIMEOUT"

	// ErrCodeAgentPanic indicates the agent process panicked and was recovered.
	// This represents an unhandled runtime error within the agent code, such as
	// nil pointer dereference, index out of bounds, or type assertion failure.
	//
	// Common causes:
	// - Nil pointer dereference in agent code
	// - Unhandled error conditions
	// - Invalid type assertions
	// - Stack overflow from deep recursion
	//
	// This error is generally not retryable without code fixes, but may be
	// retryable if caused by transient external conditions.
	ErrCodeAgentPanic = "AGENT_PANIC"

	// ErrCodeAgentInitFailed indicates the agent failed to initialize properly.
	// This occurs during the agent's startup phase before task execution begins.
	//
	// Common causes:
	// - Missing required configuration parameters
	// - Failed to load credentials or certificates
	// - Failed to connect to required services (database, cache, etc.)
	// - Invalid or corrupted agent state
	// - Failed to initialize required tools or plugins
	//
	// This error may be retryable if caused by transient infrastructure issues,
	// but often requires configuration changes or system fixes.
	ErrCodeAgentInitFailed = "AGENT_INIT_FAILED"
)

// LLM (Large Language Model) interaction errors.
const (
	// ErrCodeLLMRateLimited indicates the LLM provider is rate limiting requests.
	// This occurs when the agent exceeds the provider's rate limits for requests
	// per minute, tokens per minute, or concurrent requests.
	//
	// Common causes:
	// - Too many requests in a short time window
	// - Concurrent agents hitting the same API key quota
	// - Provider-side throttling during high load
	// - Insufficient rate limit allocation for account tier
	//
	// This error is highly retryable with exponential backoff. The agent should
	// respect any Retry-After headers or rate limit reset times provided by the API.
	ErrCodeLLMRateLimited = "LLM_RATE_LIMITED"

	// ErrCodeLLMContextExceeded indicates the request exceeded the model's context window.
	// This occurs when the combined prompt, conversation history, and tool schemas
	// exceed the maximum token limit supported by the LLM model.
	//
	// Common causes:
	// - Conversation history is too long
	// - Tool definitions are too verbose
	// - Input documents are too large
	// - Accumulated GraphRAG context is too extensive
	//
	// This error is not directly retryable without reducing context. The agent
	// should truncate history, summarize context, or switch to a larger model.
	ErrCodeLLMContextExceeded = "LLM_CONTEXT_EXCEEDED"

	// ErrCodeLLMAPIError indicates a generic API error from the LLM provider.
	// This covers HTTP errors, authentication failures, service unavailability,
	// and other provider-side issues not covered by more specific error codes.
	//
	// Common causes:
	// - Invalid API key or authentication token
	// - Provider service outage or maintenance
	// - Network connectivity issues to provider
	// - Malformed request payload
	// - Provider-side internal errors (500, 502, 503)
	//
	// This error may be retryable depending on the HTTP status code. 5xx errors
	// are typically retryable, while 4xx errors (except 429) usually are not.
	ErrCodeLLMAPIError = "LLM_API_ERROR"

	// ErrCodeLLMParseError indicates the agent failed to parse the LLM's response.
	// This occurs when the LLM returns malformed JSON, invalid tool calls, or
	// responses that don't match expected schemas.
	//
	// Common causes:
	// - LLM hallucinated invalid JSON structure
	// - LLM mixed structured output with natural language
	// - Response truncated mid-JSON due to token limits
	// - LLM used wrong schema format for tool calls
	// - Prompt engineering issues causing inconsistent output
	//
	// This error is sometimes retryable with better prompting, temperature
	// adjustments, or format instructions. Multiple retries may succeed if
	// the LLM generates different output.
	ErrCodeLLMParseError = "LLM_PARSE_ERROR"
)

// Tool execution errors.
const (
	// ErrCodeToolNotFound indicates the requested tool is not available.
	// This occurs when an agent attempts to invoke a tool that is not installed,
	// registered, or accessible in the current execution environment.
	//
	// Common causes:
	// - Tool not listed in mission dependencies
	// - Tool installation failed during agent startup
	// - Tool name mismatch between agent code and registry
	// - Tool binary not in PATH or not executable
	// - Tool removed or renamed after agent development
	//
	// This error is not retryable without installing the missing tool or
	// updating the agent to use available tools.
	ErrCodeToolNotFound = "TOOL_NOT_FOUND"

	// ErrCodeToolTimeout indicates the tool exceeded its execution time limit.
	// This occurs when a tool invocation does not complete within the configured
	// timeout period, which may be tool-specific or set by the agent.
	//
	// Common causes:
	// - Tool is blocked waiting for network I/O
	// - Tool is performing a long-running computation
	// - Target system is slow to respond
	// - Tool is stuck in an infinite loop
	// - Tool is waiting on locked resources
	//
	// This error is retryable with increased timeout, though repeated timeouts
	// may indicate a deeper issue requiring investigation.
	ErrCodeToolTimeout = "TOOL_TIMEOUT"

	// ErrCodeToolExecFailed indicates the tool execution failed with an error.
	// This covers any tool failure that is not timeout-related, including
	// process crashes, non-zero exit codes, and invalid tool responses.
	//
	// Common causes:
	// - Tool returned non-zero exit code
	// - Tool process crashed or was killed
	// - Tool received invalid input parameters
	// - Tool lacks required system permissions
	// - Tool dependencies are missing (libraries, binaries)
	// - Target system rejected tool operations
	//
	// This error may be retryable depending on the root cause. Transient
	// infrastructure issues may resolve on retry, but invalid inputs or
	// missing permissions require manual intervention.
	ErrCodeToolExecFailed = "TOOL_EXEC_FAILED"
)

// Network and connectivity errors.
const (
	// ErrCodeNetworkTimeout indicates a network operation timed out.
	// This occurs when an HTTP request, TCP connection, or other network
	// operation fails to complete within the configured timeout period.
	//
	// Common causes:
	// - Target server is slow to respond
	// - Network congestion or packet loss
	// - Firewall blocking or rate limiting traffic
	// - DNS resolution delays
	// - Target system under heavy load
	//
	// This error is highly retryable with exponential backoff. Transient
	// network issues often resolve within seconds to minutes.
	ErrCodeNetworkTimeout = "NETWORK_TIMEOUT"

	// ErrCodeNetworkUnreachable indicates the target host or network is unreachable.
	// This occurs when routing fails, DNS resolution fails, or the target
	// system is offline or behind a firewall that blocks access.
	//
	// Common causes:
	// - Target host is down or offline
	// - DNS name does not resolve
	// - Network route does not exist
	// - Firewall blocks connection attempts
	// - VPN or network segmentation issues
	// - IP address is invalid or not routable
	//
	// This error may be retryable if caused by transient routing issues,
	// but persistent unreachability requires manual investigation.
	ErrCodeNetworkUnreachable = "NETWORK_UNREACHABLE"

	// ErrCodeTLSError indicates a TLS/SSL handshake or certificate error.
	// This occurs when establishing secure connections to HTTPS endpoints,
	// databases, or other TLS-protected services.
	//
	// Common causes:
	// - Certificate expired or not yet valid
	// - Certificate signed by untrusted CA
	// - Hostname does not match certificate CN/SAN
	// - TLS version mismatch (server requires newer/older TLS)
	// - Cipher suite negotiation failure
	// - Client certificate required but not provided
	//
	// This error is generally not retryable without fixing the TLS
	// configuration, updating certificates, or adjusting trust settings.
	ErrCodeTLSError = "TLS_ERROR"
)

// Agent delegation and orchestration errors.
const (
	// ErrCodeDelegationFailed indicates the agent failed to delegate a subtask.
	// This occurs when an agent attempts to spawn a child agent or delegate
	// work to another agent, but the delegation mechanism fails before the
	// child agent begins execution.
	//
	// Common causes:
	// - Child agent not found in registry
	// - Failed to serialize task parameters
	// - Orchestrator rejected delegation request
	// - Maximum delegation depth exceeded
	// - Insufficient resources to spawn child agent
	//
	// This error may be retryable if caused by transient orchestrator issues,
	// but configuration problems require manual fixes.
	ErrCodeDelegationFailed = "DELEGATION_FAILED"

	// ErrCodeChildAgentFailed indicates a delegated child agent failed.
	// This occurs when a child agent successfully starts but encounters an
	// error during execution. The error code from the child agent should be
	// propagated in the error context for detailed diagnostics.
	//
	// Common causes:
	// - Child agent encountered any of the above error conditions
	// - Child agent's task was invalid or impossible
	// - Child agent timed out
	// - Child agent panicked or crashed
	//
	// Retryability depends on the child agent's error code. Check the wrapped
	// error or context metadata for the child's specific error.
	ErrCodeChildAgentFailed = "CHILD_AGENT_FAILED"
)

// Internal system errors.
const (
	// ErrCodeInternalError indicates an unexpected internal error.
	// This is a catch-all for errors that don't fit other categories,
	// typically representing bugs or unhandled edge cases in the agent
	// or SDK code.
	//
	// Common causes:
	// - Unhandled error conditions in agent code
	// - SDK bugs or unexpected state
	// - Data corruption or consistency violations
	// - Race conditions or concurrency issues
	// - Resource exhaustion (memory, file descriptors)
	//
	// This error is generally not retryable without investigating and
	// fixing the root cause, though transient resource issues may resolve.
	ErrCodeInternalError = "INTERNAL_ERROR"

	// ErrCodeConfigError indicates an agent configuration error.
	// This occurs when the agent's configuration is invalid, incomplete,
	// or incompatible with the current execution environment.
	//
	// Common causes:
	// - Missing required configuration parameters
	// - Invalid configuration value types
	// - Mutually exclusive options both set
	// - Configuration file parse errors
	// - Environment variables not set
	// - Secrets not available in vault
	//
	// This error is not retryable without fixing the configuration.
	// Agents should validate configuration during initialization and
	// fail fast with clear error messages.
	ErrCodeConfigError = "CONFIG_ERROR"
)

// IsRetryable determines whether an error code represents a transient failure
// that may succeed on retry. This function is used by the orchestrator and
// harness to implement automatic retry logic with exponential backoff.
//
// Retryable errors are typically caused by temporary conditions like:
// - Rate limiting (will reset after time window)
// - Network timeouts (may resolve on next attempt)
// - Service unavailability (may recover quickly)
// - Resource contention (may clear after backoff)
//
// Non-retryable errors typically require manual intervention:
// - Configuration errors (need config changes)
// - Authentication failures (need valid credentials)
// - Tool not found (need tool installation)
// - TLS errors (need certificate/config fixes)
// - Context exceeded (need prompt reduction)
//
// When implementing retry logic, consider:
// - Use exponential backoff to avoid overwhelming systems
// - Respect Retry-After headers for rate limit errors
// - Set maximum retry counts to avoid infinite loops
// - Log retry attempts for observability
// - Add jitter to backoff to prevent thundering herd
//
// Example usage:
//
//	if IsRetryable(err.Code()) {
//	    backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
//	    time.Sleep(backoff)
//	    return retry()
//	}
//	return err // Don't retry, fail fast
func IsRetryable(code string) bool {
	switch code {
	// Highly retryable - transient infrastructure issues
	case ErrCodeLLMRateLimited:
		return true // Will reset after time window
	case ErrCodeNetworkTimeout:
		return true // May succeed on next attempt
	case ErrCodeNetworkUnreachable:
		return true // May be transient routing issue
	case ErrCodeLLMAPIError:
		return true // May be temporary provider outage (check HTTP status)

	// Potentially retryable - may resolve with different output
	case ErrCodeAgentTimeout:
		return true // May succeed with more time or decomposition
	case ErrCodeToolTimeout:
		return true // Target system may respond faster next time
	case ErrCodeLLMParseError:
		return true // LLM may generate valid output on retry
	case ErrCodeToolExecFailed:
		return true // May be transient system issue

	// Conditionally retryable - depends on context
	case ErrCodeAgentPanic:
		return false // Usually requires code fix, but could be transient
	case ErrCodeChildAgentFailed:
		return false // Depends on child's error code (check wrapped error)

	// Not retryable - requires manual intervention
	case ErrCodeAgentInitFailed:
		return false // Needs config or system fixes
	case ErrCodeLLMContextExceeded:
		return false // Needs context reduction
	case ErrCodeToolNotFound:
		return false // Needs tool installation
	case ErrCodeTLSError:
		return false // Needs certificate/config fixes
	case ErrCodeDelegationFailed:
		return false // Usually config issue
	case ErrCodeInternalError:
		return false // Needs investigation and code fix
	case ErrCodeConfigError:
		return false // Needs configuration changes

	default:
		// Unknown error codes are conservatively treated as non-retryable
		// to avoid infinite retry loops on unexpected errors
		return false
	}
}
