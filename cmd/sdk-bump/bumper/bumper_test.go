// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package bumper_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/zeroroot-ai/sdk/cmd/sdk-bump/bumper"
)

// recordingRunner captures every call made to it.
type recordingRunner struct {
	mu        sync.Mutex
	calls     []runCall
	responses map[string]fakeResponse
}

type runCall struct {
	Dir  string
	Name string
	Args []string
}

type fakeResponse struct {
	out []byte
	err error
}

func newRecorder(responses map[string]fakeResponse) *recordingRunner {
	if responses == nil {
		responses = make(map[string]fakeResponse)
	}
	return &recordingRunner{responses: responses}
}

func (r *recordingRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, runCall{Dir: dir, Name: name, Args: args})
	// Try progressively shorter key prefixes: exact match first.
	key := name
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}
	if resp, ok := r.responses[key]; ok {
		return resp.out, resp.err
	}
	// Try matching by name + first arg only (for gh pr create, etc.).
	if len(args) > 0 {
		short := name + " " + args[0]
		if resp, ok := r.responses[short]; ok {
			return resp.out, resp.err
		}
	}
	// Name-only match.
	if resp, ok := r.responses[name]; ok {
		return resp.out, resp.err
	}
	return []byte{}, nil
}

// Called returns true if name was ever invoked with exactly those args.
func (r *recordingRunner) Called(name string, args ...string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c.Name == name && argsMatch(c.Args, args) {
			return true
		}
	}
	return false
}

// CalledWith returns true if name was invoked and arg appears anywhere in its args list.
func (r *recordingRunner) CalledWith(name string, arg string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c.Name != name {
			continue
		}
		for _, a := range c.Args {
			if a == arg {
				return true
			}
		}
	}
	return false
}

func argsMatch(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// goConsumerSpec returns a minimal Go consumer spec for tests.
func goConsumerSpec(name string, postBump ...string) bumper.ConsumerSpec {
	if len(postBump) == 0 {
		postBump = []string{"go mod tidy", "go build ./..."}
	}
	return bumper.ConsumerSpec{
		Name:     name,
		GitRepo:  "zeroroot-ai/" + name,
		GoModule: true,
		PostBump: postBump,
	}
}

// testOpts returns default test options pointing to a temp dir.
func testOpts(t *testing.T) bumper.Options {
	t.Helper()
	return bumper.Options{
		WorkDir:    t.TempDir(),
		DryRun:     false,
		CloneDepth: 1,
	}
}

// --- Tests ---

func TestHappyPath_GoConsumer(t *testing.T) {
	runner := newRecorder(nil)
	ghRunner := newRecorder(map[string]fakeResponse{
		// Match on "gh pr" prefix.
		"gh pr": {out: []byte("https://github.com/zeroroot-ai/myrepo/pull/42\n")},
	})

	spec := goConsumerSpec("myrepo", "go mod tidy", "go build ./...")
	o := testOpts(t)

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)

	if res.Err != nil {
		t.Fatalf("unexpected error: %v\nlog:\n%s", res.Err, res.Log.String())
	}

	// Verify git clone was called with the correct URL.
	if !runner.CalledWith("git", "git@github.com:zeroroot-ai/myrepo.git") {
		t.Error("expected git clone to be called with the correct URL")
	}

	// Verify branch creation.
	if !runner.Called("git", "checkout", "-b", "chore/bump-sdk-v1.2.3") {
		t.Error("expected git checkout -b chore/bump-sdk-v1.2.3")
	}

	// Verify go get was called with the correct pin.
	if !runner.CalledWith("go", "github.com/zeroroot-ai/sdk@v1.2.3") {
		t.Error("expected go get github.com/zeroroot-ai/sdk@v1.2.3")
	}

	// Verify push.
	if !runner.Called("git", "push", "-u", "origin", "chore/bump-sdk-v1.2.3") {
		t.Error("expected git push -u origin chore/bump-sdk-v1.2.3")
	}

	// Verify gh pr create was called.
	if !ghRunner.CalledWith("gh", "pr") {
		t.Error("expected gh pr create to be called")
	}

	// PR URL should be captured.
	if res.PRURL == "" {
		t.Error("expected non-empty PRURL")
	}
}

func TestDryRun_NoPushNoPR(t *testing.T) {
	runner := newRecorder(nil)
	ghRunner := newRecorder(nil)

	spec := goConsumerSpec("myrepo")
	o := testOpts(t)
	o.DryRun = true

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)

	if res.Err != nil {
		t.Fatalf("unexpected error: %v\nlog:\n%s", res.Err, res.Log.String())
	}

	// Push must NOT happen in dry-run.
	if runner.Called("git", "push", "-u", "origin", "chore/bump-sdk-v1.2.3") {
		t.Error("git push must not be called in dry-run mode")
	}

	// gh pr create must NOT happen in dry-run.
	if ghRunner.CalledWith("gh", "pr") {
		t.Error("gh pr create must not be called in dry-run mode")
	}

	// Log should mention DRY-RUN.
	if !strings.Contains(res.Log.String(), "DRY-RUN") {
		t.Error("expected DRY-RUN marker in log output")
	}

	// Branch name should still be set.
	if res.Branch != "chore/bump-sdk-v1.2.3" {
		t.Errorf("expected branch chore/bump-sdk-v1.2.3, got %q", res.Branch)
	}
}

