// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// StreamingTool is an optional interface for tools that support streaming execution.
// Tools implementing this interface will have StreamExecuteProto called when invoked
// via the streaming RPC. Streaming tools can emit real-time progress updates, partial
// results, and warnings during long-running operations.
//
// Example implementation:
//
//	type MyStreamingTool struct {
//	    *BaseTool
//	}
//
//	func (t *MyStreamingTool) StreamExecuteProto(ctx context.Context, input proto.Message, stream ToolStream) error {
//	    req := input.(*pb.MyRequest)
//
//	    // Emit progress updates
//	    stream.Progress(25, "discovery", "Discovering targets...")
//
//	    // Check for cancellation
//	    select {
//	    case <-stream.Cancelled():
//	        return stream.Error(context.Canceled, true)
//	    default:
//	    }
//
//	    // Emit partial results
//	    partial := &pb.MyResponse{Items: firstBatch}
//	    stream.Partial(partial, true)
//
//	    // Emit warnings for non-fatal issues
//	    stream.Warning("Connection timeout", "host_192.168.1.1")
//
//	    // Complete with final result
//	    final := &pb.MyResponse{Items: allResults}
//	    return stream.Complete(final)
//	}
type StreamingTool interface {
	Tool

	// StreamExecuteProto runs the tool with streaming event emission.
	// The tool receives input and a ToolStream for emitting progress, partial results,
	// warnings, and the final completion event.
	//
	// The input parameter must be a pointer to the proto message type specified by InputMessageType.
	// The final result must be emitted via stream.Complete() with the proto message type
	// specified by OutputMessageType.
	//
	// Returns error only for fatal failures that prevent any output. Non-fatal errors
	// should be emitted via stream.Error() or stream.Warning() instead.
	//
	// Tools should periodically check stream.Cancelled() to handle graceful cancellation:
	//
	//	for _, item := range items {
	//	    select {
	//	    case <-stream.Cancelled():
	//	        return stream.Error(context.Canceled, true)
	//	    default:
	//	    }
	//	    // Process item...
	//	}
	//
	// Context is used for timeout enforcement and request-scoped values. When the context
	// is cancelled (timeout or parent cancellation), tools should terminate gracefully.
	StreamExecuteProto(ctx context.Context, input proto.Message, stream ToolStream) error
}

