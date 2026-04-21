# Task 001 – Jeff Templating Spec (Draft v0.1)

## Goal
Provide reusable text templates in Jeff with `{{ ... }}` syntax, includes, and no Python/Jinja runtime dependency.

## CLI Command
- Template command namespace: `jeff tmpl`
- Discoverability via help text (`jeff tmpl --help`) must explain what tmpl does and where templates are loaded from.
- Planned subcommands:
  - `jeff tmpl list`
  - `jeff tmpl validate <name>`
  - `jeff tmpl render <name>`

## Engine
- Use Go `text/template`.
- Default strict mode: missing variables/functions fail rendering.

## Storage
- User templates in `~/.config/jeff/templates/`.
- One directory per template package:
  - `~/.config/jeff/templates/<templateName>/`
  - each package must contain `main.tmpl`
  - additional include files live next to it in the same directory
- Template package names may be nested paths.
  - Example: `finances/report-monthly` maps to:
    - `~/.config/jeff/templates/finances/report-monthly/main.tmpl`

Example:

```
~/.config/jeff/templates/
  prompt/
    main.tmpl
    header.tmpl
    footer.tmpl
  finances/
    report-monthly/
      main.tmpl
      table.tmpl
  release/
    main.tmpl
    summary.tmpl
```

## Allowed Syntax (v0.1)
- Variable interpolation: `{{ .VarName }}`
- Includes: `{{ include "name" . }}`
- Optional safe helpers: `default`, `upper`, `lower`, `trim`

## Not Allowed (v0.1)
- Shell execution from templates
- Arbitrary file I/O
- Remote fetch/network calls

## Include Resolution Rules
- Include names must match whitelist pattern: `[a-zA-Z0-9/_-]+`.
- Disallow `..` and absolute paths.
- Includes are template-package aware and resolve as follows:
  - **Local include** (no package prefix):
    - `include "header.tmpl"` from `prompt/main.tmpl` resolves to `prompt/header.tmpl`
  - **Cross-package include** (explicit package prefix):
    - `include "release/summary.tmpl"` resolves to `release/summary.tmpl`
    - `include "release"` (directory only) resolves to `release/main.tmpl`
- Path traversal like `include "../release/summary.tmpl"` is forbidden.
- Implicit default file:
  - if include target resolves to a package directory only, use `<package>/main.tmpl`.

## Collision Prevention / Runtime Namespace
- Jeff must not rely on plain file names like `main.tmpl` as global identifiers.
- Internal runtime identifiers are always namespaced by template path below `templates/`.
  - Example internal name form:
    - `runtime/finances/report-monthly/main.tmpl`
    - `runtime/prompt/main.tmpl`
- This guarantees collision safety even if multiple template packages are loaded in one runtime context.

## `define` / `template` Handling
- `define` and `template` are treated as advanced compatibility features.
- All `define` templates must be registered in the same runtime namespace (`runtime/...`) rather than plain names.
- These template definitions are memory-only runtime artifacts; they are never persisted as separate files.
- User docs should clearly recommend normal usage via `main.tmpl` + `include` and warn that `define/template` is advanced mode.

## Cycle Detection (must catch long cycles)
Build an include dependency graph and validate it using DFS with 3 states per node:
- `unvisited`
- `visiting`
- `done`

If DFS reaches a `visiting` node, fail with cycle error.
This catches:
- `a -> a`
- `a -> b -> a`
- `a -> b -> c -> a`

Error format should include full path, e.g.:
`include cycle: a -> b -> c -> a`

## Error Behavior
- Any template compile/load/include error aborts rendering.
- Error should include template name + line information when available.
- No silent fallback behavior in v0.1.

## Internal API Sketch
- `LoadTemplates(dir string) (*TemplateSet, error)`
- `Render(name string, data any) (string, error)`
- `ValidateIncludes(name string) error`

## Future Extensions
- Optional control-flow policy (`if`, `range`) after v0.1.
- Optional richer function map with explicit allowlist.
