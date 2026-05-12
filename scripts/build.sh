#!/usr/bin/env sh
# Build all available project artifacts (Go CLI, docs, sites).

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
WORK_DIR="$REPO_ROOT/work"
DIST_DIR="$REPO_ROOT/dist"
VERSION_FILE="$REPO_ROOT/src/jeff/internal/version/VERSION"
DO_COMPILE=1
DO_DOCS=1
DO_DEB=1
DO_INSTALL=0
DO_CLEAN=0
DEB_PACKAGE_PATH=""

if [ $# -gt 0 ]; then
	DO_COMPILE=0
	DO_DOCS=0
	DO_DEB=0
	while [ $# -gt 0 ]; do
		case "$1" in
            --clean)
                DO_CLEAN=1
                ;;
			--compile)
				DO_COMPILE=1
				;;
			--docs)
				DO_DOCS=1
				;;
			--deb)
				DO_DEB=1
				;;
			--install)
				DO_INSTALL=1
				DO_DEB=1
				DO_COMPILE=1
				;;
			-h|--help)
				cat <<EOF
Usage: scripts/build.sh [--compile] [--docs] [--deb] [--install]
Without flags, all sections run (compile, docs placeholder, deb package).
Providing any flag limits execution to the selected sections.
Use --install to build and install the generated .deb (implies --deb).
Use --clean to remove previous build artifacts (can be combined with other flags).
EOF
				exit 0
				;;
			*)
				echo "Unknown option: $1" >&2
				exit 1
				;;
		esac
		shift
	done
fi

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Missing tool: $1" >&2
		exit 1
	fi
}

log() {
	printf '%s\n' "$*"
}