// ToolStream provides methods for tools to emit streaming events during execution.
// Implementations hide gRPC complexity from tool developers, providing a simple
// interface for progress reporting, partial results, and error handling.
//
// All methods are safe for concurrent use from multiple goroutines, allowing tools
// to emit events from parallel operations.
//
// Event ordering is guaranteed through sequence numbers - events are delivered to
// clients in the order they are emitted, even with concurrent calls.
type ToolStream interface {
	// Progress emits a progress update indicating execution status.
	//
	// percent: Progress percentage from 0-100. Use 0 if progress is unknown/indeterminate.
	// phase: Current execution phase (e.g., "discovery", "scanning", "parsing").
	//        Should be a short, stable identifier rather than a full sentence.
	// message: Human-readable status message describing the current operation.
	//
	// Progress updates are advisory and do not affect tool execution. Clients may
	// use them for UI updates or logging but should not rely on them for correctness.
	//
	// Example:
	//	stream.Progress(25, "discovery", "Enumerating subdomains via DNS")
	//	stream.Progress(50, "scanning", "Port scanning 192.168.1.0/24")
	//	stream.Progress(75, "fingerprinting", "Detecting service versions")
	Progress(percent int, phase, message string) error

	// Partial emits a partial result before the tool completes.
	//
	// output: A proto message of the same type as specified by OutputMessageType.
	//         Must be a valid, complete proto message (not nil).
	// incremental: If true, this result adds to previous partial results (append mode).
	//              If false, this result replaces all previous partial results (replace mode).
	//
	// Partial results allow clients to process data as it becomes available rather than
	// waiting for complete execution. This is useful for long-running operations that
	// discover targets or findings incrementally.
	//
	// Incremental mode example (building a list):
	//	stream.Partial(&pb.Response{Hosts: []Host{host1}}, true)  // First result
	//	stream.Partial(&pb.Response{Hosts: []Host{host2}}, true)  // Adds to first
	//	// Client should have [host1, host2]
	//
	// Replace mode example (progressive refinement):
	//	stream.Partial(&pb.Response{Score: 60}, false)   // Initial estimate
	//	stream.Partial(&pb.Response{Score: 75}, false)   // Better estimate
	//	// Client should have most recent estimate only
	Partial(output proto.Message, incremental bool) error

	// Warning emits a non-fatal warning that does not stop execution.
	//
	// message: Human-readable warning message describing the issue.
	// context: Additional context identifying where the warning occurred
	//          (e.g., "host_192.168.1.1", "port_scan", "subdomain_enum").
	//          May be empty if not applicable.
	//
	// Warnings are for expected failures and recoverable errors that do not prevent
	// the tool from continuing. Examples include individual host timeouts, DNS lookup
	// failures, or permission errors on specific operations.
	//
	// Example:
	//	stream.Warning("Connection timeout after 5s", "host_192.168.1.1")
	//	stream.Warning("Permission denied on port 1-1024", "privilege_scan")
	Warning(message, context string) error

	// Complete emits the final result and signals successful stream completion.
	// After calling Complete, the stream is closed and no further events can be emitted.
	//
	// output: The final proto message of the type specified by OutputMessageType.
	//         Must be a valid, complete proto message (not nil).
	//
	// This is the only way to successfully complete a streaming tool execution.
	// Clients will receive this as the final return value from the tool call.
	//
	// After Complete returns, the tool should return from StreamExecuteProto with
	// a nil error. Any error returned after Complete will be ignored.
	//
	// Example:
	//	final := &pb.Response{
	//	    Hosts: allHosts,
	//	    Summary: &pb.Summary{TotalHosts: len(allHosts)},
	//	}
	//	return stream.Complete(final)
	Complete(output proto.Message) error

	// Error emits an error event during execution.
	//
	// err: The error that occurred. Must not be nil.
	// fatal: If true, the stream closes after this error and execution stops.
	//        If false, execution continues and this is treated as a logged error.
	//
	// Fatal errors are for unrecoverable failures that prevent the tool from producing
	// any useful output (e.g., invalid input, missing dependencies, system failures).
	//
	// Non-fatal errors are for logged issues that don't stop execution. Consider using
	// Warning() instead for non-fatal cases, as it provides better context.
	//
	// Example (fatal):
	//	if err := validateInput(req); err != nil {
	//	    return stream.Error(fmt.Errorf("invalid input: %w", err), true)
	//	}
	//
	// Example (non-fatal):
	//	if err := connectToAPI(host); err != nil {
	//	    stream.Error(fmt.Errorf("API connection failed: %w", err), false)
	//	    // Continue with local operation...
	//	}
	Error(err error, fatal bool) error

	// Cancelled returns a channel that closes when cancellation is requested.
	// Tools should check this channel periodically during long-running operations
	// and terminate gracefully when cancellation is signaled.
	//
	// Cancellation can be triggered by:
	//   - Client sending a ToolCancelRequest via the stream
	//   - Context timeout expiration
	//   - Parent context cancellation (e.g., mission abort)
	//
	// After cancellation is signaled, tools should:
	//   1. Stop all ongoing operations (scans, network requests, etc.)
	//   2. Emit any partial results if available
	//   3. Return from StreamExecuteProto with context.Canceled error
	//
	// Example:
	//	for _, target := range targets {
	//	    select {
	//	    case <-stream.Cancelled():
	//	        // Graceful shutdown
	//	        stream.Warning("Scan cancelled by user", "cancellation")
	//	        if len(results) > 0 {
	//	            return stream.Complete(&pb.Response{Hosts: results})
	//	        }
	//	        return stream.Error(context.Canceled, true)
	//	    default:
	//	        // Continue processing
	//	        result := scanTarget(ctx, target)
	//	        results = append(results, result)
	//	    }
	//	}
	Cancelled() <-chan struct{}

	// ExecutionID returns the unique execution ID for this tool invocation.
	// This ID is assigned by the caller and can be used for:
	//   - Correlating log messages across distributed systems
	//   - Tracking tool executions in observability platforms
	//   - Debugging and troubleshooting specific invocations
	//
	// The execution ID remains constant for the entire lifetime of the stream.
	//
	// Example:
	//	logger.InfoContext(ctx, "Starting scan",
	//	    "execution_id", stream.ExecutionID(),
	//	    "target", target,
	//	)
	ExecutionID() string
}
