#!/usr/bin/env bash
# Smokey test: verifies idempotent per-user shell completion installation.

set -euo pipefail

REPO_ROOT=$(cd "$SMOKEY_TEST_ROOT/.." && pwd)
HOME_ROOT="$SMOKEY_STATE_DIR/completion-home"
VERSION_FILE="$REPO_ROOT/src/jeff/internal/version/VERSION"
VERSION_BEFORE=$(cat "$VERSION_FILE")

# Bash setup writes one marked block and repeated setup leaves it unchanged.
HOME="$HOME_ROOT/bash" SHELL=/bin/bash "$REPO_ROOT/scripts/build.sh" --install-completion >"$SMOKEY_STATE_DIR/bash-first.out"
HOME="$HOME_ROOT/bash" SHELL=/bin/bash "$REPO_ROOT/scripts/build.sh" --install-completion >"$SMOKEY_STATE_DIR/bash-second.out"
grep -q 'Completion configured shell=bash' "$SMOKEY_STATE_DIR/bash-first.out"
grep -q 'Completion already configured shell=bash' "$SMOKEY_STATE_DIR/bash-second.out"
[[ $(grep -c 'source <(jeff completion bash)' "$HOME_ROOT/bash/.bashrc") -eq 1 ]]
echo "ok: bash completion setup is idempotent"

# Zsh honors ZDOTDIR and sources Jeff completion from the selected rc file.
HOME="$HOME_ROOT/zsh-home" ZDOTDIR="$HOME_ROOT/zsh-dot" SHELL=/usr/bin/zsh \
	"$REPO_ROOT/scripts/build.sh" --install-completion >"$SMOKEY_STATE_DIR/zsh.out"
grep -q 'Completion configured shell=zsh' "$SMOKEY_STATE_DIR/zsh.out"
grep -q 'source <(jeff completion zsh)' "$HOME_ROOT/zsh-dot/.zshrc"
echo "ok: zsh completion setup respects ZDOTDIR"

# Fish uses a conf.d file below its XDG configuration directory.
HOME="$HOME_ROOT/fish-home" XDG_CONFIG_HOME="$HOME_ROOT/fish-config" SHELL=/usr/bin/fish \
	"$REPO_ROOT/scripts/build.sh" --install-completion >"$SMOKEY_STATE_DIR/fish.out"
grep -q 'Completion configured shell=fish' "$SMOKEY_STATE_DIR/fish.out"
grep -q 'jeff completion fish | source' "$HOME_ROOT/fish-config/fish/conf.d/jeff.fish"
echo "ok: fish completion setup uses conf.d"

# Unsupported shells report the condition without failing or modifying a shell rc file.
HOME="$HOME_ROOT/other" SHELL=/bin/sh "$REPO_ROOT/scripts/build.sh" --install-completion >"$SMOKEY_STATE_DIR/other.out"
grep -q 'Completion unsupported shell=sh' "$SMOKEY_STATE_DIR/other.out"
[[ ! -e "$HOME_ROOT/other/.profile" ]]
echo "ok: unsupported completion shells are non-fatal"

# Completion-only setup must not build artifacts or increment the project version.
[[ $(cat "$VERSION_FILE") == "$VERSION_BEFORE" ]]
echo "ok: completion-only setup leaves the version unchanged"
