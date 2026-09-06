# LSP Integration Guide

This document describes how the LSP client integrates with the Gibson SDK's codegen system.

## Integration Points

### 1. Code Validation After Applying Patches

The LSP manager is used to validate code changes after applying SEARCH/REPLACE patches:

```go
import (
    "github.com/zeroroot-ai/sdk/codegen"
    "github.com/zeroroot-ai/sdk/codegen/lsp"
)

// Apply patches to code
changeset := codegen.CodeChangeSet{
    WorkspaceName: "my-project",
    Patches: []codegen.AppliedPatch{
        {
            FilePath: "main.go",
            SearchBlock: "old code",
            ReplaceBlock: "new code",
            MatchType: codegen.MatchExact,
        },
    },
}

// Validate changes with LSP
manager := lsp.NewLSPManager(lsp.DefaultLSPConfig(), logger)
if err := manager.Start(ctx, workspaceRoot); err != nil {
    return err
}
defer manager.Stop(ctx)

// Get diagnostics for modified file
diagnostics, err := manager.GetDiagnostics(ctx, "main.go")
if err != nil {
    return err
}

// Update changeset with validation results
changeset.Diagnostics = diagnostics
changeset.ValidationStatus = determineValidationStatus(diagnostics)
```

### 2. Determining Validation Status

Based on LSP diagnostics, determine if changes should be accepted:

```go
func determineValidationStatus(diagnostics []codegen.Diagnostic) codegen.ValidationStatus {
    hasErrors := false
    hasWarnings := false

    for _, d := range diagnostics {
        if d.IsError() {
            hasErrors = true
        }
        if d.IsWarning() {
            hasWarnings = true
        }
    }

    if hasErrors {
        return codegen.ValidationFailed
    }
    if hasWarnings {
        return codegen.ValidationWarnings
    }
    return codegen.ValidationPassed
}
```

### 3. Agent Integration

Agents can use the LSP manager to validate their code changes:

```go
type CodePatcherAgent struct {
    lspManager lsp.LSPManager
}

func (a *CodePatcherAgent) Execute(ctx context.Context, harness agent.Harness, task string) (agent.Result, error) {
    // ... apply code changes ...

    // Validate changes
    diagnostics, err := a.lspManager.GetDiagnostics(ctx, modifiedFile)
    if err != nil {
        return agent.NewResult().WithError(err), nil
    }

    // Check for errors
    hasErrors := false
    for _, d := range diagnostics {
        if d.IsError() {
            hasErrors = true
            break
        }
    }

    if hasErrors {
        // Rollback changes or retry
        return agent.NewResult().WithError(errors.New("validation failed")), nil
    }

    return agent.NewResult().WithSuccess(), nil
}
```

## Workflow Integration

### Typical Code Change Workflow

1. **Agent receives task** to modify code
2. **Generate patches** using LLM-driven SEARCH/REPLACE
3. **Apply patches** to files in workspace
4. **Start LSP manager** for the workspace
5. **Get diagnostics** for each modified file
6. **Evaluate results**:
   - If `ValidationFailed`: Rollback changes or retry with different patches
   - If `ValidationWarnings`: Accept changes with warnings logged
   - If `ValidationPassed`: Commit changes
7. **Store changeset** in mission memory with diagnostics
8. **Stop LSP manager** when done

### Example End-to-End Flow

```go
func ApplyAndValidateChanges(ctx context.Context, workspace string, patches []Patch) error {
    // 1. Apply patches
    for _, patch := range patches {
        if err := applyPatch(patch); err != nil {
            return fmt.Errorf("failed to apply patch: %w", err)
        }
    }

    // 2. Start LSP validation
    config := lsp.DefaultLSPConfig()
    config.ValidationTimeout = 15 * time.Second
    manager := lsp.NewLSPManager(config, slog.Default())

    if err := manager.Start(ctx, workspace); err != nil {
        return fmt.Errorf("failed to start LSP manager: %w", err)
    }
    defer manager.Stop(ctx)

    if err := manager.WaitForReady(ctx); err != nil {
        return fmt.Errorf("LSP manager not ready: %w", err)
    }

    // 3. Validate all modified files
    allDiagnostics := []codegen.Diagnostic{}
    hasErrors := false

    for _, patch := range patches {
        diagnostics, err := manager.GetDiagnostics(ctx, patch.FilePath)
        if err != nil {
            return fmt.Errorf("failed to get diagnostics: %w", err)
        }

        allDiagnostics = append(allDiagnostics, diagnostics...)

        for _, d := range diagnostics {
            if d.IsError() {
                hasErrors = true
                slog.Error("validation error",
                    "file", d.Path,
                    "line", d.Line,
                    "message", d.Message)
            }
        }
    }

    // 4. Decide on outcome
    if hasErrors {
        // Rollback changes
        for _, patch := range patches {
            rollbackPatch(patch)
        }
        return fmt.Errorf("validation failed with %d errors", countErrors(allDiagnostics))
    }

    // 5. Success - commit or store changeset
    slog.Info("validation passed", "diagnostics", len(allDiagnostics))
    return nil
}
```

