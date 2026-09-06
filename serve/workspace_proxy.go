// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Workspace proxy for out-of-process callback agents.
//
// callbackWorkspace satisfies workspace.Workspace by translating each
// interface method into a HarnessCallbackService.Workspace* gRPC call.
// The daemon owns the cloned repository on disk; this proxy carries no
// state beyond the gRPC client and the workspace name.
//
// v1 surface: file IO + commit + push. The Editor (SEARCH/REPLACE +
// LSP) and full GitOps (branching, snapshots) interfaces from
// codegen/workspace are intentionally NOT proxied — Editor() returns an
// errEditor whose every method returns ErrWorkspaceNotImplemented; Git()
// returns a partialGitOps that proxies Commit and Push and errors on
// the other GitOps methods. A follow-on spec will add streaming-aware
// editor + full git operations once the wire contract for LSP
// diagnostics is settled.
//
// Spec: callback-harness-workspace-rpcs.

package serve

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/codegen"
	"github.com/zeroroot-ai/sdk/codegen/editor"
	"github.com/zeroroot-ai/sdk/codegen/git"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
)

// ErrWorkspaceNotImplemented is returned by editor and full-GitOps
// methods on the callback proxy. The v1 callback Workspace surface
// covers file IO + commit + push only.
var ErrWorkspaceNotImplemented = errors.New(
	"workspace: editor / full GitOps not implemented on the callback path; " +
		"v1 surface is ReadFile/WriteFile/ListFiles/Commit/Push only",
)

// ErrWorkspaceNotConfigured is returned by the harness when a callback
// agent calls Workspace() / Workspaces() against a mission whose YAML
// configures zero repositories (the daemon answers FailedPrecondition).
var ErrWorkspaceNotConfigured = errors.New(
	"workspace: this mission has no workspace configured",
)

// callbackWorkspace satisfies workspace.Workspace for an out-of-process
// callback agent. Methods translate to the corresponding Workspace*
// callback RPC.
type callbackWorkspace struct {
	client *CallbackClient
	name   string
	path   string
}

// newCallbackWorkspace returns a proxy for a single workspace. The
// `name` corresponds to RepositoryConfig.Name; an empty name means
// "primary workspace" (single-repo missions).
func newCallbackWorkspace(client *CallbackClient, info *harnesspb.WorkspaceInfo) *callbackWorkspace {
	if info == nil {
		return nil
	}
	return &callbackWorkspace{
		client: client,
		name:   info.GetName(),
		path:   info.GetPath(),
	}
}

// Name returns the repository identifier for this workspace.
func (w *callbackWorkspace) Name() string { return w.name }

// Path returns the absolute path on the daemon. The callback agent
// does not access this path directly — file IO goes through the gRPC
// proxy.
func (w *callbackWorkspace) Path() string { return w.path }

// Editor returns a stub editor whose every method returns
// ErrWorkspaceNotImplemented. v2 spec will add a streaming-aware
// proxy for the SEARCH/REPLACE + LSP-validated edit surface.
func (w *callbackWorkspace) Editor() editor.Editor { return &errEditor{} }

// Git returns a partial GitOps that proxies Commit and Push through
// WorkspaceCommit / WorkspacePush; every other method returns
// ErrWorkspaceNotImplemented.
func (w *callbackWorkspace) Git() git.GitOps {
	return &partialGitOps{w: w}
}

// ReadFile reads a file from the workspace.
func (w *callbackWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	resp, err := w.client.WorkspaceReadFile(ctx, &harnesspb.WorkspaceReadFileRequest{
		WorkspaceName: w.name,
		Path:          path,
	})
	if err != nil {
		return nil, translateWorkspaceErr(err)
	}
	return resp.GetContent(), nil
}

// WriteFile writes content to a file in the workspace.
func (w *callbackWorkspace) WriteFile(ctx context.Context, path string, content []byte) error {
	_, err := w.client.WorkspaceWriteFile(ctx, &harnesspb.WorkspaceWriteFileRequest{
		WorkspaceName: w.name,
		Path:          path,
		Content:       content,
	})
	if err != nil {
		return translateWorkspaceErr(err)
	}
	return nil
}

