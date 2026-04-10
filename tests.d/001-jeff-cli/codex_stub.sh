#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
HEADER_FILE="$SCRIPT_DIR/codex_header.txt"

output_last=""
session=""
prompt=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-last-message)
      output_last="$2"
      shift 2
      ;;
    resume)
      shift
      session="$1"
      shift || true
      if [[ $# -gt 0 && "$1" != "--" ]]; then
        prompt="$1"
        shift
      fi
      break
      ;;
    *)
      shift
      ;;
  esac
done

cat "$HEADER_FILE"

after_prompt() {
  local answer="Antwort: $prompt"
  if [[ -n "${output_last:-}" && "$output_last" != "-" ]]; then
    printf '%s\n' "$answer" >"$output_last"
  else
    printf '%s\n' "$answer"
  fi
  echo "tokens used 42 prompt / 1 completion"
}

if [[ -n "$prompt" ]]; then
  after_prompt
else
  printf 'user\nstatus ping\n'
fi
