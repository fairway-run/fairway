# Product

## Vision

Fairway is the smallest tool that makes running 2–6 coding agents in parallel feel coordinated rather than chaotic.

The benchmark: a solo developer with three Claude lanes and one Codex lane open should never lose track of which agent is doing what, why a task is stuck, or who is blocking whom.

## Principles

1. **Local-first.** Single binary, SQLite, no network required for any core feature. The dashboard binds to localhost by default.
2. **Config over code.** Roles, branches, states, review routing, and gates are TOML. Changing them does not require a rebuild.
3. **The DB is the source of truth.** Imports are one-shot. No sync, no eventual consistency, no reconciliation worker.
4. **The CLI is feature-complete.** Anything you can do in the dashboard, you can do from the CLI. The dashboard is a view, not a privileged surface.
5. **Boringly portable.** Pure Go, no CGO, single binary, works the same on macOS / Linux / Windows.
6. **Hospitable defaults.** A new user gets value from `fairway init` + `fairway dashboard` before they touch a config file.
7. **Context over hardcoded agents.** Borrowed from Poiesis: durable docs, task notes, contracts, acceptance checks, and evidence make agents specialized; fairway does not need provider-specific agent classes.

## What "done" looks like for v1.0

A user can:

- `brew install fairway`
- `fairway init` in any git repo
- Edit five lines of TOML to name their roles
- `fairway worktree setup` to create the lanes
- `fairway import tasks.yaml` to seed work
- `fairway dashboard` to watch agents work
- Run their day from the CLI with the dashboard always informative

…without reading more than the quickstart.

## Roadmap

Scope for each cut is locked in [release-cuts.md](design/release-cuts.md). The
roadmap below is directional; release cuts decide what code must be green before
each version ships.

### v0.1 — week 1
- Repo skeleton, schema, state machine, config, dashboard.
- Read-only dashboard.
- CLI verbs: `init`, `import`, `ready`, `claim`, `set-status`, `record evidence|handoff|review`, `task-detail`, `config validate`, `dashboard`, `version`.

### v0.2 — week 2–3
- Session lifecycle (PID, tmux, heartbeats).
- Worktree commands.
- Reports (status / health / timing / dispatch).
- Review routing, merge readiness, and git consistency checks.
- Coordinator tick, context packets, watcher packets, review checkout, task checkpoints, regression packs, and bug-fix packets.

### v0.3
- Dashboard mutations (with CSRF, audit).
- TUI mode (`fairway tui`) for SSH / headless use.
- GPUaaS / ARC adoption track: parity artifact, release-readiness design,
  security/UAT packet design, and evidence bridge boundaries.

### v1.0
- Stable schema. Migrations guaranteed forward-compatible.
- Homebrew tap.
- Postgres adapter (likely), with compatibility harness first.
- Issue tracker adapter design and import/link/export prototype, with Jira and
  Linear as first targets.

### Beyond v1
- Multi-repo federation.
- Webhooks / event emission.
- Deeper issue tracker integrations for Jira, Linear, GitHub Issues, and
  similar planning tools.

## Anti-goals

These will never be in fairway:

- A workflow / DAG engine.
- An IAM / permissions system.
- An LLM provider abstraction.
- A CI runner.
- A SaaS hosted offering.

If a feature pushes toward any of those, it goes in a different tool.

## Competing approaches considered

| Approach | Why not for fairway |
|---|---|
| Pure shell scripts (status quo in GPUaaS) | No state machine, no audit trail, no dashboard. |
| Poiesis-style provider workflow engine | Useful contract/review/QA lessons, but too coupled to LLM execution. Fairway coordinates agents; it does not run them. |
| Jira / Linear / Notion | Planning and stakeholder tools, not execution stores for agent sessions, worktrees, evidence, handoffs, reviews, and merge readiness. Fairway integrates with them while keeping local execution state in its DB. |
| Temporal / Cadence | Massive overkill; not designed for human-paced coordination. |
| Custom Kanban app | Does not dispatch to worktrees, does not track sessions. |

Fairway sits between "shell scripts" and "issue tracker" — closer to the former in weight, closer to the latter in affordances.
