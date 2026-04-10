#!/usr/bin/env bash
# Smokey bootstrap: run the canonical build script before other tests.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)

SKIP_VERSION_BUMP=1 sh "$REPO_ROOT/scripts/build.sh"
