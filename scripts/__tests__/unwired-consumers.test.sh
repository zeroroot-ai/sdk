#!/usr/bin/env bash
# Fixture for scripts/unwired-consumers.sh (#112). It uses local remotes, so it
# needs no network. Each red case asserts its reason.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

g() { git -c user.name=t -c user.email=t@example.invalid -c init.defaultBranch=main "$@"; }

# mkremote <repo> <module dir>...: a remote with one commit on main. Each
# module holds a marker file that names the branch it came from.
mkremote() {
  local repo="$1"; shift
  local src="$tmp/src/$repo"
  mkdir -p "$src"; g -C "$src" init -q
  for d in "$@"; do
    mkdir -p "$src/$d"
    printf 'module example.invalid/%s\n\nrequire github.com/zeroroot-ai/sdk v1.0.0\n' "$repo" > "$src/$d/go.mod"
    echo main > "$src/$d/marker"
  done
  g -C "$src" add . && g -C "$src" commit -q -m main
  g clone -q --bare "$src" "$tmp/remotes/$repo.git"
}

# pair <repo> <branch> <module dir>...: add a branch with a changed marker.
pair() {
  local repo="$1" branch="$2"; shift 2
  local src="$tmp/src/$repo"
  g -C "$src" checkout -q -b "$branch"
  for d in "$@"; do echo "$branch" > "$src/$d/marker"; done
  g -C "$src" commit -q -am paired
  g -C "$src" push -q "$tmp/remotes/$repo.git" "$branch"
  g -C "$src" checkout -q main
}

mkdir -p "$tmp/remotes"
mkremote gibson . plugins/github plugins/gitlab
mkremote adk gibson
mkremote gibson-executor .
mkremote cve-triage .
mkremote fix-agent .
mkremote intake-agent .
mkremote incident-agent .
pair gibson epic/pair . plugins/github plugins/gitlab

run() { # run <ref file content> <cache name>
  printf '%s\n' "$1" > "$tmp/ref"
  UNWIRED_CONSUMERS_REMOTE="file://$tmp/remotes" UNWIRED_CONSUMER_REF_FILE="$tmp/ref" \
    UNWIRED_CONSUMERS_DIR="$tmp/$2" bash scripts/unwired-consumers.sh 2>"$tmp/err"
}

out="$(run "" c1)"; rc=$?
if [ "$rc" -eq 0 ] && [ "$(cat "$tmp/c1/gibson/marker")" = main ] && [ "$(tr ',' '\n' <<<"$out" | wc -l)" -eq 6 ]; then
  ok "an empty ref file measures each consumer at main"
else bad "empty ref file (rc=$rc): $out $(cat "$tmp/err")"; fi

out="$(run "epic/pair" c1)"; rc=$?
if [ "$rc" -eq 0 ] && [ "$(cat "$tmp/c1/gibson/marker")" = epic/pair ] && [ "$(cat "$tmp/c1/adk/gibson/marker")" = main ]; then
  ok "a consumer with the paired branch uses it, and the others stay at main"
else bad "paired branch (rc=$rc): $out $(cat "$tmp/err")"; fi

out="$(run "" c1)"; rc=$?
if [ "$rc" -eq 0 ] && [ "$(cat "$tmp/c1/gibson/marker")" = main ]; then
  ok "an existing clone goes back to main after the ref file is emptied"
else bad "back to main (rc=$rc): $out $(cat "$tmp/err")"; fi

out="$(run "epic/gone" c2)"; rc=$?
if [ "$rc" -eq 0 ] && grep -q "no consumer has the branch epic/gone" "$tmp/err"; then
  ok "a branch that no consumer has gives main and a notice"
else bad "absent branch (rc=$rc): $out $(cat "$tmp/err")"; fi

out="$(run "--upload-pack=x" c3)"; rc=$?
if [ "$rc" -ne 0 ] && grep -q "not a branch name" "$tmp/err"; then
  ok "a name that is not a branch name is refused"
else bad "bad name (rc=$rc): $out $(cat "$tmp/err")"; fi

# A consumer that dropped the sdk must leave the list.
src="$tmp/src/cve-triage"
printf 'module example.invalid/cve-triage\n' > "$src/go.mod"
g -C "$src" commit -q -am drop && g -C "$src" push -q "$tmp/remotes/cve-triage.git" main
out="$(run "" c4)"; rc=$?
if [ "$rc" -ne 0 ] && grep -q "no longer requires the sdk" "$tmp/err"; then
  ok "a consumer that dropped the sdk fails"
else bad "dropped sdk (rc=$rc): $out $(cat "$tmp/err")"; fi

echo "pass=$PASS fail=$FAIL"
[ "$FAIL" -eq 0 ]
