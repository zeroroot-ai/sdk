// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package bumper

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options controls the bumper's behaviour.
type Options struct {
	// WorkDir is the parent directory under which repos are cloned.
	// Default: /tmp/sdk-bump
	WorkDir string
	// DryRun skips push and PR creation; prints diff instead.
	DryRun bool
	// CloneDepth is the --depth argument passed to git clone.
	CloneDepth int
}

// Result captures the outcome for a single consumer.
type Result struct {
	// Consumer is the short repo name.
	Consumer string
	// Branch is the branch created in the consumer repo.
	Branch string
	// PRNumber is the GitHub PR number opened (0 if dry-run or error).
	PRNumber int
	// PRURL is the URL of the opened PR.
	PRURL string
	// Diff is the output of `git diff HEAD` in dry-run mode.
	Diff string
	// Log collects stdout/stderr from all steps.
	Log bytes.Buffer
	// Err is non-nil if any step failed.
	Err error
	// Steps lists each step name and whether it succeeded.
	Steps []StepResult
}

// StepResult records a single step's outcome.
type StepResult struct {
	Name    string
	Output  string
	Success bool
}

// ConsumerSpec mirrors the consumer.Consumer type to avoid a circular import.
// Callers (main.go) pass this down.
type ConsumerSpec struct {
	Name     string
	GitRepo  string
	GoModule bool
	PostBump []string
}

