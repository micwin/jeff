---
title: Unify Jeff specialist discovery
status: open
created_at: 2026-08-13T19:51:45Z
updated_at: 2026-08-13T19:51:45Z
---

# Description

As a Jeff user, I want every configured specialist to be discoverable and
callable through `jeff specialists`, regardless of whether the specialist was
originally registered through Jeff's JSON registry or codex-resume, so that I
do not need to know which historical registry owns an alias.

Jeff currently reads `~/.local/share/jeff/specialists/*.json` for
`jeff specialists list`, `show`, `call`, and completion. Most established
specialists still live in codex-resume configuration, while the continuity
archive already reads that larger set. This produces contradictory views of
the available team.

Define one deterministic resolution model. Explicit Jeff registry entries must
override imported or discovered codex-resume metadata for the same alias. The
implementation may synchronize codex-resume entries into Jeff's generic
registry or merge the sources at read time, but it must establish and document
which persisted representation is authoritative after the transition.

# Acceptance

- `jeff specialists list` includes every usable specialist known through the
  Jeff registry or configured codex-resume aliases without duplicate rows.
- `jeff specialists show`, `jeff specialists call`, and shell completion use
  the same resolved alias set as `list`.
- An explicit Jeff JSON entry wins deterministically when both sources define
  the same alias.
- Disabled specialists are visibly marked, remain discoverable for inspection,
  and cannot be called.
- The continuity archive consumes the same identity and policy resolution or
  clearly preserves additional historical aliases without contradicting the
  active specialist list.
- Existing persistent session ids are preserved. Synchronization or migration
  never silently replaces a specialist identity.
- Jeff-only specialists continue to work when codex-resume is unavailable.
- Malformed, missing, and conflicting source data fail with actionable errors
  instead of silently dropping specialists.
- Command help, documentation, and shell completion describe the unified
  behavior.
- A readable Smokey story covers a Jeff-only alias, a codex-resume-only alias,
  a collision with Jeff precedence, a disabled alias, unavailable
  codex-resume, malformed policy output, and completion at the public command
  surfaces.

# Comments

- 2026-08-13: Observed one Jeff JSON specialist (`peer-mesh`), 28 active
  codex-resume aliases, and 35 aliases in the continuity archive including
  disabled or historical entries.

# Outcome
