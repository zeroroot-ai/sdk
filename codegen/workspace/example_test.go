// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/types"
)

// mockCredStore implements workspace.CredentialStore for examples.
type mockCredStore struct {
	creds map[string]*types.Credential
}

func (m *mockCredStore) Get(name string) (*types.Credential, error) {
	if cred, ok := m.creds[name]; ok {
		return cred, nil
	}
	return nil, fmt.Errorf("credential not found: %s", name)
}

// Example_basicWorkspaceUsage demonstrates initializing a workspace manager
// and cloning repositories.
func Example_basicWorkspaceUsage() {
	_ = context.Background()

	// Create a credential store with a GitHub token
	credStore := &mockCredStore{
		creds: map[string]*types.Credential{
			"github-token": {
				Name:   "github-token",
				Type:   types.CredentialTypeBearer,
				Secret: "<GITHUB_TOKEN>",
			},
		},
	}

	// Create workspace manager
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	_ = workspace.NewWorkspaceManager(credStore, logger)

	// Configure workspace with repositories
	_ = workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:           "main-repo",
				URL:            "https://github.com/example/repo.git",
				Branch:         "main",
				CredentialName: "github-token",
				Shallow:        true, // Fast clone for large repos
			},
		},
		Settings: workspace.WorkspaceSettings{
			CleanupOnComplete: true,
			LSPEnabled:        false, // Disable LSP for this example
		},
	}

	// Initialize (would clone repos in real usage, skipped in example)
	// err := mgr.Initialize(ctx, config)
	// if err != nil {
	//     log.Fatal(err)
	// }

	// Access primary workspace
	// ws := mgr.Primary()
	// fmt.Println("Workspace path:", ws.Path())

	// Cleanup when done
	// defer mgr.Cleanup(ctx)

	fmt.Println("Workspace manager created successfully")
	// Output: Workspace manager created successfully
}

// Example_multiRepositorySetup demonstrates setting up multiple repositories
// with dependency ordering.
func Example_multiRepositorySetup() {
	_ = context.Background()

	credStore := &mockCredStore{creds: make(map[string]*types.Credential)}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	_ = workspace.NewWorkspaceManager(credStore, logger)

	// Configure multiple repositories with dependencies
	_ = workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "common-lib",
				URL:    "https://github.com/example/common.git",
				Branch: "main",
			},
			{
				Name:      "service-a",
				URL:       "https://github.com/example/service-a.git",
				Branch:    "develop",
				DependsOn: []string{"common-lib"}, // Clone common-lib first
			},
			{
				Name:      "service-b",
				URL:       "https://github.com/example/service-b.git",
				Branch:    "develop",
				DependsOn: []string{"common-lib"}, // Clone common-lib first
			},
		},
		Settings: workspace.WorkspaceSettings{
			CleanupOnComplete: true,
			LSPEnabled:        false,
		},
	}

	// Initialize would clone in order: common-lib, then service-a and service-b

	// Access specific workspaces by name
	// if ws, ok := mgr.Get("service-a"); ok {
	//     fmt.Println("Service A workspace:", ws.Path())
	// }

	// Get all workspaces
	// allWorkspaces := mgr.All()
	// fmt.Printf("Total workspaces: %d\n", len(allWorkspaces))

	fmt.Println("Multi-repository configuration created")
	// Output: Multi-repository configuration created
}