func TestPostBumpFailure_ReturnsError(t *testing.T) {
	runner := newRecorder(map[string]fakeResponse{
		// Make "go test -short ./..." fail.
		"go test -short ./...": {out: []byte("FAIL some/pkg\n"), err: errors.New("exit status 1")},
	})
	ghRunner := newRecorder(nil)

	spec := goConsumerSpec("myrepo", "go mod tidy", "go build ./...", "go test -short ./...")
	o := testOpts(t)

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)

	if res.Err == nil {
		t.Fatal("expected error from failing post-bump step")
	}
	if !strings.Contains(res.Err.Error(), "go test") {
		t.Errorf("expected error to mention 'go test', got: %v", res.Err)
	}

	// Push and PR must NOT be called after a failure.
	if runner.Called("git", "push", "-u", "origin", "chore/bump-sdk-v1.2.3") {
		t.Error("git push must not be called after a failing step")
	}
	if ghRunner.CalledWith("gh", "pr") {
		t.Error("gh pr create must not be called after a failing step")
	}
}

func TestGHPRCreateFailure_BranchRemainsDocumented(t *testing.T) {
	runner := newRecorder(nil)
	ghRunner := newRecorder(map[string]fakeResponse{
		// gh pr create fails.
		"gh pr": {err: errors.New("network error")},
	})

	spec := goConsumerSpec("myrepo")
	o := testOpts(t)

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)

	if res.Err == nil {
		t.Fatal("expected error from gh pr create failure")
	}

	// Push should have been called (before PR creation).
	if !runner.Called("git", "push", "-u", "origin", "chore/bump-sdk-v1.2.3") {
		t.Error("expected git push to have been called before gh pr create failed")
	}

	// PR URL should be empty.
	if res.PRURL != "" {
		t.Errorf("expected empty PRURL on gh pr create failure, got %q", res.PRURL)
	}
}

func TestGHPRChecks_NotCalledByBump(t *testing.T) {
	// Bump itself does not call gh pr checks — that is done by pr.CheckPR separately.
	// Verify that Bump does not invoke gh pr checks even after a successful PR.
	runner := newRecorder(nil)
	ghRunner := newRecorder(map[string]fakeResponse{
		"gh pr": {out: []byte("https://github.com/zeroroot-ai/myrepo/pull/1\n")},
	})

	spec := goConsumerSpec("myrepo")
	o := testOpts(t)

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}

	// Verify gh pr checks was NOT called by Bump.
	if ghRunner.CalledWith("gh", "checks") {
		t.Error("Bump must not call gh pr checks — that is the pr package's responsibility")
	}
}

func TestDashboardConsumer_NoGoGet(t *testing.T) {
	runner := newRecorder(nil)
	ghRunner := newRecorder(nil)

	spec := bumper.ConsumerSpec{
		Name:     "dashboard",
		GitRepo:  "zeroroot-ai/dashboard",
		GoModule: false,
		PostBump: []string{"pnpm install --no-frozen-lockfile"},
	}
	o := testOpts(t)
	o.DryRun = true // avoid push

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)

	if res.Err != nil {
		t.Fatalf("unexpected error: %v\nlog:\n%s", res.Err, res.Log.String())
	}

	// go get must NOT be called for dashboard.
	if runner.CalledWith("go", "github.com/zeroroot-ai/sdk@v1.2.3") {
		t.Error("go get must not be called for non-Go (dashboard) consumer")
	}
}

func TestOneConsumerFailure_ContinuesOthers(t *testing.T) {
	// This tests the orchestration at a higher level — ensuring that a failure in
	// one Bump call doesn't prevent others from running. The parallelism lives in
	// main.go (errgroup), but here we verify the Result.Err field is set correctly
	// for the failing consumer and that the function returns (not panics).
	runner := newRecorder(map[string]fakeResponse{
		"go test -short ./...": {err: errors.New("tests failed")},
	})
	ghRunner := newRecorder(nil)

	spec := goConsumerSpec("bad-consumer", "go test -short ./...")
	o := testOpts(t)

	res := bumper.Bump(context.Background(), spec, "v1.2.3", o, runner.Run, ghRunner.Run)

	if res.Err == nil {
		t.Fatal("expected error for failing consumer")
	}
	if res.Consumer != "bad-consumer" {
		t.Errorf("expected Consumer=%q, got %q", "bad-consumer", res.Consumer)
	}
}
