// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package codegen

import "errors"

// ErrCloneFailed indicates that cloning a Git repository failed.
// This can occur due to network issues, authentication failures,
// invalid repository URLs, or permission errors.
var ErrCloneFailed = errors.New("failed to clone repository")

// ErrCredentialMissing indicates that a required credential was not found
// in the credential store. This error is returned during workspace initialization
// before any clone attempts are made.
var ErrCredentialMissing = errors.New("credential not found")

// ErrSearchNotFound indicates that a SEARCH block could not be found
// in the target file during a SEARCH/REPLACE operation.
// This includes cases where both exact and fuzzy matching failed.
var ErrSearchNotFound = errors.New("search block not found in file")

// ErrValidationFailed indicates that LSP validation returned errors
// after applying code changes. When this error occurs, the editor
// automatically rolls back changes to the last snapshot.
var ErrValidationFailed = errors.New("LSP validation returned errors")

// ErrPushConflict indicates that pushing to a Git remote failed
// because the remote has diverged from the local branch.
// This requires manual conflict resolution or pulling changes.
// The system never force-pushes to avoid data loss.
var ErrPushConflict = errors.New("remote has diverged, pull required")

// ErrWorkspaceNotReady indicates that a workspace was accessed
// before it was properly initialized. Workspaces must be initialized
// by cloning repositories before agents can use them.
var ErrWorkspaceNotReady = errors.New("workspace not initialized")

// ErrLSPTimeout indicates that LSP validation timed out while waiting
// for diagnostics from the language server. When this occurs, changes
// are applied with a warning rather than being rejected, as the timeout
// may be due to temporary server load rather than actual validation errors.
var ErrLSPTimeout = errors.New("LSP validation timed out")
