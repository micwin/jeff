#!/usr/bin/env bash
# Smokey test: validates jeff tmpl list/validate for nested template paths.

set -euo pipefail

# Expect compiled jeff binary from build case.
JEFF_BIN="$SMOKEY_TEST_ROOT/dist/jeff"

# Prepare isolated config+template tree in Smokey state.
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-tmpl-config"
rm -rf "$CONFIG_DIR"
mkdir -p "$CONFIG_DIR/templates"
cp -R "$SMOKEY_TEST_ROOT/tests.d/002-jeff-tmpl/fixtures/." "$CONFIG_DIR/templates/"

# Expect nested package names from discovered main.tmpl files.
list_out=$("$JEFF_BIN" --config "$CONFIG_DIR" tmpl list)
expected_list=$'finances/report-monthly\nprompt'
if [[ "$list_out" != "$expected_list" ]]; then
  printf 'FAIL: tmpl list mismatch\nexpected:\n%s\nactual:\n%s\n' "$expected_list" "$list_out" >&2
  exit 1
fi

echo "ok: tmpl list shows nested package names"

# Expect validate to succeed for an existing nested template.
validate_out=$("$JEFF_BIN" --config "$CONFIG_DIR" tmpl validate finances/report-monthly)
if [[ "$validate_out" != *'Template "finances/report-monthly" is valid.'* ]]; then
  printf 'FAIL: validate output mismatch\n%s\n' "$validate_out" >&2
  exit 1
fi

echo "ok: tmpl validate accepts nested template package"

# Expect traversal names to be rejected for safety.
if "$JEFF_BIN" --config "$CONFIG_DIR" tmpl validate ../escape >/dev/null 2>&1; then
  echo "FAIL: traversal template name unexpectedly accepted" >&2
  exit 1
fi

echo "ok: tmpl validate rejects path traversal names"
