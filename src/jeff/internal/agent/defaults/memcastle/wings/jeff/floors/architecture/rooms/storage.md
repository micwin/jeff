# Storage Room

Jeff separates data by durability and sensitivity:

- Config: `$XDG_CONFIG_HOME/jeff` or `~/.config/jeff`
- Durable data: `$XDG_DATA_HOME/jeff` or `~/.local/share/jeff`
- Cache: `$XDG_CACHE_HOME/jeff` or `~/.cache/jeff`
- Secrets: local Vaultline store `jeff` under Jeff's data directory
