# Specialist Backup And Recovery

This procedure preserves and reconstructs Jeff's specialists. It is not a
backup or restore procedure for an entire workstation or every Jeff user file.
The archive contains private transcripts and may contain private repository
files, so transport it only through suitably protected storage.

## Requirements

- A Linux system with [Jeff](https://micwin.github.io/jeff/) installed. Published
  packages are available on the [Jeff downloads page](https://micwin.github.io/jeff/downloads.html).
  From a Jeff source checkout, `scripts/build.sh --install` builds and installs
  the current package.
- The [OpenAI Codex client](https://developers.openai.com/codex/cli/) logged into
  the account that will run the restored specialists.
- A copy of the Memory Castle that includes the Git-ignored private specialist
  archive payload.

No separate codex-resume installation is required. Its implementation is kept
inside the Memory Castle at:

```text
$(jeff memcastle path)/codex/tools/codex-resume/codex-resume
```

## Refresh The Archive

Run this before changing Codex accounts or handing the specialists to another
Jeff installation:

```bash
jeff specialists archive refresh --briefs
jeff specialists archive status
```

`refresh` records every configured specialist alias, exact historical session
id, codex-resume policy, local transcript, continuity brief, and relevant
repository state. `status` must report no errors and no missing transcripts. A
partial refresh exits with status 2 while preserving successful results.

The generated data is stored below:

```text
$(jeff memcastle path)/codex/specialist-archive/
$(jeff memcastle path)/codex/specialists/
```

The first directory contains metadata, the successor prompt, compressed
transcripts, and repository snapshots. The second contains the readable role
and current-state room for each specialist.

The `specialist-archive/private/` directory is ignored by the Memory Castle Git
repository. Copying, committing, or cloning only tracked Memory Castle files is
therefore not sufficient. Preserve both directories above, including
`specialist-archive/private/`, together with the rest of the Memory Castle.

## Put The Archive Back

Install and initialize Jeff first. Determine the destination instead of
assuming a particular home directory:

```bash
jeff init
jeff memcastle path
```

Restore the saved Memory Castle into the exact directory printed by
`jeff memcastle path`. After copying, these paths must exist:

```text
$(jeff memcastle path)/castle.md
$(jeff memcastle path)/codex/specialist-archive/registry.json
$(jeff memcastle path)/codex/specialist-archive/private/sessions/
$(jeff memcastle path)/codex/specialists/
$(jeff memcastle path)/codex/tools/codex-resume/codex-resume
```

Do not add another `memcastle/` directory level. For example, if
`jeff memcastle path` prints `~/.local/share/jeff/memcastle`, `castle.md` must be
directly below that directory.

Make codex-resume executable and expose it through the user's `PATH`. A typical
per-user link is:

```bash
chmod +x "$(jeff memcastle path)/codex/tools/codex-resume/codex-resume"
mkdir -p "$HOME/.local/bin"
ln -sfn "$(jeff memcastle path)/codex/tools/codex-resume/codex-resume" \
  "$HOME/.local/bin/codex-resume"
```

The active codex-resume configuration normally lives at
`~/.config/codex-resume/sessions.toml`; it is not the source of truth for this
recovery. The archive's `registry.json` contains each alias, session key,
session id, working directory, added directories, sandbox, approval mode,
delegation policy, and short aliases required to reconstruct that file.

## Restore Repositories

Read:

```text
$(jeff memcastle path)/codex/specialist-archive/repositories.json
```

- When `remote_reproducible` is true, clone `remote` into `root` and check out
  the exact `head` commit. The commit is authoritative.
- When `snapshot` is present, create the recorded `root` and extract the named
  archive from `specialist-archive/private/` into it. A snapshot contains
  tracked and untracked non-ignored files, but no `.git` history or ignored
  build output.
- Restore separately listed initialized submodules by the same rules.

If the new machine uses another home or project root, adapt `root`, `cd`, and
`add_dirs` paths during reconstruction. Never extract a snapshot over unrelated
files.

## Recover The Specialists

First verify that the archive itself is readable:

```bash
jeff specialists archive status
jeff specialists archive successor
```

Give the printed successor prompt to the new primary Jeff session. It directs
the agent to the archived registry, specialist rooms, repository records, and
offline transcript search. The successor reconstructs
`~/.config/codex-resume/sessions.toml`, restores the required session bindings,
and checks each specialist without silently replacing historical identities.

When the same Codex account is still in use and the historical session is
available, preserve its archived id and resume it. Test each reconstructed
alias with:

```bash
codex-resume --show-policy <alias>
codex-resume <alias>
```

A different Codex account may not be allowed to resume those historical
sessions. The archive cannot bypass that account boundary. In this case, the
successor uses roles, continuity briefs, transcripts, and repository state to
create separately identified replacement specialists. Historical session ids
remain archived as identity records.

## Verify The Result

```bash
jeff specialists archive status
jeff specialists list
jeff specialists archive search --alias <alias> '<known phrase>'
codex-resume --show-policy <alias>
```

Confirm that the archive reports no errors or missing transcripts, expected
aliases are present, a known archived fact can be found, recorded working
directories exist, and a non-destructive call to every critical specialist
succeeds. Run a new `refresh --briefs` only after reconstruction works; an early
refresh could replace useful current metadata with partial results.
