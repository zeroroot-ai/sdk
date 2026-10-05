#!/usr/bin/env bash
# check-no-gibson-gomod.sh — the sdk go.mod does not require the gibson daemon.
#
# ADR-0058: the sdk does not import gibson. The Go import half of that rule is
# TestNoGibsonImport. This is the go.mod half: a requirement that no Go file
# uses yet, direct or indirect, is still a dependency edge in the wrong
# direction, and the import test cannot see it.
#
# The match is the exact module path. `zeroroot-ai/gibson` as a substring also
# names gibson-executor, which is a different module.
#
# Usage: check-no-gibson-gomod.sh [path/to/go.mod]
set -euo pipefail

gomod="${1:-go.mod}"
if [ ! -s "$gomod" ]; then
  echo "ERROR: $gomod is missing or empty, so the boundary check read nothing" >&2
  exit 2
fi

if grep -nE '(^|[[:space:]])github\.com/zeroroot-ai/gibson([[:space:]/]|$)' "$gomod"; then
  echo "ERROR: $gomod must not depend on github.com/zeroroot-ai/gibson (ADR-0058)" >&2
  exit 1
fi
echo "ok  $gomod does not require github.com/zeroroot-ai/gibson"
