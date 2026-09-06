// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/codegen/editor"
	"github.com/zeroroot-ai/sdk/codegen/git"
)

func insecureTransportCreds() grpc.DialOption {
	return grpc.WithTransportCredentials(insecure.NewCredentials())
}

// fakeHarnessCallbackClient is a minimal implementation of
// harnesspb.HarnessCallbackServiceClient that records the requests it
// receives and returns canned responses. Used by the workspace proxy
// tests below.
type fakeHarnessCallbackClient struct {
	harnesspb.HarnessCallbackServiceClient // embed for default-zero implementations of the methods we don't exercise

	// Workspace responses to return + last request seen.
	listResp      *harnesspb.WorkspaceListResponse
	getInfoResp   *harnesspb.WorkspaceGetInfoResponse
	readResp      *harnesspb.WorkspaceReadFileResponse
	writeResp     *harnesspb.WorkspaceWriteFileResponse
	listFilesResp *harnesspb.WorkspaceListFilesResponse
	commitResp    *harnesspb.WorkspaceCommitResponse
	pushResp      *harnesspb.WorkspacePushResponse

	// Error to return from the next call (any RPC).
	nextErr error

	lastListReq      *harnesspb.WorkspaceListRequest
	lastGetInfoReq   *harnesspb.WorkspaceGetInfoRequest
	lastReadReq      *harnesspb.WorkspaceReadFileRequest
	lastWriteReq     *harnesspb.WorkspaceWriteFileRequest
	lastListFilesReq *harnesspb.WorkspaceListFilesRequest
	lastCommitReq    *harnesspb.WorkspaceCommitRequest
	lastPushReq      *harnesspb.WorkspacePushRequest
}

func (f *fakeHarnessCallbackClient) WorkspaceList(_ context.Context, in *harnesspb.WorkspaceListRequest, _ ...grpc.CallOption) (*harnesspb.WorkspaceListResponse, error) {
	f.lastListReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.listResp, nil
}

func (f *fakeHarnessCallbackClient) WorkspaceGetInfo(_ context.Context, in *harnesspb.WorkspaceGetInfoRequest, _ ...grpc.CallOption) (*harnesspb.WorkspaceGetInfoResponse, error) {
	f.lastGetInfoReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.getInfoResp, nil
}

func (f *fakeHarnessCallbackClient) WorkspaceReadFile(_ context.Context, in *harnesspb.WorkspaceReadFileRequest, _ ...grpc.CallOption) (*harnesspb.WorkspaceReadFileResponse, error) {
	f.lastReadReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.readResp, nil
}

func (f *fakeHarnessCallbackClient) WorkspaceWriteFile(_ context.Context, in *harnesspb.WorkspaceWriteFileRequest, _ ...grpc.CallOption) (*harnesspb.WorkspaceWriteFileResponse, error) {
	f.lastWriteReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.writeResp, nil
}

func (f *fakeHarnessCallbackClient) WorkspaceListFiles(_ context.Context, in *harnesspb.WorkspaceListFilesRequest, _ ...grpc.CallOption) (*harnesspb.WorkspaceListFilesResponse, error) {
	f.lastListFilesReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.listFilesResp, nil
}

func (f *fakeHarnessCallbackClient) WorkspaceCommit(_ context.Context, in *harnesspb.WorkspaceCommitRequest, _ ...grpc.CallOption) (*harnesspb.WorkspaceCommitResponse, error) {
	f.lastCommitReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.commitResp, nil
}

func (f *fakeHarnessCallbackClient) WorkspacePush(_ context.Context, in *harnesspb.WorkspacePushRequest, _ ...grpc.CallOption) (*harnesspb.WorkspacePushResponse, error) {
	f.lastPushReq = in
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.pushResp, nil
}

// newFakeCallbackClient wires a CallbackClient against the given fake.
// A passthrough grpc.ClientConn is created so IsConnected() returns true
// without spinning up a real server — the actual RPC dispatch goes
// through the injected `client` (the fake).
func newFakeCallbackClient(t *testing.T, fake *fakeHarnessCallbackClient) *CallbackClient {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///fake", insecureTransportCreds())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &CallbackClient{
		endpoint:  "fake",
		conn:      conn,
		client:    fake,
		connected: true,
	}
}

// ---------------------------------------------------------------------------
// callbackWorkspace happy paths
// ---------------------------------------------------------------------------

func TestCallbackWorkspace_ReadFile(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		readResp: &harnesspb.WorkspaceReadFileResponse{Content: []byte("hello\n")},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend", Path: "/work/backend"})

	got, err := w.ReadFile(context.Background(), "main.go")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello\n"), got)
	assert.Equal(t, "backend", fake.lastReadReq.GetWorkspaceName())
	assert.Equal(t, "main.go", fake.lastReadReq.GetPath())
}

func TestCallbackWorkspace_WriteFile(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		writeResp: &harnesspb.WorkspaceWriteFileResponse{},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})

	err := w.WriteFile(context.Background(), "main.go", []byte("// edited"))
	require.NoError(t, err)
	assert.Equal(t, "main.go", fake.lastWriteReq.GetPath())
	assert.Equal(t, []byte("// edited"), fake.lastWriteReq.GetContent())
}

