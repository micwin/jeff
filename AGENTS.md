# Repository Guidelines

## Read These First
Review `README.md`, `DEVELOPER.md`, `RUNBOOK.md`, and the docs under `doc/` before acting; their guidance overrules anything here whenever instructions conflict. This file only adds guardrails for automated agents.

## Layout Playbook for Agents
- Treat `README.md`, `DEVELOPER.md`, `RUNBOOK.md`, and `LICENSE.md` as human-owned references. Do not rewrite them unless the task explicitly requires it.
- When creating or modifying files under `src/`, `doc/`, or `tests.d`, follow the structures described in the human docs. If you must add a new capability or Smokey directory, mirror the established naming (e.g., `tests.d/NNN-feature-case`) and explain it in your PR.
- Any change to documentation sources, mkdocs configs, or runbooks must be cross-linked from your PR description so humans can review quickly.
- Route build intermediates to `work/`, deliverables to `dist/`, and user↔agent handoffs to `tmp/`. All three are ignored by Git. Never make runtime code depend on `tmp/`.
- Use the `develop` branch as the default base; never create `master` or `main`. Feature branches must follow `feature/<ticket-id>-<feature-slug>`, release branches `release/v<major>.<minor>.<patch>`, and cross-version publishing branches (e.g., `ghpages`, `site`) must be tagged with `GHPAGES_CURRENT`, `SITE_CURRENT`, etc.

## Build & Test Expectations
- Always drive builds and releases through the scripts documented in `README.md`/`DEVELOPER.md` (`scripts/bootstrap-dev.sh`, `scripts/build.sh`, `scripts/release.sh`, `scripts/clean.sh`). If you alter their behavior, append the change to `RUNBOOK.md`.
- Ensure `scripts/build.sh` runs the language-specific test suites before you invoke Smokey.
- Execute `smokey --dir tests.d` before posting results back to the user, and attach the summary plus any `dist/` artifacts.
- When authoring or editing Smokey cases, follow the human guidelines: keep tests readable, minimize inline env vars, and use `$SMOKEY_SKIP_CODE` only when a failure should abort the remaining suite.

## Coding & Automation Guardrails
- Apply the formatter/linter rules documented in `DEVELOPER.md`. If the language lacks tooling, default to spaces and existing style in the touched file.
- Maintain explicit bootstrap files and script headers as described in `DEVELOPER.md`. Only add automation scaffolding that humans can maintain easily.
- Keep edits minimal-invasive: do not reorganize files, rename directories, or delete human-authored prose unless asked. Link any new config/architecture content from the appropriate human docs.
- When refactoring, isolate the change, state the reason in the commit message, and avoid bundling unrelated edits.

## Change Management & Security
- Use Conventional Commits (same as humans) and include intent in the commit body. PRs must link their tracking issue, list manual/Smokey verification, and note any touches to `work/`, `dist/`, `tmp/`, or the `itzb` vaultline store.
- Secrets live only in the vaultline store `itzb`. When a task requires credentials, instruct the user to place them via vaultline or `tmp/`; never invent `.env` files.
- Treat `tmp/` as a disposable transfer area. Delete any files you place there once the user confirms receipt, and never expose the contents elsewhere.
