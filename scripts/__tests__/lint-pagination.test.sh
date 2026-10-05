#!/usr/bin/env bash
# Fixture for scripts/lint-pagination.mjs (sdk#183, sdk#186).
#
# The lint walks a proto tree that LINT_PAGINATION_PROTO_ROOT names. Each case
# builds one, and each red case asserts its reason.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

# rpc <root> <proto path> <Service> <Method> <field line>
rpc() {
  mkdir -p "$1/$(dirname "$2")"
  cat > "$1/$2" <<PROTO
syntax = "proto3";
service $3 {
  rpc $4($4Request) returns ($4Response);
}
message $4Request {
  $5
}
message $4Response {}
PROTO
}

lint() { LINT_PAGINATION_PROTO_ROOT="$1" node scripts/lint-pagination.mjs 2>&1; }

green() {
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -eq 0 ]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}
red() {
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -ne 0 ] && [[ "$out" == *"$3"* ]]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

# 1. A List RPC with page_size and page_token passes.
rpc "$tmp/aip" w/v1/w.proto WidgetService ListWidgets "int32 page_size = 1; string page_token = 2;"
green "a List RPC with page_size and page_token passes" "$tmp/aip"

# 2. A List RPC with offset is refused.
rpc "$tmp/offset" w/v1/w.proto WidgetService ListWidgets "int32 offset = 1;"
red "a List RPC with offset is refused" "$tmp/offset" "message ListWidgetsRequest declares an offset field"

# 3. A List RPC with limit is refused.
rpc "$tmp/limit" w/v1/w.proto WidgetService ListWidgets "int32 limit = 1;"
red "a List RPC with limit is refused" "$tmp/limit" "ListWidgets uses limit pagination"

# 4. THE CASE sdk#186 ADDS. An offset field in a filter message or in a
#    request that is not a List RPC is refused too.
rpc "$tmp/filter" w/v1/w.proto WidgetService GetWidgetHistory "WidgetFilter filter = 1;"
echo 'message WidgetFilter { uint32 offset = 6; }' >> "$tmp/filter/w/v1/w.proto"
red "an offset field in a filter is refused" "$tmp/filter" "message WidgetFilter declares an offset field"

# 5. A limit on a request that is not a List RPC is a result cap, not a page.
rpc "$tmp/cap" w/v1/w.proto WidgetService SearchWidgets "int32 limit = 1;"
green "a result cap on a search request passes" "$tmp/cap"

# 6. A field that a comment names is not a field.
rpc "$tmp/comment" w/v1/w.proto WidgetService ListWidgets "// int32 offset = 1;"
green "an offset in a comment passes" "$tmp/comment"

# 7. A reserved name is not a field.
rpc "$tmp/reserved" w/v1/w.proto WidgetService ListWidgets 'reserved 1; reserved "offset";'
green "a reserved offset passes" "$tmp/reserved"

# 8. THE FLOOR. An empty proto tree checked nothing.
mkdir -p "$tmp/empty"
red "an empty proto tree is not a pass" "$tmp/empty" "checked nothing"

# 9. The real tree is clean.
out="$(node scripts/lint-pagination.mjs 2>&1)"; rc=$?
if [ "$rc" -eq 0 ]; then ok "the real api/proto tree passes"; else bad "the real tree was blocked: $out"; fi

echo
echo "passed=$PASS failed=$FAIL"
if [ "$FAIL" -eq 0 ] && [ "$PASS" -lt 9 ]; then echo "FAIL: only $PASS case(s) ran, want 9"; exit 1; fi
[ "$FAIL" -eq 0 ]
