# CodeGen SDK Integration Tests

This document describes the integration tests for the CodeGen SDK and how to run them.

## Overview

The integration tests validate end-to-end functionality of the CodeGen SDK using real Git and LSP servers. These tests are more comprehensive than unit tests and verify that all components work together correctly in realistic scenarios.

## Prerequisites

### Required Tools

- **Git** (version 2.20+): Required for all tests
  ```bash
  git --version
  ```

### Optional Tools (for LSP tests)

The following language servers are optional. Tests will skip if not available:

- **gopls**: Go language server
  ```bash
  go install golang.org/x/tools/gopls@latest
  gopls version
  ```

- **pyright-langserver**: Python language server
  ```bash
  npm install -g pyright
  pyright-langserver --version
  ```

- **typescript-language-server**: TypeScript/JavaScript language server
  ```bash
  npm install -g typescript-language-server typescript
  typescript-language-server --version
  ```

## Running the Tests

### Run All Integration Tests

```bash
# From the SDK root
make test-integration

# Or directly with go test
cd core/sdk/codegen
go test -v -tags=integration -timeout=10m
```

### Run Specific Tests

```bash
cd core/sdk/codegen

# Run only the full workflow test
go test -v -tags=integration -run TestFullWorkflow

# Run all LSP tests
go test -v -tags=integration -run 'TestGoCodeWithLSP|TestPythonCodeWithLSP|TestTypeScriptCodeWithLSP'

# Run multi-repo and worktree tests
go test -v -tags=integration -run 'TestMultiRepo|TestWorktree'
```

### Run Without LSP Tests

If you don't have language servers installed, you can run tests without LSP:

```bash
go test -v -tags=integration -run 'Test(FullWorkflow|MultiRepo|Worktree|Cleanup|EdgeCases)'
```

## Test Coverage

### 1. TestFullWorkflow
**Purpose**: Tests the complete workflow from clone to commit.

**What it does**:
- Creates a test Git repository with buggy code
- Clones the repository using WorkspaceManager
- Applies a SEARCH/REPLACE edit to fix the bug
- Validates the edit was applied correctly
- Commits the changes
- Verifies the commit exists in history

**Requirements**: Git only

**Duration**: ~1 second

---

### 2. TestGoCodeWithLSP
**Purpose**: Tests Go code editing with gopls validation.

**What it does**:
- Creates a Go repository with syntax errors
- Initializes gopls language server
- Applies an edit that fixes the syntax error
- Validates with gopls that no errors remain

**Requirements**: Git, gopls

**Duration**: ~5 seconds (gopls initialization)

---

### 3. TestPythonCodeWithLSP
**Purpose**: Tests Python code editing with pyright validation.

**What it does**:
- Creates a Python repository with SQL injection vulnerability
- Initializes pyright language server
- Applies an edit that fixes the SQL injection
- Validates the fix with pyright

**Requirements**: Git, pyright-langserver

**Duration**: ~5 seconds (pyright initialization)

---

### 4. TestTypeScriptCodeWithLSP
**Purpose**: Tests TypeScript code editing with tsserver validation.

**What it does**:
- Creates a TypeScript repository with XSS vulnerability
- Initializes typescript-language-server
- Applies an edit that fixes the XSS vulnerability
- Validates the fix with tsserver

**Requirements**: Git, typescript-language-server

**Duration**: ~5 seconds (tsserver initialization)

---

### 5. TestMultiRepoScenario
**Purpose**: Tests working with multiple repositories simultaneously.

**What it does**:
- Creates two separate Git repositories
- Initializes workspace with both repositories
- Verifies both workspaces are accessible
- Applies edits to both repositories
- Commits changes to each independently
- Verifies commits in both repositories

**Requirements**: Git only

**Duration**: ~1 second

---

### 6. TestWorktreeIsolation
**Purpose**: Tests Git worktree isolation for concurrent work.

**What it does**:
- Creates a main Git repository
- Initializes workspace with worktree support
- Makes changes in the worktree
- Verifies main repository is unaffected
- Validates worktree isolation

**Requirements**: Git only

**Duration**: ~1 second

---

### 7. TestCleanup
**Purpose**: Tests workspace cleanup functionality.

**What it does**:
- Creates a workspace with test repository
- Makes changes to files
- Calls Cleanup()
- Verifies temporary directories are removed
- Ensures no leftover files remain

**Requirements**: Git only

**Duration**: <1 second

---

### 8. TestLSPValidationIntegration
**Purpose**: Tests LSP validation with automatic rollback.

**What it does**:
- Creates a Go repository with valid code
- Initializes gopls
- Applies an edit that introduces an error
- Verifies the edit is rolled back automatically
- Applies a valid edit
- Verifies the valid edit is applied

**Requirements**: Git, gopls

**Duration**: ~5 seconds

---

### 9. TestEdgeCases
**Purpose**: Tests error conditions and edge cases.

**What it does**:
- Empty repository list
- Duplicate repository names
- Invalid repository URLs
- Missing credentials

**Requirements**: Git only

**Duration**: <1 second

