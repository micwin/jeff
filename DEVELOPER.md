# Developer Guide

## 1. Environment Setup
1. Clone the repository and ensure you have Go, Rust, Python, and Node.js toolchains available (versions TBD). `[TODO] pin tool versions`
2. Run `scripts/bootstrap-dev.sh` to install project-specific dependencies, sync Smokey, and configure the `itzb` vaultline store.
3. Configure your shell to load vaultline secrets (see RUNBOOK.md) before invoking the build or dev commands. `[TODO] document exact vaultline env hooks`

## 2. Repository Conventions
- Source code for any runtime must live under `src/<capability>` with its build descriptor next to the module (e.g., `src/ingest/Cargo.toml`).
- Generated output must never be checked in; `work/`, `dist/`, and `tmp/` are ignored and should remain that way. Runtime code must not read or write `tmp/`.
- Scripts in `scripts/` must stay POSIX-compliant and begin with a short header comment explaining prerequisites.
- Follow naming conventions: directories/files in kebab-case, config keys or scripts in snake_case (constants in SCREAMING_SNAKE_CASE), and PascalCase only when the language demands it. Functions usually use camelCase.
- Provide explicit bootstrap files for every service entry point (e.g., `src/ingest/main.go`).
- Inline comments should document intent roughly every 3–5 effective lines; skip braces-only lines.
- Use helper functions/procedures only when they reduce duplication or encapsulate distinct behavior; avoid gratuitous abstraction.

## 3. Development Workflow
1. Base work off the `develop` branch; `master`/`main` must not exist. Create feature branches named `feature/<ticket-id>-<feature-slug>` (e.g., `feature/456-smokey-ingest`).
2. Implement changes with incremental commits using Conventional Commit types. Prefer small commits; if a change is large, split it into reviewable pieces without breaking builds.
3. Keep refactors focused and isolated in their own commits; make them small and targeted.
4. Update docs in `doc/`, `README.md`, `RUNBOOK.md`, or all three whenever you add a capability, script, or operational change. `[TODO] add doc template link`

## 4. Testing Requirements
- Run `scripts/build.sh` to execute language-specific unit suites (cargo/pytest/go/etc.) and to generate local artifacts.
- Organize Smokey scenarios under `tests.d/NNN-feature-case`; the zero-padded prefix controls execution order (`000` reserved for init). Shared setup must live only in `tests.d/000-init` or `tests.d/env.preseed`.
- Keep Smokey cases short, readable, and state-light. Prefer descriptive directory names and inline comments that read like an ELI5 walkthrough.
- Let Smokey manage state: avoid spawning excessive environment variables inside tests; use `env.preseed` or case-local files instead.
- Exit codes: `0` means success, `1` signals a failure, and `"$SMOKEY_SKIP_CODE"` should abort the remaining suite when the failure could damage infrastructure.
- Bias toward test-first workflows—write the Smokey case before the feature so it fails initially and goes green only after implementation. Include plenty of negative tests that assert expected errors.
- Execute `smokey --dir tests.d` before every push; capture the summary and attach any new `dist/` artifacts to the PR description.

## 5. Code Review Checklist
- [ ] Code follows formatter/lint expectations per language.
- [ ] New modules expose explicit bootstrap files.
- [ ] Tests exist for each new module and include deterministic fixtures.
- [ ] Docs (README, RUNBOOK, doc/ pages) updated if behavior or ops change.
- [ ] Vaultline usage documented when secrets/secrets storage changes.

## 6. Release & Operations
- Cut release branches as `release/v<major>.<minor>.<patch>` (semantic versioning). Merge them back into `develop` after release verification.
- Use dedicated branches for cross-version outputs (`ghpages`, `site`, `internal`, etc.) and tag the deployed commit as `GHPAGES_CURRENT`, `SITE_CURRENT`, or an analogous `*_CURRENT` tag.
- Refer to `RUNBOOK.md` for the full release checklist, incident response steps, and onboarding playbooks. `[TODO] expand RUNBOOK release section]`
