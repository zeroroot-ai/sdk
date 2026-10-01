#!/usr/bin/env bash
# check-cueschemas-fresh.sh
#
# Two checks, because the first one alone was named "fresh" and was not.
#
#   PRESENCE   every file embedded by cueschemas/cueschemas.go is on disk and
#              non-empty (sdk#147).
#   FRESHNESS  every message, field and enum value declared in an embedded
#              proto also appears in that proto's generated CUE.
#
# The presence check passes for a STALE file, which is the failure it was named
# for and did not cover. `make proto` regenerates Go bindings and leaves the CUE
# alone — `make cue-defs` generates the CUE — so a proto change lands with a
# stale embedded schema and nothing notices. That schema is what validates
# mission authoring, so the symptom is `gibson mission validate` REJECTING a
# valid mission: a new node kind exists on the wire and the schema has never
# heard of it. Observed on sdk#120, where ForEachNodeConfig, for_each_config and
# NODE_TYPE_FOR_EACH were absent from the embedded CUE after `make proto`.
#
# WHAT THIS DOES NOT DO: it is a structural check, not a regeneration. It
# compares declared NAMES, so it catches an added or renamed message, field or
# enum value — the way this actually breaks — and would miss a changed
# protovalidate constraint or a reordering. The exact check is `make cue-defs`
# followed by `git diff --exit-code`, which needs cue and a populated buf
# validate cache in CI. That is a heavier gate and worth doing if this one ever
# proves insufficient; a cheap gate that runs is worth more than an exact one
# that gets switched off.
#
# Spec: sdk#147.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

REQUIRED_FILES=(
  "cue.mod/module.cue"
  "cue.mod/gen/buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate/validate_proto_gen.cue"
  "api/proto/gibson/mission/v1/mission_definition_proto_gen.cue"
  "api/proto/gibson/types/v1/types_proto_gen.cue"
  "api/proto/gibson/job/v1/job_proto_gen.cue"
  "api/proto/gibson/common/v1/gibson_common_proto_gen.cue"
)

FAILED=0

for rel in "${REQUIRED_FILES[@]}"; do
  abs="${REPO_ROOT}/${rel}"
  if [ ! -f "${abs}" ]; then
    echo "ERROR: embedded file missing from disk: ${rel}" >&2
    echo "  Run 'make cue-defs' to regenerate CUE bindings, then commit the result." >&2
    FAILED=1
  elif [ ! -s "${abs}" ]; then
    echo "ERROR: embedded file is empty on disk: ${rel}" >&2
    echo "  Run 'make cue-defs' to regenerate CUE bindings, then commit the result." >&2
    FAILED=1
  fi
done

# ---------------------------------------------------------------------------
# FRESHNESS: every declared name in the proto appears in its generated CUE.
#
# Keyed by the PROTO, not by a list of names, so a new message or field is
# covered the moment it is declared and there is nothing to keep in step.
# ---------------------------------------------------------------------------

# proto -> cue, for each embedded pair. The two cue.mod entries have no proto
# of ours and are presence-only.
PAIRS=(
  "api/proto/gibson/mission/v1/mission_definition.proto|api/proto/gibson/mission/v1/mission_definition_proto_gen.cue"
  "api/proto/gibson/types/v1/types.proto|api/proto/gibson/types/v1/types_proto_gen.cue"
  "api/proto/gibson/job/v1/job.proto|api/proto/gibson/job/v1/job_proto_gen.cue"
  "api/proto/gibson/common/v1/gibson_common.proto|api/proto/gibson/common/v1/gibson_common_proto_gen.cue"
)

for pair in "${PAIRS[@]}"; do
  proto="${REPO_ROOT}/${pair%%|*}"
  cue="${REPO_ROOT}/${pair##*|}"
  [ -f "${proto}" ] || { echo "ERROR: proto missing: ${pair%%|*}" >&2; FAILED=1; continue; }
  [ -f "${cue}" ] || continue  # the presence pass above already reported it

  missing=""

  # messages: `message Foo {` -> the CUE emits `#Foo:`
  while read -r name; do
    [ -n "${name}" ] || continue
    grep -q "#${name}:" "${cue}" || missing="${missing} message:${name}"
  done < <(grep -oE '^[[:space:]]*message [A-Za-z0-9_]+' "${proto}" | awk '{print $2}')

  # enum values: `FOO_BAR = 3;` -> the CUE emits the name verbatim
  while read -r name; do
    [ -n "${name}" ] || continue
    grep -q "${name}" "${cue}" || missing="${missing} enum:${name}"
  done < <(grep -oE '^[[:space:]]+[A-Z][A-Z0-9_]+ = [0-9]+' "${proto}" | awk '{print $1}')

  # fields: the CUE records the original proto name in @protobuf(N,...,name=x),
  # and for fields whose lowerCamel already matches it omits the name= part, so
  # match either the name= form or the field's lowerCamel key.
  while read -r fname; do
    [ -n "${fname}" ] || continue
    lower="$(echo "${fname}" | awk -F_ '{printf "%s", $1; for(i=2;i<=NF;i++) printf "%s%s", toupper(substr($i,1,1)), substr($i,2)}')"
    grep -q "name=${fname}" "${cue}" || grep -qE "(^|[[:space:]])${lower}\??:" "${cue}"       || missing="${missing} field:${fname}"
  done < <(grep -oE '^[[:space:]]+(repeated |optional |map<[^>]+> )?[A-Za-z0-9_.]+ [a-z][a-z0-9_]* = [0-9]+' "${proto}"              | sed -E 's/ = [0-9]+$//' | awk '{print $NF}')

  if [ -n "${missing}" ]; then
    echo "ERROR: ${pair##*|} is STALE — declared in the proto, absent from the CUE:" >&2
    for m in ${missing}; do echo "    ${m}" >&2; done
    echo "  Run 'make cue-defs' and commit the result." >&2
    echo "  'make proto' does NOT regenerate CUE. A stale embedded schema makes" >&2
    echo "  'gibson mission validate' reject missions that are valid on the wire." >&2
    FAILED=1
  fi
done

if [ "${FAILED}" -ne 0 ]; then
  echo "" >&2
  echo "cueschemas drift check FAILED — see errors above." >&2
  echo "The files listed are embedded by cueschemas/cueschemas.go and must be" >&2
  echo "present and non-empty in the repository at all times." >&2
  exit 1
fi

echo "cueschemas drift check OK — all embedded files present and non-empty."