// ListFiles returns paths matching the given glob pattern.
//
// If the daemon truncated the result set (more than 10,000 matches),
// the returned slice contains the first 10,000 paths and an error of
// the shape "workspace: ListFiles result truncated; refine the pattern"
// is returned alongside the partial slice. Callers can ignore the
// error if they're only sampling, or refine the pattern if they need
// the full set.
func (w *callbackWorkspace) ListFiles(ctx context.Context, pattern string) ([]string, error) {
	resp, err := w.client.WorkspaceListFiles(ctx, &harnesspb.WorkspaceListFilesRequest{
		WorkspaceName: w.name,
		Pattern:       pattern,
	})
	if err != nil {
		return nil, translateWorkspaceErr(err)
	}
	if resp.GetTruncated() {
		return resp.GetPaths(), fmt.Errorf("workspace: ListFiles result truncated at %d paths; refine the pattern", len(resp.GetPaths()))
	}
	return resp.GetPaths(), nil
}

// Commit stages all changes and creates a commit with the given
// message. Returns the commit SHA.
func (w *callbackWorkspace) Commit(ctx context.Context, message string) (string, error) {
	resp, err := w.client.WorkspaceCommit(ctx, &harnesspb.WorkspaceCommitRequest{
		WorkspaceName: w.name,
		Message:       message,
	})
	if err != nil {
		return "", translateWorkspaceErr(err)
	}
	return resp.GetCommitSha(), nil
}

// Push pushes committed changes to the configured remote.
func (w *callbackWorkspace) Push(ctx context.Context) error {
	_, err := w.client.WorkspacePush(ctx, &harnesspb.WorkspacePushRequest{
		WorkspaceName: w.name,
	})
	if err != nil {
		return translateWorkspaceErr(err)
	}
	return nil
}

// Close is a no-op on the callback path. The daemon's
// WorkspaceManager owns the workspace lifetime and will clean up
// after the mission completes.
func (w *callbackWorkspace) Close() error { return nil }

// translateWorkspaceErr maps gRPC status codes from the daemon back to
// types meaningful to a callback agent. NotFound carries through
// (the agent may have requested a non-existent workspace name).
// FailedPrecondition is a "no workspace configured for this mission"
// signal — promoted to ErrWorkspaceNotConfigured.
func translateWorkspaceErr(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.FailedPrecondition:
		return ErrWorkspaceNotConfigured
	default:
		return err
	}
}

// ============================================================================
// Stubbed editor + partial GitOps — deferred to a follow-on spec.
// ============================================================================

// errEditor implements editor.Editor with every method returning
// ErrWorkspaceNotImplemented. This makes the v1 limitation explicit at
// the call site instead of silently returning empty results.
type errEditor struct{}

func (errEditor) Apply(_ context.Context, _ editor.Edit) (*editor.EditResult, error) {
	return nil, ErrWorkspaceNotImplemented
}

func (errEditor) ApplyBatch(_ context.Context, _ []editor.Edit) (*editor.BatchEditResult, error) {
	return nil, ErrWorkspaceNotImplemented
}

func (errEditor) Validate(_ context.Context, _ string) ([]codegen.Diagnostic, error) {
	return nil, ErrWorkspaceNotImplemented
}

func (errEditor) SetFuzzyThreshold(_ float64)          {}
func (errEditor) SetValidationTimeout(_ time.Duration) {}

// partialGitOps proxies only the two operations exposed in v1
// (Commit and Push) and errors on the rest.
type partialGitOps struct {
	w *callbackWorkspace
}

func (g *partialGitOps) Commit(ctx context.Context, message string, _ git.CommitOptions) (string, error) {
	return g.w.Commit(ctx, message)
}

func (g *partialGitOps) Push(ctx context.Context, _ git.PushOptions) error {
	return g.w.Push(ctx)
}

func (g *partialGitOps) CurrentBranch() (string, error) {
	return "", ErrWorkspaceNotImplemented
}

func (g *partialGitOps) Status() (*git.GitStatus, error) {
	return nil, ErrWorkspaceNotImplemented
}

func (g *partialGitOps) CreateBranch(_ context.Context, _ string) error {
	return ErrWorkspaceNotImplemented
}

func (g *partialGitOps) Checkout(_ context.Context, _ string) error {
	return ErrWorkspaceNotImplemented
}

func (g *partialGitOps) Add(_ context.Context, _ ...string) error {
	return ErrWorkspaceNotImplemented
}

func (g *partialGitOps) Pull(_ context.Context) error {
	return ErrWorkspaceNotImplemented
}

func (g *partialGitOps) Snapshot(_ context.Context) (string, error) {
	return "", ErrWorkspaceNotImplemented
}

func (g *partialGitOps) Rollback(_ context.Context, _ string) error {
	return ErrWorkspaceNotImplemented
}

// Compile-time assertions: the proxy types satisfy the SDK interfaces.
var (
	_ workspace.Workspace = (*callbackWorkspace)(nil)
	_ editor.Editor       = errEditor{}
	_ git.GitOps          = (*partialGitOps)(nil)
)
