#!/usr/bin/env sh
# Fast-forward merge current release branch into develop after a release.

set -eu

REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$REPO_ROOT"

if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree has uncommitted changes. Commit or stash before running post-release." >&2
  exit 1
fi

current_branch=$(git rev-parse --abbrev-ref HEAD)
case "$current_branch" in
  release/v*) ;;
  *)
    echo "Post-release script must run from a release/vX.Y.Z branch (currently $current_branch)." >&2
    exit 1
    ;;
 esac

if ! git show-ref --verify --quiet refs/heads/develop; then
  echo "develop branch not found locally. Fetch it first." >&2
  exit 1
fi

git checkout develop
if ! git merge --ff-only "$current_branch"; then
  echo "Fast-forward merge failed. Resolve manually." >&2
  exit 1
fi

echo "Release branch $current_branch fast-forward merged into develop."
