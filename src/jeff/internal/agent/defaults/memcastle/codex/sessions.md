# Sessions

Jeff uses one Codex session for the personal agent. `jeff codex init [--force]
[sessionid]` binds that session. Without a session id, Jeff prompts for one until
automatic Codex session creation is implemented.

Specialists should contact Jeff through `jeff agent contact` instead of calling
the broad Jeff operator session directly. Use this only for process
coordination, handoff structure, conflicting agent responsibilities, or where
findings belong.

Before an account migration or other continuity risk, run
`jeff specialists archive refresh --briefs` and verify the result with
`jeff specialists archive status`.
