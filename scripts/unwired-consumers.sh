#!/usr/bin/env bash
# unwired-consumers.sh: print the module directories of the first-party
# consumers of this module, comma-separated, for `unwired -consumers` (#112).
#
# D77: no outside user depends on the sdk, so a declaration that no
# first-party repo reads is dead. The readers live in other repos, so the
# measurement loads them beside this module. Each consumer is a public repo,
# cloned shallow into a cache directory outside the repository, so the tree
# guards of this repo never walk it. A directory that already holds a clone
# is fetched and reset.
#
# The ref of a consumer is `main`, with one exception. A change that deletes
# or renames a declaration cannot compile against the consumers at `main`.
# For such a change, write the name of one branch in `.unwired-consumer-ref`.
# A consumer that has a branch of that name is measured at that branch. A
# consumer that does not have it is measured at `main`. After the consumer
# branches merge and go away, each consumer is measured at `main` again, and
# the script prints a notice to empty the file.
#
# The consumer modules, one per line: <repo> <module directory in the repo>.
# A module that requires github.com/zeroroot-ai/sdk belongs here. The scaffold
# goldens of adk are not modules (their go.mod has no version), so the
# declarations that only the scaffold templates use stay in the baseline with
# that reason.
set -euo pipefail

dest="${UNWIRED_CONSUMERS_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/sdk-unwired-consumers}"

consumers='
gibson .
gibson plugins/github
gibson plugins/gitlab
adk gibson
gibson-executor .
cve-triage .
'

remote="${UNWIRED_CONSUMERS_REMOTE:-https://github.com/zeroroot-ai}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ref_file="${UNWIRED_CONSUMER_REF_FILE:-$root/.unwired-consumer-ref}"
paired=""
if [ -f "$ref_file" ]; then
  paired="$(grep -v '^#' "$ref_file" | tr -d '[:space:]' || true)"
fi
case "$paired" in
  ''|main) paired="" ;;
  -*|*..*|*[!A-Za-z0-9._/-]*)
    echo "unwired-consumers: $ref_file holds a name that is not a branch name" >&2
    exit 1 ;;
esac

mkdir -p "$dest"
out=()
used=0
while read -r repo dir; do
  [ -n "$repo" ] || continue
  clone="$dest/$repo"
  url="$remote/$repo.git"
  ref=main
  if [ -n "$paired" ] && git ls-remote --exit-code --heads "$url" "refs/heads/$paired" >/dev/null 2>&1; then
    ref="$paired"
    used=1
  fi
  if [ ! -d "$clone/.git" ]; then
    git init --quiet "$clone"
    git -C "$clone" remote add origin "$url"
  fi
  git -C "$clone" fetch --quiet --depth=1 origin "refs/heads/$ref"
  git -C "$clone" reset --quiet --hard FETCH_HEAD
  mod="$clone/$dir"
  if [ ! -f "$mod/go.mod" ]; then
    echo "unwired-consumers: $repo/$dir has no go.mod" >&2
    exit 1
  fi
  if ! grep -q 'github.com/zeroroot-ai/sdk ' "$mod/go.mod"; then
    echo "unwired-consumers: $repo/$dir no longer requires the sdk; remove it from this list" >&2
    exit 1
  fi
  out+=("$mod")
done <<< "$consumers"

if [ -n "$paired" ] && [ "$used" -eq 0 ]; then
  echo "unwired-consumers: no consumer has the branch $paired; empty $ref_file" >&2
fi

(IFS=,; echo "${out[*]}")
