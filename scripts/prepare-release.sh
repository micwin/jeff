#!/usr/bin/env bash
# Prepare a release by creating a release branch, building artifacts,
# generating release notes, and updating the GitHub Pages content.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
VERSION_FILE="$REPO_ROOT/src/jeff/internal/version/VERSION"

if [ ! -f "$VERSION_FILE" ]; then
  echo "Missing version file at $VERSION_FILE" >&2
  exit 1
fi

VERSION=$(tr -d '\r' <"$VERSION_FILE" | head -n 1)
RELEASE_BRANCH="release/v$VERSION"
STATE_DIR="$WORK_DIR/release-$VERSION"
mkdir -p "$STATE_DIR"

current_branch() {
  git rev-parse --abbrev-ref HEAD
}

step_done() {
  [ -f "$STATE_DIR/$1.done" ]
}

mark_done() {
  touch "$STATE_DIR/$1.done"
}

ensure_clean_tree() {
  if ! git diff --quiet --ignore-submodules HEAD; then
    echo "Working tree has uncommitted changes. Commit or stash them before continuing." >&2
    exit 1
  fi
}

ensure_release_branch() {
  if step_done branch; then
    echo "[prepare] branch step already done"
    return
  fi

  ensure_clean_tree
  if git rev-parse --verify --quiet "$RELEASE_BRANCH" >/dev/null; then
    if [ "$(current_branch)" != "$RELEASE_BRANCH" ]; then
      git checkout "$RELEASE_BRANCH"
    fi
  else
    git checkout -b "$RELEASE_BRANCH"
  fi
  mark_done branch
}

run_build() {
  if step_done build; then
    echo "[prepare] build step already done"
    return
  fi
  "$REPO_ROOT/scripts/build.sh" --compile --deb
  mark_done build
}

create_release_notes() {
  if step_done notes; then
    echo "[prepare] release notes already generated"
    return
  fi

  notes_dir="$REPO_ROOT/doc/ghpages/releases"
  mkdir -p "$notes_dir"
  notes_file="$notes_dir/v$VERSION.md"
  if [ ! -f "$notes_file" ]; then
    cat <<EOF >"$notes_file"
---
layout: page
title: Release v$VERSION
---

## Highlights

- _Add highlights here_

## Downloads

- [GitHub Release](https://github.com/micwin/jeff/releases/tag/v$VERSION)
- [Linux binary](https://github.com/micwin/jeff/releases/download/v$VERSION/jeff)
- [Debian package](https://github.com/micwin/jeff/releases/download/v$VERSION/jeff_${VERSION}_amd64.deb)

## Changes

- _List changes here_
EOF
    git add "$notes_file"
  fi

  release_index="$REPO_ROOT/doc/ghpages/releases.md"
  if [ ! -f "$release_index" ]; then
    cat <<EOF >"$release_index"
---
layout: page
title: Releases
---

## Releases

EOF
    git add "$release_index"
  fi
  if ! grep -q "v$VERSION" "$release_index"; then
    printf '\n- [v%s](/releases/v%s.html)\n' "$VERSION" "$VERSION" >>"$release_index"
    git add "$release_index"
  fi

  index_file="$REPO_ROOT/doc/ghpages/index.md"
  latest_block="<!-- latest-release:start -->\n## Latest Release\n\n- [Download Jeff v$VERSION](https://github.com/micwin/jeff/releases/tag/v$VERSION)\n- [Release notes](/releases/v$VERSION.html)\n<!-- latest-release:end -->"
  if grep -q "latest-release:start" "$index_file"; then
    python3 - "$index_file" "$latest_block" <<'PY'
import sys, pathlib
index_path = pathlib.Path(sys.argv[1])
block = sys.argv[2]
text = index_path.read_text()
import re
pattern = re.compile(r"<!-- latest-release:start -->.*?<!-- latest-release:end -->", re.S)
new_text, count = pattern.subn(block, text)
if count == 0:
    new_text = text.strip() + "\n\n" + block + "\n"
index_path.write_text(new_text)
PY
  else
    printf '\n%s\n' "$latest_block" >>"$index_file"
  fi
  git add "$index_file"

  mark_done notes
}

update_downloads_table() {
  if step_done downloads; then
    echo "[prepare] downloads table already updated"
    return
  fi

  downloads_file="$REPO_ROOT/doc/ghpages/downloads.md"
  if [ ! -f "$downloads_file" ]; then
    cat <<EOF >"$downloads_file"
---
layout: page
title: Downloads
---

Direct links to current and previous builds.

| Version | Binary | Debian | Notes |
|---------|--------|--------|-------|
EOF
    git add "$downloads_file"
  fi

  python3 - "$downloads_file" "$VERSION" <<'PY'
import sys, pathlib
path = pathlib.Path(sys.argv[1])
version = sys.argv[2]
row = f"| v{version} | [Binary](https://github.com/micwin/jeff/releases/download/v{version}/jeff) | [Debian](https://github.com/micwin/jeff/releases/download/v{version}/jeff_{version}_amd64.deb) | [Notes](/releases/v{version}.html) |"
lines = path.read_text().splitlines()
try:
    header_idx = next(i for i, line in enumerate(lines) if line.startswith('| Version'))
except StopIteration:
    raise SystemExit('downloads table header not found')
sep_idx = header_idx + 1
if sep_idx >= len(lines) or not lines[sep_idx].startswith('|---------'):
    raise SystemExit('downloads table separator missing')
body = [line for line in lines[sep_idx+1:] if not line.startswith(f"| v{version} ")]
body.insert(0, row)
new_lines = lines[:sep_idx+1] + body
path.write_text("\n".join(new_lines) + "\n")
PY
  git add "$downloads_file"
  mark_done downloads
}

ensure_release_branch
run_build
create_release_notes
update_downloads_table

echo "Prepare-release completed for v$VERSION. Review changes and run scripts/publish-release.sh when ready."
