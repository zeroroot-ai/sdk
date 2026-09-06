// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/zeroroot-ai/sdk/codegen"
	"github.com/zeroroot-ai/sdk/codegen/git"
	"github.com/zeroroot-ai/sdk/codegen/lsp"
	"github.com/zeroroot-ai/sdk/types"
)

// workspaceManager implements the WorkspaceManager interface.
type workspaceManager struct {
	config      WorkspaceConfig
	workspaces  map[string]*workspaceImpl
	mu          sync.RWMutex
	lspManagers map[string]lsp.LSPManager // Per-workspace LSP managers
	logger      *slog.Logger
	credStore   CredentialStore
	metrics     *codegen.CodegenMetrics
}

// CredentialStore provides access to stored credentials.
type CredentialStore interface {
	Get(name string) (*types.Credential, error)
}

// NewWorkspaceManager creates a new workspace manager with the given credential store.
func NewWorkspaceManager(credStore CredentialStore, logger *slog.Logger) WorkspaceManager {
	if logger == nil {
		logger = slog.Default()
	}

	// Initialize metrics (non-fatal if it fails)
	metrics, err := codegen.NewCodegenMetrics()
	if err != nil {
		logger.Warn("failed to initialize codegen metrics", "error", err)
		metrics = codegen.NoopCodegenMetrics()
	}

	return &workspaceManager{
		workspaces:  make(map[string]*workspaceImpl),
		lspManagers: make(map[string]lsp.LSPManager),
		logger:      logger.With("component", "workspace-manager"),
		credStore:   credStore,
		metrics:     metrics,
	}
}

// Initialize clones all repositories defined in the workspace configuration.
func (m *workspaceManager) Initialize(ctx context.Context, config WorkspaceConfig) error {
	ctx, span := codegen.StartSpan(ctx, "codegen.workspace.initialize")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config

	// Validate configuration
	if len(config.Repositories) == 0 {
		return errors.New("no repositories configured")
	}

	// Validate repository names are unique
	names := make(map[string]bool)
	for _, repo := range config.Repositories {
		if repo.Name == "" {
			return errors.New("repository name cannot be empty")
		}
		if names[repo.Name] {
			return fmt.Errorf("duplicate repository name: %s", repo.Name)
		}
		names[repo.Name] = true
	}

	// Determine base directory for clones
	baseDir := config.Settings.BaseDirectory
	if baseDir == "" {
		tmpDir, err := os.MkdirTemp("", "workspace-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary workspace directory: %w", err)
		}
		baseDir = tmpDir
		m.logger.Info("created temporary workspace directory", "path", baseDir)
	} else {
		// Ensure base directory exists
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create base directory: %w", err)
		}
	}

	// Update config with resolved base directory
	m.config.Settings.BaseDirectory = baseDir

	// Topologically sort repositories based on DependsOn
	sortedRepos, err := m.topologicalSort(config.Repositories)
	if err != nil {
		return fmt.Errorf("failed to resolve repository dependencies: %w", err)
	}

	// Clone repositories in dependency order
	for _, repo := range sortedRepos {
		if err := m.cloneRepository(ctx, repo); err != nil {
			span.SetStatus(codes.Error, "failed to clone repository "+repo.Name)
			span.RecordError(err)
			m.metrics.RecordError(ctx, m.metrics.WorkspaceOperationsTotal, "initialize")
			return fmt.Errorf("failed to clone repository %s: %w", repo.Name, err)
		}
	}

	// Initialize LSP servers if enabled
	if config.Settings.LSPEnabled {
		if err := m.initializeLSP(ctx); err != nil {
			// Log error but don't fail - LSP is optional
			m.logger.Warn("failed to initialize LSP servers", "error", err)
		}
	}

	span.SetStatus(codes.Ok, "workspace initialized")
	span.SetAttributes(attribute.Int("workspace.repository_count", len(sortedRepos)))
	m.metrics.RecordSuccess(ctx, m.metrics.WorkspaceOperationsTotal, "initialize")

	return nil
}

