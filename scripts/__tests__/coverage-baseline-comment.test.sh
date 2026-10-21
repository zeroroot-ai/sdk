#!/usr/bin/env bash
# Fixture for scripts/coverage-floor.mjs --baseline.
#
# A --baseline run must keep the full _comment rationale and write the Go
# floor from go.mod into it. A run that drops either one must fail here.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

printf 'module example.com/m\n\ngo 9.99.9\n' > "$tmp/go.mod"
cat > "$tmp/cover.out" <<'P'
mode: atomic
example.com/m/low/a.go:1.1,2.2 4 1
example.com/m/low/a.go:3.1,4.2 6 0
P

COVERAGE_BASELINE_PATH="$tmp/baseline.json" COVERAGE_GO_MOD_PATH="$tmp/go.mod" \
  node scripts/coverage-floor.mjs "$tmp/cover.out" --baseline >/dev/null 2>&1
rc=$?
if [ "$rc" -eq 0 ]; then ok "baseline run exits 0"; else bad "baseline run rc=$rc"; fi

comment="$(node -e 'process.stdout.write(JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"))._comment)' "$tmp/baseline.json")"

# 1. The comment names the Go floor from go.mod.
[[ "$comment" == *"go9.99.9"* ]] && ok "comment names the go.mod floor" || bad "comment lacks go9.99.9: $comment"
# 2. The comment says the numbers depend on the toolchain.
[[ "$comment" == *"toolchain-dependent"* ]] && ok "comment says toolchain-dependent" || bad "comment lacks toolchain-dependent"
# 3. The comment says to re-record in the PR that moves the floor.
[[ "$comment" == *"re-record this file in the same PR that moves the floor"* ]] && ok "comment says re-record in the floor PR" || bad "comment lacks the re-record rule"
# 4. The sub-floor package is recorded.
grep -q '"example.com/m/low": 40' "$tmp/baseline.json" && ok "sub-floor package recorded" || bad "package row missing"

echo "$PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
