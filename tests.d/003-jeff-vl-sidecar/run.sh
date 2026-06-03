#!/usr/bin/env bash
# Smokey test: validates that Jeff can execute and complete the embedded Vaultline sidecar.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)

JEFF_BIN="$REPO_ROOT/dist/jeff"
CONFIG_DIR="$SMOKEY_STATE_DIR/jeff-vl-config"
DATA_DIR="$SMOKEY_STATE_DIR/jeff-vl-data"
CACHE_DIR="$SMOKEY_STATE_DIR/jeff-vl-cache"
VAULTLINE_ADDR="127.0.0.1:18428"
VAULTLINE_DIR="$SMOKEY_STATE_DIR/vaultline-daemon"

export XDG_DATA_HOME="$DATA_DIR"
export XDG_CACHE_HOME="$CACHE_DIR"

cleanup() {
  if [[ -n "${VAULTLINE_PID:-}" ]]; then
    kill "$VAULTLINE_PID" >/dev/null 2>&1 || true
    wait "$VAULTLINE_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Execute the embedded sidecar and expect Vaultline to report a semantic version.
version_out=$("$JEFF_BIN" vl version)
if [[ ! "$version_out" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'FAIL: unexpected vaultline version output: %s\n' "$version_out" >&2
  exit 1
fi

echo "ok: jeff vl executes the embedded vaultline sidecar"

# Force both Vaultline selection modes so regressions in the manual switches are visible.
backpack_out=$("$JEFF_BIN" vl --use-backpack-version version)
if [[ ! "$backpack_out" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'FAIL: unexpected forced backpack vaultline version output: %s\n' "$backpack_out" >&2
  exit 1
fi
if command -v vaultline >/dev/null 2>&1; then
  local_out=$("$JEFF_BIN" vl --use-local-version version)
  if [[ ! "$local_out" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'FAIL: unexpected forced local vaultline version output: %s\n' "$local_out" >&2
    exit 1
  fi
fi

echo "ok: jeff vl supports explicit Vaultline version selection"

# Start an isolated normal Vaultline daemon and point Jeff at it.
mkdir -p "$CONFIG_DIR" "$VAULTLINE_DIR/stores"
printf '{ "vaultline": { "addr": "%s" } }\n' "$VAULTLINE_ADDR" >"$CONFIG_DIR/config.json"
"$JEFF_BIN" vl daemon \
  --addr "$VAULTLINE_ADDR" \
  --store-dir "$VAULTLINE_DIR/stores/default" \
  --config-file "$VAULTLINE_DIR/stores.json" &
VAULTLINE_PID=$!
for _ in $(seq 1 50); do
  if curl -fsS "http://$VAULTLINE_ADDR/api/v1/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
curl -fsS "http://$VAULTLINE_ADDR/api/v1/health" >/dev/null

echo "ok: test vaultline daemon is reachable"

# Ask Jeff's completion bridge for top-level Vaultline commands and expect secret support.
top_level=$("$JEFF_BIN" --config "$CONFIG_DIR" __complete vl "" 2>/dev/null)
if ! grep -qx 'secret' <<<"$top_level"; then
  printf 'FAIL: vaultline top-level completion missing secret\n%s\n' "$top_level" >&2
  exit 1
fi

echo "ok: jeff vl forwards top-level completions"

# Ask completion for a nested Vaultline command and expect secret subcommands through the jeff vl prefix.
secret_level=$("$JEFF_BIN" --config "$CONFIG_DIR" __complete vl secret "" 2>/dev/null)
if ! grep -qx 'get' <<<"$secret_level"; then
  printf 'FAIL: vaultline nested completion missing secret get\n%s\n' "$secret_level" >&2
  exit 1
fi

echo "ok: jeff vl forwards nested completions"

# Store and fetch a secret through the external daemon and Jeff's local jeff store.
"$JEFF_BIN" --config "$CONFIG_DIR" vl secret set jeff:smokey-test --value local-value >/dev/null
secret_out=$("$JEFF_BIN" --config "$CONFIG_DIR" vl secret get jeff:smokey-test)
if [[ "$secret_out" != "local-value" ]]; then
  printf 'FAIL: unexpected managed vaultline secret value: %s\n' "$secret_out" >&2
  exit 1
fi

"$JEFF_BIN" --config "$CONFIG_DIR" vl secret set smokey-default-store --value defaulted-value >/dev/null
default_secret_out=$("$JEFF_BIN" --config "$CONFIG_DIR" vl secret get smokey-default-store)
if [[ "$default_secret_out" != "defaulted-value" ]]; then
  printf 'FAIL: unexpected default-store vaultline secret value: %s\n' "$default_secret_out" >&2
  exit 1
fi
"$JEFF_BIN" --config "$CONFIG_DIR" vl secret set --name smokey-name-flag --value named-value >/dev/null
named_secret_out=$("$JEFF_BIN" --config "$CONFIG_DIR" vl secret get --name smokey-name-flag)
if [[ "$named_secret_out" != "named-value" ]]; then
  printf 'FAIL: unexpected --name default-store vaultline secret value: %s\n' "$named_secret_out" >&2
  exit 1
fi

echo "ok: jeff vl stores secrets in the jeff store"

# Jeff keeps the jeff-store passphrase in Jeff config, not Vaultline's store registry.
if ! grep -q '"jeff_store_passphrase"' "$CONFIG_DIR/config.json"; then
  echo "FAIL: Jeff config missing managed Vaultline passphrase" >&2
  exit 1
fi
if grep -q '"passphrase"' "$VAULTLINE_DIR/stores.json"; then
  echo "FAIL: Vaultline store config unexpectedly contains a passphrase" >&2
  exit 1
fi
test -f "$DATA_DIR/jeff/memcastle/jeff/vaultline/store/secrets/smokey-test.vlx"

echo "ok: jeff vl keeps unseal material in Jeff config only"
