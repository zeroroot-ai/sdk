#!/usr/bin/env bash
# check-no-agent-getcredential.sh
#
# Spec: non-plugin-secret-isolation Task 4 (Requirement 1.3)
#
# Fails if the symbol GetCredential appears anywhere under core/sdk/agent/.
# Path-scoped to agent/ so the daemon's legitimate GetCredential RPC handlers
# in core/gibson/internal/ are not flagged.
#
# Usage: ./scripts/check-no-agent-getcredential.sh [repo-root]
#   repo-root defaults to the directory containing this script's parent (the SDK root).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${1:-${SCRIPT_DIR}/..}"
AGENT_DIR="${REPO_ROOT}/agent"

if [[ ! -d "${AGENT_DIR}" ]]; then
  echo "ERROR: agent directory not found at ${AGENT_DIR}" >&2
  exit 1
fi

# Search for GetCredential in any .go file under agent/.
# We use grep -r rather than requiring ripgrep to keep CI dependencies minimal.
if grep -rl --include='*.go' '\bGetCredential\b' "${AGENT_DIR}" 2>/dev/null | grep -q .; then
  echo "FAIL: GetCredential symbol found in core/sdk/agent/. Agents must not have a credential API." >&2
  echo "Offending files:" >&2
  grep -rl --include='*.go' '\bGetCredential\b' "${AGENT_DIR}" >&2
  echo "" >&2
  echo "Fix: remove the GetCredential method or reference. Agents access credentials" >&2
  echo "only indirectly by dispatching a tool that invokes a plugin (see plugin-runtime spec)." >&2
  exit 1
fi

echo "OK: no GetCredential symbol in core/sdk/agent/"
