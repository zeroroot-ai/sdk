// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package git provides Git operations for the CodeGen SDK.
//
// This package implements Git operations including branching, committing,
// pushing, and snapshot/rollback functionality. The snapshot and rollback
// operations are designed to NOT pollute Git history - they do not create
// commits or visible refs in the repository.
//
// # Overview
//
// The git package provides:
//   - Branch management (create, checkout, list)
//   - Staging and committing changes
//   - Pushing to remote repositories
//   - Repository status inspection
//   - Snapshot creation for rollback
//   - Clean rollback without Git history pollution
//
// # Basic Usage
//
// Get Git Operations:
//
//	git := workspace.Git()
//
// Check Repository Status:
//
//	status, err := git.Status()
//	if err != nil {
//	    return err
//	}
//
//	log.Printf("Branch: %s", status.Branch)
//	log.Printf("Commit: %s", status.Commit)
//	log.Printf("Staged files: %v", status.Staged)
//	log.Printf("Unstaged files: %v", status.Unstaged)
//	log.Printf("Untracked files: %v", status.Untracked)
//	log.Printf("Ahead: %d, Behind: %d", status.Ahead, status.Behind)
//
// Create and Switch Branch:
//
//	git := workspace.Git()
//
//	// Create a new branch from current HEAD
//	branchName := "feature/fix-vulnerability"
//	if err := git.CreateBranch(ctx, branchName); err != nil {
//	    return fmt.Errorf("failed to create branch: %w", err)
//	}
//
//	// Switch to the new branch
//	if err := git.Checkout(ctx, branchName); err != nil {
//	    return fmt.Errorf("failed to checkout branch: %w", err)
//	}
//
//	// Make changes, commit, and push...
//
// Stage and Commit Changes:
//
//	git := workspace.Git()
//
//	// Stage specific files
//	if err := git.Add(ctx, "main.go", "config.yaml"); err != nil {
//	    return err
//	}
//
//	// Or stage all changes
//	if err := git.Add(ctx, "."); err != nil {
//	    return err
//	}
//
//	// Commit with options
//	commitSHA, err := git.Commit(ctx, "Fix security vulnerability", git.CommitOptions{
//	    Author: "Security Agent <security@example.com>",
//	})
//	if err != nil {
//	    return err
//	}
//
//	log.Printf("Created commit: %s", commitSHA)
//
// Push Changes:
//
//	git := workspace.Git()
//
//	// Push to origin with upstream tracking
//	err := git.Push(ctx, git.PushOptions{
//	    SetUpstream: true,
//	})
//	if err != nil {
//	    return err
//	}
//
// # Snapshot and Rollback
//
// One of the key features of this package is clean snapshot/rollback
// that doesn't pollute Git history:
//
// Create Snapshot:
//
//	git := workspace.Git()
//
//	// Create a snapshot of current working directory state
//	snapshotID, err := git.Snapshot(ctx)
//	if err != nil {
//	    return err
//	}
//
//	log.Printf("Created snapshot: %s", snapshotID)
//
//	// The snapshot includes:
//	//   - All tracked files (current content)
//	//   - Staged changes
//	//   - Unstaged changes
//	//   - Untracked files
//
//	// Make risky changes...
//
// Rollback to Snapshot:
//
//	git := workspace.Git()
//
//	// Restore working directory to snapshot state
//	if err := git.Rollback(ctx, snapshotID); err != nil {
//	    return err
//	}
//
//	log.Printf("Rolled back to snapshot: %s", snapshotID)
//
//	// The rollback:
//	//   - Restores all tracked files
//	//   - Restores untracked files that existed in snapshot
//	//   - Removes files that weren't in snapshot
//	//   - Does NOT create commits
//	//   - Does NOT create refs/branches/tags
//	//   - Does NOT affect Git history
//
// Automatic Snapshot/Rollback:
//
//	The editor automatically uses snapshots:
//
//	editor := workspace.Editor()
//
//	// Editor creates snapshot before edit
//	result, err := editor.Apply(ctx, edit)
//
//	// If validation fails, editor rolls back automatically
//	if !result.Applied {
//	    log.Println("Edit was rolled back due to validation errors")
//	}
//
//	// You can also rollback manually using the snapshot ID
//	snapshotID := result.Snapshot
//	if needsRollback {
//	    git.Rollback(ctx, snapshotID)
//	}
//
// # Design: Clean Snapshots
//
// The snapshot implementation is designed to be invisible in Git:
//
// What Snapshots Are NOT:
//
//   - NOT commits (no SHA in git log)
//   - NOT branches (no refs/heads/)
//   - NOT tags (no refs/tags/)
//   - NOT stashes (no refs/stash)
//   - NOT visible in git reflog
//   - NOT pushed to remote
//   - NOT affecting git history at all
//
// How Snapshots Work:
//
//  1. Create temporary archive of working directory
//
//  2. Store archive in .git/gibson-snapshots/
//
//  3. Generate unique snapshot ID
//
//  4. Return ID to caller
//
//     Rollback:
//
//  1. Extract archive to temporary location
//
//  2. Replace working directory with extracted files
//
//  3. Update Git index if needed
//
//  4. Delete temporary files
//
// Benefits:
//
//   - No Git history pollution
//   - Fast creation and restoration
//   - Works with any Git state (dirty, clean, conflicted)
//   - No risk of ref conflicts
//   - Safe for concurrent use
//
// # Branch Management
//
// Create Branch:
//
//	git := workspace.Git()
//
//	// Create branch from current HEAD
//	err := git.CreateBranch(ctx, "feature/new-feature")
//	if err != nil {
//	    // Branch may already exist
//	    return err
//	}
//
// Checkout Branch or Commit:
//
//	git := workspace.Git()
//
//	// Checkout existing branch
//	err := git.Checkout(ctx, "main")
//
//	// Checkout tag
//	err = git.Checkout(ctx, "v1.2.3")
//
//	// Checkout specific commit
//	err = git.Checkout(ctx, "abc123def")
//
//	// Checkout remote branch
//	err = git.Checkout(ctx, "origin/feature-branch")
//
// Get Current Branch:
//
//	git := workspace.Git()
//
//	branch, err := git.CurrentBranch()
//	if err != nil {
//	    // May be in detached HEAD state
//	    return err
//	}
//
//	log.Printf("Current branch: %s", branch)
//
// # Commit Options
//
// Basic Commit:
//
//	commitSHA, err := git.Commit(ctx, "Update configuration", git.CommitOptions{})
//
// Custom Author:
//
//	commitSHA, err := git.Commit(ctx, "Fix bug", git.CommitOptions{
//	    Author: "Agent Name <agent@example.com>",
//	})
//
// Allow Empty Commit:
//
//	// Useful for triggering CI/CD without changes
//	commitSHA, err := git.Commit(ctx, "Trigger build", git.CommitOptions{
//	    AllowEmpty: true,
//	})
//
// Custom Timestamp:
//
//	commitSHA, err := git.Commit(ctx, "Historical change", git.CommitOptions{
//	    Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
//	})
//
// Amend Previous Commit:
//
//	// Modify the last commit
//	commitSHA, err := git.Commit(ctx, "Updated message", git.CommitOptions{
//	    Amend: true,
//	})
//
//	Note: Use amend with caution, especially if commit was already pushed.
//
// # Push Options
//
// Basic Push:
//
//	err := git.Push(ctx, git.PushOptions{})
//
// Set Upstream Tracking:
//
//	// First push of a new branch
//	err := git.Push(ctx, git.PushOptions{
//	    SetUpstream: true, // Same as git push -u
//	})
//
// Push to Different Remote:
//
//	err := git.Push(ctx, git.PushOptions{
//	    Remote: "backup",
//	})
//
// Push Specific RefSpec:
//
//	err := git.Push(ctx, git.PushOptions{
//	    RefSpec: "refs/heads/feature:refs/heads/feature",
//	})
//
// Push Tags:
//
//	err := git.Push(ctx, git.PushOptions{
//	    Tags: true, // Push all tags
//	})
//
// Force Push (Use with Caution):
//
//	// DANGER: Overwrites remote history
//	err := git.Push(ctx, git.PushOptions{
//	    Force: true,
//	})
//
//	Note: Force push should be avoided. The system never force-pushes
//	      automatically to prevent data loss.
//
// # Repository Status
//
// The Status() method returns detailed repository state:
//
//	type GitStatus struct {
//	    Branch       string   // Current branch name
//	    Commit       string   // Current HEAD commit SHA
//	    Staged       []string // Files staged for commit
//	    Unstaged     []string // Modified but not staged
//	    Untracked    []string // Files not tracked by Git
//	    Ahead        int      // Commits ahead of upstream
//	    Behind       int      // Commits behind upstream
//	    HasConflicts bool     // Unresolved merge conflicts
//	}
//
// Check for Changes:
//
//	status, err := git.Status()
//	if err != nil {
//	    return err
//	}
//
//	hasChanges := len(status.Staged) > 0 ||
//	              len(status.Unstaged) > 0 ||
//	              len(status.Untracked) > 0
//
//	if hasChanges {
//	    log.Println("Working directory has changes")
//	}
//
// Check Sync Status:
//
//	status, err := git.Status()
//	if err != nil {
//	    return err
//	}
//
//	if status.Ahead > 0 {
//	    log.Printf("Local branch is %d commits ahead of remote", status.Ahead)
//	}
//
//	if status.Behind > 0 {
//	    log.Printf("Local branch is %d commits behind remote", status.Behind)
//	    log.Println("Consider pulling changes")
//	}
//
// Check for Conflicts:
//
//	status, err := git.Status()
//	if err != nil {
//	    return err
//	}
//
//	if status.HasConflicts {
//	    log.Println("Repository has unresolved merge conflicts")
//	    log.Println("Manual intervention required")
//	    return errors.New("conflicts must be resolved")
//	}
//
// # Error Handling
//
// Common errors and solutions:
//
// Push Conflict (ErrPushConflict):
//
//	Causes:
//	  - Remote has new commits
//	  - Someone else pushed while you were working
//	  - Force push from another source
//
//	Solutions:
//	  - Pull remote changes first: git.Pull(ctx)
//	  - Resolve conflicts if any
//	  - Push again
//
//	Never use Force: true unless absolutely necessary.
//
// Credential Issues:
//
//	Causes:
//	  - Invalid authentication token
//	  - SSH key not configured
//	  - Insufficient permissions
//
//	Solutions:
//	  - Verify credential in credential store
//	  - Check repository access permissions
//	  - Ensure credential has push access
//
// Detached HEAD:
//
//	Causes:
//	  - Checked out a specific commit
//	  - Checked out a tag
//	  - Rebase in progress
//
//	Solutions:
//	  - Create a branch: git.CreateBranch(ctx, "temp-branch")
//	  - Checkout a branch: git.Checkout(ctx, "main")
//	  - Check git status for guidance
//
// # Best Practices
//
// Branch Naming:
//
//	Use descriptive, hierarchical names:
//	  - feature/add-authentication
//	  - fix/sql-injection-cve-2024-1234
//	  - refactor/database-layer
//	  - hotfix/security-patch
//
// Commit Messages:
//
//	Follow conventional commit format:
//	  - "feat: Add user authentication"
//	  - "fix: Prevent SQL injection in search"
//	  - "refactor: Extract database logic"
//	  - "docs: Update API documentation"
//
// Staging Changes:
//
//	Stage related changes together:
//	  - git.Add(ctx, "auth.go", "auth_test.go")
//	  - Avoid git.Add(ctx, ".") with unrelated changes
//	  - Review staged files before committing
//
// Pushing Changes:
//
//	Always check status before pushing:
//	  status, _ := git.Status()
//	  if status.Behind > 0 {
//	      // Pull first to avoid conflicts
//	      git.Pull(ctx)
//	  }
//	  git.Push(ctx, git.PushOptions{})
//
// Using Snapshots:
//
//	Create snapshots before risky operations:
//	  snapshot, _ := git.Snapshot(ctx)
//	  defer func() {
//	      if needsRollback {
//	          git.Rollback(ctx, snapshot)
//	      }
//	  }()
//
//	  // Risky operation...
//
// # Mission Examples
//
// Feature Development Mission:
//
//	git := workspace.Git()
//	editor := workspace.Editor()
//
//	// Create feature branch
//	branchName := "feature/add-rate-limiting"
//	if err := git.CreateBranch(ctx, branchName); err != nil {
//	    return err
//	}
//	if err := git.Checkout(ctx, branchName); err != nil {
//	    return err
//	}
//
//	// Make changes
//	edit := editor.Edit{
//	    FilePath:     "server.go",
//	    SearchBlock:  "old implementation",
//	    ReplaceBlock: "new implementation with rate limiting",
//	}
//	result, err := editor.Apply(ctx, edit)
//	if err != nil || !result.Applied {
//	    return errors.New("edit failed")
//	}
//
//	// Commit changes
//	if err := git.Add(ctx, "server.go"); err != nil {
//	    return err
//	}
//	commitSHA, err := git.Commit(ctx, "feat: Add rate limiting to API", git.CommitOptions{
//	    Author: "Agent <agent@example.com>",
//	})
//	if err != nil {
//	    return err
//	}
//
//	// Push to remote
//	err = git.Push(ctx, git.PushOptions{
//	    SetUpstream: true,
//	})
//	if err != nil {
//	    return err
//	}
//
//	log.Printf("Feature pushed: %s", commitSHA)
//
// Hotfix Mission:
//
//	git := workspace.Git()
//	editor := workspace.Editor()
//
//	// Checkout main branch
//	if err := git.Checkout(ctx, "main"); err != nil {
//	    return err
//	}
//
//	// Create hotfix branch
//	if err := git.CreateBranch(ctx, "hotfix/security-patch"); err != nil {
//	    return err
//	}
//	if err := git.Checkout(ctx, "hotfix/security-patch"); err != nil {
//	    return err
//	}
//
//	// Apply security fix
//	edit := editor.Edit{
//	    FilePath:     "auth.go",
//	    SearchBlock:  "vulnerable code",
//	    ReplaceBlock: "secure code",
//	}
//	result, err := editor.Apply(ctx, edit)
//	if err != nil || !result.Applied {
//	    return errors.New("security fix failed")
//	}
//
//	// Commit and push immediately
//	git.Add(ctx, "auth.go")
//	commitSHA, err := git.Commit(ctx, "fix: Patch authentication vulnerability", git.CommitOptions{})
//	if err != nil {
//	    return err
//	}
//
//	err = git.Push(ctx, git.PushOptions{SetUpstream: true})
//	if err != nil {
//	    return err
//	}
//
//	log.Printf("Hotfix deployed: %s", commitSHA)
//
// Experimental Changes with Rollback:
//
//	git := workspace.Git()
//	editor := workspace.Editor()
//
//	// Create snapshot before experimental changes
//	snapshot, err := git.Snapshot(ctx)
//	if err != nil {
//	    return err
//	}
//
//	// Try experimental changes
//	edit := editor.Edit{
//	    FilePath:     "algorithm.go",
//	    SearchBlock:  "old algorithm",
//	    ReplaceBlock: "experimental algorithm",
//	}
//	result, err := editor.Apply(ctx, edit)
//
//	// Run tests or validation...
//	testsPass := runTests()
//
//	if !testsPass {
//	    // Rollback if experiment failed
//	    log.Println("Experiment failed, rolling back")
//	    if err := git.Rollback(ctx, snapshot); err != nil {
//	        return err
//	    }
//	    return errors.New("experimental changes did not pass tests")
//	}
//
//	// Commit if successful
//	git.Add(ctx, "algorithm.go")
//	git.Commit(ctx, "feat: Improve algorithm performance", git.CommitOptions{})
//
// # Thread Safety
//
// Git operations are not safe for concurrent use on the same repository.
// Concurrent operations on different repositories are safe:
//
//	// Safe: Different workspaces
//	go workspace1.Git().Commit(ctx, "message", git.CommitOptions{})
//	go workspace2.Git().Commit(ctx, "message", git.CommitOptions{})
//
//	// Unsafe: Same workspace
//	go workspace.Git().Commit(ctx, "message1", git.CommitOptions{})
//	go workspace.Git().Commit(ctx, "message2", git.CommitOptions{})
//
// Use proper synchronization if multiple agents work on the same repository:
//
//	var mu sync.Mutex
//
//	mu.Lock()
//	git.Commit(ctx, "message", git.CommitOptions{})
//	mu.Unlock()
//
// Or use worktrees for true isolation (recommended):
//
//	# In mission YAML:
//	workspace:
//	  settings:
//	    use_worktrees: true  # Each agent gets own working directory
//
// # Credential Management
//
// Git operations use credentials from the workspace configuration:
//
//	# In mission YAML:
//	workspace:
//	  repositories:
//	    - name: app
//	      url: https://github.com/org/app.git
//	      credential: github-token  # References stored credential
//
// HTTPS with Token:
//
//	# Credential store:
//	name: github-token
//	type: token
//	value: ghp_xxxxxxxxxxxxx
//
//	Automatically used for clone, fetch, push operations.
//
// SSH with Key:
//
//	# Credential store:
//	name: github-ssh
//	type: ssh_key
//	value: |
//	  -----BEGIN OPENSSH PRIVATE KEY-----
//	  ...
//	  -----END OPENSSH PRIVATE KEY-----
//
//	Automatically configured in SSH agent for Git operations.
//
// # Examples
//
// See integration_test.go for complete examples:
//   - TestFullMission: Complete Git mission from clone to push
//   - TestWorktreeIsolation: Using Git worktrees for isolation
//   - TestMultiRepoScenario: Working with multiple repositories
//   - TestLSPValidationIntegration: Snapshot/rollback with LSP validation
package git
