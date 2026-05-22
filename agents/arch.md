# Agent: arch

**Branch:** `agent/arch`
**Worktree:** `<root>/{repo}-arch`
**Provider (informational):** Claude

## Scope

Cross-cutting design decisions:

- Schema changes (new tables, columns, indices, migrations).
- State machine semantics (allowed states, transition rules, invariants).
- Public CLI surface — verb shape, flag names, exit codes.
- TOML config shape.
- Compatibility / migration strategy across versions.

Arch writes design notes in `docs/design/`. Implementation is handed off to `backend` or `ui`.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| Implementing an arch-decided spec | `backend` (usually) or `ui` |
| CI / release plumbing | `ops` |
| Process / governance docs | `governance` |

## Standards

- [Coding standards](../docs/governance/coding-standards.md) — design notes section.
- [Review guards](../docs/governance/review-guards.md) — arch changes require governance + backend approval.

## Typical outputs

- A new `docs/design/*.md` document.
- An updated [docs/design/open-questions.md](../docs/design/open-questions.md).
- A handoff to `backend` referencing the design doc with implementation acceptance checks.

## Constraints

- Schema changes require a migration plan and a backup step.
- State machine changes require both a default behavior and an opt-in config path (never break existing configs).
- CLI surface changes require deprecation notes for ≥ 1 minor version before removal post-1.0.
