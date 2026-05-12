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

## Agent Data

Jeff's personal-agent state lives below the durable user data directory:

```text
memcastle/   Persistent memory castle, system prompt, wings, rooms, and logbook.
skills/      User skills and skill metadata. Secret values live in Vaultline.
commands/    Bash-backed commands executed with `jeff execute <name>`.
reports/     Generated daily, finance, API, and operational reports.
vaultline/   Local Vaultline store files for Jeff-scoped secrets.
```

Jeff writes Codex-backed chat semaphores below `memcastle/state/sessions/`.
These local files record whether the Jeff preprompt was already injected for a
bound chat session.

Stored commands use this layout:

```text
commands/
  shared/*.sh
  <name>/
    init.sh
    run.sh
    cleanup.sh
    files/
```

`jeff execute <name>` runs in Bash, sources `commands/shared/*.sh`, sources `init.sh`, runs `run.sh`, and then sources `cleanup.sh` through an exit trap when present.

## Migrations

Jeff records configuration migration state in `config.json` as `schema_version`.
The Debian package runs `jeff migrate --quiet` for the installing sudo user when possible, and the CLI also applies pending migrations on startup.

The first migration moves legacy durable finance and banking data from the config directory to the XDG data directory when paths such as `finance/`, `finance.json`, `banking/`, or `banking.json` already exist under `~/.config/jeff`.
