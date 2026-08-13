# RUNBOOK

Operational procedures for Jeff. Update this file whenever the release process, onboarding steps, or incident protocols change.

## 1. Onboarding
1. Ensure access to the `jeff` [Vaultline](https://micwin.github.io/vaultline/) store. Existing Jeff stores are registered with `vaultline store add jeff <path>`; new stores are created by Jeff on first secret use.
2. Run `scripts/bootstrap-dev.sh` to install multi-language toolchains and sync Smokey.
3. Verify `smokey --dir tests.d` executes the baseline suites (starting with `tests.d/000-*`). `[TODO] list expected baseline cases`
4. Review `DEVELOPER.md` and AGENTS.md (if acting as an agent) before first commit.

## 2. Build & Release Process
1. Start from `develop`. Create a release branch named `release/v<major>.<minor>.<patch>` (semantic versioning) for the work in flight.
2. Run `scripts/build.sh` to generate binaries/docs into `work/` and `dist/`.
   The build embeds configured sidecar tools such as [Vaultline](https://micwin.github.io/vaultline/) from `sidecars/*.conf`; set `JEFF_VAULTLINE_SOURCE=/path/to/vaultline` to override the default local source path. Build metadata is written to `work/sidecars/<name>/metadata.env` and feeds the downloads table.
   At runtime, `jeff vl` compares the embedded backpack Vaultline with a local `vaultline` binary from `PATH` and uses the newer semantic version; pass `--use-backpack-version` or `--use-local-version` to force one side for debugging.
3. Execute `smokey --dir tests.d` and ensure all numbered suites pass.
4. Invoke `scripts/release.sh` to bump versions, cut tags/branches, and publish artifacts/pages. `[TODO] document release.sh flags and CI steps`
   The release workflow publishes `work/ghpages-site` to the `gh-pages` branch from a fresh publish clone; it must not switch branches inside the build workspace because ignored build directories can survive branch changes.
5. For cross-version publishing (docs sites, portals), update the dedicated branch (e.g., `ghpages`, `site`) and tag the deployed commit with `GHPAGES_CURRENT`, `SITE_CURRENT`, or another `*_CURRENT` tag.
6. Post-release, archive artifacts per compliance requirements. `[TODO] add storage location]`
7. From the release branch, run `scripts/post-release.sh` to fast-forward merge the release back into `develop` once the release is verified.

## 3. Incident Response
- **Detection:** Monitor build/test pipelines and Smokey outputs for failures. `[TODO] specify monitoring hooks]`
- **Initial triage:** Capture logs from `work/` and relevant `tests.d/NNN-*` directories; never share sensitive data outside [Vaultline](https://micwin.github.io/vaultline/)-secured channels.
- **Mitigation:** Roll back via `scripts/release.sh --rollback` if available. `[TODO] confirm rollback path]`
- **Postmortem:** Document findings in `doc/incidents/<date>.md` and link from README/DEVELOPER if process changes.

## 4. Secret Management
- All secrets reside in the [Vaultline](https://micwin.github.io/vaultline/) store `jeff`; use [Vaultline](https://micwin.github.io/vaultline/) hooks to inject at runtime.
- Jeff stores its encrypted `jeff` store under `memcastle/jeff/vaultline/store/` and registers that path with the running [Vaultline](https://micwin.github.io/vaultline/) daemon via `store add`. Use `store add` for existing stores; do not run `store init` on an existing Jeff store.
- Rotate credentials per the ops calendar and update this section with the rotation cadence. `[TODO] add rotation frequency and owners]`
- Never commit generated secrets or Smokey outputs containing sensitive data. Use `tmp/` for transient transfers and delete after confirmation.

## 5. User Data Storage
- Keep Jeff configuration in `$XDG_CONFIG_HOME/jeff` or `~/.config/jeff` when `XDG_CONFIG_HOME` is unset.
- Keep durable user data in `$XDG_DATA_HOME/jeff` or `~/.local/share/jeff` when `XDG_DATA_HOME` is unset. Finance and banking records belong here, not in `config.json`.
- Keep cacheable or rebuildable data in `$XDG_CACHE_HOME/jeff` or `~/.cache/jeff` when `XDG_CACHE_HOME` is unset.
- Store API keys, banking credentials, tokens, and other secrets only in the [Vaultline](https://micwin.github.io/vaultline/) store `jeff`; non-secret metadata may reference Vaultline keys by name.
- Treat `*.vlx` files and Vaultline store metadata below `memcastle/jeff/vaultline/store/` as encrypted binary data, not as memory-castle prose.
- Jeff tracks configuration migrations with `schema_version` in `config.json`. The Debian package runs `jeff migrate --quiet` for the installing sudo user when possible, and the CLI also applies pending migrations on startup.

## 6. Contact & Escalation
`[TODO] list on-call owners, communication channels, and escalation timeframes]`

## 7. Specialist Disaster Recovery

Before a machine move, Codex account change, or destructive maintenance, run
`jeff specialists archive refresh --briefs` and require a clean
`jeff specialists archive status`. Preserve the Memory Castle specialist rooms,
archive metadata, and Git-ignored private transcript and repository payload in
protected storage. A Memory Castle Git push excludes that private payload and
is not sufficient.

Use the [Specialist Backup And Recovery](doc/ghpages/specialist-recovery.md)
procedure for reconstruction and verification. Do not refresh a partially
reconstructed archive, replace persistent session ids, or extract repository
snapshots over unrelated files.
