#!/usr/bin/env bash
# Fixture for scripts/lint-idempotency-key.mjs (sdk#207).
#
# The lint walks a proto tree that LINT_IDEMPOTENCY_PROTO_ROOT names. Each case
# builds one, and each red case asserts its reason.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
PASS=0 FAIL=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

ok()  { PASS=$((PASS+1)); echo "ok    $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $1"; }

KEY='string idempotency_key = 2 [(buf.validate.field).string = { max_len: 128 }];'

# proto <root> <Method> <request body line>
# writes a proto with one RPC and the given line in its request message.
proto() {
  mkdir -p "$1/widget/v1"
  cat > "$1/widget/v1/widget.proto" <<PROTO
syntax = "proto3";
service WidgetService {
  rpc $2($2Request) returns ($2Response);
}
message $2Request {
  string name = 1;
  $3
}
message $2Response {}
PROTO
}

lint() { LINT_IDEMPOTENCY_PROTO_ROOT="$1" node scripts/lint-idempotency-key.mjs 2>&1; }

green() { # green <name> <root>
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -eq 0 ]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

red() { # red <name> <root> <expected substring>
  out="$(lint "$2")"; rc=$?
  if [ "$rc" -ne 0 ] && [[ "$out" == *"$3"* ]]; then ok "$1"; else bad "$1 (rc=$rc): $out"; fi
}

# 1. A create request with the field passes.
proto "$tmp/clean" CreateWidget "$KEY"
green "a create request with the field passes" "$tmp/clean"

# 2. THE CASE sdk#207 IS ABOUT. Each verb with no field is refused, and named.
for verb in Create Run Start Submit; do
  proto "$tmp/no-$verb" "${verb}Widget" ""
  red "a ${verb} request with no field is refused" "$tmp/no-$verb" "the request of rpc ${verb}Widget has no idempotency_key"
done

# 3. A field that only a comment names is not a field.
proto "$tmp/comment" CreateWidget "// $KEY"
red "a field in a comment is refused" "$tmp/comment" "has no idempotency_key"

# 4. A field with no length rule is refused.
proto "$tmp/norule" CreateWidget "string idempotency_key = 2;"
red "a field with no length rule is refused" "$tmp/norule" "has no idempotency_key"

# 5. A field of a different type is refused.
proto "$tmp/bytes" CreateWidget "bytes idempotency_key = 2;"
red "a field that is not a string is refused" "$tmp/bytes" "has no idempotency_key"

# 6. A request message with one of the verbs and no RPC is still checked.
proto "$tmp/orphan" CreateWidget "$KEY"
echo 'message StartWidgetRequest { string name = 1; }' >> "$tmp/orphan/widget/v1/widget.proto"
red "a request message with no RPC is refused" "$tmp/orphan" "message StartWidgetRequest has no idempotency_key"

# 7. A verb that is only the start of a longer word is not one of the verbs.
#    RunnerStatus is not a Run request. A get request needs no field.
mkdir -p "$tmp/other/widget/v1"
cat > "$tmp/other/widget/v1/widget.proto" <<PROTO
syntax = "proto3";
service WidgetService {
  rpc CreateWidget(CreateWidgetRequest) returns (CreateWidgetResponse);
  rpc GetWidget(GetWidgetRequest) returns (GetWidgetResponse);
  rpc RunnerStatus(RunnerStatusRequest) returns (RunnerStatusResponse);
}
message RunnerStatusRequest { string name = 1; }
message RunnerStatusResponse {}
message CreateWidgetRequest { $KEY }
message CreateWidgetResponse {}
message GetWidgetRequest { string name = 1; }
message GetWidgetResponse {}
PROTO
green "a get request and a longer word need no field" "$tmp/other"

# 8. THE FLOOR. An empty proto tree checked nothing.
mkdir -p "$tmp/empty"
red "an empty proto tree is not a pass" "$tmp/empty" "checked nothing"

# 9. THE FLOOR. A tree with no such request checked nothing.
proto "$tmp/none" GetWidget ""
red "a tree with no such request is not a pass" "$tmp/none" "checked nothing"

# 10. The real tree is clean.
out="$(node scripts/lint-idempotency-key.mjs 2>&1)"; rc=$?
if [ "$rc" -eq 0 ]; then ok "the real api/proto tree passes"; else bad "the real tree was blocked: $out"; fi

echo
echo "passed=$PASS failed=$FAIL"
if [ "$FAIL" -eq 0 ] && [ "$PASS" -lt 13 ]; then echo "FAIL: only $PASS case(s) ran, want 13"; exit 1; fi
[ "$FAIL" -eq 0 ]
