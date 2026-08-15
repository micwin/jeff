#!/usr/bin/env bash
# Smokey test: validates portable specialist continuity archives without production data.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)
JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$SMOKEY_STATE_DIR/archive-config"
DATA_DIR="$SMOKEY_STATE_DIR/archive-data"
CODEX_DIR="$SMOKEY_STATE_DIR/codex-home"
BIN_DIR="$SMOKEY_STATE_DIR/archive-bin"
BRIEF_LOG="$SMOKEY_STATE_DIR/archive-briefs.log"
REMOTE_DIR="$SMOKEY_STATE_DIR/remote.git"
CLEAN_REPO="$SMOKEY_STATE_DIR/clean-repo"
DIRTY_REPO="$SMOKEY_STATE_DIR/dirty-repo"

# Test-local tools emulate codex-ctl and Vaultline without touching personal services.
mkdir -p "$BIN_DIR" "$CODEX_DIR/sessions/2026/01/01" "$CONFIG_DIR" "$DATA_DIR/jeff/memcastle/codex/tools"
cp "$SMOKEY_TEST_DIR/fixtures/codex-ctl" "$BIN_DIR/codex-ctl"
cp "$SMOKEY_TEST_DIR/fixtures/vaultline" "$BIN_DIR/vaultline"
chmod +x "$BIN_DIR/codex-ctl" "$BIN_DIR/vaultline"
export PATH="$BIN_DIR:$PATH"
export XDG_CONFIG_HOME="$SMOKEY_STATE_DIR/xdg-config"
export XDG_DATA_HOME="$DATA_DIR"
export XDG_CACHE_HOME="$SMOKEY_STATE_DIR/archive-cache"
export CODEX_HOME="$CODEX_DIR"
export ARCHIVE_CLEAN_REPO="$CLEAN_REPO"
export ARCHIVE_DIRTY_REPO="$DIRTY_REPO"
export ARCHIVE_BRIEF_LOG="$BRIEF_LOG"

# A clean pushed repository should be referenced; a dirty repository needs one flat snapshot.
git init --bare "$REMOTE_DIR" >/dev/null
git clone "$REMOTE_DIR" "$CLEAN_REPO" >/dev/null 2>&1
git -C "$CLEAN_REPO" config user.name Smokey
git -C "$CLEAN_REPO" config user.email smokey@example.invalid
cp "$SMOKEY_TEST_DIR/fixtures/clean.txt" "$CLEAN_REPO/clean.txt"
git -C "$CLEAN_REPO" add clean.txt
git -C "$CLEAN_REPO" commit -m 'test: clean fixture' >/dev/null
git -C "$CLEAN_REPO" push -u origin HEAD >/dev/null 2>&1
git clone "$CLEAN_REPO" "$DIRTY_REPO" >/dev/null 2>&1
cp "$SMOKEY_TEST_DIR/fixtures/dirty.txt" "$DIRTY_REPO/dirty.txt"

# Two aliases share one session while a third session proves independent transcript handling.
cp "$SMOKEY_TEST_DIR/fixtures/session-shared.jsonl" \
  "$CODEX_DIR/sessions/2026/01/01/rollout-test-11111111-1111-1111-1111-111111111111.jsonl"
cp "$SMOKEY_TEST_DIR/fixtures/session-dirty.jsonl" \
  "$CODEX_DIR/sessions/2026/01/01/rollout-test-22222222-2222-2222-2222-222222222222.jsonl"

# Refresh must deduplicate sessions and repositories while preserving dirty state privately.
refresh_out=$($JEFF_BIN --config "$CONFIG_DIR" specialists archive refresh --codex-home "$CODEX_DIR")
[[ "$refresh_out" == *"Archived 4 specialist(s), 2 session(s), 2 repository state(s)."* ]]
ARCHIVE_ROOT="$DATA_DIR/jeff/memcastle/codex/specialist-archive"
jq -e '.specialists | length == 4' "$ARCHIVE_ROOT/registry.json" >/dev/null
jq -e 'length == 2' "$ARCHIVE_ROOT/repositories.json" >/dev/null
[[ $(find "$ARCHIVE_ROOT/private/sessions" -name '*.jsonl.gz' | wc -l) -eq 2 ]]
[[ $(find "$ARCHIVE_ROOT/private/repositories" -name '*.tar.gz' | wc -l) -eq 1 ]]
grep -q '^\*$' "$ARCHIVE_ROOT/private/.gitignore"
echo "ok: refresh deduplicates specialist continuity data"

# Offline search defaults to messages and supports aliases, regex, and bounded output.
search_out=$($JEFF_BIN --config "$CONFIG_DIR" specialists archive search --alias gov 'Needle Phrase')
[[ "$search_out" == gov$'\t'*"Needle Phrase"* ]]
regex_out=$($JEFF_BIN --config "$CONFIG_DIR" specialists archive search --regex 'dirty-[0-9]+')
[[ "$regex_out" == dirty$'\t'*"dirty-42"* ]]
echo "ok: archived transcripts are searchable offline"

# Completion exposes archived aliases and valid scopes while treating queries as free text.
alias_completion=$($JEFF_BIN --config "$CONFIG_DIR" __complete specialists archive search --alias g 2>/dev/null)
[[ "$alias_completion" == *"gov"* ]]
scope_completion=$($JEFF_BIN --config "$CONFIG_DIR" __complete specialists archive search --scope m 2>/dev/null)
[[ "$scope_completion" == *"messages"* ]]
query_completion=$($JEFF_BIN --config "$CONFIG_DIR" __complete specialists archive search needle 2>/dev/null)
[[ "$query_completion" == *":4"* ]]
echo "ok: archive completion covers aliases, scopes, and free-text queries"

# Status and successor interfaces expose recovery metadata without reading private payloads.
status_out=$($JEFF_BIN --config "$CONFIG_DIR" specialists archive status)
[[ "$status_out" == *'"sessions": 2'* ]]
successor_out=$($JEFF_BIN --config "$CONFIG_DIR" specialists archive successor)
[[ "$successor_out" == *"Treat every archived session id as historical identity"* ]]
[[ -L "$CODEX_DIR/skills/specialist-archive" ]]
echo "ok: status, successor prompt, and skill link are available"

# Brief refresh calls one representative per session and links shared output to both aliases.
: >"$BRIEF_LOG"
$JEFF_BIN --config "$CONFIG_DIR" specialists archive refresh --briefs --codex-home "$CODEX_DIR" >/dev/null
[[ $(wc -l <"$BRIEF_LOG") -eq 2 ]]
[[ -f "$DATA_DIR/jeff/memcastle/codex/specialists/gov/current-state.md" ]]
[[ -f "$DATA_DIR/jeff/memcastle/codex/specialists/gov-copy/current-state.md" ]]
grep -q 'Collected through alias: `gov-copy`' "$DATA_DIR/jeff/memcastle/codex/specialists/gov/current-state.md"
echo "ok: continuity briefs use one exec-capable representative per session"

# Repeating refresh prunes obsolete payloads after a dirty working tree changes.
printf 'changed again\n' >>"$DIRTY_REPO/dirty.txt"
$JEFF_BIN --config "$CONFIG_DIR" specialists archive refresh --codex-home "$CODEX_DIR" >/dev/null
[[ $(find "$ARCHIVE_ROOT/private/sessions" -name '*.jsonl.gz' | wc -l) -eq 2 ]]
[[ $(find "$ARCHIVE_ROOT/private/repositories" -name '*.tar.gz' | wc -l) -eq 1 ]]
echo "ok: specialist archive refresh is reentrant"
