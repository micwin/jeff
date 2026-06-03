#!/usr/bin/env bash
# Smokey test: validates Jeff init, memory logging, and stored commands.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)

JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-config"
DATA_DIR="$SMOKEY_STATE_DIR/jeff-data"
COMMANDS_DIR="$DATA_DIR/jeff/commands"
FIXTURES_DIR="$SMOKEY_TEST_DIR/fixtures"
CODEX_HOME="$SMOKEY_STATE_DIR/codex-home"
CODEX_STUB="$SMOKEY_STATE_DIR/codex-stub.sh"
CODEX_STUB_ARGS_FILE="$SMOKEY_STATE_DIR/codex-agent-args.log"
AGENT_BOOTSTRAP_OUT="$SMOKEY_STATE_DIR/jeff-agent-bootstrap.out"
CODEX_FORCE_OUT="$SMOKEY_STATE_DIR/jeff-force.out"

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
export CODEX_STUB_ARGS_FILE
cp "$FIXTURES_DIR/codex_stub.sh" "$CODEX_STUB"
chmod +x "$CODEX_STUB"

# Init deploys the embedded memory castle and creates data subdirectories.
init_out=$("$JEFF_BIN" --config "$CONFIG_DIR" init)
assert_contains "$init_out" "Jeff initialized:" "jeff init reports defaults"
test -f "$DATA_DIR/jeff/memcastle/castle.md"
test -f "$DATA_DIR/jeff/memcastle/system.md"
test -f "$DATA_DIR/jeff/memcastle/gatehouse/index.md"
test -f "$DATA_DIR/jeff/memcastle/codex/continuity/index.md"
test -f "$DATA_DIR/jeff/memcastle/finance/banking/api-access.md"
test ! -d "$DATA_DIR/jeff/memcastle/wings"
test -d "$DATA_DIR/jeff/skills"
test -d "$DATA_DIR/jeff/reports"
test -f "$DATA_DIR/jeff/memcastle/jeff/vaultline/index.md"
test -f "$DATA_DIR/jeff/memcastle/jeff/vaultline/skill.md"
echo "ok: jeff init creates data layout"

# The old explicit agent bootstrap command is intentionally gone.
if "$JEFF_BIN" --config "$CONFIG_DIR" agent bootstrap >"$AGENT_BOOTSTRAP_OUT" 2>&1; then
	echo "FAIL: agent bootstrap unexpectedly exists" >&2
	exit 1
fi
echo "ok: agent bootstrap is not available"

# Memcastle status prints metadata, while tree prints directory structure only.
status_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle status)
assert_contains "$status_out" "Root: $DATA_DIR/jeff/memcastle" "memcastle status uses Smokey state"
assert_contains "$status_out" "Wings:" "memcastle status prints wing count"
assert_contains "$status_out" "Rooms:" "memcastle status prints room count"
if grep -q "Structure" <<<"$status_out"; then
	echo "FAIL: memcastle status printed tree structure" >&2
	echo "$status_out" >&2
	exit 1
fi
tree_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle tree)
assert_contains "$tree_out" "gatehouse/" "memcastle tree shows gatehouse"
assert_contains "$tree_out" "finance/" "memcastle tree shows flat wings"
assert_contains "$tree_out" "Structure" "memcastle tree prints structure"
assert_contains "$tree_out" "|--" "memcastle tree uses tree layout"
if grep -q "castle.md" <<<"$tree_out"; then
	echo "FAIL: memcastle tree exposed files" >&2
	echo "$tree_out" >&2
	exit 1
fi
echo "ok: memcastle tree hides files"
mkdir -p "$DATA_DIR/jeff/memcastle/.git/objects"
printf 'ref: refs/heads/develop\n' >"$DATA_DIR/jeff/memcastle/.git/HEAD"
tree_out=$("$JEFF_BIN" --config "$CONFIG_DIR" mc tree)
assert_contains "$tree_out" "Structure" "mc tree aliases memcastle tree"
if grep -q '.git' <<<"$tree_out"; then
	echo "FAIL: memcastle tree exposed git repository internals" >&2
	echo "$tree_out" >&2
	exit 1
