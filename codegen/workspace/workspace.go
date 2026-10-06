// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace

import (
	"context"
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
}

// WorkspaceManager manages the lifecycle of workspaces for a mission.
// It clones repositories during initialization and provides access to workspaces
// throughout mission execution. It handles cleanup of workspaces when the mission completes.
//
// The manager is created by the daemon during mission setup and injected into the harness.
type WorkspaceManager interface {

	// Primary returns the default workspace for single-repository missions.
	// Returns the first repository defined in the configuration.
	// Returns nil if no repositories are configured.
	Primary() Workspace

	// All returns a map of all workspaces keyed by repository name.
	// Returns an empty map if no workspaces are initialized.
	All() map[string]Workspace
}