// Bump clones the consumer repo, updates the SDK pin, runs PostBump commands,
// commits, and (unless DryRun) pushes and opens a PR.
//
// The runner and ghRunner parameters are injection points for tests; pass nil to
// use the real executors.
func Bump(ctx context.Context, spec ConsumerSpec, version string, opts Options, runner CommandRunner, ghRunner CommandRunner) Result {
	if runner == nil {
		runner = RealRunner()
	}
	if ghRunner == nil {
		ghRunner = RealRunner()
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "/tmp/sdk-bump"
	}
	if opts.CloneDepth <= 0 {
		opts.CloneDepth = 50
	}

	res := Result{Consumer: spec.Name}
	branch := "chore/bump-sdk-" + version
	res.Branch = branch

	repoDir := filepath.Join(opts.WorkDir, spec.Name)

	step := func(name string, fn func() ([]byte, error)) bool {
		out, err := fn()
		sr := StepResult{Name: name, Output: string(out), Success: err == nil}
		res.Steps = append(res.Steps, sr)
		fmt.Fprintf(&res.Log, "[%s] %s\n", spec.Name, name)
		if len(out) > 0 {
			fmt.Fprintf(&res.Log, "%s\n", string(out))
		}
		if err != nil {
			fmt.Fprintf(&res.Log, "[%s] FAILED %s: %v\n", spec.Name, name, err)
			res.Err = fmt.Errorf("step %q: %w", name, err)
			return false
		}
		return true
	}

	// 1. Ensure workdir exists.
	if err := os.MkdirAll(opts.WorkDir, 0o750); err != nil {
		res.Err = fmt.Errorf("mkdir %s: %w", opts.WorkDir, err)
		return res
	}

	// 2. Remove stale clone if present.
	_ = os.RemoveAll(repoDir)

	// 3. Clone.
	cloneURL := "git@github.com:" + spec.GitRepo + ".git"
	depthArg := fmt.Sprintf("--depth=%d", opts.CloneDepth)
	if !step("git clone", func() ([]byte, error) {
		return runner(ctx, opts.WorkDir, "git", "clone", depthArg, cloneURL, spec.Name)
	}) {
		return res
	}

	// 4. Create bump branch.
	if !step("git checkout -b "+branch, func() ([]byte, error) {
		return runner(ctx, repoDir, "git", "checkout", "-b", branch)
	}) {
		return res
	}

	// 5. Update the SDK pin.
	if spec.GoModule {
		if !step("go get sdk@"+version, func() ([]byte, error) {
			return bumpGoMod(ctx, runner, repoDir, version)
		}) {
			return res
		}
	} else {
		// Dashboard: no Go module bump; pnpm install handles it via PostBump.
		_, _ = bumpDashboard(ctx, runner, repoDir)
	}

	// 6. Run per-consumer PostBump commands.
	for _, cmd := range spec.PostBump {
		cmdCopy := cmd // capture for closure
		if !step("post-bump: "+cmd, func() ([]byte, error) {
			return runCmd(ctx, runner, repoDir, cmdCopy)
		}) {
			// Record but continue — we still want to attempt the rest of the consumers.
			// For this consumer, mark failure and return early.
			return res
		}
	}

	// 7. Collect diff (for dry-run display and PR body).
	var diffOutput string
	diffOut, _ := runner(ctx, repoDir, "git", "diff", "HEAD")
	diffOutput = string(diffOut)
	res.Diff = diffOutput

	// 8. Commit.
	commitMsg := "chore: bump SDK to " + version
	if !step("git add", func() ([]byte, error) {
		return runner(ctx, repoDir, "git", "add", "-A")
	}) {
		return res
	}
	if !step("git commit", func() ([]byte, error) {
		return runner(ctx, repoDir, "git", "commit", "-m", commitMsg)
	}) {
		return res
	}

	if opts.DryRun {
		// In dry-run mode, show what would have happened but stop before push.
		fmt.Fprintf(&res.Log, "[%s] DRY-RUN: would push branch %s and open PR\n", spec.Name, branch)
		fmt.Fprintf(&res.Log, "[%s] DRY-RUN: commit message: %s\n", spec.Name, commitMsg)
		if diffOutput != "" {
			fmt.Fprintf(&res.Log, "[%s] DRY-RUN: diff:\n%s\n", spec.Name, diffOutput)
		}
		return res
	}

	// 9. Push.
	if !step("git push", func() ([]byte, error) {
		return runner(ctx, repoDir, "git", "push", "-u", "origin", branch)
	}) {
		// NOTE: if push succeeded but a later step fails, the branch remains pushed.
		// This is intentional — the consumer's CI may still be useful. Document in README.
		return res
	}

	// 10. Open PR via gh CLI.
	prBody := buildPRBody(version, spec, repoDir)
	prTitle := "chore: bump SDK to " + version
	prOut, prErr := ghRunner(ctx, repoDir, "gh", "pr", "create",
		"--title", prTitle,
		"--body", prBody,
		"--base", "main",
		"--head", branch,
	)
	sr := StepResult{Name: "gh pr create", Output: string(prOut), Success: prErr == nil}
	res.Steps = append(res.Steps, sr)
	fmt.Fprintf(&res.Log, "[%s] gh pr create\n%s\n", spec.Name, string(prOut))
	if prErr != nil {
		fmt.Fprintf(&res.Log, "[%s] FAILED gh pr create: %v\n", spec.Name, prErr)
		res.Err = fmt.Errorf("step %q: %w", "gh pr create", prErr)
		// NOTE: Branch is already pushed. The PR was not created. The user can
		// manually run `gh pr create` in the cloned repo.
		return res
	}

	// Extract PR URL from gh output.
	res.PRURL = strings.TrimSpace(string(prOut))
	return res
}

// buildPRBody constructs the pull request description.
func buildPRBody(version string, spec ConsumerSpec, repoDir string) string {
	var sb strings.Builder
	sb.WriteString("## SDK bump to " + version + "\n\n")
	sb.WriteString("This PR was opened automatically by `sdk-bump`.\n\n")

	// List post-bump commands that were run.
	if len(spec.PostBump) > 0 {
		sb.WriteString("### Post-bump commands run\n\n")
		for _, cmd := range spec.PostBump {
			sb.WriteString("- `" + cmd + "`\n")
		}
		sb.WriteString("\n")
	}

	// Changelog or git log snippet.
	sb.WriteString("### SDK changes\n\n")
	sb.WriteString("See the [SDK CHANGELOG](https://github.com/zeroroot-ai/sdk/blob/main/CHANGELOG.md) " +
		"for details on " + version + ".\n\n")

	sb.WriteString("### CI\n\n")
	sb.WriteString("CI status will appear in the checks section of this PR.\n")

	return sb.String()
}
