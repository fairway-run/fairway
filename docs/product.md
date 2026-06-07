# Product

## Vision

Fairway is the smallest tool that makes running 2–6 coding agents in parallel feel coordinated rather than chaotic.

Fairway is a local-first coordination control plane for multi-agent engineering
work: tasks, ownership, evidence, reviews, handoffs, sessions, readiness, and
risk.

The benchmark: a solo developer with three Claude lanes and one Codex lane open should never lose track of which agent is doing what, why a task is stuck, or who is blocking whom.

## Principles

1. **Local-first.** Single binary, SQLite, no network required for any core feature. The dashboard binds to localhost by default.
2. **Config over code.** Roles, branches, states, review routing, and gates are TOML. Changing them does not require a rebuild.
3. **The DB is the source of truth.** Imports are one-shot. No sync, no eventual consistency, no reconciliation worker.
4. **The CLI is feature-complete.** Anything you can do in the dashboard, you can do from the CLI. The dashboard is a view, not a privileged surface.
5. **Boringly portable.** Pure Go, no CGO, single binary, works the same on macOS / Linux / Windows.
6. **Hospitable defaults.** A new user gets value from `fairway init` + `fairway dashboard` before they touch a config file.
7. **Context over hardcoded agents.** Borrowed from Poiesis: durable docs, task notes, contracts, acceptance checks, and evidence make agents specialized; fairway does not need provider-specific agent classes.
8. **Profiles over project-specific workflow.** Workstream profiles can define
   task kinds, packet templates, review domains, evidence expectations, and
   dashboard grouping without baking one product's process into core.
9. **Automate the repeated checks.** The operating model should stay short.
   Repeated rules such as commit boundaries, push/CI signal, deploy-run
   tracking, and active-session reconciliation should become CLI guards and
   dashboard findings.

## What "done" looks like for v1.0

A user can:

- `brew tap fairway-run/tap && brew install --cask fairway`
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
- Dashboard mutations for claim/status (with CSRF, audit).
- TUI mode (`fairway tui`) for SSH / headless use.
- Generic workstream profile track: profile config, declarative packet
  templates, named readiness gates, dashboard grouping, task ownership
  metadata, and structured guard evidence. Profile gates and task metadata have
  started landing, the dashboard now has an initial profile/kind grouping, and
  dashboard filters over profile metadata; configured packet templates can
  render packets, guard reports can be recorded as typed evidence, and
  readiness reports summarize profile gates. Merge readiness also honors
  task-level review domains. Workflow checks now flag dirty docs/code,
  unpushed commits, deploy-run prerequisites, and active reconciliation
  findings, while release-run packets and release verification guard public
  release, asset, and Homebrew readiness. Provider usage attribution records
  normalized counts from adapters and rolls them up by task/lane/provider for
  retrospective planning, without pricing or provider API polling. OTel usage
  ingestion, Codex `exec --json` mapping, Claude Code OTel mapping, and work
  batches now cover the first production lessons from GPUaaS stabilization:
  usage should be attributed without reading provider-private state, and
  multiple granular tasks should be validated as one batch when they share a
  branch, CI run, and proof surface. GPUaaS / ARC remains the adoption example,
  not the core product shape.

### v1.0
- Stable schema. Migrations guaranteed forward-compatible.
- Homebrew tap.
- Postgres adapter (likely), with compatibility harness first.
- Issue tracker adapter design and import/link/export prototype, with Plane,
  Jira, and Linear as first targets. Plane is the local open-source evaluation
  target for product/external-team collaboration and adapter semantics; its
  local evaluation runbook comes before the provider-neutral tracker adapter
  contract.

### Beyond v1
- Multi-repo federation.
- Webhooks / event emission.
- Deeper issue tracker integrations for Plane, Jira, Linear, GitHub Issues, and
  similar planning tools.

## Anti-goals

These will never be in fairway:

- A workflow / DAG engine.
- An IAM / permissions system.
- An LLM provider abstraction.
- A CI runner.
- A SaaS hosted offering.
- An autonomous approval, merge, deploy, or cleanup engine.
- A transcript, prompt, secret, or provider-credential store by default.
- A product-specific task taxonomy hardcoded into core.

If a feature pushes toward any of those, it goes in a different tool.

The durable boundary rules are defined in
[Product boundaries](design/product-boundaries.md). The active backlog source
rules are defined in [Backlog sources](design/backlog-sources.md).

## Competing approaches considered

| Approach | Why not for fairway |
|---|---|
| Pure shell scripts (status quo in GPUaaS) | No state machine, no audit trail, no dashboard. |
| Poiesis-style provider workflow engine | Useful contract/review/QA lessons, but too coupled to LLM execution. Fairway coordinates agents; it does not run them. |
| Plane / Jira / Linear / Notion | Planning and stakeholder tools, not execution stores for agent sessions, worktrees, evidence, handoffs, reviews, and merge readiness. Fairway integrates with them while keeping local execution state in its DB. |
| Temporal / Cadence | Massive overkill; not designed for human-paced coordination. |
| Custom Kanban app | Does not dispatch to worktrees, does not track sessions. |

Fairway sits between "shell scripts" and "issue tracker" — closer to the former in weight, closer to the latter in affordances.
