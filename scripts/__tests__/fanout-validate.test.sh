#!/usr/bin/env bash
# Fixture for scripts/fanout-validate.sh. Each REQUIRED RED case is an input that
# would reach a branch name or a command argument if the check let it through.
#
# Run by the `ci-required-selftest` job in .github/workflows/go-ci.yml, and
# locally with `bash scripts/__tests__/fanout-validate.test.sh`.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK="${HERE}/fanout-validate.sh"
pass=0; fail=0

# expect <want-rc> <kind> <value>
expect() {
  local want="$1" kind="$2" value="$3" got
  bash "$CHECK" "$kind" "$value" >/dev/null 2>&1
  got=$?
  if [ "$got" = "$want" ]; then
    echo "ok   $kind '$value'"; pass=$((pass + 1))
  else
    echo "FAIL $kind '$value' (wanted rc=$want, got rc=$got)"; fail=$((fail + 1))
  fi
}

echo "--- pass ---"
expect 0 tag  v0.208.0
expect 0 tag  v1.2.3
expect 0 slug plugins-github
expect 0 slug examples-minimal-agent
expect 0 slug a

echo "--- required red: tag ---"
expect 1 tag  ""
expect 1 tag  0.208.0
expect 1 tag  v0.208
expect 1 tag  v0.208.0-rc1
expect 1 tag  'v0.208.0; id'
expect 1 tag  'v0.208.0 evil'
expect 1 tag  '$(id)'
expect 1 tag  $'v0.208.0\nx'

echo "--- required red: slug ---"
expect 1 slug ""
expect 1 slug v1
expect 1 slug V1
expect 1 slug 1abc
expect 1 slug plugins-v2
expect 1 slug Plugins
expect 1 slug 'a b'
expect 1 slug 'a/../b'
expect 1 slug 'a;id'
expect 1 slug '-a'

echo "--- required red: an untrusted value does not reach the output ---"
# A newline in the value must not start a second workflow command, and the value
# must not be echoed at all.
for kind in tag slug; do
  out="$(bash "$CHECK" "$kind" $'v1\n::set-output name=x::y' 2>&1)"
  lines="$(printf '%s\n' "$out" | grep -c '^::error::')"
  total="$(printf '%s\n' "$out" | wc -l)"
  if [ "$lines" = 1 ] && [ "$total" = 1 ] && ! printf '%s' "$out" | grep -q 'set-output'; then
    echo "ok   $kind: one ::error:: line and the value left out"; pass=$((pass + 1))
  else
    echo "FAIL $kind: output was: $out"; fail=$((fail + 1))
  fi
done

echo "--- required red: usage ---"
expect 2 other v0.1.0

echo
echo "passed: $pass, failed: $fail"
[ "$fail" -eq 0 ]
