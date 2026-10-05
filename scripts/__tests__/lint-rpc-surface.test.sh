#!/usr/bin/env bash
# Fixture for scripts/lint-rpc-surface.mjs (sdk#189).
#
# The lint reads the proto tree that LINT_RPC_SURFACE_PROTO_ROOT names and the
# list that LINT_RPC_SURFACE_LIST names. Each case builds both, and each red
# case asserts its reason.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

# tree <root>: one service with two RPCs, and a list that holds both.
tree() {
  mkdir -p "$1/protos/widget/v1"
  cat > "$1/protos/widget/v1/widget.proto" <<'PROTO'
syntax = "proto3";
package gibson.widget.v1;
service WidgetService {
  // rpc InAComment(A) returns (B);
  rpc GetWidget(GetWidgetRequest) returns (GetWidgetResponse);
  rpc ListWidgets(ListWidgetsRequest) returns (stream ListWidgetsResponse) {
    option deprecated = false;
  }
}
message GetWidgetRequest {}
message GetWidgetResponse {}
message ListWidgetsRequest {}
message ListWidgetsResponse {}
PROTO
  cat > "$1/list.txt" <<'LIST'
# a comment
gibson.widget.v1.WidgetService/GetWidget
gibson.widget.v1.WidgetService/ListWidgets
LIST
}

lint() { LINT_RPC_SURFACE_PROTO_ROOT="$1/protos" LINT_RPC_SURFACE_LIST="$1/list.txt" node scripts/lint-rpc-surface.mjs 2>&1; }

red() { # red <name> <root> <expected substring>
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -ne 0 ] && [[ "$out" == *"$3"* ]]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

# 1. The protos and the list agree: green.
tree "$tmp/clean"
out="$(lint "$tmp/clean")"; rc=$?
if [ "$rc" -eq 0 ] && [[ "$out" == *"1 services and 2 RPCs"* ]]; then ok "a tree that matches its list passes"; else bad "a clean tree was blocked: $out"; fi

# 2. THE CASE sdk#189 IS ABOUT. A new service is refused, and named.
tree "$tmp/newsvc"
cat > "$tmp/newsvc/protos/widget/v1/admin.proto" <<'PROTO'
syntax = "proto3";
package gibson.widget.v1;
service WidgetAdminService {
  rpc PurgeWidgets(PurgeWidgetsRequest) returns (PurgeWidgetsResponse);
}
message PurgeWidgetsRequest {}
message PurgeWidgetsResponse {}
PROTO
red "a new service is refused" "$tmp/newsvc" "gibson.widget.v1.WidgetAdminService/PurgeWidgets is not in api/proto/rpc-surface.txt"

# 3. A new RPC on a listed service is refused.
tree "$tmp/newrpc"
sed -i 's|^  rpc GetWidget|  rpc DeleteWidget(GetWidgetRequest) returns (GetWidgetResponse);\n  rpc GetWidget|' "$tmp/newrpc/protos/widget/v1/widget.proto"
red "a new RPC on a listed service is refused" "$tmp/newrpc" "gibson.widget.v1.WidgetService/DeleteWidget is not in"

# 4. A list line whose RPC is gone is refused. A stale line must never become
#    a ready permission.
tree "$tmp/stale"
echo "gibson.widget.v1.WidgetService/RenameWidget" >> "$tmp/stale/list.txt"
red "a list line with no RPC is refused" "$tmp/stale" "the list holds gibson.widget.v1.WidgetService/RenameWidget, and no proto declares it"

# 5. The same method name on a different service is not in the list.
tree "$tmp/samename"
sed -i 's|service WidgetService|service OtherService|' "$tmp/samename/protos/widget/v1/widget.proto"
red "a listed method name on a different service is refused" "$tmp/samename" "gibson.widget.v1.OtherService/GetWidget is not in"

# 6. A line that is in the list two times is refused.
tree "$tmp/dup"
echo "gibson.widget.v1.WidgetService/GetWidget" >> "$tmp/dup/list.txt"
red "a line that is in the list two times is refused" "$tmp/dup" "two times"

# 7. THE FLOOR. An empty proto tree checked nothing.
tree "$tmp/empty"
rm -r "$tmp/empty/protos/widget"
red "an empty proto tree is not a pass" "$tmp/empty" "checked nothing"

# 8. THE FLOOR. A tree with no RPC checked nothing.
tree "$tmp/norpc"
echo 'syntax = "proto3"; message Only {}' > "$tmp/norpc/protos/widget/v1/widget.proto"
red "a tree with no RPC is not a pass" "$tmp/norpc" "checked nothing"

# 9. A list that is not there is refused.
tree "$tmp/nolist"
rm "$tmp/nolist/list.txt"
red "a list that is not there is refused" "$tmp/nolist" "cannot read the list"

# 10. The real tree matches the real list.
out="$(node scripts/lint-rpc-surface.mjs 2>&1)"; rc=$?
if [ "$rc" -eq 0 ]; then ok "the real api/proto tree matches its list"; else bad "the real tree was blocked: $out"; fi

echo
echo "passed=$PASS failed=$FAIL"
if [ "$FAIL" -eq 0 ] && [ "$PASS" -lt 10 ]; then echo "FAIL: only $PASS case(s) ran, want 10"; exit 1; fi
[ "$FAIL" -eq 0 ]
