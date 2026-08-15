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
specialists/ Machine-readable specialist registry entries for `jeff specialists`.
specialist-calls/ Logged specialist requests and responses grouped by UTC date.
reports/     Generated daily, finance, API, and operational reports.
```

Jeff writes Codex-backed chat semaphores below `memcastle/state/sessions/`.
These local files record whether the Jeff preprompt was already injected for a
bound chat session.

Use `jeff memcastle status` to inspect memory castle metadata, `jeff
memcastle tree` to print the directory-only castle structure, `jeff memcastle
path [query]` to print the root or resolve directory paths by name, `jeff
memcastle search` for offline text search, `jeff memcastle ask` for
Codex-backed answers constrained to memory castle sources, `jeff memcastle
continuity import [path]` to store Codex compact notes under
`codex/continuity/`, and `jeff memcastle cleanup` to ask Jeff to sort unsorted
`gatehouse/` material into the castle.

`jeff memcastle path` without arguments prints the active castle root. A query
starting with `/` resolves an exact castle-relative directory such as
`/projects/bde`. A query without a leading slash searches directory name
segments below the castle root: `dist` finds directories named `dist`,
`bde/dist` finds matching path sequences, and patterns such as `bde/*/main` or
`**/dist` support segment wildcards.

Top-level directories below `memcastle/` are wings. Nested directories are
topic areas, and Markdown files are rooms or focused notes.

Jeff's encrypted Vaultline store lives below
`memcastle/jeff/vaultline/store/`. Jeff registers that external store with the
running [Vaultline](https://micwin.github.io/vaultline/) daemon, keeps the
`jeff` store's unseal material in `config.json`, and passes it transiently when
the store has to be opened. The encrypted store files (`*.vlx` and related
Vaultline metadata) are treated as binary data by memory-castle search and ask
workflows.

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

Specialist registry entries are JSON files below `specialists/<alias>.json`.
They contain non-secret transport metadata such as the alias, display name,
description, transport, target, tags, and enabled state. `jeff specialists
call <alias> -- <message>` wraps the request in Jeff's specialist prompt,
dispatches it through the configured transport, and writes a durable call log
below `specialist-calls/YYYY-MM-DD/`.

## Specialist Continuity Archive

`jeff specialists archive refresh` records specialist aliases, exact session
ids, invocation policies, local Codex transcripts, and repository state below
`memcastle/codex/specialist-archive/`. Add `--briefs` to request current role,
knowledge, open-work, and restart summaries from enabled specialists.

The archive separates tracked recovery metadata from private payloads:

```text
codex/specialist-archive/
  registry.json          Specialist identities and policies.
  repositories.json      Reproducible repository references and snapshots.
  successor-prompt.md    Recovery instructions for a replacement Jeff agent.
  status.json             Completeness and error report.
  private/sessions/       Compressed local Codex JSONL transcripts.
  private/repositories/   Flat snapshots of non-reproducible working trees.
codex/specialists/<alias>/
  index.md                Specialist identity and archive pointer.
  role.md                 Human-curated role description.
  current-state.md        Optional specialist continuity brief.
  runtime.json            Machine-readable archived runtime record.
```

`private/` is ignored by the Memory Castle Git repository because transcripts
and working trees may contain private or secret material. Portable and disaster
recovery backups must include that ignored directory explicitly.

Clean repositories whose exact commit exists on `origin` are stored only as a
remote URL, branch, and commit hash. Dirty trees, repositories without a remote,
and commits not present on the remote receive one deduplicated flat snapshot of
tracked and untracked non-ignored files; `.git`, ignored build output, and Git
history are not copied.

Use `jeff specialists archive status` before relying on an archive,
`jeff specialists archive search [--alias NAME] [--regex] QUERY` for offline
transcript search, and `jeff specialists archive successor` to print the
replacement-agent recovery prompt. Archived session ids remain identity
records and must not be overwritten without explicit human approval.

The archive preserves specialists, not a complete Jeff installation. Follow
[Specialist Backup And Recovery](specialist-recovery.md) for Jeff installation,
exact archive target paths, the Memory Castle codex-ctl helper, repository
restoration, and the account-boundary behavior of Codex sessions.

## Migrations

Jeff records configuration migration state in `config.json` as `schema_version`.
The Debian package runs `jeff migrate --quiet` for the installing sudo user when possible, and the CLI also applies pending migrations on startup.

The first migration moves legacy durable finance and banking data from the config directory to the XDG data directory when paths such as `finance/`, `finance.json`, `banking/`, or `banking.json` already exist under `~/.config/jeff`.
