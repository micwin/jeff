# Unreleased Release Notes

Drop short Markdown snippets here (one file per change) while working on a release.
Use a descriptive filename like `012-menu-submenus.md` and add bullet points, for example:

```
- feat: add nested `jeff menu` submenus with inline management
```

`scripts/prepare-release.sh` will concatenate all `.md` files in this directory into the new
release notes under the "Changes" section and then delete the snippets so the directory is ready
for the next cycle.
