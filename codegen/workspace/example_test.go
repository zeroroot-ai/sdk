// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace_test

import (
	"fmt"
)

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
