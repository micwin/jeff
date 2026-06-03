# Repository Guidelines

First read and follow the active memory-castle agent instructions at
`~/.local/share/jeff/memcastle/codex/index.md`; this file only adds Jeff
repository-specific rules.

## Read These First

Review `README.md`, `DEVELOPER.md`, `RUNBOOK.md`, and the docs under `doc/`
before acting. Their guidance overrides this file when instructions conflict.

## Local Layout

- Treat `README.md`, `DEVELOPER.md`, `RUNBOOK.md`, and `LICENSE.md` as
  human-owned references. Do not rewrite them unless the task explicitly
  requires it.
- Follow the established structure for changes under `src/`, `doc/`, and
  `tests.d`.
- Route build intermediates to `work/`, release artifacts to `dist/`, and
  user-agent handoffs to `tmp/`. Runtime code must not depend on `tmp/`.
- Use `develop` as the default base branch. Feature branches use
  `feature/<ticket-id>-<feature-slug>`; release branches use
  `release/v<major>.<minor>.<patch>`.

## Build And Test

- Use the documented scripts: `scripts/bootstrap-dev.sh`, `scripts/build.sh`,
  `scripts/prepare-release.sh`, `scripts/publish-release.sh`,
  `scripts/post-release.sh`, and `scripts/clean.sh`.
- If script behavior changes, update `RUNBOOK.md`.
- Run the full Smokey suite with `smokey --tests-dir tests.d` before reporting
  final verification.
- Smokey tests must follow `smokey agents-help`: suite-only execution,
  Smokey-managed state, readable directory tests, committed fixtures, and no
  direct-test fallbacks.

## Jeff-Specific Safety

- Secrets live only in the Vaultline store `jeff`. Documentation may mention
  Vaultline key names, never secret values.
- The embedded default memory-castle tree under
  `src/jeff/internal/agent/defaults/memcastle/` is public bootstrap content.
  Keep it generic and never copy personal memory-castle data into it.
- Do not commit `*.vlx`, personal memory-castle content, `work/`, `dist/`, or
  `tmp/` artifacts unless the user explicitly requests artifact versioning.
- Do not run `sudo`; provide exact commands for the user when root access is
  required.
