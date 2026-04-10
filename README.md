# Jeff

## Overview
The Jeff workspace hosts multi-language services focused on integration workflows and operational tooling. Modules live in `src/` under capability folders (e.g., `src/ingest`, `src/orchestrators`, `src/infra`). Build artifacts are produced via the scripts documented below and routed into `work/` (intermediates) and `dist/` (release outputs).

## Key Components
- **Multi-runtime services** — binaries, CLIs, or daemons written in Go, Rust, Python, etc.
- **Smokey integration suites** — deterministic end-to-end checks stored under `tests.d/NNN-feature-case` with zero-padded ordering.
- **Documentation** — Markdown + mkdocs sources in `doc/`, with operational procedures consolidated in `RUNBOOK.md`.

## Directory Layout
```
src/        # service implementations grouped by capability; keep build files beside code
scripts/    # bootstrap/build/release/clean helpers
work/       # ignored build intermediates
dist/       # ignored release artifacts
tmp/        # ignored handoff directory between user and agent (runtime code must not read it)
tests.d/    # Smokey suites named NNN-feature-case (NNN = zero-padded order)
doc/        # docs + mkdocs config and runbooks
```
Smokey directories follow `tests.d/NNN-feature-case`, where `NNN` is a zero-padded ordinal (`000` reserved for init). Fixtures must remain inside each case directory; shared setup lives only in `tests.d/000-init` or `tests.d/env.preseed`.

## Getting Started
1. Run `scripts/bootstrap-dev.sh` to install toolchains, sync Smokey, and configure the `jeff` vaultline store.
2. Implement or update services under the relevant `src/<capability>` folder, keeping language-specific build files beside the code.
3. Use `scripts/build.sh` for local builds, `scripts/release.sh` for version/tag flows, and `scripts/clean.sh` to reset the workspace.
4. Update `RUNBOOK.md` if you change how any of the scripts operate.

## Branching & Releases
- Default branch is `develop`. Do not create `master` or `main` branches.
- Feature work lives on branches named `feature/<ticket-id>-<feature-slug>` (e.g., `feature/123-ingest-batching`).
- Releases use `release/v<major>.<minor>.<patch>` (semantic versioning). Merge back into `develop` via PR once validated.
- Cross-version publishing branches (e.g., docs sites) should describe their audience (`ghpages`, `site`, `internal`, …). Tag the currently deployed commit with `GHPAGES_CURRENT`, `SITE_CURRENT`, or equivalent service-specific tags.

## Testing
- Run language-specific unit suites through `scripts/build.sh` (it should invoke `cargo test`, `pytest`, `go test`, etc.).
- Execute `smokey --dir tests.d` before pushing a branch or opening a PR; include the summary and generated artifacts (`dist/`) in the PR template.
- Prefer many small, readable Smokey cases (aim for ELI5-style descriptions). Keep per-case environment variables minimal—let Smokey drive shared state via `env.preseed` or the case directory.
- Exit codes: use `exit 0` for success, `exit 1` for expected failures, and `exit "$SMOKEY_SKIP_CODE"` when a case must abort the remaining suite (e.g., infrastructure-impacting faults).
- Default to test-first: add the Smokey case first, let it fail, then implement the feature until the suite passes. Bias toward ample negative tests that only pass when the code throws the expected error.

## Contributing
Follow Conventional Commits, keep pull requests under ~400 touched lines, and link the relevant issue plus Smokey evidence. Agents must read `AGENTS.md` for automation guardrails; humans should start with `DEVELOPER.md` and `RUNBOOK.md`.
