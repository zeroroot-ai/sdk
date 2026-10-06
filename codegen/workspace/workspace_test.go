// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace

import (
	"context"

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

// TestEditorTypeAlias_SatisfiesInterface verifies at compile time that
// the Editor type alias is equivalent to editor.Editor.
// If the type alias is removed or broken, this file will not compile.
var _ Editor = (Editor)(nil)

// TestGitOpsTypeAlias_SatisfiesInterface verifies at compile time that the
// GitOps type alias is equivalent to git.GitOps. The mock defined above
// implements git.GitOps, so it must also satisfy the workspace.GitOps alias.
var _ GitOps = (*mockGitOps)(nil)
