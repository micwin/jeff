#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}" )/.." && pwd)
SITE_DIR="$ROOT/doc/ghpages"
BUNDLE_PATH="$ROOT/vendor/bundle"

if ! command -v bundle >/dev/null 2>&1; then
  echo "Bundler is required. Please install Ruby and run 'gem install bundler'." >&2
  exit 1
fi

cd "$SITE_DIR"
BUNDLE_PATH="$BUNDLE_PATH" bundle install >/dev/null
BUNDLE_PATH="$BUNDLE_PATH" bundle exec jekyll serve --livereload --baseurl ""
