# Gibson SDK Makefile
# The SDK is a library - no binary to compile, but we build examples and run tests

.PHONY: all bootstrap build examples image test test-race test-coverage test-integration test-integration-all lint lint-deadcode fmt vet tidy clean deps check check-no-gibson check-coverage check-buf-pinned proto proto-deps proto-clean proto-breaking taxonomy-gen taxonomy-proto generate verify-idempotent release-prep help tool-runner-image tool-runner-load sdk-bump mission-jsonschema mission-docs mission-authoring-bundle cue-defs ensure-cue

# Protoc plugin versions (single source of truth for deterministic generation)
# buf CLI version is pinned as a go.mod tool dependency (github.com/bufbuild/buf)
PROTOC_GEN_GO_VERSION := v1.34.2
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1

# golangci-lint version — pinned for reproducible lint output (mirrors
# .tool-versions). v2 schema (.golangci.yml `version: "2"`). Built from source
# with the repo's own Go toolchain so its internal Go version is never lower
# than go.mod's `go` directive (golangci refuses to load a newer target).
GOLANGCI_LINT_VERSION := v2.4.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Directories
BIN_DIR=bin
EXAMPLES_DIR=examples
PROTO_DIR=api/proto
PROTO_OUT=api/gen
COMMONPB_OUT=$(PROTO_OUT)/commonpb
GRAPHRAGPB_OUT=$(PROTO_OUT)/graphragpb
TAXONOMYPB_OUT=$(PROTO_OUT)/taxonomypb
TOOLSPB_OUT=$(PROTO_OUT)/toolspb
COMPONENTPB_OUT=$(PROTO_OUT)/componentpb

# Taxonomy generation
TAXONOMY_YAML=taxonomy/core.yaml
# Taxonomy generation uses the gibson taxonomy-gen command from the module cache
TAXONOMY_GEN_CMD=go run github.com/zeroroot-ai/gibson/cmd/taxonomy-gen

# Buf code generation (uses local buf.yaml + buf.gen.yaml in this directory).
# buf is a go.mod `tool` dependency, so `go tool buf` is hermetic: plain
# `go test ./...` and every Make target self-provide buf with no npm step.
SDK_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BUF := go tool buf

# Example binaries to build
EXAMPLES=minimal-agent custom-tool custom-plugin

# Default target
all: test examples

# build: compile all packages (the SDK is a library; this verifies
# every package type-checks cleanly, satisfying the org Makefile
# contract from zeroroot-ai/.github#87 / gibson#171 slice 1.4).
.PHONY: build
build:
	$(GOBUILD) ./...

# Build example binaries to $(BIN_DIR)/
examples: $(BIN_DIR)
	@echo "Building SDK examples..."
	@for example in $(EXAMPLES); do \
		if [ -d "$(EXAMPLES_DIR)/$$example" ]; then \
			echo "  Building $$example..."; \
			cd $(EXAMPLES_DIR)/$$example && $(GOBUILD) -o ../../$(BIN_DIR)/$$example . && cd - > /dev/null; \
		fi; \
	done
	@echo "Build complete: $(BIN_DIR)/"
	@ls -la $(BIN_DIR)/

# Create bin directory
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Run all tests
test:
	@echo "Running SDK tests..."
	$(GOTEST) -v ./...
	@echo "Tests complete"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report:"
	@$(GOCMD) tool cover -func=coverage.out

# Run integration tests (requires git, optionally gopls/pyright/tsserver)
test-integration:
	@echo "Running integration tests (Git only)..."
	@echo "Note: LSP tests will be skipped if language servers are not installed"
	@cd codegen && $(GOTEST) -v -tags=integration -timeout=10m -run 'Test(FullWorkflow|MultiRepo|Worktree|Cleanup|EdgeCases)'
	@echo "Integration tests complete"

# Run all integration tests including LSP
test-integration-all:
	@echo "Running all integration tests (including LSP)..."
	@echo "Installing language servers if needed..."
	@command -v gopls > /dev/null || echo "  gopls not found - Go LSP tests will be skipped"
	@command -v pyright-langserver > /dev/null || echo "  pyright-langserver not found - Python LSP tests will be skipped"
	@command -v typescript-language-server > /dev/null || echo "  typescript-language-server not found - TypeScript LSP tests will be skipped"
	@cd codegen && $(GOTEST) -v -tags=integration -timeout=15m
	@echo "All integration tests complete"

