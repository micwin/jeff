#!/usr/bin/env bash
# Smokey test: validates jeff codex init/ask/status flows using a codex stub.

set -euo pipefail

# Locate the compiled binary produced by the suite setup case.
REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)
JEFF_BIN="$REPO_ROOT/dist/jeff"

# Keep all per-test configuration, data, cache, and logs in Smokey state.
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-cli-config"
export XDG_DATA_HOME="$SMOKEY_STATE_DIR/jeff-cli-data"
export XDG_CACHE_HOME="$SMOKEY_STATE_DIR/jeff-cli-cache"
CODEX_STUB_DIR="$SMOKEY_STATE_DIR/jeff-cli-codex-stub"
CODEX_STUB="$CODEX_STUB_DIR/codex_stub.sh"
CODEX_STUB_ARGS_FILE="$SMOKEY_STATE_DIR/jeff-cli-codex-args.log"
CHAT_FAIL_OUT="$SMOKEY_STATE_DIR/jeff-chat-fail.out"
export CODEX_STUB_ARGS_FILE

assert_eq() {
	local expected=$1
	local actual=$2
	local label=$3
	if [[ "$actual" != "$expected" ]]; then
		printf 'FAIL: %s\nexpected: %s\nactual:   %s\n' "$label" "$expected" "$actual" >&2
		exit 1
	fi
	printf 'ok: %s\n' "$label"
}

# Install the Codex fixture in state so Jeff can execute it as a binary.
mkdir -p "$CODEX_STUB_DIR"
cp "$SMOKEY_TEST_DIR/codex_stub.sh" "$CODEX_STUB"
cp "$SMOKEY_TEST_DIR/codex_header.txt" "$CODEX_STUB_DIR/"
chmod +x "$CODEX_STUB"

# Bind the stub session before exercising Codex-backed commands.
"$JEFF_BIN" --config "$CONFIG_DIR" codex init stub-session --codex-binary "$CODEX_STUB" >/dev/null

# Ask returns the plain answer without token metadata by default.
ask_plain=$("$JEFF_BIN" --config "$CONFIG_DIR" codex ask 'Test question?')
expected_plain="Answer: Test question?"
assert_eq "$expected_plain" "$ask_plain" "ask emits plain answer with dir context"

# Token output is appended only when explicitly requested.
ask_tokens=$("$JEFF_BIN" --config "$CONFIG_DIR" codex ask --show-token-cost 'Another question?')
tokens_answer=$(printf '%s\n' "$ask_tokens" | head -n 1)
tokens_usage=$(printf '%s\n' "$ask_tokens" | tail -n +2)
expected_answer="Answer: Another question?"
if [[ "$tokens_answer" != "$expected_answer" ]]; then
	printf 'FAIL: token answer line mismatch\nexpected: %s\nactual: %s\n' "$expected_answer" "$tokens_answer" >&2
	exit 1
fi
if [[ "$tokens_usage" != Token:* ]]; then
	printf 'FAIL: token usage line missing\n%s\n' "$tokens_usage" >&2
	exit 1
fi
if ! grep -q 'Token: tokens used' <<<"$ask_tokens"; then
	printf 'FAIL: token output missing wording\n%s\n' "$ask_tokens" >&2
	exit 1
fi
echo "ok: ask shows token usage when requested"

# The global status flag prints metadata before the normal answer.
status_out=$("$JEFF_BIN" --status --config "$CONFIG_DIR" codex ask 'Status question?')
expected_answer_suffix=$'\n\n'
expected_answer_suffix+="Answer: Status question?"
if [[ "$status_out" != *"$expected_answer_suffix" ]]; then
	printf 'FAIL: status output missing answer separation\n%s\n' "$status_out" >&2
	exit 1
fi
meta="${status_out%$expected_answer_suffix}"
check_meta_line() { local key=$1; grep -q "^$key: " <<<"$meta" || { printf 'FAIL: missing %s line\n' "$key"; exit 1; }; }
check_meta_line "workdir"
check_meta_line "model"
check_meta_line "provider"
check_meta_line "sandbox"
if ! grep -q '^session id: [A-Za-z0-9][*]*-[A-Za-z0-9][*]*-[A-Za-z0-9][*]*-[A-Za-z0-9][*]*-[A-Za-z0-9][*]*$' <<<"$meta"; then
	printf 'FAIL: session id line not masked as expected\n%s\n' "$meta" >&2
	exit 1
fi
echo "ok: status flag prints metadata before answer"

# Chat injects the Jeff preprompt on first entry and exits cleanly.
chat_out=$("$JEFF_BIN" --config "$CONFIG_DIR" chat)
if [[ "$chat_out" != OpenAI* ]]; then
	printf 'FAIL: chat did not emit header\n%s\n' "$chat_out" >&2
	exit 1
fi
if ! grep -q 'Agent prompt injected' <<<"$chat_out"; then
	printf 'FAIL: chat did not inject agent prompt on first run\n%s\n' "$chat_out" >&2
	exit 1
fi
if ! grep -q 'jeff sagt tschüß' <<<"$chat_out"; then
	printf 'FAIL: chat did not print success farewell\n%s\n' "$chat_out" >&2
	exit 1
fi
echo "ok: chat injects agent prompt on first run"

# A second chat call observes the semaphore and skips prompt reinjection.
chat_out=$("$JEFF_BIN" --config "$CONFIG_DIR" chat)
if ! grep -q '^user$' <<<"$chat_out"; then
	printf 'FAIL: chat did not skip prompt injection after semaphore\n%s\n' "$chat_out" >&2
	exit 1
fi
echo "ok: chat skips agent prompt after semaphore"

# Failing Codex exits still print the configured failure farewell.
if CODEX_STUB_EXIT=2 "$JEFF_BIN" --config "$CONFIG_DIR" chat >"$CHAT_FAIL_OUT" 2>&1; then
	printf 'FAIL: chat unexpectedly succeeded with failing Codex stub\n' >&2
	exit 1
fi
if ! grep -q 'Aaaaaaahhhh' "$CHAT_FAIL_OUT"; then
	printf 'FAIL: chat did not print failure farewell\n%s\n' "$(cat "$CHAT_FAIL_OUT")" >&2
	exit 1
fi
echo "ok: chat reports positive exit code failure"

# All managed Codex calls use YOLO mode.
if ! grep -q -- '--dangerously-bypass-approvals-and-sandbox' "$CODEX_STUB_ARGS_FILE"; then
	printf 'FAIL: Codex calls did not use YOLO mode\n%s\n' "$(cat "$CODEX_STUB_ARGS_FILE")" >&2
	exit 1
fi
echo "ok: codex calls use YOLO mode"
