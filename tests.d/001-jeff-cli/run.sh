#!/usr/bin/env bash
# Smokey test: validates jeff init/ask/status flows using a codex stub.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
tmp_root=$(mktemp -d)
trap 'rm -rf "$tmp_root"' EXIT
mkdir -p "$tmp_root/go-cache" "$tmp_root/go-tmp"

JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$tmp_root/config"
CODEX_STUB_DIR="$tmp_root/codex_stub"
CODEX_STUB="$CODEX_STUB_DIR/codex_stub.sh"

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

mkdir -p "$CODEX_STUB_DIR"
cp "$SCRIPT_DIR/codex_stub.sh" "$CODEX_STUB"
cp "$SCRIPT_DIR/codex_header.txt" "$CODEX_STUB_DIR/"
chmod +x "$CODEX_STUB"

if [[ ! -x "$JEFF_BIN" ]]; then
	echo "Build artefact dist/jeff fehlt – bitte tests.d/000-build ausführen" >&2
	exit 1
fi

"$JEFF_BIN" --config "$CONFIG_DIR" init --session stub-session --codex-binary "$CODEX_STUB" >/dev/null

ask_plain=$("$JEFF_BIN" --config "$CONFIG_DIR" ask 'Testfrage?')
assert_eq "Antwort: Testfrage?" "$ask_plain" "ask emits plain answer"

ask_tokens=$("$JEFF_BIN" --config "$CONFIG_DIR" ask --show-token-cost 'Noch eine Frage?')
if [[ "$ask_tokens" != $'Antwort: Noch eine Frage?\nToken: '* ]]; then
	printf 'FAIL: ask token flag missing expected prefix\n%s\n' "$ask_tokens" >&2
	exit 1
fi
if ! grep -q 'Token: tokens used' <<<"$ask_tokens"; then
	printf 'FAIL: token output missing wording\n%s\n' "$ask_tokens" >&2
	exit 1
fi
echo "ok: ask shows token usage when requested"

status_out=$("$JEFF_BIN" --status --config "$CONFIG_DIR" ask 'Statusfrage?')
if [[ "$status_out" != *$'\n\nAntwort: Statusfrage?' ]]; then
	printf 'FAIL: status output missing answer separation\n%s\n' "$status_out" >&2
	exit 1
fi
meta="${status_out%$'\n\nAntwort: Statusfrage?'}"
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

chat_out=$("$JEFF_BIN" --config "$CONFIG_DIR" chat)
if [[ "$chat_out" != OpenAI* ]]; then
	printf 'FAIL: chat did not emit header\n%s\n' "$chat_out" >&2
	exit 1
fi
if ! grep -q '^user$' <<<"$chat_out"; then
	printf 'FAIL: chat missing user line\n' >&2
	exit 1
fi
echo "ok: chat streams interactive output"
