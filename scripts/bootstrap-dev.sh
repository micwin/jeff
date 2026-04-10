#!/usr/bin/env sh
# Bootstrap Go-based tooling for the jeff CLI.

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Fehlendes Tool: $1" >&2
		exit 1
	fi
}

log() {
	printf '%s\n' "$*"
}

need_cmd go

WORK_DIR="$REPO_ROOT/work"
DIST_DIR="$REPO_ROOT/dist"

log "Erzeuge lokale Arbeitsverzeichnisse..."
mkdir -p "$WORK_DIR/go-cache" "$WORK_DIR/go-tmp" "$DIST_DIR"

log "Go-Version:"
go version

log "Bootstrap abgeschlossen. Verwende scripts/build.sh für Builds."
