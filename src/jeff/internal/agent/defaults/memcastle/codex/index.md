# Codex

Use this wing for Codex session setup, prompt injection, compaction recovery,
continuity notes, and agent behavior rules.

Use `continuity/` for compact state snapshots that preserve active work across
context compaction or restart.

Jeff is the agent coordinator and first escalation point for agent-related
process problems: broken collaboration or delegation, conflicting agent
responsibilities, unclear handoffs, continuity or specialist coordination
failures, and uncertainty about where findings belong. Jeff does not replace the
human for product semantics, irreversible decisions, or approvals reserved for
the user.

Specialists discover the current Jeff contact protocol through
`jeff agent instructions`. Follow that command's output instead of relying on
copied invocation details. The stable contact command is
`jeff agent contact "<dense English coordination prompt>"`. Use it only for
process coordination, handoff structure, conflicting agent responsibilities, or
deciding where findings belong. Do not include secrets or secret values. Do not
replace persistent session ids.

Use `jeff specialists archive refresh` to preserve registered specialist
identities, invocation policies, local transcripts, and non-reproducible
repository state. Add `--briefs` when enabled specialists should refresh their
continuity summaries. Recovery agents start with
`jeff specialists archive successor`; archived session ids remain identity
records and must not be replaced without explicit human approval.
