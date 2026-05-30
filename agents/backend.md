# Agent: backend

**Branch:** `agent/backend`
**Worktree:** `<root>/{repo}-backend`
**Provider (informational):** Claude

## Scope

Everything that is not the dashboard frontend, CI / release, or pure docs:

- `cmd/fairway/` — CLI command wiring.
- `internal/config/` — TOML loader / validator.
- `internal/store/` — SQLite schema, migrations, queries.
- `internal/state/` — state machine validation.
- `internal/session/` — PID / tmux / heartbeat tracking.
- `internal/coordinator/` — preflight/status/tick composition.
- `internal/packet/` — context, bugfix, and watcher packet rendering.
- `internal/git/` — worktree shellouts.
- `internal/report/` — report generators.
- `internal/dashboard/` — *server* side (HTTP handlers, SSE source). UI templates and assets belong to `ui`.
- Adoption/profile plumbing: `fairway adoption artifact`, configured
  workstream profile validation, named profile gates, evidence-backed gate
  evaluation, route samples, and future packet-template rendering hooks after
  `arch` signs off the schema.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| HTML templates, CSS, HTMX | `ui` |
| GitHub Actions, goreleaser, packaging | `ops` |
| Schema design choices, state machine semantics | `arch` |
| Workstream profile semantics or new gate meanings | `arch` / `governance` |
| Docs, governance, AGENTS.md | `governance` |

## Standards

Read before touching code:

- [Coding standards](../docs/governance/coding-standards.md)
- [Testing](../docs/governance/testing.md)
- [Review guards](../docs/governance/review-guards.md)
- [Commits](../docs/governance/commits.md)

## Typical handoffs out

- New dashboard data endpoint → handoff to `ui` with the JSON shape.
- Schema column change → handoff to `arch` for review of migration safety.

## Typical handoffs in

- `arch` lands a state machine spec change → `backend` implements.
- `ui` needs a new query (e.g. "tasks blocked > 24h") → `backend` adds it to `store.Views`.
