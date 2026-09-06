// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package bumper implements the per-consumer clone-bump-push workflow.
package bumper

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// CommandRunner is the interface used to execute external programs.
// It is a function type so tests can inject fake executors.
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

// RealRunner returns a CommandRunner that actually executes processes.
func RealRunner() CommandRunner {
	return func(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		return buf.Bytes(), err
	}
}

// splitCmd splits a command string (e.g. "go build ./...") into name + args.
// It splits on ASCII spaces only — no shell interpretation, no glob expansion.
// User-controlled strings (version tags) are never embedded in command strings;
// they are passed as explicit arguments.
func splitCmd(cmdStr string) (name string, args []string) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// runCmd is a convenience wrapper that splits a command string and runs it.
func runCmd(ctx context.Context, runner CommandRunner, dir string, cmdStr string) ([]byte, error) {
	name, args := splitCmd(cmdStr)
	if name == "" {
		return nil, errors.New("empty command string")
	}
	return runner(ctx, dir, name, args...)
}
