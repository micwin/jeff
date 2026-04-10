#!/usr/bin/env bash
# Publish a prepared release: validate artifacts, push release branch, create
# GitHub release (if gh CLI is available), and deploy GitHub Pages content.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
VERSION_FILE="$REPO_ROOT/src/jeff/internal/version/VERSION"

current_branch() {
  git rev-parse --abbrev-ref HEAD
}

BRANCH=$(current_branch)
if [[ $BRANCH =~ ^release/v(.+)$ ]]; then
  RELEASE_VERSION="${BASH_REMATCH[1]}"
else
  echo "You must be on a release branch (release/vX.Y.Z) to publish." >&2
  exit 1
fi

RELEASE_BRANCH="$BRANCH"
STATE_DIR="$WORK_DIR/release-$RELEASE_VERSION"
mkdir -p "$STATE_DIR"

step_done() {
  [ -f "$STATE_DIR/publish-$1.done" ]
}

mark_done() {
  touch "$STATE_DIR/publish-$1.done"
}

require_on_release_branch() {
  if [ "$(current_branch)" != "$RELEASE_BRANCH" ]; then
    echo "You must be on $RELEASE_BRANCH to publish (currently $(current_branch))." >&2
    exit 1
  fi
}

ensure_clean_tree() {
  if ! git diff --quiet --ignore-submodules HEAD; then
    echo "Working tree has uncommitted changes. Commit or stash before publishing." >&2
    exit 1
  fi
}

require_on_release_branch
ensure_clean_tree

verify_artifacts() {
  if step_done verify; then
    echo "[publish] verification already done"
    return
  fi

  DIST_DIR="$REPO_ROOT/dist"
  BIN="$DIST_DIR/jeff"
  if [ ! -x "$BIN" ]; then
    echo "Missing compiled binary at $BIN" >&2
    exit 1
  fi

  bin_version=$("$BIN" version | tr -d '\r' | head -n 1)
  if [ "$bin_version" != "$RELEASE_VERSION" ]; then
    echo "Binary reports version $bin_version but expected $RELEASE_VERSION" >&2
    exit 1
  fi

  arch=$(dpkg --print-architecture 2>/dev/null || uname -m)
  DEB="$DIST_DIR/jeff_${RELEASE_VERSION}_${arch}.deb"
  if [ ! -f "$DEB" ]; then
    echo "Missing Debian package $DEB" >&2
    exit 1
  fi
  deb_version=$(dpkg-deb --info "$DEB" 2>/dev/null | awk '/Version:/ {print $2; exit}')
  if [ "$deb_version" != "$RELEASE_VERSION" ]; then
    echo "Debian package reports version $deb_version but expected $RELEASE_VERSION" >&2
    exit 1
  fi

  if ! grep -q "v$RELEASE_VERSION" "$REPO_ROOT/doc/ghpages/index.md"; then
    echo "Latest release section in doc/ghpages/index.md does not mention v$RELEASE_VERSION" >&2
    exit 1
  fi

  mark_done verify
}

push_release_branch() {
  if step_done push-release; then
    echo "[publish] release branch already pushed"
    return
  fi
  git push origin "$RELEASE_BRANCH"
  mark_done push-release
}

create_github_release() {
  if step_done github-release; then
    echo "[publish] GitHub release already created"
    return
  fi
  DIST_DIR="$REPO_ROOT/dist"
  arch=$(dpkg --print-architecture 2>/dev/null || uname -m)
  DEB="$DIST_DIR/jeff_${RELEASE_VERSION}_${arch}.deb"
  BIN="$DIST_DIR/jeff"

  if command -v gh >/dev/null 2>&1; then
    gh release create "v$RELEASE_VERSION" "$BIN#jeff" "$DEB#jeff_${RELEASE_VERSION}_${arch}.deb" \
      --title "Jeff v$RELEASE_VERSION" \
      --notes-file "$REPO_ROOT/doc/ghpages/releases/v$RELEASE_VERSION.md" || {
        echo "gh release create failed" >&2
        exit 1
      }
    mark_done github-release
  else
    cat <<EOF
GitHub CLI (gh) not found. Please create the release manually:
  gh release create v$VERSION dist/jeff dist/jeff_${VERSION}_${arch}.deb \\
      --title "Jeff v$VERSION" --notes-file doc/ghpages/releases/v$VERSION.md
After creating the release, re-run this script.
EOF
    exit 1
  fi
}

publish_ghpages() {
  if step_done ghpages; then
    echo "[publish] ghpages already updated"
    return
  fi

  BUNDLE_PATH="$REPO_ROOT/vendor/bundle"
  SITE_DIR="$WORK_DIR/ghpages-site"
  rm -rf "$SITE_DIR"
  (cd "$REPO_ROOT/doc/ghpages" && BUNDLE_PATH="$BUNDLE_PATH" bundle exec jekyll build -d "$SITE_DIR")

  WORKTREE_DIR="$WORK_DIR/ghpages-worktree"
  rm -rf "$WORKTREE_DIR"
  if git show-ref --verify --quiet refs/heads/ghpages; then
    git worktree add -B ghpages "$WORKTREE_DIR" ghpages
  else
    git worktree add "$WORKTREE_DIR" --detach
    (cd "$WORKTREE_DIR" && git checkout --orphan ghpages)
  fi

  (cd "$WORKTREE_DIR" && git rm -rf . >/dev/null 2>&1 || true)
  rsync -a --delete "$SITE_DIR"/ "$WORKTREE_DIR"/
  (cd "$WORKTREE_DIR" && git add --all && git commit -m "Publish site for v$VERSION" && git push origin ghpages)
  git worktree remove "$WORKTREE_DIR"
  mark_done ghpages
}

verify_artifacts
push_release_branch
create_github_release
publish_ghpages

echo "Release v$VERSION published."