// topologicalSort sorts repositories based on DependsOn relationships.
func (m *workspaceManager) topologicalSort(repos []RepositoryConfig) ([]RepositoryConfig, error) {
	// Build dependency graph
	repoMap := make(map[string]*RepositoryConfig)
	for i := range repos {
		repoMap[repos[i].Name] = &repos[i]
	}

	// Validate dependencies exist
	for _, repo := range repos {
		for _, dep := range repo.DependsOn {
			if _, exists := repoMap[dep]; !exists {
				return nil, fmt.Errorf("repository %s depends on non-existent repository %s", repo.Name, dep)
			}
		}
	}

	// Kahn's algorithm for topological sort
	// inDegree tracks how many dependencies each repo has
	inDegree := make(map[string]int)
	for _, repo := range repos {
		inDegree[repo.Name] = len(repo.DependsOn)
	}

	// Find all nodes with no dependencies (inDegree = 0)
	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	sorted := make([]RepositoryConfig, 0, len(repos))
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]

		sorted = append(sorted, *repoMap[current])

		// Reduce inDegree for repos that depend on current
		// When a dependency is processed, decrement the inDegree of repos waiting on it
		for _, repo := range repos {
			for _, dep := range repo.DependsOn {
				if dep == current {
					inDegree[repo.Name]--
					if inDegree[repo.Name] == 0 {
						queue = append(queue, repo.Name)
					}
				}
			}
		}
	}

	// Check for cycles
	if len(sorted) != len(repos) {
		return nil, errors.New("circular dependency detected in repository dependencies")
	}

	return sorted, nil
}

// cloneRepository clones a single repository with credential handling.
func (m *workspaceManager) cloneRepository(ctx context.Context, repo RepositoryConfig) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.workspace.clone",
		attribute.String("repository.name", repo.Name),
		attribute.String("repository.url", codegen.SanitizeRepoURL(repo.URL)),
		attribute.String("repository.branch", repo.Branch),
		attribute.Bool("repository.shallow", repo.Shallow))
	defer span.End()

	m.logger.Info("cloning repository",
		"name", repo.Name,
		"url", sanitizeURL(repo.URL),
		"branch", repo.Branch,
		"shallow", repo.Shallow)

	// Determine clone destination
	destPath := filepath.Join(m.config.Settings.BaseDirectory, repo.Name)

	// Check if already cloned
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("repository directory already exists: %s", destPath)
	}

	// Get credentials if specified
	var credProvider git.CredentialProvider
	if repo.CredentialName != "" {
		if m.credStore == nil {
			return fmt.Errorf("credential store not configured but credential %s required", repo.CredentialName)
		}

		cred, err := m.credStore.Get(repo.CredentialName)
		if err != nil {
			return fmt.Errorf("failed to retrieve credential %s: %w", repo.CredentialName, err)
		}

		credProvider = git.NewCredentialProvider(cred)
		m.logger.Debug("retrieved credential for repository", "name", repo.Name, "credential", repo.CredentialName)
	}

	// Configure clone options
	cloneOpts := git.CloneOptions{
		Branch:       repo.Branch,
		SingleBranch: true, // Always use single branch for efficiency
	}

	if repo.Shallow {
		cloneOpts.Depth = 1
	}

	// Clone the repository
	if err := git.Clone(ctx, repo.URL, destPath, credProvider, cloneOpts); err != nil {
		span.SetStatus(codes.Error, "git clone failed")
		span.RecordError(err)
		return fmt.Errorf("git clone failed: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	m.logger.Info("successfully cloned repository", "name", repo.Name, "path", destPath)
	span.SetStatus(codes.Ok, "repository cloned")
	m.metrics.RecordDuration(ctx, m.metrics.WorkspaceCloneDurationSeconds, duration,
		attribute.String("repository.name", repo.Name))

	// Create workspace instance
	ws := &workspaceImpl{
		name:       repo.Name,
		path:       destPath,
		gitOps:     git.NewGitOps(destPath, credProvider),
		credProv:   credProvider,
		logger:     m.logger.With("workspace", repo.Name),
		isWorktree: false,
	}

	m.workspaces[repo.Name] = ws

	return nil
}

// initializeLSP starts LSP servers for all workspaces.
func (m *workspaceManager) initializeLSP(ctx context.Context) error {
	lspConfig := lsp.LSPConfig{
		InitTimeout:       m.config.Settings.LSPTimeout,
		ValidationTimeout: m.config.Settings.LSPTimeout,
		EnableGo:          true,
		EnablePython:      true,
		EnableTypeScript:  true,
	}

	var initErrors []error

	for name, ws := range m.workspaces {
		lspMgr := lsp.NewLSPManager(lspConfig, m.logger)

		if err := lspMgr.Start(ctx, ws.path); err != nil {
			initErrors = append(initErrors, fmt.Errorf("%s: %w", name, err))
			m.logger.Error("failed to start LSP for workspace", "workspace", name, "error", err)
			continue
		}

		// Wait for LSP to be ready
		if err := lspMgr.WaitForReady(ctx); err != nil {
			lspMgr.Stop(ctx)
			initErrors = append(initErrors, fmt.Errorf("%s: %w", name, err))
			m.logger.Error("LSP failed to become ready", "workspace", name, "error", err)
			continue
		}

		m.lspManagers[name] = lspMgr
		ws.lspManager = lspMgr
		m.logger.Info("initialized LSP for workspace",
			"workspace", name,
			"languages", lspMgr.SupportedLanguages())
	}

	if len(initErrors) > 0 && len(m.lspManagers) == 0 {
		return fmt.Errorf("all LSP initializations failed: %v", initErrors)
	}

	return nil
}

// Primary returns the default workspace for single-repository missions.
func (m *workspaceManager) Primary() Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.config.Repositories) == 0 {
		return nil
	}

	// Return first repository
	primaryName := m.config.Repositories[0].Name
	return m.workspaces[primaryName]
}

