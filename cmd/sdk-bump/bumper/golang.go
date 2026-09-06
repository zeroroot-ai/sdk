// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package bumper

import (
	"context"
	"fmt"
)

// sdkModulePath is the Go module path for the SDK.
const sdkModulePath = "github.com/zeroroot-ai/sdk"

// bumpGoMod updates the SDK pin in a Go module by running:
//
//	go get github.com/zeroroot-ai/sdk@<version>
//	go mod tidy
//
// The caller is responsible for running any consumer-specific PostBump commands
// afterward (those may include an additional "go mod tidy").
func bumpGoMod(ctx context.Context, runner CommandRunner, repoDir string, version string) ([]byte, error) {
	pin := sdkModulePath + "@" + version
	out, err := runner(ctx, repoDir, "go", "get", pin)
	if err != nil {
		return out, fmt.Errorf("go get %s: %w\n%s", pin, err, string(out))
	}
	tidyOut, tidyErr := runner(ctx, repoDir, "go", "mod", "tidy")
	combined := append(out, tidyOut...)
	if tidyErr != nil {
		return combined, fmt.Errorf("go mod tidy: %w\n%s", tidyErr, string(tidyOut))
	}
	return combined, nil
}
