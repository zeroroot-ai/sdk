#!/usr/bin/env bash
# Asserts that a failed fan-out shard can still turn the workflow run red.
#
# fan-out.yml ends in a `summary` job whose only step is a gate:
#
#   if: needs.fan-out.result != 'success'
#     -> exit 1
#
# That gate was unreachable for as long as the `fan-out` matrix job carried
# `continue-on-error: true`. GitHub reports a continue-on-error job's failure
# in its CONCLUSION, but sets `needs.<job>.result` to 'success', so the gate's
# condition was never true, the step was skipped, and the run was green.
#
# Measured on the sdk v0.187.0 fan-out, run 36917226668: the `gibson` and
# `gibson-executor` shards both FAILED, the gate step was SKIPPED, and the run
# reported `success`. Two consumers silently did not get their bump, with a
# green badge and no failure mail. gibson-executor sat on v0.186.0 until someone
# compared go.mod files by hand.
#
# The workflow's own comment claimed the opposite — "The summary job below
# explicitly re-checks every shard and fails iff at least one failed, so
# org-level visibility is preserved" — which is why this is a script and not a
# comment. A machine checks it now.
#
# `fail-fast: false` is what keeps every shard running, and it is unaffected.
# `if: always()` on the summary job is what keeps it reporting. Neither needed
# continue-on-error.
#
# Usage: check-fanout-failure-visible.sh [workflow.yml]
#        defaults to .github/workflows/fan-out.yml
#
# Exit 0 = a failed shard reaches the run status. Exit 1 = it cannot.
set -uo pipefail

wf="${1:-.github/workflows/fan-out.yml}"
rc=0

if [ ! -f "$wf" ]; then
  echo "::error::$wf does not exist" >&2
  exit 1
fi

# The fan-out job's own block: from `^  fan-out:` to the next top-level job key.
job="$(awk '
  /^  fan-out:[[:space:]]*$/ { inside = 1; next }
  inside && /^  [a-zA-Z_-]+:[[:space:]]*$/ { exit }
  inside { print }
' "$wf")"

if [ -z "$job" ]; then
  echo "::error::$wf has no 'fan-out:' job. If it was renamed, this check must be" >&2
  echo "updated with it — it is asserting a property of that job, not of a name." >&2
  exit 1
fi

# Job-level continue-on-error: any occurrence at the job's own indentation.
# A step-level one (deeper indent, under `steps:`) is a different decision and
# is not what breaks the gate.
if printf '%s\n' "$job" | grep -qE '^    continue-on-error:[[:space:]]*true'; then
  echo "::error::the fan-out job sets continue-on-error: true, which makes" >&2
  echo "needs.fan-out.result read 'success' even when a shard fails. The summary" >&2
  echo "job's fail-the-run gate is then skipped and the run goes green with" >&2
  echo "consumers unbumped. Use fail-fast: false alone; it already lets every" >&2
  echo "shard run, and if: always() already lets the summary report." >&2
  rc=1
fi

# The gate itself must still exist and still key off the shard result.
if ! grep -qE "needs\.fan-out\.result" "$wf"; then
  echo "::error::$wf no longer has a summary gate keyed on needs.fan-out.result." >&2
  echo "Without it nothing maps a shard failure onto the run status." >&2
  rc=1
fi

if [ "$rc" -eq 0 ]; then
  echo "fan-out: a failed shard reaches the run status (no job-level continue-on-error, gate present)"
fi
exit "$rc"
