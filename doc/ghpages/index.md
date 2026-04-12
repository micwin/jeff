---
layout: home
title: Jeff – Your Friendly CLI-pal
---

Welcome to the Jeff CLI project! Jeff is your CLI helper with carefully placed boosts—focused on speeding up the shells you already know.

[View on GitHub](https://github.com/micwin/jeff)

## Getting Started

1. Install the CLI via `scripts/build.sh --compile` (or grab the .deb package).
2. Configure your Codex session with `jeff codex init`.
3. Ask questions using `jeff codex ask "What does this repo do?"`.
4. Launch the tmux overlay with `jeff tmux` (Ctrl-T opens the quick menu).

Happy hacking!

## Quick Menu Tips

- `Ctrl-T` opens the shortcut popup inside `jeff tmux`; use `jeff menu tui` outside tmux for the same UI.
- Prefix any menu command with `!` to run it via sudo (`!` alone becomes `sudo su -`). Completion happens before the rewrite, so you can still tab-expand the original command.

## Downloads

- [Download overview]({{ "/downloads.html" | relative_url }})

<!-- latest-release:start -->
## Latest Release

- [Download Jeff v0.3.7](https://github.com/micwin/jeff/releases/tag/v0.3.7)
- [Release notes](/releases/v0.3.7.html)
<!-- latest-release:end -->
