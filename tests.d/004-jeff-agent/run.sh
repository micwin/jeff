#!/usr/bin/env bash
# Smokey test: validates agent bootstrap, memory logging, and stored commands.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)

JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-config"
DATA_DIR="$SMOKEY_STATE_DIR/jeff-data"
COMMANDS_DIR="$DATA_DIR/jeff/commands"
FIXTURES_DIR="$SMOKEY_TEST_DIR/fixtures"
CODEX_HOME="$SMOKEY_STATE_DIR/codex-home"
CODEX_STUB="$SMOKEY_STATE_DIR/codex-stub.sh"

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

export XDG_DATA_HOME="$DATA_DIR"
export XDG_CACHE_HOME="$SMOKEY_STATE_DIR/jeff-cache"
export CODEX_HOME
cp "$FIXTURES_DIR/codex_stub.sh" "$CODEX_STUB"
chmod +x "$CODEX_STUB"

# Bootstrap deploys the embedded memory castle and creates data subdirectories.
bootstrap_out=$("$JEFF_BIN" --config "$CONFIG_DIR" agent bootstrap)
assert_contains "$bootstrap_out" "Agent defaults ready:" "agent bootstrap reports defaults"
test -f "$DATA_DIR/jeff/memcastle/castle.md"
test -f "$DATA_DIR/jeff/memcastle/system.md"
test -f "$DATA_DIR/jeff/memcastle/gatehouse/index.md"
test -d "$DATA_DIR/jeff/skills"
test -d "$DATA_DIR/jeff/reports"
test -d "$DATA_DIR/jeff/vaultline"
echo "ok: agent bootstrap creates data layout"

# Memcastle info prints metadata and a structure-only tree.
info_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle info)
assert_contains "$info_out" "Wings:    3" "memcastle info counts wings"
assert_contains "$info_out" "Rooms:    3" "memcastle info counts markdown rooms"
assert_contains "$info_out" "gatehouse/" "memcastle info shows gatehouse"
assert_contains "$info_out" "Structure" "memcastle info prints structure"
assert_contains "$info_out" "|--" "memcastle info uses tree layout"
mc_info_out=$("$JEFF_BIN" --config "$CONFIG_DIR" mc info)
assert_contains "$mc_info_out" "Memory Castle" "mc aliases memcastle"

# Memcastle search is offline, case-insensitive, and tolerates collapsed whitespace.
search_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle search "persistent  working")
assert_contains "$search_out" "castle.md:" "memcastle search finds collapsed words"
assert_contains "$search_out" "file-level whitespace-normalized match" "memcastle search reports cross-line matches"

# Codex init binds the single session and refuses accidental overwrite.
"$JEFF_BIN" --config "$CONFIG_DIR" codex init smokey-agent --codex-binary "$CODEX_STUB" >/dev/null
if "$JEFF_BIN" --config "$CONFIG_DIR" codex init other-agent >/tmp/jeff-force.out 2>&1; then
	echo "FAIL: codex init without --force overwrote existing session" >&2
	exit 1
fi
"$JEFF_BIN" --config "$CONFIG_DIR" codex init --force other-agent >/dev/null
grep -q '"session_id": "other-agent"' "$CONFIG_DIR/config.json"
grep -q "\"codex_binary\": \"$CODEX_STUB\"" "$CONFIG_DIR/config.json"
echo "ok: codex init manages one guarded session"

# Codex init --last resolves and prints the newest local Codex session id.
last_session="11111111-2222-3333-4444-555555555555"
older_session="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
mkdir -p "$CODEX_HOME/sessions/2026/05/12"
printf '{}\n' >"$CODEX_HOME/sessions/2026/05/12/rollout-old-$older_session.jsonl"
printf '{}\n' >"$CODEX_HOME/sessions/2026/05/12/rollout-new-$last_session.jsonl"
touch -t 202605121100 "$CODEX_HOME/sessions/2026/05/12/rollout-old-$older_session.jsonl"
touch -t 202605121200 "$CODEX_HOME/sessions/2026/05/12/rollout-new-$last_session.jsonl"
last_out=$("$JEFF_BIN" --config "$CONFIG_DIR" codex init --force --last)
assert_contains "$last_out" "$last_session" "codex init --last reports resolved session"
grep -q "\"session_id\": \"$last_session\"" "$CONFIG_DIR/config.json"
echo "ok: codex init --last stores resolved session"

# Memcastle ask forwards a castle-only prompt to the configured Codex session.
ask_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle ask "where do secrets live?")
assert_contains "$ask_out" "Memory castle sources:" "memcastle ask sends castle sources"
assert_contains "$ask_out" "Do not use web search." "memcastle ask forbids web search"

# Memcastle cleanup asks Codex to sort the gatehouse within the castle root only.
cleanup_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle cleanup)
assert_contains "$cleanup_out" "Clean up Jeff's memory castle gatehouse." "memcastle cleanup sends cleanup prompt"
assert_contains "$cleanup_out" "Work only below this memory castle root:" "memcastle cleanup restricts scope"

# Remember appends to the memory castle logbook.
remember_out=$("$JEFF_BIN" --config "$CONFIG_DIR" agent remember "finance meeting summary")
assert_contains "$remember_out" "Remembered in" "agent remember reports log path"
grep -R "finance meeting summary" "$DATA_DIR/jeff/memcastle/logbook" >/dev/null
echo "ok: agent remember writes logbook"

# Execute runs shared, init, run, and cleanup scripts in a Bash subshell.
mkdir -p "$COMMANDS_DIR/shared" "$COMMANDS_DIR/hello"
cp "$FIXTURES_DIR/commands/hello/"*.sh "$COMMANDS_DIR/hello/"
cp "$FIXTURES_DIR/commands/shared/"*.sh "$COMMANDS_DIR/shared/"
execute_out=$("$JEFF_BIN" --config "$CONFIG_DIR" execute hello alpha beta)
assert_contains "$execute_out" "shared=ready init=ready args=alpha beta" "execute runs command with args"
assert_contains "$execute_out" "cleanup=ready" "execute runs cleanup trap"

# Completion offers stored command names and ignores the shared helper directory.
completion_out=$("$JEFF_BIN" --config "$CONFIG_DIR" __complete execute h 2>/dev/null)
assert_contains "$completion_out" "hello" "execute completion lists stored commands"
if grep -q '^shared$' <<<"$completion_out"; then
	echo "FAIL: execute completion exposed shared helper directory" >&2
	exit 1
fi
echo "ok: execute completion hides shared helpers"
