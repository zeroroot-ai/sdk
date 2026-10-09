#!/usr/bin/env bash
# unwired-consumers.sh: print the module directories of the first-party
# consumers of this module, comma-separated, for `unwired -consumers` (#112).
#
# D77: no outside user depends on the sdk, so a declaration that no
# first-party repo reads is dead. The readers live in other repos, so the
# measurement loads them beside this module. Each consumer is a public repo,
# cloned shallow at main into a cache directory outside the repository, so
# the tree guards of this repo never walk it. A directory that
# already holds a clone is fetched and reset to origin/main.
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

mkdir -p "$dest"
out=()
while read -r repo dir; do
  [ -n "$repo" ] || continue
  clone="$dest/$repo"
  if [ -d "$clone/.git" ]; then
    git -C "$clone" fetch --quiet --depth=1 origin main
    git -C "$clone" reset --quiet --hard origin/main
  else
    git clone --quiet --depth=1 --branch main "https://github.com/zeroroot-ai/$repo.git" "$clone"
  fi
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

(IFS=,; echo "${out[*]}")