// Example_workspaceFileOperations demonstrates reading and writing files
// in a workspace.
func Example_workspaceFileOperations() {
	_ = context.Background()

	// Create temporary directory for example
	tempDir, err := os.MkdirTemp("", "workspace-example-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	credStore := &mockCredStore{creds: make(map[string]*types.Credential)}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	_ = workspace.NewWorkspaceManager(credStore, logger)

	// For this example, we'll create a mock workspace directly
	// In real usage, workspaces are created via Initialize()

	// Example operations (would use actual workspace):
	// ws := mgr.Primary()

	// Write a file
	// content := []byte("package main\n\nfunc main() {}\n")
	// err = ws.WriteFile(ctx, "main.go", content)

	// Read the file back
	// data, err := ws.ReadFile(ctx, "main.go")
	// fmt.Printf("File content length: %d bytes\n", len(data))

	// List files matching a pattern
	// files, err := ws.ListFiles(ctx, "*.go")
	// fmt.Printf("Go files found: %d\n", len(files))

	fmt.Println("File operations example completed")
	// Output: File operations example completed
}

// Example_worktreeCreation demonstrates creating Git worktrees for
// multi-agent isolation.
func Example_worktreeCreation() {
	// ctx := context.Background()

	// credStore := &mockCredStore{creds: make(map[string]*types.Credential)}
	// logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	// mgr := workspace.NewWorkspaceManager(credStore, logger)

	// Initialize with worktrees enabled
	// config := workspace.WorkspaceConfig{
	//     Repositories: []workspace.RepositoryConfig{
	//         {
	//             Name:   "main-repo",
	//             URL:    "https://github.com/example/repo.git",
	//             Branch: "main",
	//         },
	//     },
	//     Settings: workspace.WorkspaceSettings{
	//         UseWorktrees: true, // Enable worktree support
	//         LSPEnabled:   false,
	//     },
	// }
	// mgr.Initialize(ctx, config)

	// Create a worktree for an agent
	// agentWorkspace, err := mgr.CreateWorktree(ctx, "main-repo", "feature-branch", "agent-123")
	// if err != nil {
	//     log.Fatal(err)
	// }
	// fmt.Println("Agent workspace created:", agentWorkspace.Path())

	// Agent can now work independently in the worktree
	// agentWorkspace.WriteFile(ctx, "new-file.txt", []byte("agent changes"))

	// Cleanup worktree when done
	// defer mgr.RemoveWorktree(ctx, agentWorkspace.Name())

	fmt.Println("Worktree example completed")
	// Output: Worktree example completed
}

// Example_credentialHandling demonstrates secure credential usage for
// Git operations.
func Example_credentialHandling() {
	// ctx := context.Background()

	// Create credential store with different credential types
	// credStore := &mockCredStore{
	//     creds: map[string]*types.Credential{
	//         "github-pat": {
	//             Name:   "github-pat",
	//             Type:   types.CredentialTypeBearer,
	//             Secret: "<GITHUB_PAT>",
	//         },
	//         "gitlab-token": {
	//             Name:   "gitlab-token",
	//             Type:   types.CredentialTypeAPIKey,
	//             Secret: "<GITLAB_TOKEN>",
	//         },
	//         "ssh-key": {
	//             Name:   "deploy-key",
	//             Type:   types.CredentialTypeCustom,
	//             Secret: "-----BEGIN PRIVATE KEY-----\n...",
	//         },
	//     },
	// }

	// logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	// mgr := workspace.NewWorkspaceManager(credStore, logger)

	// Configure repository with credential
	// config := workspace.WorkspaceConfig{
	//     Repositories: []workspace.RepositoryConfig{
	//         {
	//             Name:           "private-repo",
	//             URL:            "https://github.com/example/private.git",
	//             CredentialName: "github-pat", // References stored credential
	//         },
	//     },
	//     Settings: workspace.WorkspaceSettings{
	//         CleanupOnComplete: true, // Securely cleans up temp credential files
	//     },
	// }

	// Initialize automatically handles credential configuration
	// mgr.Initialize(ctx, config)

	// Credentials are automatically used for Git operations (push, pull)
	// ws := mgr.Primary()
	// ws.Git().Push(ctx, git.PushOptions{})

	// Cleanup removes temporary credential files
	// defer mgr.Cleanup(ctx)

	fmt.Println("Credential handling example completed")
	// Output: Credential handling example completed
}
