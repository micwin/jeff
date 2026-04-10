#!/usr/bin/env bash
# Build the GitHub Pages site into work/ghpages-site

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
SITE_DIR="$WORK_DIR/ghpages-site"

rm -rf "$SITE_DIR"
mkdir -p "$SITE_DIR"

pushd "$REPO_ROOT/doc/ghpages" >/dev/null
bundle config set --local path "$REPO_ROOT/vendor/bundle"
bundle install
BUNDLE_PATH="$REPO_ROOT/vendor/bundle" bundle exec jekyll build -d "$SITE_DIR"
popd >/dev/null

echo "Site built into $WORK_DIR/ghpages-site"
