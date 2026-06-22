#!/usr/bin/env bash
# Smokey test: validates Jeff's generic specialist registry and call bridge.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)

JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-specialists-config"
DATA_DIR="$SMOKEY_STATE_DIR/jeff-specialists-data"
BIN_DIR="$SMOKEY_STATE_DIR/bin"
REGISTRY_DIR="$DATA_DIR/jeff/specialists"
CODEX_RESUME_STUB_LOG="$SMOKEY_STATE_DIR/codex-resume.log"

assert_contains() {
	local haystack=$1
	local needle=$2
	local label=$3
	if [[ "$haystack" != *"$needle"* ]]; then
		printf 'FAIL: %s\nmissing: %s\noutput:\n%s\n' "$label" "$needle" "$haystack" >&2
		exit 1
	fi
	printf 'ok: %s\n' "$label"
}

assert_fails_contains() {
	local label=$1
	local needle=$2
	shift 2
	local output
	if output=$("$@" 2>&1); then
		printf 'FAIL: %s\ncommand unexpectedly succeeded\noutput:\n%s\n' "$label" "$output" >&2
		exit 1
	fi
	assert_contains "$output" "$needle" "$label"
}

export XDG_DATA_HOME="$DATA_DIR"
export XDG_CACHE_HOME="$SMOKEY_STATE_DIR/jeff-specialists-cache"
export CODEX_RESUME_STUB_LOG
mkdir -p "$BIN_DIR" "$REGISTRY_DIR"
cp "$SMOKEY_TEST_DIR/fixtures/codex-resume" "$BIN_DIR/codex-resume"
chmod +x "$BIN_DIR/codex-resume"
export PATH="$BIN_DIR:$PATH"

# Registry files are machine-readable state; fixture copies contain no secrets.
cp "$SMOKEY_TEST_DIR"/fixtures/*.json "$REGISTRY_DIR/"

# Listing, showing, and shell completion expose registered specialists.
list_out=$("$JEFF_BIN" --config "$CONFIG_DIR" specialists list)
assert_contains "$list_out" $'disabled\techo\tdisabled' "specialists list shows disabled state"
assert_contains "$list_out" $'gov\tcodex-resume\tenabled\tGovernance specialist' "specialists list shows codex-resume specialist"
show_out=$("$JEFF_BIN" --config "$CONFIG_DIR" specialists show gov)
assert_contains "$show_out" '"target": "governor"' "specialists show emits registry JSON"
completion_out=$("$JEFF_BIN" --config "$CONFIG_DIR" __complete specialists show g 2>/dev/null)
assert_contains "$completion_out" "gov" "specialist completion includes alias"

# Calling a codex-resume specialist wraps the request and sets JEFF_AGENT_ALIAS.
: >"$CODEX_RESUME_STUB_LOG"
call_out=$("$JEFF_BIN" --config "$CONFIG_DIR" specialists call gov -- "check status")
assert_contains "$call_out" "governor handled" "specialists call prints cleaned transport response"
grep -q "alias=governor" "$CODEX_RESUME_STUB_LOG"
grep -q "JEFF_AGENT_ALIAS=gov" "$CODEX_RESUME_STUB_LOG"
grep -q "Specialist alias: gov" "$CODEX_RESUME_STUB_LOG"
grep -q "check status" "$CODEX_RESUME_STUB_LOG"
echo "ok: specialists call wraps prompt and exports alias"

# File-backed requests and the transport-neutral echo adapter work without codex-resume.
file_out=$("$JEFF_BIN" --config "$CONFIG_DIR" specialists call echoer --file "$SMOKEY_TEST_DIR/request.md")
assert_contains "$file_out" "echo specialist echoer" "echo transport handles file requests"
assert_contains "$file_out" "file-backed specialist request" "file request text reaches specialist"

# Failed and malformed registry cases fail clearly.
assert_fails_contains "disabled specialists are rejected" 'specialist "disabled" is disabled' \
	"$JEFF_BIN" --config "$CONFIG_DIR" specialists call disabled -- "hello"
assert_fails_contains "unknown specialists are rejected" 'specialist "missing" does not exist' \
	"$JEFF_BIN" --config "$CONFIG_DIR" specialists show missing
assert_fails_contains "alias mismatches are rejected" 'has alias "wrong"' \
	"$JEFF_BIN" --config "$CONFIG_DIR" specialists show mismatch

# Calls are logged for later coordination review.
call_log_count=$(find "$DATA_DIR/jeff/specialist-calls" -type f | wc -l)
if [[ "$call_log_count" -lt 2 ]]; then
	printf 'FAIL: expected specialist call logs, got %s\n' "$call_log_count" >&2
	find "$DATA_DIR/jeff" -maxdepth 4 -type f >&2
	exit 1
fi
echo "ok: specialist calls are logged"
