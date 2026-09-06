# LSP Client Implementation

This package provides a Language Server Protocol (LSP) client implementation for validating code changes in the Gibson SDK. It supports multiple language servers through a unified interface.

## Architecture

The implementation consists of three main components:

### 1. `client.go` - Base LSP Protocol Client

Implements JSON-RPC 2.0 communication over stdio with LSP servers:

- **Message Framing**: Handles `Content-Length` header-based message framing
- **Request/Response Correlation**: Tracks pending requests using unique IDs
- **Notification Handling**: Dispatches server notifications to registered handlers
- **Process Management**: Manages language server process lifecycle
- **Concurrent Safe**: Thread-safe request handling and notification dispatching

Key types:
- `lspClient`: Base client that communicates with any LSP server via stdio
- `jsonrpcRequest`, `jsonrpcResponse`, `jsonrpcNotification`: JSON-RPC 2.0 message types

### 2. `gopls.go` - gopls-Specific Implementation

Wraps the base client with gopls-specific functionality:

- **LSP Initialization**: Performs the `initialize` → `initialized` handshake
- **Document Management**: Handles `textDocument/didOpen` and `textDocument/didClose`
- **Diagnostics**: Processes `textDocument/publishDiagnostics` notifications
- **Ready Notification**: Signals when the server is initialized and ready

Key features:
- Converts LSP diagnostic format to SDK `Diagnostic` type
- Handles file URI conversion (`file://` prefix)
- Manages diagnostic storage per file
- Provides timeout-based diagnostic retrieval

### 3. `manager.go` - Multi-Language Server Manager

Orchestrates multiple language servers for a workspace:

- **Server Lifecycle**: Starts/stops language servers based on configuration
- **Language Detection**: Routes validation requests to appropriate server by file extension
- **Workspace Management**: Manages a single workspace root for all servers
- **Concurrent Access**: Thread-safe server access and diagnostic retrieval

## Usage

### Basic Example

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "github.com/zeroroot-ai/sdk/codegen/lsp"
)

func main() {
    // Create LSP manager with default config
    config := lsp.DefaultLSPConfig()
    config.ValidationTimeout = 15 * time.Second

    manager := lsp.NewLSPManager(config, slog.Default())

    ctx := context.Background()

    // Start language servers for workspace
    if err := manager.Start(ctx, "/path/to/workspace"); err != nil {
        panic(err)
    }
    defer manager.Stop(ctx)

    // Wait for servers to be ready
    if err := manager.WaitForReady(ctx); err != nil {
        panic(err)
    }

    // Get diagnostics for a file
    diagnostics, err := manager.GetDiagnostics(ctx, "main.go")
    if err != nil {
        panic(err)
    }

    // Check for errors
    for _, d := range diagnostics {
        if d.IsError() {
            log.Printf("Error at line %d: %s", d.Line, d.Message)
        }
    }
}
```

### Configuration

```go
config := lsp.LSPConfig{
    // Language server binary paths (empty = search in PATH)
    GoplsPath:            "/usr/local/bin/gopls",
    PyrightPath:          "",
    TypeScriptServerPath: "",

    // Timeouts
    InitTimeout:       30 * time.Second,
    ValidationTimeout: 10 * time.Second,

    // Enable/disable specific languages
    EnableGo:         true,
    EnablePython:     true,
    EnableTypeScript: true,
}

manager := lsp.NewLSPManager(config, slog.Default())
```

## Requirements

### Language Server Binaries

The manager requires language server binaries to be installed:

- **Go**: `gopls` - Install with: `go install golang.org/x/tools/gopls@latest`
- **Python**: `pyright-langserver` (TODO: not yet implemented)
- **TypeScript**: `typescript-language-server` (TODO: not yet implemented)

If binary paths are not specified in the config, they will be searched in `$PATH`.

## LSP Protocol Details

### Initialization Handshake

1. Client sends `initialize` request with workspace root and capabilities
2. Server responds with server capabilities
3. Client sends `initialized` notification
4. Server is now ready to accept requests

### Diagnostic Flow

1. Client sends `textDocument/didOpen` notification with file content
2. Server analyzes the file
3. Server sends `textDocument/publishDiagnostics` notification with issues
4. Client stores diagnostics and makes them available via `GetDiagnostics()`
5. Client sends `textDocument/didClose` notification

### Graceful Shutdown

1. Client sends `shutdown` request
2. Server responds to acknowledge
3. Client sends `exit` notification
4. Server terminates

## Error Handling

The implementation defines several error types:

- `ErrLSPTimeout`: Operation exceeded deadline (usually validation timeout)
- `ErrLSPNotInitialized`: Operation attempted before initialization complete
- `ErrLSPShutdown`: Operation attempted after shutdown

## Testing

Run unit tests:
```bash
go test ./codegen/lsp/
```

Run integration tests (requires gopls):
```bash
go test -v ./codegen/lsp/
```

Run specific test:
```bash
go test -v -run TestGetDiagnosticsValidCode ./codegen/lsp/
```

## Implementation Notes

### Thread Safety

- All public methods are thread-safe
- Uses `sync.Mutex` for protecting shared state (pending requests, diagnostics)
- Uses `atomic` operations for simple flags (initialized, shutdown)

### Message Framing

LSP uses HTTP-style headers for message framing:
```
Content-Length: 123\r\n
\r\n
{JSON-RPC message}
```

The client reads headers line-by-line, extracts content length, then reads exactly that many bytes for the JSON payload.

### Request ID Allocation

Request IDs are allocated using `atomic.Int64` for thread-safe incrementing. Each request gets a unique ID for correlation.

### Diagnostic Timing

Diagnostics are retrieved with a polling mechanism:
1. Clear existing diagnostics for the file
2. Open the document (triggers analysis)
3. Poll every 50ms for diagnostics
4. Return when diagnostics arrive or timeout occurs
5. Empty diagnostics on timeout (not an error - might be no issues)

### Context Handling

All public methods accept `context.Context` for cancellation:
- Initialization uses `InitTimeout` context
- Validation uses `ValidationTimeout` context
- Shutdown uses caller-provided context

## Future Enhancements

- [ ] Implement pyright support for Python
- [ ] Implement typescript-language-server support
- [ ] Add support for `textDocument/didChange` for incremental updates
- [ ] Cache opened documents to avoid repeated open/close
- [ ] Support workspace-wide diagnostics
- [ ] Add metrics and tracing with OpenTelemetry
- [ ] Support LSP code actions and quickfixes
- [ ] Implement hover information and completions
