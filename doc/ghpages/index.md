---
layout: home
title: Jeff – Your Friendly CLI-pal
---

Welcome to the Jeff CLI project! Jeff is your CLI helper with carefully placed boosts—focused on speeding up the shells you already know.

[View on GitHub](https://github.com/micwin/jeff)

## Getting Started

1. Install the CLI via `scripts/build.sh --compile` (or grab the .deb package).
2. Configure your Jeff chat with `jeff codex init --last` or `jeff codex init <session-id>`.
3. Start your Jeff chat with `jeff chat`.
4. Launch the tmux overlay with `jeff tmux` (Ctrl-T opens the quick menu).

Happy hacking!

## Some Highlights

- `jeff menu` gives you a simple, nestable menu for frequently used shell shortcuts.
- `jeff tmux` launches the overlay with a configurable status bar and lets you pop open the menu via `Ctrl-T` while pair-programming in your favorite TUI editor.
- `jeff tmpl` lets you list, validate, and render reusable template packages (with shell completion for template names).
- `jeff vl` runs the bundled [Vaultline](https://micwin.github.io/vaultline/) CLI without extracting a helper executable to disk.
- `jeff init` unpacks Jeff's embedded memory castle and persistent personal-agent defaults into the XDG data directory.
- `jeff chat` opens the long-lived Jeff agent session for ongoing conversations.
- `jeff execute` runs Bash-backed user commands stored under Jeff's data directory, with completion for command names.
- `jeff completion` installs command completions for bash, zsh, and fish.


## Downloads

- [Download overview]({{ "/downloads.html" | relative_url }})
- [Sidecar build details]({{ "/sidecars.html" | relative_url }})
- [Storage layout]({{ "/storage.html" | relative_url }})

<!-- latest-release:start -->
## Latest Release

- [Download Jeff v0.3.28](https://github.com/micwin/jeff/releases/tag/v0.3.28)
- [Release notes]({{ "/releases/v0.3.28.html" | relative_url }})
<!-- latest-release:end -->
