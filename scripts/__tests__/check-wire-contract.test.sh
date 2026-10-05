#!/usr/bin/env bash
# Fixture for `make proto-breaking` (sdk#177). The go.mod boundary is the org
# permissive-floor workflow, and its fixture is in zeroroot-ai/.github.
#
# Each red case asserts its reason, so a check that starts failing for a
# different reason is a test failure and not a pass.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"
wt="$tmp/wt"
trap 'git worktree remove --force "$wt" >/dev/null 2>&1; rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

# ---- wire contract ---------------------------------------------------------
# The real target against the real protos, in a throwaway worktree of HEAD. The
# base is HEAD itself, so an unchanged tree has nothing to break.
head_sha="$(git rev-parse HEAD)"
git worktree add --detach -q "$wt" "$head_sha" || { echo "FAIL: could not create the fixture worktree"; exit 1; }
git -C "$wt" branch -f wire-fixture-base "$head_sha" >/dev/null

breaking() { ( cd "$wt" && GITHUB_BASE_REF=wire-fixture-base make --no-print-directory proto-breaking 2>&1 ); }

out="$(breaking)"; rc=$?
if [ "$rc" -eq 0 ] && [[ "$out" == *"No breaking proto changes"* ]]; then ok "an unchanged proto tree passes"
else bad "an unchanged proto tree (rc=$rc): $out"; fi

# A field that keeps its name and changes its number: old messages decode into
# the wrong field. This is the change the WIRE rule exists to stop.
proto="$wt/api/proto/gibson/bank/v1/bank.proto"
grep -q '    string tenant_id = 2;' "$proto" || { echo "FAIL: the fixture field is gone from bank.proto; pick another"; exit 1; }
sed -i 's/    string tenant_id = 2;/    string tenant_id = 902;/' "$proto"
git -C "$wt" -c user.name=fixture -c user.email=fixture@example.invalid commit -qam "renumber a field"
out="$(breaking)"; rc=$?
if [ "$rc" -ne 0 ] && [[ "$out" == *"Breaking proto changes detected"* ]]; then ok "a renumbered field is refused"
else bad "a renumbered field (rc=$rc): $out"; fi

# The old escape: a marker in the pull request body. It must do nothing.
out="$( cd "$wt" && PR_BODY="buf:breaking:ignore" GITHUB_BASE_REF=wire-fixture-base make --no-print-directory proto-breaking 2>&1 )"; rc=$?
if [ "$rc" -ne 0 ]; then ok "the buf:breaking:ignore marker does not override the check"
else bad "the buf:breaking:ignore marker still overrides the check: $out"; fi
git -C "$wt" branch -D wire-fixture-base >/dev/null 2>&1 || true
git branch -D wire-fixture-base >/dev/null 2>&1 || true

echo
echo "passed=$PASS failed=$FAIL"
if [ "$FAIL" -eq 0 ] && [ "$PASS" -lt 3 ]; then echo "FAIL: only $PASS case(s) ran, want 3"; exit 1; fi
[ "$FAIL" -eq 0 ]
