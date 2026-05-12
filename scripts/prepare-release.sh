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

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
FILE_VERSION=$(tr -d '\r' <"$VERSION_FILE" | head -n 1)
if [[ $CURRENT_BRANCH =~ ^release/v(.+)$ ]]; then
  RELEASE_VERSION="${BASH_REMATCH[1]}"
else
  RELEASE_VERSION="$FILE_VERSION"
fi
RELEASE_BRANCH="release/v$RELEASE_VERSION"
STATE_DIR="$WORK_DIR/release-$RELEASE_VERSION"
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

vaultline_version_label() {
  metadata="$WORK_DIR/sidecars/vaultline/metadata.env"
  if [ -f "$metadata" ]; then
    label=$(awk -F= '$1 == "VAULTLINE_VERSION_LABEL" {print substr($0, length($1) + 2)}' "$metadata" | tail -n 1)
    if [ -n "$label" ]; then
      printf '%s' "$label"
      return
    fi
  fi
  printf 'unknown'
}

ensure_clean_tree() {
  if [ "$(current_branch)" = "$RELEASE_BRANCH" ]; then
    return
  fi
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

  if [ "$(current_branch)" = "$RELEASE_BRANCH" ]; then
    mark_done branch
    return
  fi

  ensure_clean_tree
  if git rev-parse --verify --quiet "$RELEASE_BRANCH" >/dev/null; then
    git checkout "$RELEASE_BRANCH"
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
  SKIP_VERSION_BUMP=1 "$REPO_ROOT/scripts/build.sh" --compile --deb
  mark_done build
}

create_release_notes() {
  if step_done notes; then
    echo "[prepare] release notes already generated"
    return
  fi

notes_dir="$REPO_ROOT/doc/ghpages/releases"
snippets_dir="$notes_dir/unreleased"
mkdir -p "$notes_dir" "$snippets_dir"
shopt -s nullglob
snippet_files=("$snippets_dir"/*.md)
shopt -u nullglob
notes_file="$notes_dir/v$RELEASE_VERSION.md"
snippets_used=0
if [ ! -f "$notes_file" ]; then
  changes_block=$(python3 - "$snippets_dir" <<'PY'
import sys, pathlib
dir_path = pathlib.Path(sys.argv[1])
snippets = sorted(dir_path.glob('*.md'))
lines = []
for path in snippets:
    text = path.read_text().strip()
    if text:
        lines.append(text)
if not lines:
    lines = ["- _List changes here_"]
print("\n\n".join(lines))
PY
)
  highlights_block=$(python3 - "$snippets_dir" <<'PY'
import re, sys, pathlib
dir_path = pathlib.Path(sys.argv[1])
snippets = sorted(dir_path.glob('*.md'))
highlights = []
for path in snippets:
    for line in path.read_text().splitlines():
        text = line.strip()
        if not text.startswith("- "):
            continue
        text = text[2:].strip()
        text = re.sub(r"^(feat|fix|docs|chore|refactor|test|build|ci):\s*", "", text)
        if text:
            highlights.append("- " + text[:1].upper() + text[1:])
        break
if not highlights:
    highlights = ["- See changes below."]
print("\n".join(highlights[:5]))
PY
)
  vaultline_label=$(vaultline_version_label)
  cat <<EOF >"$notes_file"
---
layout: page
title: Release v$RELEASE_VERSION
---

## Highlights

$highlights_block

## Downloads

- [GitHub Release](https://github.com/micwin/jeff/releases/tag/v$RELEASE_VERSION)
- [Linux binary](https://github.com/micwin/jeff/releases/download/v$RELEASE_VERSION/jeff)
- [Debian package](https://github.com/micwin/jeff/releases/download/v$RELEASE_VERSION/jeff_${RELEASE_VERSION}_amd64.deb)
- Bundled [Vaultline](https://micwin.github.io/vaultline/): \`$vaultline_label\`

## Changes

$changes_block
EOF
  git add "$notes_file"
  snippets_used=1
fi

  index_file="$REPO_ROOT/doc/ghpages/index.md"
  latest_block="<!-- latest-release:start -->\n## Latest Release\n\n- [Download Jeff v$RELEASE_VERSION](https://github.com/micwin/jeff/releases/tag/v$RELEASE_VERSION)\n- [Release notes]({{ \"/releases/v$RELEASE_VERSION.html\" | relative_url }})\n<!-- latest-release:end -->"
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

  if [ $snippets_used -eq 1 ] && [ ${#snippet_files[@]} -gt 0 ]; then
    rm -f "${snippet_files[@]}"
    git add -u "$snippets_dir"
  fi

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

  vaultline_label=$(vaultline_version_label)
  python3 - "$downloads_file" "$RELEASE_VERSION" "$vaultline_label" <<'PY'
import sys, pathlib
path = pathlib.Path(sys.argv[1])
version = sys.argv[2]
vaultline = sys.argv[3]
row = f"| v{version} | [Binary](https://github.com/micwin/jeff/releases/download/v{version}/jeff) | [Debian](https://github.com/micwin/jeff/releases/download/v{version}/jeff_{version}_amd64.deb) | `{vaultline}` | [Notes]({{{{ \"/releases/v{version}.html\" | relative_url }}}}) |"
lines = path.read_text().splitlines()
try:
    header_idx = next(i for i, line in enumerate(lines) if line.startswith('| Version'))
except StopIteration:
    raise SystemExit('downloads table header not found')
sep_idx = header_idx + 1
if sep_idx >= len(lines) or not lines[sep_idx].startswith('|---------'):
    raise SystemExit('downloads table separator missing')
if 'Vaultline' not in lines[header_idx]:
    header_cells = [cell.strip() for cell in lines[header_idx].strip('|').split('|')]
    sep_cells = [cell.strip() for cell in lines[sep_idx].strip('|').split('|')]
    try:
        notes_idx = header_cells.index('Notes')
    except ValueError:
        notes_idx = len(header_cells)
    header_cells.insert(notes_idx, '[Vaultline](https://micwin.github.io/vaultline/)-Version')
    sep_cells.insert(notes_idx, '-------------------')
    migrated = []
    for line in lines[sep_idx+1:]:
        if not line.startswith('|'):
            migrated.append(line)
            continue
        cells = [cell.strip() for cell in line.strip('|').split('|')]
        if len(cells) == len(header_cells) - 1:
            cells.insert(notes_idx, '_unknown_')
        migrated.append('| ' + ' | '.join(cells) + ' |')
    lines[header_idx] = '| ' + ' | '.join(header_cells) + ' |'
    lines[sep_idx] = '| ' + ' | '.join(sep_cells) + ' |'
    lines = lines[:sep_idx+1] + migrated
body = [line for line in lines[sep_idx+1:] if not line.startswith(f"| v{version} ")]
body.insert(0, row)
new_lines = lines[:sep_idx+1] + body
path.write_text("\n".join(new_lines) + "\n")
PY
  git add "$downloads_file"
  mark_done downloads
}

commit_release_changes() {
  if git diff --cached --quiet; then
    echo "[prepare] no staged changes to commit"
    return
  fi
  git commit -m "chore: release prep v$RELEASE_VERSION"
  echo "[prepare] committed release prep changes"
}

ensure_release_branch
run_build
create_release_notes
update_downloads_table
commit_release_changes

echo "Prepare-release completed for v$RELEASE_VERSION. Review changes and run scripts/publish-release.sh when ready."
