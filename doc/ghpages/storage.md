---
layout: page
title: Storage
---

Jeff separates configuration, user data, cache data, and secrets.

| Purpose | Default location | Notes |
|---------|------------------|-------|
| Configuration | `$XDG_CONFIG_HOME/jeff` or `~/.config/jeff` | `config.json`, `menu.json`, completions, and template packages. |
| Durable user data | `$XDG_DATA_HOME/jeff` or `~/.local/share/jeff` | Finance and banking records belong here. |
| Cache data | `$XDG_CACHE_HOME/jeff` or `~/.cache/jeff` | Rebuildable downloads, indexes, and temporary runtime caches. |
| Secrets | [Vaultline](https://micwin.github.io/vaultline/) store `jeff` | API keys, banking credentials, tokens, and other secrets. |

Runtime code must not depend on `tmp/`, and generated build artifacts must remain under `work/` or `dist/`.