# Generate coverage HTML report
coverage-html: test-coverage
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage HTML report: coverage.html"

# golangci-lint binary, pinned + repo-local (under bin/tools/, gitignored).
# Built from source with the local Go toolchain so its embedded Go version is
# never below go.mod's target (golangci v2 refuses to load otherwise).
GOLANGCI_LINT := bin/tools/golangci-lint

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) to $(CURDIR)/bin/tools..."
	@mkdir -p $(CURDIR)/bin/tools
	@GOBIN=$(CURDIR)/bin/tools GOFLAGS=-mod=mod \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Baseline revision for the incremental lint gate. PRs lint against the
# merge-base with origin/main; override for local branches as needed.
LINT_BASE ?= origin/main

# lint — the BLOCKING gate (QUALITY-BARS §3), no `|| true` swallow. Runs the
# full golangci-lint suite but reports only NEW issues since LINT_BASE, so the
# pre-existing style backlog (burndown tracked in sdk#385) is baselined while
# any new violation of any enabled linter — the dead-code gate (`unused`)
# included — fails. This is the same invocation the CI `lint` job uses.
lint: $(GOLANGCI_LINT)
	@echo "Running linter (blocking; new since $(LINT_BASE))..."
	$(GOLANGCI_LINT) run --new-from-merge-base=$(LINT_BASE) ./...
	@node scripts/lint-pagination.mjs
	@node scripts/lint-allowed-identities.mjs

# lint-all — full-tree, non-baselined. Surfaces the entire backlog for the
# sdk#385 burndown. Not wired into `check` until the backlog is cleared.
.PHONY: lint-all
lint-all: $(GOLANGCI_LINT)
	@echo "Running linter (full tree; informational)..."
	$(GOLANGCI_LINT) run ./...

# Dead-code gate only — fast, whole-tree, blocking. (QUALITY-BARS §3.)
# Clears once the sdk#385 `unused` backlog (22 items at the gate-landing
# baseline) is burned down, after which it can join `check`.
.PHONY: lint-deadcode
lint-deadcode: $(GOLANGCI_LINT)
	@echo "Running dead-code gate (unused, blocking)..."
	$(GOLANGCI_LINT) run --enable-only=unused ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Vet code
# Note: protoresolver is excluded because it intentionally defines an UnmarshalJSON method
# with a custom signature (not implementing json.Unmarshaler) which go vet stdmethods flags
# as a false positive. The graphrag, serve, and eval packages are excluded because their test
# files import api/gen/toolspb which is only generated when tool protos are present (external
# tooling from core/tools/). These exclusions are tracked in pre-existing issue #toolspb.
vet:
	@echo "Vetting code..."
	$(GOCMD) vet $(shell go list ./... | grep -v 'github.com/zeroroot-ai/sdk/protoresolver' | grep -v 'github.com/zeroroot-ai/sdk/graphrag$$' | grep -v 'github.com/zeroroot-ai/sdk/eval' | grep -v 'github.com/zeroroot-ai/sdk/serve')

# Tidy modules
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy
	@for example in $(EXAMPLES); do \
		if [ -d "$(EXAMPLES_DIR)/$$example" ]; then \
			echo "  Tidying $$example..."; \
			cd $(EXAMPLES_DIR)/$$example && $(GOMOD) tidy && cd - > /dev/null; \
		fi; \
	done

# Build a sandboxed-tool runner image. Usage: make tool-runner-image TOOL=hello
# Produces ghcr.io/zeroroot-ai/gibson-tool-runner:<TOOL>-dev
TOOL ?= hello
TOOL_IMAGE_TAG ?= ghcr.io/zeroroot-ai/gibson-tool-runner:$(TOOL)-dev
SETEC_K3S_KUBECONFIG ?= ../../opensource/setec/development/k3s/kubeconfig

tool-runner-image:
	@if [ ! -d "examples/tool-runner-$(TOOL)" ]; then \
		echo "ERROR: examples/tool-runner-$(TOOL) does not exist"; exit 1; \
	fi
	@echo "Building $(TOOL_IMAGE_TAG)..."
	docker build -f examples/tool-runner-$(TOOL)/Dockerfile -t $(TOOL_IMAGE_TAG) .

