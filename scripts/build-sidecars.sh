#!/usr/bin/env sh
# Build and stage optional embedded sidecar binaries for Jeff.

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
SIDECAR_DIR="$REPO_ROOT/src/jeff/internal/sidecars/embedded"
VAULTLINE_CONF="$REPO_ROOT/sidecars/vaultline.conf"

read_conf() {
	key="$1"
	awk -F= -v key="$key" '$1 == key {print substr($0, length(key) + 2)}' "$VAULTLINE_CONF" | tail -n 1
}

dirty_allowed() {
	jeff_branch="$1"
	sidecar_branch="$2"
	is_development_branch "$jeff_branch" && is_development_branch "$sidecar_branch"
}

is_development_branch() {
	case "$1" in
		develop|feature/*) return 0 ;;
		*) return 1 ;;
	esac
}

if [ ! -f "$VAULTLINE_CONF" ]; then
	echo "Missing sidecar config: $VAULTLINE_CONF" >&2
	exit 1
fi

name=$(read_conf name)
source_dir=${JEFF_VAULTLINE_SOURCE:-$(read_conf source)}
source_url=$(read_conf url)
ref=$(read_conf ref)
package=$(read_conf package)
source_kind=local

case "$source_dir" in
	/*) ;;
	*) source_dir="$REPO_ROOT/$source_dir" ;;
esac

if [ ! -d "$source_dir" ]; then
	if [ -z "$source_url" ]; then
		echo "Vaultline source directory not found: $source_dir" >&2
		echo "Set JEFF_VAULTLINE_SOURCE=/path/to/vaultline or update $VAULTLINE_CONF." >&2
		exit 1
	fi
	source_kind=url
	source_dir="$WORK_DIR/sidecars/source/$name"
	if [ -d "$source_dir/.git" ]; then
		echo "==> Updating sidecar source $name from $source_url"
		git -C "$source_dir" fetch --tags origin
	else
		echo "==> Cloning sidecar source $name from $source_url"
		rm -rf "$source_dir"
		mkdir -p "$(dirname "$source_dir")"
		git clone "$source_url" "$source_dir"
	fi
	if [ -n "$ref" ]; then
		git -C "$source_dir" checkout "$ref"
		git -C "$source_dir" pull --ff-only origin "$ref" 2>/dev/null || true
	fi
fi

if [ -d "$source_dir/.git" ]; then
	jeff_ref=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || true)
	current_ref=$(git -C "$source_dir" rev-parse --abbrev-ref HEAD 2>/dev/null || true)
	current_commit=$(git -C "$source_dir" rev-parse --short=12 HEAD 2>/dev/null || true)
	if [ -n "$ref" ] && [ -n "$current_ref" ] && [ "$current_ref" != "$ref" ]; then
		echo "==> Warning: vaultline source is on $current_ref, config requests $ref" >&2
	fi
	dirty=$(git -C "$source_dir" status --porcelain 2>/dev/null || true)
	if [ -n "$dirty" ]; then
		if dirty_allowed "$jeff_ref" "$current_ref"; then
			echo "==> Warning: vaultline source has uncommitted changes (allowed because Jeff and Vaultline are both on development branches)" >&2
		else
			echo "Vaultline source has uncommitted changes." >&2
			echo "Dirty sidecars are only allowed when both Jeff and Vaultline are on development branches (develop or feature/*)." >&2
			echo "Jeff branch: ${jeff_ref:-unknown}" >&2
			echo "Vaultline branch: ${current_ref:-unknown}" >&2
			exit 1
		fi
	fi
else
	current_ref=""
	current_commit=""
	dirty=""
fi

mkdir -p "$WORK_DIR/sidecars/$name" "$SIDECAR_DIR"

echo "==> Building sidecar $name from $source_dir"
(cd "$source_dir" && env CGO_ENABLED=0 go test ./...)
(cd "$source_dir" && env CGO_ENABLED=0 go build -o "$WORK_DIR/sidecars/$name/$name" "$package")

cp "$WORK_DIR/sidecars/$name/$name" "$SIDECAR_DIR/$name"
chmod 0644 "$SIDECAR_DIR/$name"

binary_version=$("$WORK_DIR/sidecars/$name/$name" version 2>/dev/null | tr -d '\r' | head -n 1 || true)
if [ "$source_kind" = "url" ] && [ -n "$binary_version" ]; then
	version_label="$binary_version"
elif [ -n "$dirty" ]; then
	version_label="${current_ref:-dirty}"
elif [ -n "$current_commit" ]; then
	version_label="$current_commit"
elif [ -n "$binary_version" ]; then
	version_label="$binary_version"
else
	version_label="unknown"
fi

cat >"$WORK_DIR/sidecars/$name/metadata.env" <<EOF
VAULTLINE_SOURCE_KIND=$source_kind
VAULTLINE_SOURCE_DIR=$source_dir
VAULTLINE_REF=$ref
VAULTLINE_BRANCH=$current_ref
VAULTLINE_COMMIT=$current_commit
VAULTLINE_DIRTY=$([ -n "$dirty" ] && printf yes || printf no)
VAULTLINE_BINARY_VERSION=$binary_version
VAULTLINE_VERSION_LABEL=$version_label
EOF

echo "==> Staged sidecar $name at $SIDECAR_DIR/$name"
echo "==> Sidecar $name version label: $version_label"
