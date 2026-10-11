#!/usr/bin/env bash
# Fixture for scripts/check-examples-build.sh (sdk#268).
#
# Each case builds a small examples tree in a temporary directory. The red
# cases prove that the check fails on an example that calls a symbol which
# this sdk tree does not have, and on a tree with no example module.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
root="$(pwd)"
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

# example <examples dir> <name> <body of main>
# The go.mod and go.sum come from examples/custom-tool, so the module pins a
# real sdk release, as each example does.
example() {
  mkdir -p "$1/$2"
  sed "s#^module .*#module example.test/$2#" examples/custom-tool/go.mod > "$1/$2/go.mod"
  cp examples/custom-tool/go.sum "$1/$2/go.sum"
  cat > "$1/$2/main.go" <<GO
package main

import "github.com/zeroroot-ai/sdk/serve"

func main() {
	$3
}
GO
}

check() { bash scripts/check-examples-build.sh "$root" "$1" 2>&1; }

green() {
  out="$(check "$2")"; rc=$?
  if [ "$rc" -eq 0 ]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}
red() {
  out="$(check "$2")"; rc=$?
  if [ "$rc" -ne 0 ] && [[ "$out" == *"$3"* ]]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

# 1. An example that calls a symbol of this tree builds.
example "$tmp/good" live '_ = serve.Tool'
green "an example that calls a live symbol builds" "$tmp/good"

# 2. An example that calls a deleted symbol fails, although its go.mod pins a
#    version that the workspace replaces.
example "$tmp/bad" deleted '_ = serve.NoSuchBuilderInThisTree'
red "an example that calls a deleted symbol fails" "$tmp/bad" "undefined: serve.NoSuchBuilderInThisTree"

# 3. A tree with no example module fails: a check that built nothing proved nothing.
mkdir -p "$tmp/empty/readme-only"
red "a tree with no example module fails" "$tmp/empty" "no example module"

echo "passed: $PASS  failed: $FAIL"
[ "$FAIL" -eq 0 ]
