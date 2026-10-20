#!/usr/bin/env bash
# Validate an input of the fan-out workflow before it names a branch or an
# argument of a command.
#
#   fanout-validate.sh tag  <value>   a release tag, vMAJOR.MINOR.PATCH
#   fanout-validate.sh slug <value>   the branch slug of a module that sits
#                                     beside the root module of its repo
#
# A slug starts with a lower case letter other than v, and holds only lower case
# letters, digits and hyphens. It holds no "-v", so the branch prefix of one
# module (chore/bump-sdk-<slug>-v) never matches the branch of another module or
# the branch of a root bump (chore/bump-sdk-v...).
set -euo pipefail

kind="${1:-}"
value="${2-}"

case "$kind" in
  tag)
    if [[ ! "$value" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "::error::fan-out tag '$value' is not a release tag (vMAJOR.MINOR.PATCH)"
      exit 1
    fi
    ;;
  slug)
    if [[ ! "$value" =~ ^[a-uw-z][a-z0-9-]*$ ]] || [[ "$value" == *-v* ]]; then
      echo "::error::branch_slug '$value' must match ^[a-uw-z][a-z0-9-]*\$ and hold no '-v'"
      exit 1
    fi
    ;;
  *)
    echo "usage: fanout-validate.sh tag|slug <value>" >&2
    exit 2
    ;;
esac