# Import the built image into the local k3s containerd (no remote registry needed in dev).
# Requires: SETEC_K3S_KUBECONFIG points at a working k3s kubeconfig.
tool-runner-load: tool-runner-image
	@echo "Loading $(TOOL_IMAGE_TAG) into local k3s containerd..."
	docker save $(TOOL_IMAGE_TAG) | sudo k3s ctr images import -

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) ./...
	$(GOMOD) tidy
	@for example in $(EXAMPLES); do \
		if [ -d "$(EXAMPLES_DIR)/$$example" ]; then \
			echo "  Dependencies for $$example..."; \
			cd $(EXAMPLES_DIR)/$$example && $(GOGET) ./... && $(GOMOD) tidy && cd - > /dev/null; \
		fi; \
	done

# bootstrap installs every dev/CI tool this repo needs from its pinned sources,
# so a fresh checkout reaches a buildable state with one command. Part of the
# uniform Makefile contract (RESTRUCTURE-QUALITY-BARS §1):
#   make bootstrap | build | test | check | image
# "Just works" = `make bootstrap` is the same command in every repo.
.PHONY: bootstrap
bootstrap: $(GOLANGCI_LINT)
	@echo "Bootstrapping SDK toolchain (Go $$(go env GOVERSION))..."
	@command -v node > /dev/null || { echo "ERROR: Node.js (>=20) required — see .tool-versions"; exit 1; }
	$(GOMOD) download
	@echo "Bootstrap complete."

# image builds the reference sandboxed-tool container image. Part of the uniform
# contract; delegates to the parameterised tool-runner-image recipe (TOOL=hello).
.PHONY: image
image: tool-runner-image

# Coverage gate — QUALITY-BARS §4: uniform 80% package floor + 85% diff-coverage,
# blocking. Two halves, one shared coverage profile (coverage.out):
#
#   * coverage-floor  — every package ≥80%. The pre-existing sub-floor backlog
#     (sdk#388) is BASELINED in scripts/coverage-baseline.json: baselined
#     packages may not regress and must ratchet up; no NEW sub-floor package may
#     land. (Same baseline-the-past design as the lint gate, PR #386.)
#   * diff-coverage   — every Go statement ADDED/CHANGED on this branch since the
#     merge-base must be ≥85% covered. This is the teeth on new code: regardless
#     of the floor backlog, new debt cannot land.
#
# This supersedes the old auth-critical-only floors (daemonclient ≥60%, agent
# ≥90%); those packages are now covered by the uniform floor.
#
# COVERAGE_PROFILE is the shared profile; COVERAGE_BASE is the diff-coverage
# baseline ref (CI passes the PR target branch).
COVERAGE_PROFILE ?= coverage.out
COVERAGE_BASE ?= origin/main

# coverage-profile regenerates the shared profile across all packages. Build/test
# failures are surfaced (no stdout swallow) so the gate is debuggable in CI.
.PHONY: coverage-profile
coverage-profile:
	@echo "Generating coverage profile ($(COVERAGE_PROFILE))..."
	@$(GOTEST) -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic ./...

# check-coverage — the blocking gate (floor + diff). Regenerates the profile
# then runs both halves. Wired into `check` and the CI `coverage` job.
.PHONY: check-coverage
check-coverage: coverage-profile
	@node scripts/coverage-floor.mjs $(COVERAGE_PROFILE)
	@node scripts/diff-coverage.mjs $(COVERAGE_PROFILE) --base $(COVERAGE_BASE)

# coverage-baseline regenerates scripts/coverage-baseline.json from the current
# tree. Run after burning down coverage in sdk#388 (or when a new package
# legitimately must be baselined) and commit the result — the gate fails if a
# package graduates past 80% but is left in the baseline (drift).
.PHONY: coverage-baseline
coverage-baseline: coverage-profile
	@node scripts/coverage-floor.mjs $(COVERAGE_PROFILE) --baseline

# Verify SDK has no dependency on the private gibson daemon repo.
# The SDK is the lowest-level module (like k8s.io/api or go.etcd.io/etcd/api).
# The daemon imports the SDK, never the reverse.
check-no-gibson:
	# go.mod check: mechanical grep, still cheapest at this layer.
	@if grep -q 'zeroroot-ai/gibson' go.mod; then \
		echo "ERROR: SDK go.mod must not depend on github.com/zeroroot-ai/gibson"; exit 1; \
	fi
	# .go import check: typed AST inspection via the ast-checks harness.
	# Replaces the previous grep — catches aliased imports the grep misses.
	# See check_no_gibson_test.go.
	@go test -run TestNoGibsonImport ./...
	@echo "No gibson dependency found — SDK boundary is clean."

