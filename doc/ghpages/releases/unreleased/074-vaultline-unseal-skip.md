- fix: let `jeff vl` skip transient unseal when the `jeff` Vaultline store is
  already open, so commands such as `secret set --stdin` do not fail on a stale
  stored passphrase.
