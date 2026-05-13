# Vaultline Infrastructure

Vaultline is embedded in Jeff as a sidecar binary, but Jeff uses the normal
Vaultline daemon as the runtime process. Jeff registers its encrypted store with
that daemon instead of managing a separate short-lived daemon.

Jeff's encrypted store lives below this room:

```text
jeff/vaultline/store/
```

The store contains encrypted Vaultline data such as `.vlx` files. Treat those
files as binary encrypted data, not as memory prose. Memory castle search, ask,
and cleanup workflows must not interpret or summarize store contents.

Operational rules:

- Register the store with `store add`, not `store init`, when it already exists.
- Use transient unseal for the `jeff` store.
- Keep the store passphrase outside memory files.
- Use `jeff vl ...` for Jeff-owned secrets.
- Never write secret values to prompts, Markdown files, logs, or source files.