func TestCallbackWorkspace_ListFiles_Untruncated(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		listFilesResp: &harnesspb.WorkspaceListFilesResponse{
			Paths:     []string{"main.go", "go.mod"},
			Truncated: false,
		},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})

	got, err := w.ListFiles(context.Background(), "*.go")
	require.NoError(t, err)
	assert.Equal(t, []string{"main.go", "go.mod"}, got)
	assert.Equal(t, "*.go", fake.lastListFilesReq.GetPattern())
}

func TestCallbackWorkspace_ListFiles_Truncated(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		listFilesResp: &harnesspb.WorkspaceListFilesResponse{
			Paths:     []string{"a.go"},
			Truncated: true,
		},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})

	got, err := w.ListFiles(context.Background(), "**/*.go")
	require.Error(t, err, "truncated result must surface as an error")
	assert.Contains(t, err.Error(), "truncated")
	assert.Equal(t, []string{"a.go"}, got, "partial paths must still be returned alongside the error")
}

func TestCallbackWorkspace_Commit(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		commitResp: &harnesspb.WorkspaceCommitResponse{CommitSha: "abc123"},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})

	sha, err := w.Commit(context.Background(), "fix: typo")
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
	assert.Equal(t, "fix: typo", fake.lastCommitReq.GetMessage())
}

func TestCallbackWorkspace_Push(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		pushResp: &harnesspb.WorkspacePushResponse{},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})

	require.NoError(t, w.Push(context.Background()))
	assert.NotNil(t, fake.lastPushReq, "push RPC must be invoked")
}

func TestCallbackWorkspace_Close_NoOp(t *testing.T) {
	w := &callbackWorkspace{}
	assert.NoError(t, w.Close(),
		"Close must be a no-op on the callback path; the daemon owns the workspace lifetime")
}

// ---------------------------------------------------------------------------
// Editor + GitOps deferred-surface enforcement
// ---------------------------------------------------------------------------

func TestCallbackWorkspace_Editor_NotImplemented(t *testing.T) {
	w := &callbackWorkspace{}
	ed := w.Editor()
	require.NotNil(t, ed)

	// Apply, ApplyBatch, Validate must all return ErrWorkspaceNotImplemented.
	_, err := ed.Apply(context.Background(), editor.Edit{})
	require.ErrorIs(t, err, ErrWorkspaceNotImplemented)

	_, err = ed.ApplyBatch(context.Background(), nil)
	require.ErrorIs(t, err, ErrWorkspaceNotImplemented)

	_, err = ed.Validate(context.Background(), "main.go")
	require.ErrorIs(t, err, ErrWorkspaceNotImplemented)

	// Setters are no-ops (don't panic).
	ed.SetFuzzyThreshold(0.9)
	ed.SetValidationTimeout(0)
}

func TestCallbackWorkspace_Git_PartialOps(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		commitResp: &harnesspb.WorkspaceCommitResponse{CommitSha: "abc123"},
		pushResp:   &harnesspb.WorkspacePushResponse{},
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})
	g := w.Git()

	// Commit + Push are the two methods that ARE proxied.
	sha, err := g.Commit(context.Background(), "msg", git.CommitOptions{})
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
	require.NoError(t, g.Push(context.Background(), git.PushOptions{}))

	// Every other GitOps method returns ErrWorkspaceNotImplemented.
	_, err = g.CurrentBranch()
	require.ErrorIs(t, err, ErrWorkspaceNotImplemented)
	_, err = g.Status()
	require.ErrorIs(t, err, ErrWorkspaceNotImplemented)
	require.ErrorIs(t, g.CreateBranch(context.Background(), "feature/x"), ErrWorkspaceNotImplemented)
	require.ErrorIs(t, g.Checkout(context.Background(), "main"), ErrWorkspaceNotImplemented)
	require.ErrorIs(t, g.Add(context.Background(), "."), ErrWorkspaceNotImplemented)
	require.ErrorIs(t, g.Pull(context.Background()), ErrWorkspaceNotImplemented)
	_, err = g.Snapshot(context.Background())
	require.ErrorIs(t, err, ErrWorkspaceNotImplemented)
	require.ErrorIs(t, g.Rollback(context.Background(), "snap-1"), ErrWorkspaceNotImplemented)
}

// ---------------------------------------------------------------------------
// Error translation
// ---------------------------------------------------------------------------

func TestCallbackWorkspace_FailedPrecondition_BecomesNotConfigured(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		nextErr: status.Error(codes.FailedPrecondition, "no workspace"),
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "backend"})

	_, err := w.ReadFile(context.Background(), "main.go")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWorkspaceNotConfigured,
		"FailedPrecondition must translate to ErrWorkspaceNotConfigured")
}

func TestCallbackWorkspace_NotFound_PassesThrough(t *testing.T) {
	fake := &fakeHarnessCallbackClient{
		nextErr: status.Error(codes.NotFound, "workspace not found"),
	}
	c := newFakeCallbackClient(t, fake)
	w := newCallbackWorkspace(c, &harnesspb.WorkspaceInfo{Name: "ghost"})

	_, err := w.ReadFile(context.Background(), "main.go")
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "underlying error must be a gRPC status")
	assert.Equal(t, codes.NotFound, st.Code(),
		"NotFound must pass through unchanged so callers can distinguish missing workspace from missing file")
}
