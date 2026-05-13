#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${CODEX_STUB_ARGS_FILE:-}" ]]; then
	printf '%s\n' "$*" >>"$CODEX_STUB_ARGS_FILE"
fi

prompt=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	resume)
		shift
		shift || true
		if [[ $# -gt 0 ]]; then
			prompt="$1"
		fi
		break
		;;
	*)
		shift
		;;
	esac
done

if grep -q "Clean up Jeff's memory castle gatehouse." <<<"$prompt"; then
	printf 'cleanup prompt: %s\n' "$prompt"
elif grep -q "Memory castle sources:" <<<"$prompt"; then
	printf 'castle answer: %s\n' "$prompt"
else
	printf 'stub answer\n'
fi