# Run all checks before commit.
#
# golangci-lint is deliberately excluded: it type-checks the whole module (~3 GB
# resident, a full core for minutes), and the repos in this workspace share one
# 8-core machine. CI runs it directly (`go-ci.yml` calls `make lint LINT_BASE=…`),
# so nothing is lost here. Run `make lint` by hand when you want it.
check: fmt vet test check-coverage check-no-gibson check-buf-pinned proto-breaking
	@echo "All checks passed! (golangci-lint not included — run 'make lint' separately)"

# Proto generation
#
# Plugins install to a local toolchain dir ($(TOOLS_BIN_DIR), gitignored
# under bin/) and the `proto` recipe prepends that dir to PATH only for
# `buf generate`. This isolation prevents silent version drift: a
# contributor's global $GOBIN may contain a different version of
# protoc-gen-go-grpc from another project, and `go install pkg@v` can
# behave unpredictably across $GOBIN states (sdk#75 — root cause of how
# sdk#68's v1.6.0 output ever got committed despite the v1.5.1 pin).
#
# Belt + suspenders: we delete the local binary before each install AND
# grep the --version output to fail loud on mismatch.
TOOLS_BIN_DIR := $(CURDIR)/bin/tools

# BUF_GENERATE is the ONLY sanctioned way to invoke `buf generate`: it
# puts the pinned toolchain dir at the front of PATH so buf's `local:`
# protoc-gen-go / protoc-gen-go-grpc plugins resolve to the versions
# proto-deps installed, never the contributor's ambient PATH. Every
# recipe that uses it must also depend on proto-deps. Recurrence guard:
# check-buf-pinned (sdk#68 -> #75 -> #427 were all call sites that
# bypassed the pin and regenerated against ambient plugin versions).
BUF_GENERATE := PATH="$(TOOLS_BIN_DIR):$$PATH" $(BUF) generate

proto-deps:
	@echo "Installing protoc plugins to $(TOOLS_BIN_DIR)..."
	@mkdir -p $(TOOLS_BIN_DIR)
	@rm -f $(TOOLS_BIN_DIR)/protoc-gen-go $(TOOLS_BIN_DIR)/protoc-gen-go-grpc
	@GOBIN=$(TOOLS_BIN_DIR) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@GOBIN=$(TOOLS_BIN_DIR) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@# Fail loud on version mismatch. The trailing v-stripped match is
	@# because the binary prints "1.5.1" not "v1.5.1".
	@_want="$$(echo $(PROTOC_GEN_GO_GRPC_VERSION) | sed 's/^v//')"; \
	  _got="$$($(TOOLS_BIN_DIR)/protoc-gen-go-grpc --version 2>&1 | awk '{print $$NF}')"; \
	  if [ "$$_got" != "$$_want" ]; then \
	    echo "ERROR: protoc-gen-go-grpc version mismatch (expected $$_want, got $$_got)"; \
	    exit 1; \
	  fi
	@echo "Installed protoc-gen-go@$(PROTOC_GEN_GO_VERSION), protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION) (isolated to $(TOOLS_BIN_DIR))"

proto: proto-deps proto-clean
	@echo "Generating Go code from SDK proto files via Buf..."
	@mkdir -p $(PROTO_OUT)
	@$(BUF_GENERATE)
	@echo "Proto generation complete"
	@echo "NOTE: rendered authz registry artifacts now live in core/gibson/internal/authz/registry/."
	@echo "      Run 'make proto-authz-registry-emit' manually to verify proto annotation changes locally."

# proto-authz-registry was previously a dep of proto, but the proto recipe
# does not consume the binary it builds — only proto-authz-registry-emit
# and lint-authz do, and they declare it directly. Keeping it as a dep of
# proto created a chicken-and-egg failure on a clean tree because
# proto-clean wipes api/gen/ and the gen tool imports from there. Removed.

# proto-authz-registry: build the authz-registry-gen binary that emits
# auth/registry/{registry.go,registry.yaml,permissions.ts} from the SDK's
# proto annotations. Spec: unified-identity-and-authorization Requirement 7.5.
proto-authz-registry:
	@echo "Building authz-registry-gen..."
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) -o $(BIN_DIR)/protoc-gen-authz-registry ./cmd/authz-registry-gen