## Memory Integration

Store validated changesets in mission memory for sharing between agents:

```go
// Create changeset with validation results
changeset := codegen.CodeChangeSet{
    ID:               uuid.New().String(),
    WorkspaceName:    workspace,
    Patches:          appliedPatches,
    ValidationStatus: validationStatus,
    Diagnostics:      diagnostics,
    CreatedAt:        time.Now(),
    CreatedBy:        agentID,
}

// Store in mission memory
memory.StoreMissionMemory(ctx, missionID, "code_changes", changeset)

// Later, other agents can read and understand what was changed
previousChanges := memory.GetMissionMemory(ctx, missionID, "code_changes")
```

## Performance Considerations

### Server Startup

- Language servers take 1-5 seconds to initialize
- Start the manager once per workspace, not per file
- Use `WaitForReady()` before validation requests

### Validation Timing

- gopls typically returns diagnostics within 500ms-2s
- Complex files or large projects may take longer
- Default timeout is 10 seconds, increase if needed

### Resource Usage

- Each language server is a separate process
- gopls uses ~50-200MB RAM per workspace
- Multiple workspaces = multiple gopls instances
- Stop the manager when done to free resources

## Error Handling

### Common Errors

```go
// Language server not installed
err := manager.Start(ctx, workspace)
if err != nil && strings.Contains(err.Error(), "not found in PATH") {
    // Handle missing binary - skip validation or return error
}

// Timeout during validation
diagnostics, err := manager.GetDiagnostics(ctx, file)
if errors.Is(err, lsp.ErrLSPTimeout) {
    // Increase timeout or retry
}

// Invalid workspace
err := manager.Start(ctx, "/invalid/path")
if err != nil && strings.Contains(err.Error(), "workspace root not accessible") {
    // Validate workspace path before starting
}
```

### Graceful Degradation

If LSP validation is not available, agents can still function:

```go
manager := lsp.NewLSPManager(config, logger)
if err := manager.Start(ctx, workspace); err != nil {
    logger.Warn("LSP validation unavailable, skipping", "error", err)
    // Continue without validation
    return applyPatchesWithoutValidation(patches)
}
defer manager.Stop(ctx)

// Normal validation flow...
```

## Testing Integration

### Unit Tests

Test agents with a mock LSP manager:

```go
type mockLSPManager struct {
    diagnostics []codegen.Diagnostic
}

func (m *mockLSPManager) GetDiagnostics(ctx context.Context, path string) ([]codegen.Diagnostic, error) {
    return m.diagnostics, nil
}

// Use in tests
agent := &CodePatcherAgent{
    lspManager: &mockLSPManager{
        diagnostics: []codegen.Diagnostic{
            {
                Severity: codegen.SeverityError,
                Message:  "test error",
            },
        },
    },
}
```

### Integration Tests

For integration tests with real language servers:

```go
func TestAgentWithRealLSP(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    workspace := setupTestWorkspace(t)
    defer cleanupWorkspace(workspace)

    manager := lsp.NewLSPManager(lsp.DefaultLSPConfig(), slog.Default())
    if err := manager.Start(context.Background(), workspace); err != nil {
        t.Skipf("LSP not available: %v", err)
    }
    defer manager.Stop(context.Background())

    // Test agent with real LSP validation
}
```

## Future Enhancements

- **Incremental Updates**: Use `textDocument/didChange` instead of open/close
- **Document Caching**: Keep documents open for repeated validation
- **Workspace Diagnostics**: Get all diagnostics for a workspace at once
- **Code Actions**: Support LSP quickfixes and refactorings
- **Multi-Language**: Add Python and TypeScript support
- **Metrics**: Track validation performance and error rates
