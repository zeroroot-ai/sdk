#!/usr/bin/env bash
#
# proto-breaking-selftest.sh: prove that `make proto-breaking` can fail.
#
# ADR-0028, rule 5: the sdk runs `buf breaking` with the FILE rule (sdk#208).
# FILE refuses a renamed field. WIRE permits it, because the binary encoding
# keeps only the number. So a renamed field is the fixture that tells the two
# rules apart:
#
#   1. A copy of the protos with no change must pass.
#   2. A copy with one renamed field must fail, and the failure must name it.
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

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

copy() {
  mkdir -p "$1"
  cp buf.yaml buf.lock "$1/"
  cp -r --parents api/proto "$1/"
}

copy "$tmp/same"
copy "$tmp/renamed"

f="$tmp/renamed/$FIXTURE_FILE"
grep -qxF "$FIXTURE_LINE" "$f" || { log "FAIL: the fixture line is gone from $FIXTURE_FILE; pick another field"; exit 1; }
sed -i "s/^$FIXTURE_LINE\$/$RENAMED_LINE/" "$f"
grep -qxF "$RENAMED_LINE" "$f" || { log "FAIL: the fixture rename did not apply"; exit 1; }

# $BUF is a command with arguments ("go tool buf"), so it is not quoted.
# shellcheck disable=SC2086
if $BUF breaking "$tmp/same" --against "$ROOT" >/dev/null 2>&1; then
  log "ok    an unchanged copy passes"
else
  log "FAIL  an unchanged copy was refused"; exit 1
fi

# shellcheck disable=SC2086
if out="$($BUF breaking "$tmp/renamed" --against "$ROOT" 2>&1)"; then
  log "FAIL  a renamed field passed: the rule is not FILE"; exit 1
fi
if [[ "$out" != *"jti"* ]]; then
  log "FAIL  a renamed field failed for a different reason: $out"; exit 1
fi
log "ok    a renamed field is refused"
log "SELFTEST PASSED"
