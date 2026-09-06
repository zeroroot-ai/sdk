// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/zeroroot-ai/sdk/codegen/git"
)

// mockGitOps records calls to Add, Commit, and Push for assertion in tests.
type mockGitOps struct {
	addCalls    [][]string // each element is the paths passed to Add
	commitCalls []struct {
		message string
		opts    git.CommitOptions
	}
	pushCalls []git.PushOptions

	// Configurable return values.
	addErr    error
	commitSHA string
	commitErr error
	pushErr   error
}

func (m *mockGitOps) Add(_ context.Context, paths ...string) error {
	m.addCalls = append(m.addCalls, paths)
	return m.addErr
}

func (m *mockGitOps) Commit(_ context.Context, message string, opts git.CommitOptions) (string, error) {
	m.commitCalls = append(m.commitCalls, struct {
		message string
		opts    git.CommitOptions
	}{message, opts})
	return m.commitSHA, m.commitErr
}

func (m *mockGitOps) Push(_ context.Context, opts git.PushOptions) error {
	m.pushCalls = append(m.pushCalls, opts)
	return m.pushErr
}

func (m *mockGitOps) CurrentBranch() (string, error)                      { return "main", nil }
func (m *mockGitOps) Status() (*git.GitStatus, error)                     { return &git.GitStatus{}, nil }
func (m *mockGitOps) CreateBranch(_ context.Context, name string) error   { return nil }
func (m *mockGitOps) Checkout(_ context.Context, ref string) error        { return nil }
func (m *mockGitOps) Pull(_ context.Context) error                        { return nil }
func (m *mockGitOps) Snapshot(_ context.Context) (string, error)          { return "", nil }
func (m *mockGitOps) Rollback(_ context.Context, snapshotID string) error { return nil }

// newTestWorkspace creates a minimal workspaceImpl wired with the given mock.
func newTestWorkspace(mock *mockGitOps) *workspaceImpl {
	return &workspaceImpl{
		name:   "test-repo",
		path:   "/tmp/test-workspace",
		gitOps: mock,
	}
}

// TestWorkspaceCommit_CallsAddThenCommitInOrder verifies the sequencing guarantee:
// Add must be called before Commit, and the correct path "." is used.
func TestWorkspaceCommit_CallsAddThenCommitInOrder(t *testing.T) {
	mock := &mockGitOps{commitSHA: "abc123"}
	ws := newTestWorkspace(mock)

	sha, err := ws.Commit(context.Background(), "initial commit")
	if err != nil {
		t.Fatalf("Commit returned unexpected error: %v", err)
	}
	if sha != "abc123" {
		t.Errorf("Commit SHA = %q, want %q", sha, "abc123")
	}

	// Add must have been called once with "."
	if len(mock.addCalls) != 1 {
		t.Fatalf("Add called %d times, want 1", len(mock.addCalls))
	}
	if len(mock.addCalls[0]) != 1 || mock.addCalls[0][0] != "." {
		t.Errorf("Add called with %v, want [\".\"]", mock.addCalls[0])
	}

	// Commit must have been called once with the right message.
	if len(mock.commitCalls) != 1 {
		t.Fatalf("Commit called %d times, want 1", len(mock.commitCalls))
	}
	if mock.commitCalls[0].message != "initial commit" {
		t.Errorf("Commit message = %q, want %q", mock.commitCalls[0].message, "initial commit")
	}
}

// TestWorkspacePush_CallsPushAndPropagatesError verifies that Push calls the
// underlying GitOps.Push and forwards any error.
func TestWorkspacePush_CallsPushAndPropagatesError(t *testing.T) {
	wantErr := errors.New("remote unreachable")
	mock := &mockGitOps{pushErr: wantErr}
	ws := newTestWorkspace(mock)

	err := ws.Push(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Push error = %v, want wrapping %v", err, wantErr)
	}
	if len(mock.pushCalls) != 1 {
		t.Errorf("Push called %d times, want 1", len(mock.pushCalls))
	}
}

// TestWorkspaceCommit_AddFailure_CommitNotCalled verifies that when Add fails,
// Commit is not invoked and the error is propagated.
func TestWorkspaceCommit_AddFailure_CommitNotCalled(t *testing.T) {
	wantErr := errors.New("disk full")
	mock := &mockGitOps{addErr: wantErr}
	ws := newTestWorkspace(mock)

	sha, err := ws.Commit(context.Background(), "should not commit")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Commit error = %v, want wrapping %v", err, wantErr)
	}
	if sha != "" {
		t.Errorf("expected empty SHA on error, got %q", sha)
	}
	// Commit must not have been called.
	if len(mock.commitCalls) != 0 {
		t.Errorf("Commit called %d times after Add failure, want 0", len(mock.commitCalls))
	}
}

// TestWorkspacePush_Success_NilError verifies that a successful Push returns nil.
func TestWorkspacePush_Success_NilError(t *testing.T) {
	mock := &mockGitOps{}
	ws := newTestWorkspace(mock)

	if err := ws.Push(context.Background()); err != nil {
		t.Errorf("Push returned unexpected error: %v", err)
	}
	if len(mock.pushCalls) != 1 {
		t.Errorf("Push called %d times, want 1", len(mock.pushCalls))
	}
}

// TestEditorTypeAlias_SatisfiesInterface verifies at compile time that
// the Editor type alias is equivalent to editor.Editor.
// If the type alias is removed or broken, this file will not compile.
var _ Editor = (Editor)(nil)

// TestGitOpsTypeAlias_SatisfiesInterface verifies at compile time that the
// GitOps type alias is equivalent to git.GitOps. The mock defined above
// implements git.GitOps, so it must also satisfy the workspace.GitOps alias.
var _ GitOps = (*mockGitOps)(nil)
