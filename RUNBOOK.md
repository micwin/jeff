# RUNBOOK

Operational procedures for Jeff. Update this file whenever the release process, onboarding steps, or incident protocols change.

## 1. Onboarding
1. Ensure access to the `jeff` vaultline store and run `vaultline store create jeff` if it does not exist locally.
2. Run `scripts/bootstrap-dev.sh` to install multi-language toolchains and sync Smokey.
3. Verify `smokey --dir tests.d` executes the baseline suites (starting with `tests.d/000-*`). `[TODO] list expected baseline cases`
4. Review `DEVELOPER.md` and AGENTS.md (if acting as an agent) before first commit.

## 2. Build & Release Process
1. Start from `develop`. Create a release branch named `release/v<major>.<minor>.<patch>` (semantic versioning) for the work in flight.
2. Run `scripts/build.sh` to generate binaries/docs into `work/` and `dist/`.
3. Execute `smokey --dir tests.d` and ensure all numbered suites pass.
4. Invoke `scripts/release.sh` to bump versions, cut tags/branches, and publish artifacts/pages. `[TODO] document release.sh flags and CI steps`
5. For cross-version publishing (docs sites, portals), update the dedicated branch (e.g., `ghpages`, `site`) and tag the deployed commit with `GHPAGES_CURRENT`, `SITE_CURRENT`, or another `*_CURRENT` tag.
6. Post-release, archive artifacts per compliance requirements. `[TODO] add storage location]`
7. From the release branch, run `scripts/post-release.sh` to fast-forward merge the release back into `develop` once the release is verified.

## 3. Incident Response
- **Detection:** Monitor build/test pipelines and Smokey outputs for failures. `[TODO] specify monitoring hooks]`
- **Initial triage:** Capture logs from `work/` and relevant `tests.d/NNN-*` directories; never share sensitive data outside vaultline-secured channels.
- **Mitigation:** Roll back via `scripts/release.sh --rollback` if available. `[TODO] confirm rollback path]`
- **Postmortem:** Document findings in `doc/incidents/<date>.md` and link from README/DEVELOPER if process changes.

## 4. Secret Management
- All secrets reside in the vaultline store `jeff`; use vaultline hooks to inject at runtime.
- Rotate credentials per the ops calendar and update this section with the rotation cadence. `[TODO] add rotation frequency and owners]`
- Never commit generated secrets or Smokey outputs containing sensitive data. Use `tmp/` for transient transfers and delete after confirmation.

## 5. Contact & Escalation
`[TODO] list on-call owners, communication channels, and escalation timeframes]`
