#!/usr/bin/env sh
# Build all available project artifacts (Go CLI, docs, sites).

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
DIST_DIR="$REPO_ROOT/dist"
VERSION_FILE="$REPO_ROOT/src/jeff/internal/version/VERSION"

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Fehlendes Tool: $1" >&2
		exit 1
	fi
}

log() {
	printf '%s\n' "$*"
}

bump_version() {
	if [ "${SKIP_VERSION_BUMP:-0}" = "1" ]; then
		log "==> Version bump übersprungen (SKIP_VERSION_BUMP=1)"
		return
	fi
	if [ ! -f "$VERSION_FILE" ]; then
		echo "0.1.0" >"$VERSION_FILE"
	fi
	current=$(tr -d '\r' <"$VERSION_FILE" | head -n 1)
	IFS='.' set -- $current
	major=${1:-0}
	minor=${2:-1}
	patch=${3:-0}
	case $patch in
		*[^0-9]*) patch=0 ;;
		*) patch=$((patch + 1)) ;;
	esac
	new_version="$major.$minor.$patch"
	echo "$new_version" >"$VERSION_FILE"
	log "==> Version erhöht auf $new_version"
}

need_cmd go

bump_version

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
