// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/zeroroot-ai/sdk/codegen"
)

// gitOps implements the GitOps interface using the git binary via exec.Command.
type gitOps struct {
	// repoPath is the absolute path to the Git repository
	repoPath string

	// credential is optional credential for remote operations
	credential CredentialProvider

	// metrics for observability
	metrics *codegen.CodegenMetrics
}

// CredentialProvider provides credentials for Git operations.
// This is implemented in credentials.go.
type CredentialProvider interface {
	ConfigureAuth(ctx context.Context, repoPath string) (cleanup func(), err error)
}

// NewGitOps creates a new GitOps instance for the given repository path.
// The credential provider is optional and only required for remote operations
// (clone, push, pull) that require authentication.
func NewGitOps(repoPath string, credential CredentialProvider) GitOps {
	// Initialize metrics (non-fatal if it fails)
	metrics, err := codegen.NewCodegenMetrics()
	if err != nil {
		metrics = codegen.NoopCodegenMetrics()
	}

	return &gitOps{
		repoPath:   repoPath,
		credential: credential,
		metrics:    metrics,
	}
}

// execGit executes a git command in the repository directory.
// It sanitizes the command to prevent injection attacks.
func (g *gitOps) execGit(ctx context.Context, args ...string) (string, error) {
	// Sanitize arguments to prevent command injection
	for _, arg := range args {
		if err := validateGitArg(arg); err != nil {
			return "", fmt.Errorf("invalid git argument: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}

// validateGitArg validates a git argument to prevent command injection.
// It rejects arguments containing shell metacharacters and dangerous patterns.
// Note: We allow < and > for email addresses in author strings.
func validateGitArg(arg string) error {
	// Allow empty arguments
	if arg == "" {
		return nil
	}

	// Reject shell metacharacters that could be used for injection
	// Note: < and > are allowed for email addresses (e.g., "Name <email@example.com>")
	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "\n", "\r"}
	for _, char := range dangerous {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains dangerous character: %s", char)
		}
	}

	return nil
}

// CurrentBranch returns the name of the currently checked out branch.
func (g *gitOps) CurrentBranch() (string, error) {
	output, err := g.execGit(context.Background(), "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if HEAD is detached
	if output == "HEAD" {
		return "", errors.New("HEAD is detached")
	}

	return output, nil
}

// Status returns the current repository status.
func (g *gitOps) Status() (*GitStatus, error) {
	status := &GitStatus{}

	// Get current branch
	branch, err := g.CurrentBranch()
	if err != nil && !strings.Contains(err.Error(), "HEAD is detached") {
		return nil, err
	}
	status.Branch = branch

	// Get current commit
	commit, err := g.execGit(context.Background(), "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	status.Commit = commit

	// Get porcelain status for file changes
	output, err := g.execGit(context.Background(), "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Parse porcelain output
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		if len(line) < 4 {
			continue
		}

		statusCode := line[0:2]
		filePath := strings.TrimSpace(line[3:])

		switch {
		case statusCode == "??":
			status.Untracked = append(status.Untracked, filePath)
		case statusCode[0] != ' ' && statusCode[0] != '?':
			status.Staged = append(status.Staged, filePath)
		case statusCode[1] != ' ' && statusCode[1] != '?':
			status.Unstaged = append(status.Unstaged, filePath)
		}

		// Check for conflicts
		if strings.Contains(statusCode, "U") || strings.Contains(statusCode, "A") && strings.Contains(statusCode, "A") {
			status.HasConflicts = true
		}
	}

	// Get ahead/behind counts if branch exists
	if status.Branch != "" {
		ahead, behind, err := g.getAheadBehind(status.Branch)
		if err == nil {
			status.Ahead = ahead
			status.Behind = behind
		}
		// Ignore errors here as the branch might not have an upstream
	}

	return status, nil
}

// getAheadBehind returns the number of commits ahead and behind the upstream.
func (g *gitOps) getAheadBehind(branch string) (int, int, error) {
	output, err := g.execGit(context.Background(), "rev-list", "--left-right", "--count", branch+"...@{u}")
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(output)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %s", output)
	}

	ahead, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse ahead count: %w", err)
	}

	behind, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse behind count: %w", err)
	}

	return ahead, behind, nil
}

// CreateBranch creates a new branch with the given name from the current HEAD.
func (g *gitOps) CreateBranch(ctx context.Context, name string) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.create_branch",
		attribute.String("branch.name", name))
	defer span.End()

	// Validate branch name
	if name == "" {
		span.SetStatus(codes.Error, "empty branch name")
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "create_branch")
		return errors.New("branch name cannot be empty")
	}

	// Create the branch
	_, err := g.execGit(ctx, "branch", name)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "create branch failed")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "create_branch")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "create_branch"))
		return fmt.Errorf("failed to create branch: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "branch created")
	g.metrics.RecordSuccess(ctx, g.metrics.GitOperationsTotal, "create_branch")
	g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "create_branch"))
	return nil
}