# sdk-bump: cross-repo meta-tool that opens bump PRs in every SDK consumer.
# Spec: unified-identity-and-authorization Requirement 18.1/18.2 (Task 9.1).
sdk-bump:
	@echo "Building sdk-bump..."
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) -o $(BIN_DIR)/sdk-bump ./cmd/sdk-bump

# proto-authz-registry-emit: contributor-facing verification target. Builds
# the registry to a temp dir (default: tmp/authz/) so you can verify that
# proto annotation changes render correctly, WITHOUT committing the output.
#
# The rendered registry now lives in core/gibson/internal/authz/registry/.
# SDK `make proto` no longer runs this target automatically. Use it when:
#   - You add or rename a gRPC method (verify it gets an entry).
#   - You change a (gibson.auth.v1.authz) annotation (verify the YAML output).
# After verifying, push the SDK change; then bump the SDK pin in
# zeroroot-ai/gibson and run `make authz-registry` there to commit the update.
#
# Spec: private-authz-registry Task 6.
OUTPUT_DIR ?= tmp/authz
proto-authz-registry-emit: proto-authz-registry
	@echo "Generating authz artifacts to $(OUTPUT_DIR) (verification only — not committed)..."
	@mkdir -p $(OUTPUT_DIR)
	@$(BUF) build -o /tmp/sdk-fds.binpb
	@$(BIN_DIR)/protoc-gen-authz-registry -input /tmp/sdk-fds.binpb -output $(OUTPUT_DIR)
	@rm -f /tmp/sdk-fds.binpb
	@echo "Registry artifacts written to $(OUTPUT_DIR)/ (not committed; add to .gitignore if needed)."

# lint-authz: fail-closed annotation guard. The authz-registry-gen tool
# refuses to emit when any RPC lacks the (gibson.auth.v1.authz)
# annotation; this target runs it in check-only mode (output goes to a
# tmp dir that we discard) so CI fails on missing annotations without
# touching the committed registry artifacts. Spec: Requirement 7.3, 14.1.
lint-authz: proto-authz-registry
	@echo "Checking every RPC has a (gibson.auth.v1.authz) annotation..."
	@mkdir -p /tmp/authz-lint-out
	@$(BUF) build -o /tmp/sdk-fds-lint.binpb
	@$(BIN_DIR)/protoc-gen-authz-registry -input /tmp/sdk-fds-lint.binpb -output /tmp/authz-lint-out
	@rm -f /tmp/sdk-fds-lint.binpb
	@rm -rf /tmp/authz-lint-out
	@echo "All RPCs annotated."

proto-clean:
	@echo "Cleaning generated proto files..."
	@rm -rf $(PROTO_OUT)

# check-buf-pinned: fail if any recipe invokes `$(BUF) generate`
# directly instead of via $(BUF_GENERATE). Third recurrence of this
# drift class (sdk#68, #75, #427): each time, a call site landed
# without the pinned PATH and regenerated api/gen/ against whatever
# protoc-gen-go/-go-grpc the contributor happened to have on PATH.
check-buf-pinned:
	@bad="$$(grep -nE '\$$\(BUF\)[[:space:]]+generate' Makefile | grep -vE '^[0-9]+:[[:space:]]*#' | grep -v 'BUF_GENERATE' || true)"; \
	if [ -n "$$bad" ]; then \
		echo "ERROR: raw 'buf generate' call site(s) bypass the pinned toolchain (sdk#427)."; \
		echo "Invoke \$$(BUF_GENERATE) instead:"; \
		echo "$$bad"; \
		exit 1; \
	fi
	@echo "All buf generate call sites use the pinned toolchain."

# ensure-cue: fail-fast guard for the cue binary, pinned via
# `go install cuelang.org/go/cmd/cue@latest`. The version is taken
# from cue.mod/module.cue's language.version field.
# Spec: mission-authoring-cue Requirement 1 (task 1).
ensure-cue:
	@command -v cue >/dev/null 2>&1 || { \
		echo "ERROR: cue binary not found on PATH." >&2; \
		echo "  Install with: go install cuelang.org/go/cmd/cue@v0.16.1" >&2; \
		echo "  Or via your tool-pinning (mise/asdf) — pin v0.16.1." >&2; \
		exit 1; \
	}
	@echo "cue: $$(cue version | head -1)"

