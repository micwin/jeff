#!/usr/bin/env sh
# Bootstrap Go-based tooling for the jeff CLI.

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Missing tool: $1" >&2
		exit 1
	fi
}

log() {
	printf '%s\n' "$*"
}

need_cmd go

WORK_DIR="$REPO_ROOT/work"
DIST_DIR="$REPO_ROOT/dist"

log "Creating local work directories..."
mkdir -p "$WORK_DIR/go-cache" "$WORK_DIR/go-tmp" "$DIST_DIR"

log "Go version:"
go version

if [ -f "$REPO_ROOT/doc/ghpages/Gemfile" ]; then
	if ! command -v bundle >/dev/null 2>&1; then
		sudo apt-get update
		sudo apt-get install -y ruby-full ruby-dev ruby3.2-dev build-essential
		sudo gem install bundler
	fi
	if command -v bundle >/dev/null 2>&1; then
		log "Installing ghpages dependencies (bundle install)"
		if ! (cd "$REPO_ROOT/doc/ghpages" && BUNDLE_PATH="$REPO_ROOT/vendor/bundle" bundle install >/dev/null); then
			log "bundle install failed – please check the errors above"
		fi
	else
		log "Skipping ghpages dependencies; bundler still unavailable"
	fi
fi

log "Bootstrap complete. Use scripts/build.sh for builds."
