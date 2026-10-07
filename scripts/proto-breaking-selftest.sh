#!/usr/bin/env bash
#
# proto-breaking-selftest.sh: prove that `make proto-breaking` can fail.
#
# ADR-0028, rule 5: the sdk runs `buf breaking` with the WIRE_JSON rule.
# WIRE_JSON refuses a change to what a field number or a JSON name means. It
# permits a deleted field whose number and name are reserved. Four fixtures:
#
#   1. A copy of the protos with no change must pass.
#   2. A copy with one renamed field must fail, because the JSON name changes
#      (WIRE alone would permit it). The failure must name the field.
#   3. A copy that reuses the number of a field for another type must fail.
#   4. A copy that deletes a field and reserves its number and name must pass
#      (FILE would refuse it).
#
# Usage: BUF="go tool buf" scripts/proto-breaking-selftest.sh
set -euo pipefail

BUF="${BUF:-go tool buf}"
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

log() { echo "[proto-breaking-selftest] $*"; }

FIXTURE_FILE="api/proto/gibson/capability/v1/capability.proto"
FIXTURE_LINE="  string jti = 1;"
RENAMED_LINE="  string jti_renamed = 1;"
REUSED_LINE="  int64 jti_count = 1;"
RESERVED_LINE="  reserved 1; reserved \"jti\";"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

copy() {
  mkdir -p "$1"
  cp buf.yaml buf.lock "$1/"
  cp -r --parents api/proto "$1/"
}

# mutate <dir> <new line>: replaces the fixture line in one copy.
mutate() {
  local f="$1/$FIXTURE_FILE"
  copy "$1"
  grep -qxF "$FIXTURE_LINE" "$f" || { log "FAIL: the fixture line is gone from $FIXTURE_FILE; pick another field"; exit 1; }
  awk -v old="$FIXTURE_LINE" -v new="$2" '$0 == old { print new; next } { print }' "$f" > "$f.new"
  mv "$f.new" "$f"
  grep -qxF "$2" "$f" || { log "FAIL: the fixture change did not apply in $1"; exit 1; }
}

copy "$tmp/same"
mutate "$tmp/renamed" "$RENAMED_LINE"
mutate "$tmp/reused" "$REUSED_LINE"
mutate "$tmp/reserved" "$RESERVED_LINE"

# $BUF is a command with arguments ("go tool buf"), so it is not quoted.
# shellcheck disable=SC2086
if $BUF breaking "$tmp/same" --against "$ROOT" >/dev/null 2>&1; then
  log "ok    an unchanged copy passes"
else
  log "FAIL  an unchanged copy was refused"; exit 1
fi

# shellcheck disable=SC2086
if out="$($BUF breaking "$tmp/renamed" --against "$ROOT" 2>&1)"; then
  log "FAIL  a renamed field passed: the rule does not check JSON names"; exit 1
fi
if [[ "$out" != *"jti"* ]]; then
  log "FAIL  a renamed field failed for a different reason: $out"; exit 1
fi
log "ok    a renamed field is refused"

# shellcheck disable=SC2086
if out="$($BUF breaking "$tmp/reused" --against "$ROOT" 2>&1)"; then
  log "FAIL  a reused field number passed: the rule does not check the wire"; exit 1
fi
if [[ "$out" != *'"1"'* ]]; then
  log "FAIL  a reused field number failed for a different reason: $out"; exit 1
fi
log "ok    a reused field number is refused"

# shellcheck disable=SC2086
if out="$($BUF breaking "$tmp/reserved" --against "$ROOT" 2>&1)"; then
  log "ok    a deleted field with a reserved number and name passes"
else
  log "FAIL  a deleted field with a reserved number and name was refused: $out"; exit 1
fi
log "SELFTEST PASSED"
