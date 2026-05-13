# Vaultline Skill

## Human-Readable Description

Use Jeff's embedded Vaultline sidecar to work with Jeff-owned secrets through the
normal Vaultline daemon. This skill applies whenever Jeff needs credentials for
apps, skills, API integrations, reports, or command automation.

## Skill-Specific Preprompt

Rules:

- Use `jeff vl`, not a separate `vaultline` command, unless the user explicitly
  asks to debug Vaultline outside Jeff.
- Treat the `jeff` store as Jeff's own local secret store.
- Treat `jeff/vaultline/store/` as encrypted binary data inside the memory
  castle.
- When the user gives an unqualified secret id for Jeff-owned data, assume the
  `jeff:` store prefix.
- Never store secrets in prompts, memory files, command files, logs, release
  notes, or repo files.
- Prefer existence checks and key discovery over value reads.
- Do not print secret values back to the user unless the user explicitly asks
  for the value itself.
- Use app/skill prefixes for credentials, for example
  `jeff:skills.paperless.token`.

## Store Registration

For an existing external Jeff store, register it with the running Vaultline
daemon:

```sh
vaultline store add jeff /absolute/path/to/jeff/vaultline/store
```

Do not use `store init` on an existing store. It is for new stores.

## Key Discovery

Use `secret list jeff: --raw` for inventory and filter key names locally. Avoid
exposing values:

```sh
jeff vl secret list jeff: --raw
```

Use `secret get` only when the secret value is required for an immediate
operation:

```sh
jeff vl secret get jeff:skills.paperless.token
```

## Examples

- Check whether Paperless credentials exist before connecting to Paperless.
- Find skill-owned tokens by listing `jeff:` keys and filtering for
  `skills.*.token`.
- Store a new app API token under a `skills.<app>.token` key.
- Confirm that required secrets exist without printing their values.
