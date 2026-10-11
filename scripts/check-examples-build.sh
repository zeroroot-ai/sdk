#!/usr/bin/env bash
# Build each example module against the sdk tree of this commit (sdk#268).
#
# Each directory under examples/ with its own go.mod pins an sdk release, so a
# plain build of it reads the old release and never sees a deletion in this
# tree. This script puts the example and the repository root into one Go
# workspace in a temporary directory and builds the example there. The
# workspace file never enters the tree: a go.work and a replace directive are
# not allowed in a repository of the org.
#
# Usage: scripts/check-examples-build.sh [sdk root] [examples dir]
set -euo pipefail

root="$(cd "${1:-$(dirname "$0")/..}" && pwd)"
examples="$(cd "${2:-$root/examples}" && pwd)"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

built=0
failed=0
for mod in "$examples"/*/go.mod; do
  [ -e "$mod" ] || continue
  dir="$(dirname "$mod")"
  name="$(basename "$dir")"
  rm -f "$work/go.work" "$work/go.work.sum"
  (cd "$work" && go work init "$root" "$dir")
  if (cd "$dir" && GOWORK="$work/go.work" go build -o /dev/null ./...); then
    echo "ok    examples/$name builds against this sdk tree"
  else
    echo "FAIL  examples/$name does not build against this sdk tree" >&2
    failed=$((failed+1))
  fi
  built=$((built+1))
done

# A check that built nothing proved nothing.
if [ "$built" -eq 0 ]; then
  echo "FAIL  no example module with a go.mod under $examples" >&2
  exit 1
fi
if [ "$failed" -ne 0 ]; then
  echo "$failed of $built example modules do not build. Fix the example in the change that changes the API." >&2
  exit 1
fi
echo "$built example modules build against this sdk tree"
