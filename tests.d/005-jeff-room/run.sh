#!/usr/bin/env bash
# Smokey test: validates deterministic Jeff room routing through codex-resume.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)

JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-room-config"
DATA_DIR="$SMOKEY_STATE_DIR/jeff-room-data"
BIN_DIR="$SMOKEY_STATE_DIR/bin"
ROOM_DIR="$DATA_DIR/jeff/rooms/bde-dev"
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
export XDG_CACHE_HOME="$SMOKEY_STATE_DIR/jeff-room-cache"
export CODEX_RESUME_STUB_LOG
mkdir -p "$BIN_DIR"
cp "$SMOKEY_TEST_DIR/fixtures/codex-resume" "$BIN_DIR/codex-resume"
chmod +x "$BIN_DIR/codex-resume"
export PATH="$BIN_DIR:$PATH"

# Invalid room setup fails before writing room state.
assert_fails_contains "room rejects path-like names" "invalid room name" \
	"$JEFF_BIN" --config "$CONFIG_DIR" room new ../bad --experts gov
assert_fails_contains "room rejects empty expert lists" "at least one expert is required" \
	"$JEFF_BIN" --config "$CONFIG_DIR" room new empty --experts ",,,"
assert_fails_contains "room rejects unknown experts" "unknown alias: missing" \
	"$JEFF_BIN" --config "$CONFIG_DIR" room new bad-expert --experts missing
assert_fails_contains "room rejects malformed codex-resume policy output" "policy output has no alias field" \
	"$JEFF_BIN" --config "$CONFIG_DIR" room new malformed --experts noalias
test ! -d "$DATA_DIR/jeff/rooms/bad-expert"
echo "ok: room setup failures leave no partial room"

# Creating a room resolves short aliases to canonical codex-resume aliases.
create_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room new bde-dev --experts gov,engine)
assert_contains "$create_out" 'Room "bde-dev" created with 2 expert(s).' "room new reports experts"
grep -q '"alias": "bde"' "$ROOM_DIR/room.json"
grep -q '"alias": "bde-engine"' "$ROOM_DIR/room.json"
echo "ok: room stores canonical expert aliases"

# Duplicate expert selectors collapse to one canonical participant.
dedupe_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room new duplicate-room --experts gov,bde,governance)
assert_contains "$dedupe_out" 'Room "duplicate-room" created with 1 expert(s).' "room deduplicates canonical experts"
if [[ $(grep -c '"alias": "bde"' "$DATA_DIR/jeff/rooms/duplicate-room/room.json") -ne 1 ]]; then
	echo "FAIL: duplicate room stored duplicate canonical experts" >&2
	cat "$DATA_DIR/jeff/rooms/duplicate-room/room.json" >&2
	exit 1
fi
echo "ok: room dedupe stores one canonical expert"

# Listing and completion expose the configured room name.
list_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room list)
assert_contains "$list_out" "bde-dev" "room list includes room"
completion_out=$("$JEFF_BIN" --config "$CONFIG_DIR" __complete room enter b 2>/dev/null)
assert_contains "$completion_out" "bde-dev" "room enter completion includes room"

# A message without selector uses the configured room order.
: >"$CODEX_RESUME_STUB_LOG"
turn_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "check invoices")
assert_contains "$turn_out" "bde:" "default turn prints first expert"
assert_contains "$turn_out" "bde-engine:" "default turn prints second expert"
mapfile -t calls < <(grep '^exec ' "$CODEX_RESUME_STUB_LOG")
[[ "${calls[0]}" == "exec bde" ]]
[[ "${calls[1]}" == "exec bde-engine" ]]
grep -q "Previous room answers:" "$CODEX_RESUME_STUB_LOG"
echo "ok: room default turn routes through all experts in order"

# A short alias selector rotates that expert to the front and strips the prefix.
: >"$CODEX_RESUME_STUB_LOG"
selector_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "engine: inspect storage")
assert_contains "$selector_out" "bde-engine:" "selector turn prints selected expert"
mapfile -t selected_calls < <(grep '^exec ' "$CODEX_RESUME_STUB_LOG")
[[ "${selected_calls[0]}" == "exec bde-engine" ]]
[[ "${selected_calls[1]}" == "exec bde" ]]
if grep -q "User message:.*engine:" "$CODEX_RESUME_STUB_LOG"; then
	echo "FAIL: room selector leaked into specialist prompt" >&2
	cat "$CODEX_RESUME_STUB_LOG" >&2
	exit 1
fi
grep -q "inspect storage" "$CODEX_RESUME_STUB_LOG"
echo "ok: room selector rotates first speaker and strips prefix"

# A canonical selector works too, while an unknown selector stays part of the message.
: >"$CODEX_RESUME_STUB_LOG"
canonical_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "bde-engine: inspect canonical")
assert_contains "$canonical_out" "bde-engine:" "canonical selector prints selected expert"
mapfile -t canonical_calls < <(grep '^exec ' "$CODEX_RESUME_STUB_LOG")
[[ "${canonical_calls[0]}" == "exec bde-engine" ]]
if grep -q "User message:.*bde-engine:" "$CODEX_RESUME_STUB_LOG"; then
	echo "FAIL: canonical selector leaked into specialist prompt" >&2
	cat "$CODEX_RESUME_STUB_LOG" >&2
	exit 1
fi
echo "ok: room canonical selector rotates first speaker"

: >"$CODEX_RESUME_STUB_LOG"
unknown_selector_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "random: keep prefix")
assert_contains "$unknown_selector_out" "bde:" "unknown selector uses default first expert"
mapfile -t unknown_calls < <(grep '^exec ' "$CODEX_RESUME_STUB_LOG")
[[ "${unknown_calls[0]}" == "exec bde" ]]
grep -q "random: keep prefix" "$CODEX_RESUME_STUB_LOG"
echo "ok: room ignores unknown selectors without dropping text"

# Empty and failing turns fail clearly and do not continue silently.
assert_fails_contains "room rejects empty message after selector stripping" "message must not be empty" \
	"$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "engine:"
: >"$CODEX_RESUME_STUB_LOG"
assert_fails_contains "room reports codex-resume exec failure" "codex-resume exec bde-engine failed" \
	"$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "engine: break engine"
mapfile -t failed_calls < <(grep '^exec ' "$CODEX_RESUME_STUB_LOG")
[[ "${failed_calls[0]}" == "exec bde-engine" ]]
if [[ ${#failed_calls[@]} -ne 1 ]]; then
	echo "FAIL: room continued after failing first expert" >&2
	cat "$CODEX_RESUME_STUB_LOG" >&2
	exit 1
fi
echo "ok: room stops on failing expert"

# PASS answers are recorded but not printed to the user.
: >"$CODEX_RESUME_STUB_LOG"
pass_out=$("$JEFF_BIN" --config "$CONFIG_DIR" room enter bde-dev "no comment")
if [[ -n "$pass_out" ]]; then
	echo "FAIL: PASS answers were printed" >&2
	printf '%s\n' "$pass_out" >&2
	exit 1
fi
grep -q "PASS" "$ROOM_DIR/transcript.md"
echo "ok: room suppresses PASS output while preserving transcript"

# Transcript persists user and expert turns for later room context.
grep -q "check invoices" "$ROOM_DIR/transcript.md"
grep -q "inspect storage" "$ROOM_DIR/transcript.md"
grep -q "inspect canonical" "$ROOM_DIR/transcript.md"
grep -q "bde-engine" "$ROOM_DIR/transcript.md"
echo "ok: room transcript is durable"
