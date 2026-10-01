#!/usr/bin/env bash
# Fixture for scripts/check-fanout-failure-visible.sh.
#
# The checker exists because a gate that is SKIPPED reports the same as a gate
# that PASSED. Every REQUIRED RED case below is a way the fan-out run can go
# green while a consumer never got its bump.
#
# Run by the `ci-required-selftest` job in .github/workflows/go-ci.yml, and
# locally with `bash scripts/__tests__/check-fanout-failure-visible.test.sh`.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECKER="${HERE}/check-fanout-failure-visible.sh"
REAL="${HERE}/../.github/workflows/fan-out.yml"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
pass=0; fail=0
ok()  { echo "✅ $1"; pass=$((pass + 1)); }
bad() { echo "❌ $1"; fail=$((fail + 1)); }

# expect <want-rc> <label> <file>
expect() {
  local want="$1" label="$2" file="$3" got
  bash "$CHECKER" "$file" >/dev/null 2>&1
  got=$?
  if [ "$got" = "$want" ]; then ok "$label"; else bad "$label (wanted rc=$want, got rc=$got)"; fi
}

echo "--- pass: the real workflow ---"
expect 0 "the committed fan-out.yml lets a shard failure reach the run status" "$REAL"

echo "--- required red: a shard failure cannot reach the run status ---"

# 1. The exact regression: job-level continue-on-error returns.
awk '
  /^  fan-out:[[:space:]]*$/ { print; print "    continue-on-error: true"; next }
  { print }
' "$REAL" > "$work/coe.yml"
expect 1 "MUTATION job-level continue-on-error: true -> fail" "$work/coe.yml"

# 2. The gate is deleted, so nothing maps a shard failure onto the run.
grep -v 'needs\.fan-out\.result' "$REAL" > "$work/nogate.yml"
expect 1 "MUTATION the summary gate is removed -> fail" "$work/nogate.yml"

# 3. Both at once, which is the state this repo actually shipped.
awk '
  /^  fan-out:[[:space:]]*$/ { print; print "    continue-on-error: true"; next }
  { print }
' "$REAL" | grep -v 'needs\.fan-out\.result' > "$work/both.yml"
expect 1 "MUTATION continue-on-error AND no gate -> fail" "$work/both.yml"

# 4. The job is renamed out from under the check. Silence would be worse than a
#    failure here: the check would pass while asserting nothing.
sed 's/^  fan-out:[[:space:]]*$/  fanout-renamed:/' "$REAL" > "$work/renamed.yml"
expect 1 "MUTATION the fan-out job is renamed -> fail, not silently pass" "$work/renamed.yml"

# 5. A missing file must fail, not pass by vacuum.
expect 1 "a workflow file that does not exist -> fail" "$work/absent.yml"

echo "--- pass: a step-level continue-on-error is a different decision ---"
# Deeper indentation, under steps:. It does not affect needs.<job>.result, so
# the checker must not flag it — otherwise it would forbid a legitimate
# best-effort step and get switched off.
awk '
  /^      - name: Commit$/ { print "        continue-on-error: true"; print; next }
  { print }
' "$REAL" > "$work/steplevel.yml"
expect 0 "a step-level continue-on-error is not flagged" "$work/steplevel.yml"

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