// Checkout switches to the specified branch or commit ref.
func (g *gitOps) Checkout(ctx context.Context, ref string) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.checkout",
		attribute.String("ref", ref))
	defer span.End()

	if ref == "" {
		span.SetStatus(codes.Error, "empty ref")
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "checkout")
		return errors.New("ref cannot be empty")
	}

	_, err := g.execGit(ctx, "checkout", ref)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "checkout failed")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "checkout")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "checkout"))
		return fmt.Errorf("failed to checkout %s: %w", ref, err)
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "checkout complete")
	g.metrics.RecordSuccess(ctx, g.metrics.GitOperationsTotal, "checkout")
	g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "checkout"))
	return nil
}

// Add stages the specified paths for commit.
func (g *gitOps) Add(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths specified")
	}

	args := append([]string{"add", "--"}, paths...)
	_, err := g.execGit(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to add paths: %w", err)
	}

	return nil
}

// Commit creates a new commit with the given message and options.
func (g *gitOps) Commit(ctx context.Context, message string, opts CommitOptions) (string, error) {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.commit")
	defer span.End()

	if message == "" {
		span.SetStatus(codes.Error, "empty commit message")
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "commit")
		return "", errors.New("commit message cannot be empty")
	}

	args := []string{"commit", "-m", message}

	// Add author if specified
	if opts.Author != "" {
		args = append(args, "--author", opts.Author)
	}

	// Allow empty commits if requested
	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}

	// Amend previous commit if requested
	if opts.Amend {
		args = append(args, "--amend")
	}

	// Set timestamp if specified
	if !opts.Timestamp.IsZero() {
		dateStr := opts.Timestamp.Format(time.RFC3339)
		args = append(args, "--date", dateStr)
	}

	_, err := g.execGit(ctx, args...)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "commit failed")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "commit")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "commit"))
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	// Get the commit SHA
	sha, err := g.execGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "failed to get commit SHA")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "commit")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "commit"))
		return "", fmt.Errorf("failed to get commit SHA: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "commit created")
	span.SetAttributes(attribute.String("commit.sha", sha))
	g.metrics.RecordSuccess(ctx, g.metrics.GitOperationsTotal, "commit")
	g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "commit"))
	return sha, nil
}

// Push pushes commits to the remote repository.
func (g *gitOps) Push(ctx context.Context, opts PushOptions) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.push",
		attribute.String("remote", opts.Remote),
		attribute.Bool("force", opts.Force))
	defer span.End()

	// Configure authentication if credential provider is available
	var cleanup func()
	if g.credential != nil {
		var err error
		cleanup, err = g.credential.ConfigureAuth(ctx, g.repoPath)
		if err != nil {
			span.SetStatus(codes.Error, "auth configuration failed")
			span.RecordError(err)
			g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "push")
			return fmt.Errorf("failed to configure authentication: %w", err)
		}
		defer cleanup()
	}

	// Default to origin if no remote specified
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}

	args := []string{"push"}

	// Add force flag if requested (WARNING: use with caution)
	if opts.Force {
		args = append(args, "--force")
	}

	// Set upstream if requested
	if opts.SetUpstream {
		args = append(args, "--set-upstream")
	}

	// Push tags if requested
	if opts.Tags {
		args = append(args, "--tags")
	}

	// Add remote
	args = append(args, remote)

	// Add refspec if specified, otherwise push current branch
	if opts.RefSpec != "" {
		args = append(args, opts.RefSpec)
	}

	_, err := g.execGit(ctx, args...)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "push failed")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "push")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "push"))
		// Check if the error is due to diverged branches
		if strings.Contains(err.Error(), "rejected") && strings.Contains(err.Error(), "non-fast-forward") {
			return fmt.Errorf("remote has diverged, pull required: %w", err)
		}
		return fmt.Errorf("failed to push: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "push complete")
	g.metrics.RecordSuccess(ctx, g.metrics.GitOperationsTotal, "push")
	g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "push"))
	return nil
}

