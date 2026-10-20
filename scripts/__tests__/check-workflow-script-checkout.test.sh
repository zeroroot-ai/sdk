#!/usr/bin/env bash
# Fixture for scripts/check-workflow-script-checkout.sh.
#
# Each REQUIRED RED case is a job that runs a script of the repo before it has
# the repo in its workspace.
#
# Run by the `ci-required-selftest` job in .github/workflows/go-ci.yml, and
# locally with `bash scripts/__tests__/check-workflow-script-checkout.test.sh`.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK="${HERE}/check-workflow-script-checkout.sh"
REAL="${HERE}/../.github/workflows"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
pass=0; fail=0

# expect <want-rc> <label> <dir>
expect() {
  local want="$1" label="$2" dir="$3" got
  bash "$CHECK" "$dir" >/dev/null 2>&1
  got=$?
  if [ "$got" = "$want" ]; then echo "ok   $label"; pass=$((pass + 1)); else echo "FAIL $label (wanted rc=$want, got rc=$got)"; fail=$((fail + 1)); fi
}

# case <name> <workflow-body>
case_dir() { mkdir -p "$work/$1"; cat > "$work/$1/w.yml"; echo "$work/$1"; }

echo "--- pass ---"
expect 0 "the committed workflows" "$REAL"
expect 0 "a job that checks out, then runs a script" "$(case_dir green <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - run: bash scripts/x.sh
Y
)"
expect 0 "a sparse checkout that covers the script" "$(case_dir sparse <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          sparse-checkout: scripts/x.sh
          sparse-checkout-cone-mode: false
      - run: bash scripts/x.sh
Y
)"
expect 0 "this repo, then another repo, with the script copied out first" "$(case_dir copied <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - run: cp scripts/x.sh "${RUNNER_TEMP}/x.sh"
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          repository: zeroroot-ai/other
      - run: bash "${RUNNER_TEMP}/x.sh"
Y
)"
expect 0 "another repo into a path leaves this repo in place" "$(case_dir pathed <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          repository: zeroroot-ai/other
          path: other
      - run: bash scripts/x.sh
Y
)"
expect 0 "this repo into path ." "$(case_dir dotpath <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          path: .
      - run: bash scripts/x.sh
Y
)"
expect 0 "a job that runs no script of the repo" "$(case_dir noscript <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - run: cp scripts/x.sh /tmp/x.sh && echo hello
Y
)"

echo "--- required red ---"
expect 1 "a job with no checkout runs bash scripts/" "$(case_dir nocheckout <<'Y'
jobs:
  resolve-tag:
    runs-on: ubuntu-latest
    steps:
      - name: Resolve tag
        run: |
          set -e
          bash scripts/fanout-validate.sh tag v1.2.3
Y
)"
expect 1 "a job with no checkout runs ./scripts/" "$(case_dir dotslash <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - run: ./scripts/x.sh
Y
)"
expect 1 "the checkout comes after the script" "$(case_dir late <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - run: bash scripts/x.sh
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
Y
)"
expect 1 "only a checkout of another repo before bash scripts/" "$(case_dir other <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          repository: zeroroot-ai/other
      - run: bash scripts/x.sh
Y
)"
expect 1 "this repo, then another repo, then ./scripts/" "$(case_dir replaced <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          repository: zeroroot-ai/${{ matrix.consumer }}
      - run: ./scripts/x.sh
Y
)"
expect 1 "this repo, then another repo into path ., then bash scripts/" "$(case_dir dotreplace <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          repository: zeroroot-ai/gibson
          path: .
      - run: bash scripts/x.sh
Y
)"
expect 1 "this repo, then another repo into the workspace path, then bash scripts/" "$(case_dir wsreplace <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          repository: zeroroot-ai/gibson
          path: ${{ github.workspace }}
      - run: bash scripts/x.sh
Y
)"
expect 1 "a sparse checkout that does not cover the script" "$(case_dir sparsemiss <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          sparse-checkout: scripts/y.sh
      - run: bash scripts/x.sh
Y
)"
expect 1 "one job is fine and a second job has no checkout" "$(case_dir second <<'Y'
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - run: bash scripts/x.sh
  b:
    runs-on: ubuntu-latest
    steps:
      - run: bash scripts/y.sh
Y
)"

echo
echo "passed: $pass, failed: $fail"
[ "$fail" -eq 0 ]
