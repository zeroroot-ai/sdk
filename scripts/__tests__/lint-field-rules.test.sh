#!/usr/bin/env bash
# Fixture for scripts/lint-field-rules.mjs (sdk#185).
#
# The lint walks a proto tree that LINT_FIELD_RULES_PROTO_ROOT names. Each case
# builds one, and each red case asserts its reason.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

RULE='[(buf.validate.field).string = { max_len: 1024 }]'
ENUM_RULE='[(buf.validate.field).enum = { defined_only: true }]'

# proto <root> <name field line> <color field line> [extra lines]
proto() {
  mkdir -p "$1/widget/v1"
  cat > "$1/widget/v1/widget.proto" <<PROTO
syntax = "proto3";
enum Color { COLOR_UNSPECIFIED = 0; }
service WidgetService {
  rpc CreateWidget(CreateWidgetRequest) returns (CreateWidgetResponse);
}
message CreateWidgetRequest {
  $2
  $3
  repeated string tags = 3;
  map<string, string> labels = 4;
  int32 count = 5;
  message Inner { string free = 1; }
  ${4:-}
}
message CreateWidgetResponse { string id = 1; }
PROTO
}

lint() { LINT_FIELD_RULES_PROTO_ROOT="$1" node scripts/lint-field-rules.mjs 2>&1; }

green() {
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -eq 0 ]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}
red() {
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -ne 0 ] && [[ "$out" == *"$3"* ]]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

# 1. A request with a rule on each string and enum field passes. A repeated
#    field, a map, a number, a nested message and the response are not checked.
proto "$tmp/clean" "string name = 1 $RULE;" "Color color = 2 $ENUM_RULE;"
green "a request with rules passes" "$tmp/clean"

# 2. THE CASE sdk#185 IS ABOUT. A string field with no rule is refused.
proto "$tmp/str" "string name = 1;" "Color color = 2 $ENUM_RULE;"
red "a string field with no rule is refused" "$tmp/str" "field name of CreateWidgetRequest has no buf.validate rule"

# 3. An enum field with no rule is refused.
proto "$tmp/enum" "string name = 1 $RULE;" "Color color = 2;"
red "an enum field with no rule is refused" "$tmp/enum" "field color of CreateWidgetRequest has no buf.validate rule"

# 4. An optional string field with no rule is refused.
proto "$tmp/opt" "optional string name = 1;" "Color color = 2 $ENUM_RULE;"
red "an optional string field with no rule is refused" "$tmp/opt" "field name of CreateWidgetRequest"

# 5. A string field in a oneof with no rule is refused.
proto "$tmp/oneof" "string name = 1 $RULE;" "Color color = 2 $ENUM_RULE;" "oneof pick { string a = 6; }"
red "a oneof string field with no rule is refused" "$tmp/oneof" "field a of CreateWidgetRequest"

# 6. A rule that only a comment names is not a rule.
proto "$tmp/comment" "string name = 1; // $RULE" "Color color = 2 $ENUM_RULE;"
red "a rule in a comment is refused" "$tmp/comment" "field name of CreateWidgetRequest"

# 7. THE FLOOR. An empty tree checked nothing.
mkdir -p "$tmp/empty"
red "an empty proto tree is not a pass" "$tmp/empty" "checked nothing"

# 8. THE FLOOR. A tree with no RPC checked nothing.
mkdir -p "$tmp/norpc/w"; echo 'syntax = "proto3"; message Only { string a = 1; }' > "$tmp/norpc/w/w.proto"
red "a tree with no RPC is not a pass" "$tmp/norpc" "checked nothing"

# 9. The real tree passes.
out="$(node scripts/lint-field-rules.mjs 2>&1)"; rc=$?
if [ "$rc" -eq 0 ]; then ok "the real api/proto tree passes"; else bad "the real tree was blocked: $out"; fi

echo
echo "passed=$PASS failed=$FAIL"
if [ "$FAIL" -eq 0 ] && [ "$PASS" -lt 9 ]; then echo "FAIL: only $PASS case(s) ran, want 9"; exit 1; fi
[ "$FAIL" -eq 0 ]
