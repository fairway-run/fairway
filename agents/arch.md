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
- Coordinator, packet, checkpoint, session-launch, tracker, and Postgres
  adapter boundaries before implementation starts.
- Workstream profile semantics: named gates, review domains, task metadata,
  packet-template schema, and how profile config maps to readiness behavior.
- `docs/design/release-cuts.md` — release scope and ship gates.

Arch writes design notes in `docs/design/`. Implementation is handed off to `backend` or `ui`.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| Implementing an arch-decided spec | `backend` (usually) or `ui` |
| CI / release plumbing | `ops` |
| Process / governance docs | `governance` |
| Provider/session backend glue after the adapter contract is set | `ops` |

## Standards

- [Coding standards](../docs/governance/coding-standards.md) — design notes section.
- [Review guards](../docs/governance/review-guards.md) — arch changes require governance + backend approval.

## Typical outputs

- A new `docs/design/*.md` document.
- An updated [docs/design/open-questions.md](../docs/design/open-questions.md).
- An updated [docs/design/release-cuts.md](../docs/design/release-cuts.md)
  when scope moves between releases.
- A handoff to `backend` referencing the design doc with implementation acceptance checks.

## Constraints

- Schema changes require a migration plan and a backup step.
- State machine changes require both a default behavior and an opt-in config path (never break existing configs).
- CLI surface changes require deprecation notes for ≥ 1 minor version before removal post-1.0.
- Workstream profile changes must stay generic. Consumer projects may motivate
  an example, but profile semantics should work for release-readiness,
  frontend-migration, service-extraction, SDK-readiness, and security-hardening
  tracks.
- Tracker credential/storage policy requires arch sign-off before backend
  implementation.
