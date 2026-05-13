---
layout: page
title: Sidecars
---

Jeff can embed Linux helper tools as sidecars. Sidecars are built during `scripts/build.sh`, embedded into the Jeff binary, and executed from memory with `memfd_create` instead of being extracted to a cache directory.

## [Vaultline](https://micwin.github.io/vaultline/)

[Vaultline](https://micwin.github.io/vaultline/) is configured in `sidecars/vaultline.conf`:

```text
name=vaultline
command=vl
source=../vaultline
url=https://github.com/micwin/vaultline.git
ref=develop
package=./cmd/vaultline
```

Build order:

1. `JEFF_VAULTLINE_SOURCE` wins when set.
2. Otherwise `source` is resolved relative to the Jeff repository.
3. If that local source exists, Jeff builds exactly that checkout.
4. If the local source is missing, Jeff clones or updates `url` under `work/sidecars/source/vaultline` and checks out `ref`.
5. [Vaultline](https://micwin.github.io/vaultline/) is built into `work/sidecars/vaultline/vaultline`.
6. The binary is staged at `src/jeff/internal/sidecars/embedded/vaultline` for Go embed. This staged file is ignored by Git.

Dirty local [Vaultline](https://micwin.github.io/vaultline/) checkouts are allowed only when both Jeff and [Vaultline](https://micwin.github.io/vaultline/) are on development branches (`develop` or `feature/*`). Otherwise the build stops before embedding a non-reproducible sidecar.

At runtime, `jeff vl` uses the embedded [Vaultline](https://micwin.github.io/vaultline/)
CLI but talks to the normal running Vaultline daemon. Jeff registers its
encrypted store from `memcastle/jeff/vaultline/store/` with that daemon and
opens it transiently; it does not maintain a separate daemon lifecycle or
runtime PID cache.

Release metadata is written to `work/sidecars/vaultline/metadata.env`. Releases use `VAULTLINE_VERSION_LABEL` for the downloads table:

- URL source: [Vaultline](https://micwin.github.io/vaultline/)'s reported version.
- Clean local source: short commit slug.
- Dirty development source: branch name such as `develop` or `feature/example`.
