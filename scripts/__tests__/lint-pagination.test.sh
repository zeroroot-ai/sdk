#!/usr/bin/env bash
# Fixture for scripts/lint-pagination.mjs (sdk#183).
#
# The lint walks a proto tree that LINT_PAGINATION_PROTO_ROOT names. Each case
# builds one, and each red case asserts its reason.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

# rpc <root> <proto path> <Service> <Method> <field>
# writes a proto with one List RPC whose request has the named paging field.
rpc() {
  mkdir -p "$1/$(dirname "$2")"
  cat > "$1/$2" <<PROTO
syntax = "proto3";
service $3 {
  rpc $4($4Request) returns ($4Response);
}
message $4Request {
  int32 $5 = 1;
}
message $4Response {}
PROTO
}

# The RPCs the allowlist names today, each with its limit field. A root that
# holds all of them leaves no entry unused.
allowlisted() {
  rpc "$1" gibson/pluginadmin/v1/plugin_admin.proto PluginAdminService ListPluginInstalls limit
  mkdir -p "$1/gibson/daemon/v1"
  cat > "$1/gibson/daemon/v1/daemon.proto" <<'PROTO'
syntax = "proto3";
service DaemonService {
  rpc ListMissions(ListMissionsRequest) returns (ListMissionsResponse);
  rpc ListMissionDefinitions(ListMissionDefinitionsRequest) returns (ListMissionDefinitionsResponse);
}
message ListMissionsRequest { int32 limit = 1; }
message ListMissionsResponse {}
message ListMissionDefinitionsRequest { int32 limit = 1; int32 offset = 2; }
message ListMissionDefinitionsResponse {}
PROTO
  rpc "$1" gibson/secrets/v1/secrets.proto SecretsService ListSecrets offset
}

lint() { LINT_PAGINATION_PROTO_ROOT="$1" node scripts/lint-pagination.mjs 2>&1; }

red() { # red <name> <root> <expected substring>
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -ne 0 ] && [[ "$out" == *"$3"* ]]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

# 1. Every allowlisted RPC present, nothing else: green.
allowlisted "$tmp/clean"
out="$(lint "$tmp/clean")"; rc=$?
if [ "$rc" -eq 0 ]; then ok "the allowlisted RPCs pass"; else bad "the allowlisted RPCs were blocked: $out"; fi

# 2. A new List RPC with offset pagination is refused, and named.
allowlisted "$tmp/new"
rpc "$tmp/new" gibson/widget/v1/widget.proto WidgetService ListWidgets offset
red "a new List RPC with offset is refused" "$tmp/new" "WidgetService/ListWidgets uses limit/offset pagination"

# 3. A new List RPC with page_size is not this lint's business.
allowlisted "$tmp/aip"
rpc "$tmp/aip" gibson/widget/v1/widget.proto WidgetService ListWidgets page_size
out="$(lint "$tmp/aip")"; rc=$?
if [ "$rc" -eq 0 ]; then ok "a new List RPC with page_size passes"; else bad "page_size was blocked: $out"; fi

# 4. THE CASE sdk#183 IS ABOUT. An entry whose proto left the tree is stale.
allowlisted "$tmp/stale"
rm -r "$tmp/stale/gibson/secrets"
red "an allowlist entry whose proto is gone is refused" "$tmp/stale" "GRANDFATHER_LIST entry ListSecrets matches no List* RPC"

# 5. An entry whose RPC moved to page_size no longer needs its exemption.
allowlisted "$tmp/moved"
rpc "$tmp/moved" gibson/secrets/v1/secrets.proto SecretsService ListSecrets page_size
red "an allowlist entry whose RPC no longer uses offset is refused" "$tmp/moved" "GRANDFATHER_LIST entry ListSecrets matches no List* RPC"

# 6. The same method name at another path is not exempt. A stale entry must
#    never become a ready exemption.
allowlisted "$tmp/samename"
rpc "$tmp/samename" gibson/other/v1/other.proto OtherService ListSecrets offset
red "an allowlisted method name at another path is refused" "$tmp/samename" "OtherService/ListSecrets uses limit/offset pagination"

# 7. THE FLOOR. An empty proto tree checked nothing.
mkdir -p "$tmp/empty"
red "an empty proto tree is not a pass" "$tmp/empty" "checked nothing"

# 8. The real tree is clean.
out="$(node scripts/lint-pagination.mjs 2>&1)"; rc=$?
if [ "$rc" -eq 0 ]; then ok "the real api/proto tree passes"; else bad "the real tree was blocked: $out"; fi

echo
echo "passed=$PASS failed=$FAIL"
if [ "$FAIL" -eq 0 ] && [ "$PASS" -lt 8 ]; then echo "FAIL: only $PASS case(s) ran, want 8"; exit 1; fi
[ "$FAIL" -eq 0 ]