fi
echo "ok: memcastle tree hides git repository internals"
mc_status_out=$("$JEFF_BIN" --config "$CONFIG_DIR" mc status)
assert_contains "$mc_status_out" "Memory Castle" "mc aliases memcastle status"
path_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle path)
assert_contains "$path_out" "$DATA_DIR/jeff/memcastle" "memcastle path prints root"
mc_path_out=$("$JEFF_BIN" --config "$CONFIG_DIR" mc path)
assert_contains "$mc_path_out" "$DATA_DIR/jeff/memcastle" "mc path aliases memcastle path"

# Memcastle path resolves exact castle-relative paths and searches directory
# segments without exposing locations outside the active Smokey castle root.
mkdir -p "$DATA_DIR/jeff/memcastle/projects/bde/dist"
mkdir -p "$DATA_DIR/jeff/memcastle/projects/bde/module/main"
mkdir -p "$DATA_DIR/jeff/memcastle/jeff/dist"
mkdir -p "$DATA_DIR/jeff/memcastle/gatehouse/dist"
exact_path_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle path /projects/bde)
assert_contains "$exact_path_out" "$DATA_DIR/jeff/memcastle/projects/bde" "memcastle path resolves exact absolute castle path"
dist_path_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle path dist)
assert_contains "$dist_path_out" "$DATA_DIR/jeff/memcastle/projects/bde/dist" "memcastle path finds segment matches"
assert_contains "$dist_path_out" "$DATA_DIR/jeff/memcastle/jeff/dist" "memcastle path finds multiple segment matches"
bde_dist_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle path bde/dist)
assert_contains "$bde_dist_out" "$DATA_DIR/jeff/memcastle/projects/bde/dist" "memcastle path finds relative path sequence"
bde_main_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle path 'bde/*/main')
assert_contains "$bde_main_out" "$DATA_DIR/jeff/memcastle/projects/bde/module/main" "memcastle path supports single-segment wildcard"
quoted_dist_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle path '`**/dist`')
assert_contains "$quoted_dist_out" "$DATA_DIR/jeff/memcastle/gatehouse/dist" "memcastle path accepts quoted glob-like input"
if "$JEFF_BIN" --config "$CONFIG_DIR" memcastle path /missing >/dev/null 2>&1; then
	echo "FAIL: memcastle path accepted missing exact path" >&2
	exit 1
fi
echo "ok: memcastle path resolves castle directories by name"

# Continuity import stores the newest Codex compact note in the castle.
mkdir -p "$CODEX_HOME/memories"
printf '# Compact Note\n\nNeed: preserve this state.\n' >"$CODEX_HOME/memories/2026-05-27-jeff-compact-note.md"
continuity_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle continuity import)
assert_contains "$continuity_out" "Imported continuity note:" "continuity import reports import"
test -f "$DATA_DIR/jeff/memcastle/codex/continuity/2026-05-27-jeff-compact-note.md"
grep -q "preserve this state" "$DATA_DIR/jeff/memcastle/codex/continuity/2026-05-27-jeff-compact-note.md"
echo "ok: memcastle continuity import stores compact notes"

# Memcastle search is offline, case-insensitive, and tolerates collapsed whitespace.
search_out=$("$JEFF_BIN" --config "$CONFIG_DIR" memcastle search "persistent  working")
assert_contains "$search_out" "castle.md:" "memcastle search finds collapsed words"
assert_contains "$search_out" "file-level whitespace-normalized match" "memcastle search reports cross-line matches"

# Codex init also deploys embedded defaults, binds the single session, and refuses accidental overwrite.
"$JEFF_BIN" --config "$CONFIG_DIR" codex init smokey-agent --codex-binary "$CODEX_STUB" >/dev/null
if "$JEFF_BIN" --config "$CONFIG_DIR" codex init other-agent >"$CODEX_FORCE_OUT" 2>&1; then
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
if ! grep -q -- '--dangerously-bypass-approvals-and-sandbox' "$CODEX_STUB_ARGS_FILE"; then
	echo "FAIL: memcastle Codex calls did not use YOLO mode" >&2
	cat "$CODEX_STUB_ARGS_FILE" >&2
	exit 1
fi
echo "ok: memcastle Codex calls use YOLO mode"

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
