// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package workspace provides workspace management for the CodeGen SDK.
//
// A workspace represents a Git repository clone that agents can read from
// and write to during mission execution. The workspace manager handles
// repository cloning, credential management, and lifecycle operations.
//
// # Overview
//
// Workspaces provide isolated working directories for agents to perform
// code generation and modification tasks. Each workspace corresponds to
// a single Git repository and provides:
//
//   - File system access (read, write, list files)
//   - Code editing via the editor interface
//   - Git operations via the git interface
//   - Automatic cleanup after mission completion
//
// # Architecture
//
// The workspace package defines two main interfaces:
//
//   - Workspace: Represents a single repository clone
//   - WorkspaceManager: Manages multiple workspaces for a mission
//
// Agents access workspaces through the Harness interface, which provides
// the WorkspaceManager implementation.
//
// # Basic Usage
//
// Single Repository Mission:
//
//	// Get the primary workspace
//	ws := harness.Workspace()
//	if ws == nil {
//	    return errors.New("no workspace configured")
//	}
//
//	// Read a file
//	content, err := ws.ReadFile(ctx, "config.yaml")
//	if err != nil {
//	    return fmt.Errorf("failed to read config: %w", err)
//	}
//
//	// Write a file
//	newConfig := []byte("updated: true\n")
//	err = ws.WriteFile(ctx, "config.yaml", newConfig)
//	if err != nil {
//	    return fmt.Errorf("failed to write config: %w", err)
//	}
//
//	// List files matching a pattern
//	goFiles, err := ws.ListFiles(ctx, "**/*.go")
//	if err != nil {
//	    return err
//	}
//	for _, file := range goFiles {
//	    log.Printf("Found Go file: %s", file)
//	}
//
// Multi-Repository Mission:
//
//	// Get a specific workspace by name
//	frontend, ok := harness.GetWorkspace("frontend")
//	if !ok {
//	    return errors.New("frontend workspace not found")
//	}
//
//	backend, ok := harness.GetWorkspace("backend")
//	if !ok {
//	    return errors.New("backend workspace not found")
//	}
//
//	// Work with both workspaces
//	frontendFiles, _ := frontend.ListFiles(ctx, "src/**/*.ts")
//	backendFiles, _ := backend.ListFiles(ctx, "cmd/**/*.go")
//
//	// Access all workspaces
//	for name, ws := range harness.AllWorkspaces() {
//	    log.Printf("Workspace %s at %s", name, ws.Path())
//	}
//
// # Configuration
//
// Workspaces are configured in the mission YAML under the workspace section:
//
//	workspace:
//	  repositories:
//	    - name: main-app
//	      url: https://github.com/org/app.git
//	      branch: main
//	      credential: github-token
//	      shallow: false
//
//	  settings:
//	    cleanup_on_complete: true
//	    use_worktrees: false
//	    lsp_enabled: true
//	    lsp_timeout: 10s
//	    base_directory: /tmp/gibson-workspaces
//
// # Repository Configuration
//
// Each repository is configured with:
//
// Name (required):
//
//	Unique identifier for this repository within the mission.
//	Used to access the workspace via harness.GetWorkspace(name).
//
// URL (required):
//
//	Git repository URL. Supports HTTPS and SSH protocols.
//	Examples:
//	  - https://github.com/org/repo.git
//	  - git@github.com:org/repo.git
//	  - file:///path/to/local/repo (for testing)
//
// Branch:
//
//	Git branch to checkout after cloning.
//	Defaults to the repository's default branch if not specified.
//
// Credential:
//
//	Name of a stored credential to use for authentication.
//	The credential name is passed to the workspace manager's CredStore for
//	resolution. Agents do not receive the secret value; credential access
//	flows through the plugin credential path (see plugin-runtime spec).
//	Supports:
//	  - API tokens for HTTPS URLs
//	  - SSH keys for SSH URLs
//
// Shallow:
//
//	Enable shallow cloning with --depth 1.
//	Useful for large repositories where full history is not needed.
//	Reduces clone time and disk usage.
//	Default: false
//
// DependsOn:
//
//	List of repository names that must be cloned before this one.
//	Enables dependency ordering for multi-repository projects.
//	The manager will clone repositories in topologically sorted order.
//
// # Workspace Settings
//
// Global settings that apply to all workspaces:
//
// CleanupOnComplete:
//
//	Whether to delete workspace directories after mission completion.
//	Set to false to preserve workspaces for debugging.
//	Default: true
//
// UseWorktrees:
//
//	Enable Git worktrees for agent isolation.
//	When true, each agent gets a separate worktree from the same repository,
//	allowing concurrent modifications without conflicts.
//	Default: false
//
// LSPEnabled:
//
//	Whether to start language servers for code validation.
//	When true, editors will validate changes using LSP before applying them.
//	Requires language server binaries to be installed (gopls, pyright, etc.).
//	Default: false
//
// LSPTimeout:
//
//	Maximum time to wait for LSP validation responses.
//	If validation exceeds this timeout, changes are applied with a warning.
//	Default: 10s
//
// BaseDirectory:
//
//	Directory where workspace clones are created.
//	Defaults to a temporary directory if not specified.
//	Example: /tmp/gibson-workspaces
//
// # Workspace Operations
//
// File Operations:
//
//	// Read a file (returns bytes)
//	content, err := ws.ReadFile(ctx, "path/to/file.txt")
//
//	// Write a file (creates parent directories if needed)
//	err = ws.WriteFile(ctx, "path/to/file.txt", []byte("content"))
//
//	// List files matching a glob pattern
//	files, err := ws.ListFiles(ctx, "**/*.go")
//	files, err := ws.ListFiles(ctx, "src/**/*.ts")
//	files, err := ws.ListFiles(ctx, "*.json")
//
// Code Editing:
//
//	// Get the editor for this workspace
//	editor := ws.Editor()
//
//	// Apply a SEARCH/REPLACE edit
//	edit := editor.Edit{
//	    FilePath:     "main.go",
//	    SearchBlock:  "old code",
//	    ReplaceBlock: "new code",
//	}
//	result, err := editor.Apply(ctx, edit)
//
// Git Operations:
//
//	// Get the Git interface for this workspace
//	git := ws.Git()
//
//	// Create a branch
//	err = git.CreateBranch(ctx, "feature/new-feature")
//
//	// Stage changes
//	err = git.Add(ctx, "main.go", "config.yaml")
//
//	// Commit changes
//	sha, err := git.Commit(ctx, "Add feature", git.CommitOptions{
//	    Author: "Agent <agent@example.com>",
//	})
//
// # Multi-Repository Scenarios
//
// Coordinated Changes Across Repositories:
//
//	frontend, _ := harness.GetWorkspace("frontend")
//	backend, _ := harness.GetWorkspace("backend")
//
//	// Update API contract in backend
//	backendEdit := editor.Edit{
//	    FilePath:     "api/handlers.go",
//	    SearchBlock:  "old endpoint",
//	    ReplaceBlock: "new endpoint",
//	}
//	_, err := backend.Editor().Apply(ctx, backendEdit)
//
//	// Update client code in frontend
//	frontendEdit := editor.Edit{
//	    FilePath:     "src/api/client.ts",
//	    SearchBlock:  "old endpoint",
//	    ReplaceBlock: "new endpoint",
//	}
//	_, err = frontend.Editor().Apply(ctx, frontendEdit)
//
//	// Commit to both repositories
//	backend.Git().Add(ctx, "api/handlers.go")
//	backend.Git().Commit(ctx, "Update API endpoint", git.CommitOptions{})
//
//	frontend.Git().Add(ctx, "src/api/client.ts")
//	frontend.Git().Commit(ctx, "Update API client", git.CommitOptions{})
//
// Dependency Ordering:
//
//	# In mission YAML:
//	repositories:
//	  - name: shared-lib
//	    url: https://github.com/org/shared.git
//	    branch: main
//
//	  - name: api-service
//	    url: https://github.com/org/api.git
//	    branch: main
//	    depends_on:
//	      - shared-lib  # Clone shared-lib first
//
//	  - name: web-app
//	    url: https://github.com/org/web.git
//	    branch: main
//	    depends_on:
//	      - shared-lib
//	      - api-service  # Clone after both dependencies
//
// # Workspace Lifecycle
//
// Initialization:
//
//	The WorkspaceManager is created by the daemon during mission setup
//	and initialized before any agents execute:
//
//	1. Validate all repository configurations
//	2. Retrieve required credentials from the credential store
//	3. Clone repositories in dependency order (respecting DependsOn)
//	4. Check out the specified branch for each repository
//	5. Initialize LSP servers if LSPEnabled is true
//
// Cleanup:
//
//	After mission completion, the daemon calls Cleanup() if CleanupOnComplete is true:
//
//	1. Stop all running LSP servers
//	2. Remove workspace directories
//	3. Release system resources
//
//	Workspaces can be preserved for debugging by setting cleanup_on_complete: false.
//
// # Credentials
//
// Workspaces support multiple credential types:
//
// API Token (HTTPS):
//
//	# In credential store:
//	name: github-token
//	type: token
//	value: ghp_xxxxxxxxxxxxx
//
//	# In mission YAML:
//	repositories:
//	  - name: app
//	    url: https://github.com/org/app.git
//	    credential: github-token
//
// SSH Key:
//
//	# In credential store:
//	name: github-ssh
//	type: ssh_key
//	value: |
//	  -----BEGIN OPENSSH PRIVATE KEY-----
//	  ...
//	  -----END OPENSSH PRIVATE KEY-----
//
//	# In mission YAML:
//	repositories:
//	  - name: app
//	    url: git@github.com:org/app.git
//	    credential: github-ssh
//
// # Error Handling
//
// Common errors and how to handle them:
//
// Clone Failed (ErrCloneFailed):
//
//	Causes:
//	  - Network issues
//	  - Invalid repository URL
//	  - Authentication failure
//	  - Insufficient permissions
//
//	Solution:
//	  - Verify repository URL is correct
//	  - Check credential is valid and has access
//	  - Ensure network connectivity
//
// Credential Missing (ErrCredentialMissing):
//
//	Causes:
//	  - Credential name not found in store
//	  - Typo in credential name
//
//	Solution:
//	  - Verify credential exists in credential store
//	  - Check spelling of credential name in config
//
// Workspace Not Ready (ErrWorkspaceNotReady):
//
//	Causes:
//	  - Accessing workspace before initialization
//	  - Initialization failed
//
//	Solution:
//	  - Check mission logs for initialization errors
//	  - Verify repository configuration is valid
//
// # Performance Optimization
//
// Shallow Clones:
//
//	Use shallow cloning for large repositories:
//
//	repositories:
//	  - name: big-repo
//	    url: https://github.com/org/big-repo.git
//	    shallow: true  # Clone only latest commit
//
//	Benefits:
//	  - Faster clone times
//	  - Reduced disk usage
//	  - Lower network bandwidth
//
//	Limitations:
//	  - Cannot access full Git history
//	  - Some Git operations may be restricted
//
// Worktrees:
//
//	Enable worktrees for concurrent agent execution:
//
//	settings:
//	  use_worktrees: true
//
//	Benefits:
//	  - Each agent gets isolated working directory
//	  - Concurrent modifications without conflicts
//	  - Share Git objects directory (saves disk space)
//
//	Use cases:
//	  - Multiple agents working on same repository
//	  - Parallel code generation tasks
//	  - A/B testing different approaches
//
// # Best Practices
//
// Credential Management:
//
//   - Store credentials in the credential store, never in code
//   - Use separate credentials for different repositories if possible
//   - Rotate credentials regularly
//   - Use SSH keys for automation, tokens for development
//
// Repository Configuration:
//
//   - Use descriptive names for repositories
//   - Always specify a branch explicitly
//   - Use shallow clones for large repositories
//   - Set appropriate cleanup policies
//
// Error Handling:
//
//   - Always check workspace existence before use
//   - Handle file operation errors gracefully
//   - Log errors with context for debugging
//   - Use defer for cleanup operations
//
// # Thread Safety
//
// All workspace operations are thread-safe and support concurrent access:
//
//   - Multiple goroutines can read files simultaneously
//   - Write operations are serialized per workspace
//   - Editor and Git operations include internal locking
//
// However, concurrent modifications to the same file from multiple agents
// should be coordinated at the application level to avoid conflicts.
//
// # Examples
//
// See integration_test.go for complete examples:
//   - TestFullMission: Complete mission from clone to commit
//   - TestMultiRepoScenario: Working with multiple repositories
//   - TestWorktreeIsolation: Using worktrees for isolation
//   - TestCleanup: Workspace cleanup operations
package workspace