## Test Fixtures

The integration tests create realistic code fixtures for testing:

### Go Fixtures
- Simple calculator function with common bug patterns
- Syntax errors (missing braces)
- Type errors (undefined variables)

### Python Fixtures
- SQL injection vulnerability (string concatenation)
- Fixed version using parameterized queries

### TypeScript Fixtures
- XSS vulnerability (direct HTML injection)
- Fixed version with HTML escaping

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install gopls
        run: go install golang.org/x/tools/gopls@latest

      - name: Install Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install LSP servers
        run: |
          npm install -g pyright typescript-language-server typescript

      - name: Run integration tests
        run: |
          cd core/sdk/codegen
          go test -v -tags=integration -timeout=10m
```

### GitLab CI Example

```yaml
integration:
  image: golang:1.21
  before_script:
    - apt-get update && apt-get install -y nodejs npm
    - go install golang.org/x/tools/gopls@latest
    - npm install -g pyright typescript-language-server typescript
  script:
    - cd core/sdk/codegen
    - go test -v -tags=integration -timeout=10m
```

## Debugging Tests

### Enable Verbose Output

```bash
go test -v -tags=integration -run TestFullWorkflow
```

### Enable Debug Logging

The tests use slog with different log levels. To see debug output:

```go
// In test code, change:
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
```

### Preserve Test Artifacts

By default, tests use `t.TempDir()` which is automatically cleaned up. To preserve test artifacts for inspection:

```go
// Change from:
tmpDir := t.TempDir()

// To:
tmpDir := filepath.Join("/tmp", "codegen-test-"+time.Now().Format("20060102-150405"))
os.MkdirAll(tmpDir, 0755)
t.Logf("Test artifacts in: %s", tmpDir)
```

### Run Single Test with Debugging

```bash
# Run with verbose output and debugger
dlv test -tags=integration -- -test.run TestFullWorkflow -test.v

# Or with VSCode launch.json:
{
  "name": "Debug Integration Test",
  "type": "go",
  "request": "launch",
  "mode": "test",
  "program": "${workspaceFolder}/core/sdk/codegen",
  "args": ["-test.run", "TestFullWorkflow", "-test.v"],
  "buildFlags": "-tags=integration"
}
```

## Common Issues

### LSP Tests Skipped

**Symptom**: Tests show "SKIP: gopls not available in PATH"

**Solution**: Install the required language server:
```bash
# For Go
go install golang.org/x/tools/gopls@latest

# For Python
pip install pyright

# For TypeScript
npm install -g typescript-language-server typescript
```

### Timeout Errors

**Symptom**: "context deadline exceeded" or "timeout"

**Solution**: Increase timeout:
```bash
go test -v -tags=integration -timeout=15m
```

### Permission Errors

**Symptom**: "permission denied" when creating temp directories

**Solution**: Check `/tmp` permissions or set TMPDIR:
```bash
export TMPDIR=$HOME/tmp
mkdir -p $TMPDIR
go test -v -tags=integration
```

### Git Not Found

**Symptom**: "git not available in PATH"

**Solution**: Install Git or add to PATH:
```bash
# Ubuntu/Debian
sudo apt-get install git

# macOS
brew install git

# Verify
git --version
```

## Performance

Typical test execution times on modern hardware:

| Test | Duration | Notes |
|------|----------|-------|
| TestFullWorkflow | 1s | No LSP |
| TestGoCodeWithLSP | 5s | Includes gopls init |
| TestPythonCodeWithLSP | 5s | Includes pyright init |
| TestTypeScriptCodeWithLSP | 5s | Includes tsserver init |
| TestMultiRepoScenario | 1s | No LSP |
| TestWorktreeIsolation | 1s | No LSP |
| TestCleanup | <1s | No LSP |
| TestLSPValidationIntegration | 5s | Includes gopls init |
| TestEdgeCases | <1s | No LSP |
| **Total (all tests)** | **~25s** | With all LSP servers |
| **Total (no LSP)** | **~5s** | Git only |

## Contributing

When adding new integration tests:

1. Use the `//go:build integration` build tag
2. Use `t.TempDir()` for temporary directories
3. Check for required tools with `checkCommandAvailable()`
4. Skip gracefully if tools are not available
5. Clean up resources with `defer`
6. Use meaningful test names that describe what is being tested
7. Add test documentation to this README

### Example Test Template

```go
//go:build integration

func TestMyNewFeature(t *testing.T) {
    if !checkCommandAvailable("git") {
        t.Skip("git not available in PATH")
    }
    // Optional: check for language server
    if !checkCommandAvailable("gopls") {
        t.Skip("gopls not available in PATH")
    }

    ctx := context.Background()
    tmpDir := t.TempDir()

    // Test setup
    // ...

    // Test execution
    // ...

    // Assertions
    require.NoError(t, err)
    assert.Equal(t, expected, actual)

    t.Logf("Test completed successfully")
}
```

## Questions?

For questions or issues with integration tests:
- Check this README
- Review existing tests for examples
- Check CI logs for detailed error messages
- File an issue with test output
