# Jeff Memory Castle

This memory castle is Jeff's persistent working memory. Use it before guessing
about prior conversations, procedures, or personal data structures.

Search order:

1. Read this castle map.
2. Read the nearest `index.md` files for likely wings, floors, rooms, cabinets,
   and drawers before interpreting their contents.
3. Search room files first, then wing indexes.
4. Fall back to the chronological logbook and the gatehouse.
5. Update the right room and add a short logbook entry when a durable fact
   changes. If the right place is unclear, put the material in `gatehouse/`.

Wings:

- `jeff`: Jeff architecture, commands, releases, and operating rules.
- `finance`: Banking, finance records, API access, reports, and reconciliations.
- `codex`: Codex session handling, prompts, compaction recovery, and skills.

Gatehouse:

- `gatehouse`: Unsorted intake for files, notes, and observations the human or
  agent cannot place yet. Treat it as the castle entrance and sorting room, not
  as permanent storage.

Storage rules:

- Configuration lives in `$XDG_CONFIG_HOME/jeff` or `~/.config/jeff`.
- Durable data lives in `$XDG_DATA_HOME/jeff` or `~/.local/share/jeff`.
- Cache data lives in `$XDG_CACHE_HOME/jeff` or `~/.cache/jeff`.
- Secrets are stored in the local Vaultline store `jeff` below Jeff's data dir.
