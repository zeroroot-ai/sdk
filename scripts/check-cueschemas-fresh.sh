#!/usr/bin/env bash
# check-cueschemas-fresh.sh
#
# Verifies that every file embedded by cueschemas/cueschemas.go is present on
# disk and non-empty. Exits 1 with a clear message if any file is missing or
# empty so CI catches divergence between the embed directive and the source tree.
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

if [ "${FAILED}" -ne 0 ]; then
  echo "" >&2
  echo "cueschemas drift check FAILED — see errors above." >&2
  echo "The files listed are embedded by cueschemas/cueschemas.go and must be" >&2
  echo "present and non-empty in the repository at all times." >&2
  exit 1
fi

echo "cueschemas drift check OK — all embedded files present and non-empty."
