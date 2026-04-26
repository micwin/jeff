#!/usr/bin/env bash
# Smokey test: validates that Jeff can execute and complete the embedded Vaultline sidecar.

set -euo pipefail

JEFF_BIN="$SMOKEY_TEST_ROOT/../dist/jeff"

# Execute the embedded sidecar and expect Vaultline to report a semantic version.
version_out=$("$JEFF_BIN" vl version)
if [[ ! "$version_out" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'FAIL: unexpected vaultline version output: %s\n' "$version_out" >&2
  exit 1
fi

echo "ok: jeff vl executes the embedded vaultline sidecar"

# Ask Jeff's completion bridge for top-level Vaultline commands and expect secret support.
top_level=$("$JEFF_BIN" __complete vl "" 2>/dev/null)
if ! grep -qx 'secret' <<<"$top_level"; then
  printf 'FAIL: vaultline top-level completion missing secret\n%s\n' "$top_level" >&2
  exit 1
fi

echo "ok: jeff vl forwards top-level completions"

# Ask completion for a nested Vaultline command and expect secret subcommands through the jeff vl prefix.
secret_level=$("$JEFF_BIN" __complete vl secret "" 2>/dev/null)
if ! grep -qx 'get' <<<"$secret_level"; then
  printf 'FAIL: vaultline nested completion missing secret get\n%s\n' "$secret_level" >&2
  exit 1
fi

echo "ok: jeff vl forwards nested completions"