// Get returns the workspace for the specified repository name.
func (m *workspaceManager) Get(name string) (Workspace, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ws, ok := m.workspaces[name]
	return ws, ok
}

// All returns a map of all workspaces keyed by repository name.
func (m *workspaceManager) All() map[string]Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Workspace, len(m.workspaces))
	for name, ws := range m.workspaces {
		result[name] = ws
	}

	return result
}

// Cleanup removes all workspace directories and stops LSP servers.
func (m *workspaceManager) Cleanup(ctx context.Context) error {
	ctx, span := codegen.StartSpan(ctx, "codegen.workspace.cleanup")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	var cleanupErrors []error

	// Stop all LSP servers
	for name, lspMgr := range m.lspManagers {
		if err := lspMgr.Stop(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to stop LSP for %s: %w", name, err))
			m.logger.Error("failed to stop LSP server", "workspace", name, "error", err)
		}
	}
	m.lspManagers = make(map[string]lsp.LSPManager)

	// Close all workspaces
	for name, ws := range m.workspaces {
		if err := ws.Close(); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to close workspace %s: %w", name, err))
			m.logger.Error("failed to close workspace", "workspace", name, "error", err)
		}
	}

	// Remove workspace directories if configured
	if m.config.Settings.CleanupOnComplete && m.config.Settings.BaseDirectory != "" {
		if err := os.RemoveAll(m.config.Settings.BaseDirectory); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to remove workspace directory: %w", err))
			m.logger.Error("failed to remove workspace directory",
				"path", m.config.Settings.BaseDirectory,
				"error", err)
		} else {
			m.logger.Info("removed workspace directory", "path", m.config.Settings.BaseDirectory)
		}
	}

	m.workspaces = make(map[string]*workspaceImpl)

	if len(cleanupErrors) > 0 {
		span.SetStatus(codes.Error, "cleanup had errors")
		m.metrics.RecordError(ctx, m.metrics.WorkspaceOperationsTotal, "cleanup")
		return fmt.Errorf("cleanup errors occurred: %v", cleanupErrors)
	}

	span.SetStatus(codes.Ok, "workspace cleaned up")
	m.metrics.RecordSuccess(ctx, m.metrics.WorkspaceOperationsTotal, "cleanup")
	return nil
}

// workspaceImpl implements the Workspace interface.
type workspaceImpl struct {
	name       string
	path       string
	gitOps     git.GitOps
	credProv   git.CredentialProvider // Store credential provider for worktree creation
	editor     Editor
	lspManager lsp.LSPManager
	logger     *slog.Logger
	isWorktree bool
	mu         sync.RWMutex
}

// Name returns the repository identifier for this workspace.
func (w *workspaceImpl) Name() string {
	return w.name
}

// Path returns the absolute path to the workspace root directory.
func (w *workspaceImpl) Path() string {
	return w.path
}

// Editor returns the code editor for this workspace.
func (w *workspaceImpl) Editor() Editor {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.editor
}

// Git returns the Git operations interface for this workspace.
func (w *workspaceImpl) Git() GitOps {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.gitOps
}