clean_artifacts() {
	log "==> Cleaning build artifacts"
	rm -rf "$WORK_DIR/go-cache" "$WORK_DIR/go-tmp" "$WORK_DIR/jeff-deb-root"
	rm -f "$REPO_ROOT/src/jeff/internal/sidecars/embedded/vaultline"
	if [ -d "$DIST_DIR" ]; then
		rm -rf "$DIST_DIR"/*
	fi
}

bump_version() {
	if [ "${SKIP_VERSION_BUMP:-0}" = "1" ]; then
		log "==> Version bump skipped (SKIP_VERSION_BUMP=1)"
		return
	fi
	if [ ! -f "$VERSION_FILE" ]; then
		echo "0.1.0" >"$VERSION_FILE"
	fi
current=$(tr -d '\r' <"$VERSION_FILE" | head -n 1)
IFS='.' read -r major minor patch <<EOF
$current
EOF
major=${major:-0}
minor=${minor:-1}
patch=${patch:-0}
	case $patch in
		*[!0-9]*) patch=0 ;;
		*) patch=$((patch + 1)) ;;
	esac
	new_version="$major.$minor.$patch"
	echo "$new_version" >"$VERSION_FILE"
	log "==> Version bumped to $new_version"
}

need_cmd go
mkdir -p "$WORK_DIR/go-cache" "$WORK_DIR/go-tmp" "$DIST_DIR"

if [ "$DO_CLEAN" -eq 1 ]; then
	clean_artifacts
	if [ "$DO_COMPILE" -eq 0 ] && [ "$DO_DOCS" -eq 0 ] && [ "$DO_DEB" -eq 0 ]; then
		log "Clean complete."
		exit 0
	fi
fi

run_compile() {
	GO_ENV="GOCACHE=$WORK_DIR/go-cache GOTMPDIR=$WORK_DIR/go-tmp"
	log "==> Sidecars"
	"$REPO_ROOT/scripts/build-sidecars.sh"
	log "==> Go tests"
	(cd "$REPO_ROOT/src/jeff" && env $GO_ENV go test -tags sidecars ./...)
	log "==> Go build"
	(cd "$REPO_ROOT/src/jeff" && env $GO_ENV go build -tags sidecars -o "$DIST_DIR/jeff" ./cmd/jeff)
}

build_docs_section() {
	if [ -d "$REPO_ROOT/doc" ] || [ -d "$REPO_ROOT/site" ]; then
		log "==> Docs placeholder"
		log "    (no documentation build steps defined yet)"
	else
		log "==> Docs skipped (no doc directory)"
	fi
}

build_deb() {
	if ! command -v dpkg-deb >/dev/null 2>&1; then
		log "==> dpkg-deb not available; skipping deb build"
		return
	fi
	version=$(tr -d '\r' <"$VERSION_FILE" | head -n 1)
	if [ ! -x "$DIST_DIR/jeff" ]; then
		log "==> jeff binary missing; running compile step for deb"
		run_compile
	fi
	arch=${DEB_ARCH:-$(dpkg --print-architecture 2>/dev/null || uname -m)}
	root="$WORK_DIR/jeff-deb-root"
	rm -rf "$root"
	mkdir -p "$root/DEBIAN" "$root/usr/local/bin"
	cp "$DIST_DIR/jeff" "$root/usr/local/bin/jeff"
	chmod 755 "$root/usr/local/bin/jeff"
	cat >"$root/DEBIAN/control" <<EOF
Package: jeff
Version: $version
Section: utils
Priority: optional
Architecture: $arch
Maintainer: Unknown <unknown@example.com>
Description: Jeff CLI assistant packaged for Debian-based systems.
Depends: tmux
EOF
	cat >"$root/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e

if [ "$1" = "configure" ] && [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
	user_home=$(getent passwd "$SUDO_USER" | cut -d: -f6)
	if [ -n "$user_home" ] && [ -d "$user_home" ]; then
		if command -v runuser >/dev/null 2>&1; then
			HOME="$user_home" runuser -u "$SUDO_USER" -- jeff migrate --quiet || true
		else
			su "$SUDO_USER" -c "HOME='$user_home' jeff migrate --quiet" || true
		fi
	fi
fi

exit 0
EOF
	chmod 755 "$root/DEBIAN/postinst"
	mkdir -p "$DIST_DIR"
	output="$DIST_DIR/jeff_${version}_${arch}.deb"
	log "==> Building deb package $output"
	dpkg-deb --build "$root" "$output" >/dev/null
	DEB_PACKAGE_PATH="$output"
}

install_deb() {
	pkg_path="$1"
	if [ -z "$pkg_path" ] || [ ! -f "$pkg_path" ]; then
		log "==> Cannot install: package $pkg_path not found"
		exit 1
	fi
	if ! command -v dpkg >/dev/null 2>&1; then
		log "==> dpkg not available; cannot install package"
		exit 1
	fi
	log "==> Installing $pkg_path (requires sudo)"
	if command -v sudo >/dev/null 2>&1; then
		sudo dpkg -i "$pkg_path"
	else
		log "==> sudo not found; attempting dpkg -i without it"
		dpkg -i "$pkg_path"
	fi
}

bump_version

compile_ran=0
if [ "$DO_COMPILE" -eq 1 ]; then
	run_compile
	compile_ran=1
fi

if [ "$DO_DOCS" -eq 1 ]; then
	build_docs_section
fi

if [ "$DO_DEB" -eq 1 ]; then
	if [ "$compile_ran" -eq 0 ]; then
		if [ -x "$DIST_DIR/jeff" ]; then
			log "==> Reusing existing jeff binary for deb packaging"
		else
			log "==> jeff binary missing; running compile step for deb"
			run_compile
		fi
	fi
	build_deb
	if [ "$DO_INSTALL" -eq 1 ]; then
		if [ -n "$DEB_PACKAGE_PATH" ]; then
			install_deb "$DEB_PACKAGE_PATH"
		else
			log "==> No .deb package built; cannot install."
			exit 1
		fi
	fi
fi

log "Build complete. Artifacts available in $DIST_DIR"