# cue-defs: import the gibson mission + daemon protos as CUE
# definitions. Output lives alongside each .proto as
# *_proto_gen.cue (CUE's import-in-place convention). Buf-validate
# proto resolved from the local buf cache.
# Spec: mission-authoring-cue Requirement 1 (task 2).
#
# Two bugs are fixed here (closes #48):
#
# Bug 1 — Wrong import path: `cue import proto` derives CUE import paths
#   from the proto go_package option, which uses `api/gen/...` (correct for
#   Go codegen). But *_proto_gen.cue files live at `api/proto/...`. A sed
#   pass after generation rewrites every `api/gen/` occurrence to `api/proto/`.
#
# Bug 2 — Package alias mismatch: the original `make cue-defs` passed `-p v1`
#   which forced all generated CUE files to declare `package v1`. When one
#   file imports another (e.g., missionpb imports typespb), CUE requires the
#   target directory to contain files with the matching package name. Because
#   every file said `package v1`, CUE could not find a package named `typespb`
#   at the import path, causing "no files in package directory" errors. The
#   fix is to drop `-p v1` so CUE uses the Go package names from go_package
#   (`typespb`, `commonpb`, `missionpb`, etc.) as both the file's package
#   declaration and the cross-package alias, which then match correctly.
BUF_VALIDATE_DIR := $(shell find $$HOME/.cache/buf -type d -path '*protovalidate*/files' 2>/dev/null | head -1)

# cue-defs generates CUE bindings for the subset of protos that have
# cross-language consumers (mission DSL, daemon API). Dependency order
# matters: common → types → mission; common → manifest → daemon.
# auth and admin/grants are generated as dependencies of the chain.
cue-defs: ensure-cue
	@if [ -z "$(BUF_VALIDATE_DIR)" ]; then \
		echo "ERROR: protovalidate proto cache not found." >&2; \
		echo "  Run \`buf dep update\` from this directory first." >&2; \
		exit 1; \
	fi
	@echo "Importing protos as CUE (dependency order: common→capability→admin→identity; common→job→types→mission; common→manifest→daemon)..."
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/common/v1/gibson_common.proto
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/capability/v1/capability.proto
	@# gibson.admin.v1.grants moved to platform-sdk in slice #108. The
	@# capability.v1 line above replaces it as the dependency identity.v1
	@# uses for CapabilityGrantInfo.
	@# job before types: gibson.types.v1.Task carries a gibson.job.v1.JobSpec
	@# (gibson#1706). gibson.job.v1 imports nothing above gibson.common.v1.
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/job/v1/job.proto
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/types/v1/types.proto
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/mission/v1/mission_definition.proto
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/identity/v1/identity.proto
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/manifest/v1/manifest.proto
	@cue import proto -f --files \
		-I api/proto -I "$(BUF_VALIDATE_DIR)" \
		api/proto/gibson/daemon/v1/daemon.proto
	@echo "Fixing generated CUE import paths (#48, bug 1)..."
	@# Rewrite api/gen/ -> api/proto/ in all generated CUE import paths.
	@# api/gen/ is the Go code output directory; CUE packages live in api/proto/.
	@find api/proto -name '*_proto_gen.cue' | xargs sed -i \
		's|"github\.com/zeroroot-ai/sdk/api/gen/|"github.com/zeroroot-ai/sdk/api/proto/|g'
	@echo "CUE definitions emitted alongside .proto files (*_proto_gen.cue)"

# mission-jsonschema: emit a JSON Schema Draft 2020-12 document for
# the gibson.mission.v1.MissionDefinition root message and its
# transitive types. Consumed by the dashboard's YAML editor for
# inline completion / hover / validation. The daemon remains the
# authoritative validator at runtime via protovalidate.
# Spec: mission-authoring-cue Requirement 9.
mission-jsonschema:
	@echo "Building mission-jsonschema-gen..."
	@mkdir -p $(BIN_DIR) gen
	@$(GOBUILD) -o $(BIN_DIR)/mission-jsonschema-gen ./cmd/mission-jsonschema-gen
	@echo "Building FileDescriptorSet..."
	@$(BUF) build -o /tmp/mission-fds.binpb
	@echo "Generating JSON Schema..."
	@$(BIN_DIR)/mission-jsonschema-gen -input /tmp/mission-fds.binpb -output gen/mission-definition.schema.json
	@rm -f /tmp/mission-fds.binpb
	@echo "Schema written to gen/mission-definition.schema.json"