// Pull fetches and merges changes from the remote tracking branch.
func (g *gitOps) Pull(ctx context.Context) error {
	// Configure authentication if credential provider is available
	var cleanup func()
	if g.credential != nil {
		var err error
		cleanup, err = g.credential.ConfigureAuth(ctx, g.repoPath)
		if err != nil {
			return fmt.Errorf("failed to configure authentication: %w", err)
		}
		defer cleanup()
	}

	_, err := g.execGit(ctx, "pull")
	if err != nil {
		// Check for merge conflicts
		if strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "CONFLICT") {
			return fmt.Errorf("merge conflicts detected: %w", err)
		}
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}

// Clone clones a repository to the specified destination path.
// This is a standalone function since it doesn't require an existing repository.
func Clone(ctx context.Context, url, destPath string, credential CredentialProvider, opts CloneOptions) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.clone",
		attribute.String("repository.url", codegen.SanitizeRepoURL(url)),
		attribute.String("branch", opts.Branch),
		attribute.Int("depth", opts.Depth))
	defer span.End()

	// Initialize metrics for the standalone function
	metrics, err := codegen.NewCodegenMetrics()
	if err != nil {
		metrics = codegen.NoopCodegenMetrics()
	}

	if url == "" {
		span.SetStatus(codes.Error, "empty URL")
		metrics.RecordError(ctx, metrics.GitOperationsTotal, "clone")
		return errors.New("repository URL cannot be empty")
	}
	if destPath == "" {
		span.SetStatus(codes.Error, "empty destination path")
		metrics.RecordError(ctx, metrics.GitOperationsTotal, "clone")
		return errors.New("destination path cannot be empty")
	}

	// Validate URL format
	if err := validateRepoURL(url); err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(destPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Configure authentication if credential provider is available
	var cleanup func()
	if credential != nil {
		// For clone, we need to configure auth before the repo exists
		// The credential provider will handle this appropriately
		var err error
		cleanup, err = credential.ConfigureAuth(ctx, destPath)
		if err != nil {
			return fmt.Errorf("failed to configure authentication: %w", err)
		}
		defer cleanup()
	}

	args := []string{"clone"}

	// Add depth for shallow clone
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}

	// Add branch
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}

	// Single branch clone optimization
	if opts.SingleBranch {
		args = append(args, "--single-branch")
	}

	// Add URL and destination
	args = append(args, url, destPath)

	// Execute clone command
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "git clone failed")
		span.RecordError(err)
		metrics.RecordError(ctx, metrics.GitOperationsTotal, "clone")
		metrics.RecordDuration(ctx, metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "clone"))
		return fmt.Errorf("git clone failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "clone complete")
	metrics.RecordSuccess(ctx, metrics.GitOperationsTotal, "clone")
	metrics.RecordDuration(ctx, metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "clone"))
	return nil
}

// CloneOptions configures clone behavior.
type CloneOptions struct {
	// Depth specifies the number of commits to fetch (shallow clone).
	// Use 0 for full clone.
	Depth int

	// Branch specifies the branch to checkout after cloning.
	// If empty, the default branch will be used.
	Branch string

	// SingleBranch fetches only the specified branch (or default branch).
	// This reduces clone size and time.
	SingleBranch bool
}

// validateRepoURL validates that a repository URL has a safe format.
func validateRepoURL(url string) error {
	// Must be HTTPS, SSH, or file format (file:// or local path for testing)
	httpsPattern := regexp.MustCompile(`^https?://[a-zA-Z0-9\-\.]+/[a-zA-Z0-9\-_./]+$`)
	sshPattern := regexp.MustCompile(`^git@[a-zA-Z0-9\-\.]+:[a-zA-Z0-9\-_./]+$`)
	filePattern := regexp.MustCompile(`^file://[a-zA-Z0-9\-_./]+$`)

	// Also allow absolute local paths (for testing/local clones)
	isLocalPath := filepath.IsAbs(url)

	if !httpsPattern.MatchString(url) && !sshPattern.MatchString(url) && !filePattern.MatchString(url) && !isLocalPath {
		return errors.New("URL must be in HTTPS, SSH, file://, or absolute local path format")
	}

	return nil
}