// ReadFile reads a file from the workspace.
func (w *workspaceImpl) ReadFile(ctx context.Context, path string) ([]byte, error) {
	_, span := codegen.StartSpan(ctx, "codegen.workspace.read_file",
		attribute.String("workspace.name", w.name),
		attribute.String("file.path", codegen.SanitizeFilePath(path, w.path)))
	defer span.End()

	// Reject absolute paths
	if filepath.IsAbs(path) {
		span.SetStatus(codes.Error, "absolute path rejected")
		return nil, fmt.Errorf("path must be relative to workspace: %s", path)
	}

	absPath := filepath.Join(w.path, path)

	// Validate path is within workspace
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !isSubPath(w.path, absPath) {
		return nil, fmt.Errorf("path is outside workspace: %s", path)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		span.SetStatus(codes.Error, "read file failed")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	span.SetStatus(codes.Ok, "file read")
	span.SetAttributes(attribute.Int("file.size_bytes", len(data)))
	return data, nil
}

// WriteFile writes content to a file in the workspace.
func (w *workspaceImpl) WriteFile(ctx context.Context, path string, content []byte) error {
	_, span := codegen.StartSpan(ctx, "codegen.workspace.write_file",
		attribute.String("workspace.name", w.name),
		attribute.String("file.path", codegen.SanitizeFilePath(path, w.path)),
		attribute.Int("file.size_bytes", len(content)))
	defer span.End()

	// Reject absolute paths
	if filepath.IsAbs(path) {
		span.SetStatus(codes.Error, "absolute path rejected")
		return fmt.Errorf("path must be relative to workspace: %s", path)
	}

	absPath := filepath.Join(w.path, path)

	// Validate path is within workspace
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if !isSubPath(w.path, absPath) {
		return fmt.Errorf("path is outside workspace: %s", path)
	}

	// Create parent directories if needed
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Write file
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		span.SetStatus(codes.Error, "write file failed")
		span.RecordError(err)
		return fmt.Errorf("failed to write file: %w", err)
	}

	span.SetStatus(codes.Ok, "file written")
	return nil
}

// ListFiles returns all file paths matching the given glob pattern.
func (w *workspaceImpl) ListFiles(ctx context.Context, pattern string) ([]string, error) {
	_, span := codegen.StartSpan(ctx, "codegen.workspace.list_files",
		attribute.String("workspace.name", w.name),
		attribute.String("pattern", pattern))
	defer span.End()

	// Convert glob pattern to full path pattern
	fullPattern := filepath.Join(w.path, pattern)

	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	// Convert absolute paths back to relative paths
	relativePaths := make([]string, 0, len(matches))
	for _, match := range matches {
		relPath, err := filepath.Rel(w.path, match)
		if err != nil {
			w.logger.Warn("failed to convert path to relative", "path", match, "error", err)
			continue
		}

		// Filter out directories (only return files)
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}

		relativePaths = append(relativePaths, relPath)
	}

	span.SetStatus(codes.Ok, "files listed")
	span.SetAttributes(attribute.Int("file.count", len(relativePaths)))
	return relativePaths, nil
}

// Commit stages all tracked and untracked changes then creates a commit.
// It calls Add(ctx, ".") to stage everything, then Commit(ctx, message) to
// persist. Returns the commit SHA on success, or an error if either operation
// fails. If Add fails, Commit is not attempted.
func (w *workspaceImpl) Commit(ctx context.Context, message string) (string, error) {
	if err := w.gitOps.Add(ctx, "."); err != nil {
		return "", fmt.Errorf("git add failed: %w", err)
	}
	sha, err := w.gitOps.Commit(ctx, message, git.CommitOptions{})
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}
	return sha, nil
}

// Push pushes committed changes to the default remote (origin).
func (w *workspaceImpl) Push(ctx context.Context) error {
	if err := w.gitOps.Push(ctx, git.PushOptions{}); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

// Close releases resources associated with this workspace.
func (w *workspaceImpl) Close() error {
	// For worktrees, remove the worktree directory
	// For regular clones, this is a no-op (cleanup is handled by manager)
	if w.isWorktree {
		// Worktree cleanup is handled in worktree.go
		return nil
	}

	return nil
}

// isSubPath checks if child is a subdirectory of parent.
func isSubPath(parent, child string) bool {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}

	child, err = filepath.Abs(child)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the parent
	return !filepath.IsAbs(relPath) && !filepath.HasPrefix(relPath, "..")
}

// sanitizeURL removes credentials from URLs for safe logging.
func sanitizeURL(url string) string {
	// Remove credentials from HTTPS URLs
	if len(url) > 8 && url[:8] == "https://" {
		url = url[8:]
		if atIdx := findChar(url, '@'); atIdx != -1 {
			return "https://***:***" + url[atIdx:]
		}
		return "https://" + url
	}

	// SSH URLs don't typically contain credentials
	return url
}

// findChar finds the first occurrence of a character in a string.
func findChar(s string, ch rune) int {
	for i, c := range s {
		if c == ch {
			return i
		}
	}
	return -1
}