# mission-docs: emit MDX + glossary documentation for the mission
# DSL. Produces verbs.mdx, nouns.mdx, schema-ref.mdx, templates.mdx,
# glossary.json under gen/mission-docs/.
# Spec: mission-authoring-cue Requirement 8.
mission-docs:
	@echo "Building mission-docs-gen..."
	@mkdir -p $(BIN_DIR) gen/mission-docs
	@$(GOBUILD) -o $(BIN_DIR)/mission-docs-gen ./cmd/mission-docs-gen
	@echo "Building FileDescriptorSet..."
	@$(BUF) build -o /tmp/mission-fds.binpb
	@echo "Generating MDX docs..."
	@$(BIN_DIR)/mission-docs-gen -input /tmp/mission-fds.binpb -output gen/mission-docs
	@rm -f /tmp/mission-fds.binpb
	@echo "Docs written to gen/mission-docs/"

# mission-authoring-bundle: assemble the OCI-publishable bundle
# from the mission-jsonschema, mission-docs, and (forthcoming)
# cue-defs targets. Output is a tarball ready for `oras push` by
# the publish-mission-authoring CI workflow.
# Spec: mission-authoring-cue Requirement 2.
mission-authoring-bundle: mission-jsonschema mission-docs
	@echo "Assembling mission-authoring bundle..."
	@rm -rf gen/mission-authoring-bundle .tmp/mission-authoring
	@mkdir -p .tmp/mission-authoring/docs
	@cp gen/mission-definition.schema.json .tmp/mission-authoring/
	@cp -r gen/mission-docs/*.mdx .tmp/mission-authoring/docs/ 2>/dev/null || true
	@cp gen/mission-docs/glossary.json .tmp/mission-authoring/ 2>/dev/null || true
	@mkdir -p gen
	@tar -czf gen/mission-authoring-bundle.tar.gz -C .tmp/mission-authoring .
	@rm -rf .tmp/mission-authoring
	@echo "Bundle: gen/mission-authoring-bundle.tar.gz"
	@ls -la gen/mission-authoring-bundle.tar.gz

proto-breaking:
	@TARGET=$${GITHUB_BASE_REF:-$${CI_MERGE_REQUEST_TARGET_BRANCH:-main}}; \
	if $(BUF) breaking --against ".git#branch=$$TARGET"; then \
		echo "✅ No breaking proto changes detected"; \
	else \
		if echo "$$PR_BODY" | grep -q "buf:breaking:ignore"; then \
			echo "⚠️ Breaking proto changes detected but escape hatch enabled"; \
			exit 0; \
		else \
			echo "❌ Breaking proto changes detected. Add 'buf:breaking:ignore' to PR body to override."; \
			exit 1; \
		fi; \
	fi

# Taxonomy generation from YAML
taxonomy-gen:
	@echo "Generating taxonomy from YAML..."
	@mkdir -p $(TAXONOMYPB_OUT) $(PROTO_DIR)/taxonomy/v1 graphrag/domain graphrag/validation graphrag/query graphrag/taxonomy
	@rm -f $(PROTO_DIR)/taxonomy.proto
	@echo "  Generating proto and domain/validators (package: domain)..."
	@go run ./cmd/taxonomy-gen \
		--base $(TAXONOMY_YAML) \
		--output-proto $(PROTO_DIR)/taxonomy/v1/taxonomy.proto \
		--output-domain graphrag/domain/domain_generated.go \
		--output-validators graphrag/validation/validators_generated.go \
		--package domain
	@echo "  Generating constants (package: graphrag)..."
	@go run ./cmd/taxonomy-gen \
		--base $(TAXONOMY_YAML) \
		--output-constants graphrag/constants_generated.go \
		--package graphrag
	@echo "  Generating query builders (package: query)..."
	@go run ./cmd/taxonomy-gen \
		--base $(TAXONOMY_YAML) \
		--output-query graphrag/query/query_generated.go \
		--package query
	@echo "  Generating SDK helpers (package: graphrag)..."
	@go run ./cmd/taxonomy-gen \
		--base $(TAXONOMY_YAML) \
		--output-helpers graphrag/helpers_generated.go \
		--package graphrag
	@echo "  Generating relationships mapping (package: taxonomy)..."
	@go run ./cmd/taxonomy-gen \
		--base $(TAXONOMY_YAML) \
		--output-relationships graphrag/taxonomy/relationships_generated.go \
		--package taxonomy
	@echo "Formatting generated files..."
	@gofmt -w graphrag/domain/domain_generated.go \
		graphrag/validation/validators_generated.go \
		graphrag/constants_generated.go \
		graphrag/query/query_generated.go \
		graphrag/helpers_generated.go \
		graphrag/taxonomy/relationships_generated.go
	@echo "Taxonomy generation complete"

# Generate taxonomy proto
taxonomy-proto: taxonomy-gen proto-deps
	@echo "Generating Go code from taxonomy.proto via Buf..."
	@$(BUF_GENERATE)
	@echo "Taxonomy proto generation complete"

# Full generate: YAML -> Proto -> Go code
# Always starts with a clean api/gen/ to prevent orphan files from renamed/deleted protos
generate: proto-deps proto-clean taxonomy-gen
	@echo "Generating Go code from all proto files via Buf..."
	@mkdir -p $(PROTO_OUT)
	@$(BUF_GENERATE)
	@echo "All generation complete!"

# Verify idempotent generation (periodic CI check, not on every PR)
# Runs generate twice and diffs output to ensure deterministic results
verify-idempotent:
	@echo "Verifying idempotent generation..."
	$(MAKE) generate
	@cp -r $(PROTO_OUT) /tmp/gen-first
	$(MAKE) generate
	@diff -r $(PROTO_OUT) /tmp/gen-first || (echo "ERROR: Generation is not idempotent" && rm -rf /tmp/gen-first && exit 1)
	@rm -rf /tmp/gen-first
	@echo "Generation is idempotent!"

# Release preparation: stage generated files for a tagged release
# Generated proto files are gitignored for development but MUST be included in
# tagged releases for Go module proxy compatibility. External consumers using
# 'go get github.com/zeroroot-ai/sdk' need gen/ files present in the tagged commit.
# Workflow: make release-prep -> commit -> git tag vX.Y.Z -> push tag ->
#           git rm --cached -r api/gen/ -> commit -> push main
release-prep: generate
	@echo ""
	@echo "Generated bindings refreshed under api/gen/ — now TRACKED SOURCE (ADR-0039)."
	@echo "No force-add and no post-tag 'git rm --cached' ritual. Commit normally:"
	@echo "  git add api/gen/ && git commit"
	@echo "Releases go through release-please; the CI 'proto bindings drift gate'"
	@echo "verifies the committed bindings match the protos."

# Help target
help:
	@echo "Gibson SDK - Makefile Targets"
	@echo ""
	@echo "  Uniform contract:  bootstrap | build | test | check | image"
	@echo "  make bootstrap     - Install pinned dev/CI tooling (one-command setup)"
	@echo "  make build         - Compile all packages (library type-check)"
	@echo "  make image         - Build the reference sandboxed-tool image (TOOL=hello)"
	@echo "  make examples      - Build example binaries to bin/"
	@echo "  make test               - Run all tests"
	@echo "  make test-race          - Run tests with race detection"
	@echo "  make test-coverage      - Run tests with coverage"
	@echo "  make coverage-html      - Generate HTML coverage report"
	@echo "  make test-integration   - Run integration tests (Git only)"
	@echo "  make test-integration-all - Run all integration tests (requires LSP servers)"
	@echo "  make lint          - Run golangci-lint"
	@echo "  make fmt           - Format Go code"
	@echo "  make vet           - Run go vet"
	@echo "  make tidy          - Tidy go modules"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make deps          - Download dependencies"
	@echo "  make check         - Run all checks (fmt, vet, lint, test)"
	@echo "  make check-coverage - Enforce per-package coverage thresholds (daemonclient ≥60%, agent ≥90%)"
	@echo "  make proto         - Generate Go code from proto files"
	@echo "  make proto-deps    - Install protoc plugins"
	@echo "  make proto-clean   - Remove generated proto files"
	@echo "  make proto-breaking - Check for breaking proto changes against target branch"
	@echo "  make taxonomy-gen  - Generate taxonomy from YAML (proto, domain, validators, helpers)"
	@echo "  make taxonomy-proto- Generate Go code from taxonomy.proto"
	@echo "  make generate      - Full generation: YAML -> Proto -> Go"
	@echo "  make verify-idempotent - Verify generation is idempotent (periodic CI check)"
	@echo "  make release-prep   - Stage generated files for a release tag"
	@echo "  make help          - Show this help message"
	@echo ""
	@echo "Note: The SDK is a library. 'make examples' builds the example applications."
