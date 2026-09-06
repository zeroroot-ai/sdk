// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace

import (
	"context"
	"time"

	"github.com/zeroroot-ai/sdk/codegen/editor"
	"github.com/zeroroot-ai/sdk/codegen/git"
)

// Workspace provides access to a Git repository clone with integrated editing and Git operations.
// Each workspace corresponds to a single repository and provides isolated file access,
// code editing with validation, and Git operations for branching and committing changes.
//
// Workspaces are created and managed by the WorkspaceManager during mission initialization.
// Agents access workspaces through the Harness interface.
type Workspace interface {
	// Name returns the repository identifier for this workspace.
	// This corresponds to the RepositoryConfig.Name from the mission configuration.
	Name() string

	// Path returns the absolute path to the workspace root directory.
	// All file paths used with this workspace should be relative to this path.
	Path() string

	// Editor returns the code editor for this workspace.
	// The editor provides SEARCH/REPLACE operations with LSP validation.
	Editor() Editor

	// Git returns the Git operations interface for this workspace.
	// Provides branching, committing, pushing, and snapshot/rollback capabilities.
	Git() GitOps

	// ReadFile reads a file from the workspace.
	// The path should be relative to the workspace root.
	// Returns an error if the file does not exist or cannot be read.
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// WriteFile writes content to a file in the workspace.
	// The path should be relative to the workspace root.
	// Creates parent directories if they don't exist.
	// Returns an error if the file cannot be written.
	WriteFile(ctx context.Context, path string, content []byte) error

	// ListFiles returns all file paths matching the given glob pattern.
	// The pattern is matched against paths relative to the workspace root.
	// Examples: "*.go", "**/*.py", "src/**/*.ts"
	// Returns an empty slice if no files match.
	ListFiles(ctx context.Context, pattern string) ([]string, error)

	// Commit stages all changes and creates a commit with the given message.
	// It calls git.Add(ctx, ".") to stage everything, then git.Commit(ctx, message).
	// Returns the commit SHA on success.
	Commit(ctx context.Context, message string) (string, error)

	// Push pushes committed changes to the remote repository.
	// Returns an error if the remote is unreachable or if authentication fails.
	Push(ctx context.Context) error

	// Close releases resources associated with this workspace.
	// This should be called when the workspace is no longer needed.
	// For worktrees, this removes the worktree directory.
	// Returns an error if cleanup fails.
	Close() error
}

// WorkspaceManager manages the lifecycle of workspaces for a mission.
// It clones repositories during initialization and provides access to workspaces
// throughout mission execution. It handles cleanup of workspaces when the mission completes.
//
// The manager is created by the daemon during mission setup and injected into the harness.
type WorkspaceManager interface {
	// Initialize clones all repositories defined in the workspace configuration.
	// This is called by the daemon before any agents execute.
	// Returns an error if any repository clone fails.
	//
	// The initialization process:
	// 1. Validates all repository configurations
	// 2. Retrieves required credentials from the credential store
	// 3. Clones repositories in dependency order (respecting DependsOn)
	// 4. Checks out the specified branch for each repository
	// 5. Initializes LSP servers if LSPEnabled is true
	Initialize(ctx context.Context, config WorkspaceConfig) error

	// Primary returns the default workspace for single-repository missions.
	// Returns the first repository defined in the configuration.
	// Returns nil if no repositories are configured.
	Primary() Workspace

	// Get returns the workspace for the specified repository name.
	// The name corresponds to RepositoryConfig.Name from the configuration.
	// Returns nil and false if no workspace exists with that name.
	Get(name string) (Workspace, bool)

	// All returns a map of all workspaces keyed by repository name.
	// Returns an empty map if no workspaces are initialized.
	All() map[string]Workspace

	// Cleanup removes all workspace directories and stops LSP servers.
	// This is called by the daemon after mission completion if CleanupOnComplete is true.
	// Returns an error if any cleanup operation fails.
	Cleanup(ctx context.Context) error
}

// WorkspaceConfig defines the workspace configuration for a mission.
// This is deserialized from the mission YAML configuration and passed to the manager.
type WorkspaceConfig struct {
	// Repositories contains the list of Git repositories to clone.
	// Each repository becomes a workspace accessible to agents.
	Repositories []RepositoryConfig

	// Settings contains workspace-wide settings for cleanup, LSP, and isolation.
	Settings WorkspaceSettings
}

// RepositoryConfig defines a single Git repository to clone for the mission.
type RepositoryConfig struct {
	// Name is the unique identifier for this repository within the mission.
	// Agents use this name to access the workspace via harness.Workspace(name).
	Name string

	// URL is the Git repository URL (HTTPS or SSH).
	// Examples:
	//   HTTPS: https://github.com/org/repo.git
	//   SSH:   git@github.com:org/repo.git
	URL string

	// Branch is the Git branch to checkout after cloning.
	// Defaults to the repository's default branch if empty.
	Branch string

	// CredentialName references a stored credential for authentication.
	// The credential is retrieved from the workspace manager's CredStore (see manager.go).
	// Agents pass a credential name; the manager resolves it through the plugin credential
	// path — agents never receive the secret value directly.
	// Supports API tokens for HTTPS and SSH keys for SSH URLs.
	CredentialName string

	// Shallow enables shallow cloning with --depth 1 for faster clones.
	// Use this for large repositories where full history is not needed.
	Shallow bool

	// DependsOn lists repository names that must be cloned before this one.
	// This enables dependency ordering for multi-repository projects.
	// The manager will clone repositories in topologically sorted order.
	DependsOn []string
}

// WorkspaceSettings contains workspace-wide configuration options.
type WorkspaceSettings struct {
	// CleanupOnComplete determines whether to delete workspace directories
	// after mission completion. Set to false to preserve workspaces for debugging.
	CleanupOnComplete bool

	// UseWorktrees enables Git worktrees for agent isolation.
	// When true, each agent gets a separate worktree from the same repository,
	// allowing concurrent modifications without conflicts.
	UseWorktrees bool

	// LSPEnabled determines whether to start language servers for code validation.
	// When true, editors will validate changes using LSP before applying them.
	LSPEnabled bool

	// LSPTimeout is the maximum time to wait for LSP validation responses.
	// If validation exceeds this timeout, changes are applied with a warning.
	LSPTimeout time.Duration

	// BaseDirectory is the directory where workspace clones are created.
	// Defaults to a temporary directory if not specified.
	BaseDirectory string
}

// Editor provides code editing operations with SEARCH/REPLACE blocks and LSP validation.
// It is a type alias for the fully-defined interface in codegen/editor.
type Editor = editor.Editor

// GitOps provides Git operations for a workspace.
// It is a type alias for the fully-defined interface in codegen/git.
type GitOps = git.GitOps
