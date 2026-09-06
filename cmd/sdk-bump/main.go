// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Command sdk-bump opens pull requests in every SDK consumer repo to bump the
// github.com/zeroroot-ai/sdk Go module (or equivalent TS bindings) to a new version.
//
// Usage:
//
//	sdk-bump --to vX.Y.Z [--dry-run] [--repos repo1,repo2] [--workdir /tmp/sdk-bump] [--json] [--ci-delay 30s]
//
// Behaviour:
//  1. Reads the hard-coded CONSUMERS list.
//  2. For each consumer (in parallel, up to GOMAXPROCS goroutines):
//     a. git clone --depth=50 git@github.com:<repo>.git <name>
//     b. git checkout -b chore/bump-sdk-<version>
//     c. For Go repos: go get github.com/zeroroot-ai/sdk@<version> && go mod tidy
//     d. Run per-consumer PostBump commands.
//     e. git add -A && git commit -m "chore: bump SDK to <version>"
//     f. git push -u origin chore/bump-sdk-<version>
//     g. gh pr create …
//  3. With --dry-run: clone + bump but do NOT push or open a PR.
//  4. On consumer failure: record, continue, exit non-zero at the end.
//  5. After all PRs open, poll gh pr checks (after --ci-delay) for each PR.
//
// Spec: unified-identity-and-authorization Requirements 18.1, 18.2 (Task 9.1).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/zeroroot-ai/sdk/cmd/sdk-bump/bumper"
	"github.com/zeroroot-ai/sdk/cmd/sdk-bump/pr"
	"github.com/zeroroot-ai/sdk/cmd/sdk-bump/report"
)

func main() {
	os.Exit(run())
}

func run() int {
	// --- Flags ---
	var (
		toVersion  = flag.String("to", "", "SDK version to bump to, e.g. v1.2.3 (required)")
		dryRun     = flag.Bool("dry-run", false, "clone and bump but do not push or open PRs")
		reposFlag  = flag.String("repos", "", "comma-separated subset of consumer names; default=all")
		workdir    = flag.String("workdir", "/tmp/sdk-bump", "directory under which repos are cloned")
		jsonOutput = flag.Bool("json", false, "emit JSON summary instead of text table")
		ciDelay    = flag.Duration("ci-delay", pr.DefaultCheckDelay, "time to wait after PR open before polling gh pr checks; 0 disables CI check")
		workspace  = flag.String("workspace", defaultWorkspace(), "path to the polyrepo workspace root (used for local-path validation warnings)")
	)
	flag.Parse()

	if *toVersion == "" {
		fmt.Fprintln(os.Stderr, "sdk-bump: --to is required")
		flag.Usage()
		return 2
	}

	// Guard against obviously bad version strings that could be used to inject
	// arguments into git/go commands (no spaces, no shell metacharacters).
	if err := validateVersion(*toVersion); err != nil {
		fmt.Fprintf(os.Stderr, "sdk-bump: invalid version %q: %v\n", *toVersion, err)
		return 2
	}

	// Parse --repos filter.
	var repoNames []string
	if *reposFlag != "" {
		for _, r := range strings.Split(*reposFlag, ",") {
			if t := strings.TrimSpace(r); t != "" {
				repoNames = append(repoNames, t)
			}
		}
	}

	consumers, err := filterConsumers(CONSUMERS, repoNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdk-bump: %v\n", err)
		return 2
	}

	// Emit startup warnings for any consumer whose local checkout is absent.
	validateLocalPaths(*workspace, consumers)

	// Context: cancel on SIGINT/SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	opts := bumper.Options{
		WorkDir:    *workdir,
		DryRun:     *dryRun,
		CloneDepth: 50,
	}

	runner := bumper.RealRunner()
	ghRunner := bumper.RealRunner()

	// Run consumers in parallel, bounded by GOMAXPROCS.
	maxWorkers := int64(runtime.GOMAXPROCS(0))
	sem := semaphore.NewWeighted(maxWorkers)

	results := make([]bumper.Result, len(consumers))
	var wg sync.WaitGroup

	for i, c := range consumers {
		i, c := i, c // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			if acquireErr := sem.Acquire(ctx, 1); acquireErr != nil {
				results[i] = bumper.Result{
					Consumer: c.Name,
					Err:      fmt.Errorf("context cancelled before acquire: %w", acquireErr),
				}
				return
			}
			defer sem.Release(1)

			fmt.Fprintf(os.Stderr, "[sdk-bump] starting consumer %s\n", c.Name)
			spec := bumper.ConsumerSpec{
				Name:     c.Name,
				GitRepo:  c.GitRepo,
				GoModule: c.GoModule,
				PostBump: c.PostBump,
			}
			res := bumper.Bump(ctx, spec, *toVersion, opts, runner, ghRunner)
			results[i] = res

			// Print log immediately so the user sees progress.
			fmt.Fprint(os.Stderr, res.Log.String())

			if res.Err != nil {
				fmt.Fprintf(os.Stderr, "[sdk-bump] consumer %s FAILED: %v\n", c.Name, res.Err)
			} else {
				fmt.Fprintf(os.Stderr, "[sdk-bump] consumer %s OK (PR: %s)\n", c.Name, res.PRURL)
			}
		}()
	}
	wg.Wait()

	// --- CI checks (non-dry-run only) ---
	// Wait once after all PRs are open, then check each PR concurrently (no per-PR delay).
	if !*dryRun && *ciDelay > 0 {
		fmt.Fprintf(os.Stderr, "[sdk-bump] waiting %s before polling CI status...\n", *ciDelay)
		select {
		case <-ctx.Done():
			// Interrupted — skip CI check.
		case <-time.After(*ciDelay):
			for _, res := range results {
				if res.Err != nil || res.PRURL == "" {
					continue
				}
				// gh pr checks accepts the full PR URL as the ref.
				// We pass the consumer's clone directory as the working directory.
				repoDir := filepath.Join(*workdir, res.Consumer)
				// Pass delay=0 so CheckPR does not wait again.
				cr := pr.CheckPR(ctx, pr.CommandRunner(ghRunner), repoDir, res.PRURL, 0)
				fmt.Fprint(os.Stderr, pr.FormatCheckResult(cr))
			}
		}
	}

	sum := report.Summary{
		Version: *toVersion,
		Results: results,
		DryRun:  *dryRun,
	}

	if *jsonOutput {
		if writeErr := sum.WriteJSON(os.Stdout); writeErr != nil {
			fmt.Fprintf(os.Stderr, "sdk-bump: json encode: %v\n", writeErr)
			return 1
		}
	} else {
		sum.WriteText(os.Stdout)
	}

	if sum.HasFailures() {
		return 1
	}
	return 0
}

// validateVersion rejects version strings that contain characters that could
// be used for shell injection. Versions must start with 'v' and contain only
// alphanumeric characters plus '.', '-', and '+'.
func validateVersion(v string) error {
	if v == "" {
		return errors.New("empty string")
	}
	if !strings.HasPrefix(v, "v") {
		return errors.New("version must start with 'v'")
	}
	for _, c := range v {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' || c == '-' || c == '+' {
			continue
		}
		return fmt.Errorf("character %q is not allowed in version strings", c)
	}
	return nil
}

// defaultWorkspace returns the expected polyrepo root on this workstation.
func defaultWorkspace() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/Code/zeroroot.ai"
}
