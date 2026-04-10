#!/usr/bin/env sh
# Build all available project artifacts (Go CLI, docs, sites).

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
DIST_DIR="$REPO_ROOT/dist"

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

mkdir -p "$WORK_DIR/go-cache" "$WORK_DIR/go-tmp" "$DIST_DIR"

GO_ENV="GOCACHE=$WORK_DIR/go-cache GOTMPDIR=$WORK_DIR/go-tmp"

log "==> Go Tests"
(cd "$REPO_ROOT/src/jeff" && env $GO_ENV go test ./...)

log "==> Go Build"
(cd "$REPO_ROOT/src/jeff" && env $GO_ENV go build -o "$DIST_DIR/jeff" ./cmd/jeff)

build_docs() {
	if [ -d "$1" ]; then
		log "==> Dokumentation in $1 gefunden (noch kein Build-Schritt implementiert)"
	fi
}

build_docs "$REPO_ROOT/doc"
build_docs "$REPO_ROOT/site"

log "Build abgeschlossen. Artefakte liegen in $DIST_DIR"
