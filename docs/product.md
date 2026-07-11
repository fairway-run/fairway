# Product

## Vision

Fairway is the smallest tool that makes governed agentic engineering practical:
2-6 coding agents can work in parallel while task ownership, evidence, reviews,
handoffs, sessions, readiness, and risk stay visible.

The operating model is
[governed agentic engineering](governed-agentic-engineering.md): agents can do
substantial implementation work, but evidence, review, ownership, promotion,
and human comprehension remain first-class engineering controls.
For small teams using Fairway in AI Cloud-style loops, see the
[Small-team autonomy operating model](design/small-team-autonomy-operating-model.md).
The [common-path automation model](design/common-path-automation.md) makes
routine reversible work compact while preserving the underlying task, session,
checkpoint, evidence, review, and promotion records.
The [task decision memory model](design/task-decision-memory.md) preserves the
material reasoning needed after context compaction while keeping transcripts
optional and non-authoritative.

Fairway is the local-first coordination control plane for that model.
The [agent-native product interface](design/agent-native-product-interface.md)
defines agents as the primary operational users and humans as the authority for
consequential judgment. It also defines grounded code-explanation packets and
the optional advisory LLM narrative boundary.
Traffic-control lanes were the first useful primitive: one lane, one role, one
worktree, one visible task state. The product direction is broader but still
bounded: Fairway coordinates the facts around multi-agent engineering work
without becoming the system that performs or approves that work.
For teams that outgrow one local coordination host, the
[Shared-team operating model](design/shared-team-operating-model.md) defines
when a shared Fairway control plane is justified and what authority boundaries
must stay intact.

The benchmark: a solo developer with three Claude lanes and one Codex lane open
should never lose track of which agent is doing what, why a task is stuck, which
evidence exists, or who is blocking whom.

## Principles

1. **Local-first.** Single binary, SQLite, no network required for any core feature. The dashboard binds to localhost by default.
2. **Config over code.** Roles, branches, states, review routing, and gates are TOML. Changing them does not require a rebuild.
3. **The DB is the runtime source of truth.** Backlog files define task shape;
   the DB owns claims, status, evidence, sessions, reviews, checkpoints, and
   handbacks. No hidden sync loop.
4. **The CLI is feature-complete.** Anything you can do in the dashboard, you can do from the CLI. The dashboard is a view, not a privileged surface.
5. **Boringly portable.** Pure Go, no CGO, single binary, works the same on macOS / Linux / Windows.
6. **Hospitable defaults.** A new user gets value from `fairway init` + `fairway dashboard` before they touch a config file.
7. **Context over hardcoded agents.** Borrowed from Poiesis: durable docs, task notes, contracts, acceptance checks, and evidence make agents specialized; fairway does not need provider-specific agent classes.
8. **Profiles over project-specific workflow.** Workstream profiles can define
   task kinds, packet templates, review domains, evidence expectations, and
   dashboard grouping without baking one product's process into core.
9. **Rule packs over copied process docs.** Reusable operating knowledge belongs
   in versioned rule-pack repositories, such as
   `fairway-run/fairway-rules-platform`. Project/domain-specific rules belong
   in that project's own rule-pack repository. Fairway core owns loading,
   matching, evidence, dashboard, and closeout behavior; it does not hardcode
   one project's rules.
10. **Automate the repeated checks.** The operating model should stay short.
   Repeated rules such as commit boundaries, push/CI signal, deploy-run
   tracking, and active-session reconciliation should become CLI guards and
   dashboard findings.
11. **Promotion is explicit.** Provider and thread branches are local scratch by
    default. Remote push is a recorded promotion action with intent, normally
    performed by the orchestrator or reviewer/merge lane after local
    verification.
12. **Deterministic coordination before advisory intelligence.** Fairway should
    compute routine next actions, waits, handbacks, live-window phase, failure
    routing, and session state from durable facts before asking an LLM or human
    to interpret them. See
    [Coordination intelligence](design/coordination-intelligence.md).

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
  branch, CI run, and proof surface. Remote push intent guards now keep
  disposable provider branches local by default and report unintentional remote
  branches as closeout debt. GPUaaS / ARC remains the adoption example, not the
  core product shape.

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

- Auto-claiming or silently moving work between lanes.
- Auto-approving reviews or waiving required review domains.
- Auto-merging branches, auto-pushing commits, or auto-deploying releases.
- Destructive branch, worktree, remote, or state cleanup without an explicit
  operator command.
- A workflow / DAG engine.
- An IAM / permissions system.
- An LLM provider runtime or credential/proxy abstraction. Optional advisory
  adapters may exist only as bounded, replaceable, non-authoritative inputs
  validated against Fairway state and policy.
- A CI runner.
- A SaaS hosted offering.
- An autonomous approval, merge, deploy, or cleanup engine.
- A transcript, prompt, secret, generated-content, cookie, or
  provider-credential store by default.
- A cost gate for task completion, review, merge readiness, or release
  promotion. Provider usage accounting is advisory planning telemetry.
- A product-specific task taxonomy hardcoded into core.

If a feature pushes toward any of those, it goes in a different tool.

The durable boundary rules are defined in
[Product boundaries](design/product-boundaries.md). The active backlog source
rules are defined in [Backlog sources](design/backlog-sources.md). The
coordination-intelligence direction is defined in
[Coordination intelligence](design/coordination-intelligence.md).

## Competing approaches considered

| Approach | Why not for fairway |
|---|---|
| Pure shell scripts (status quo in GPUaaS) | No state machine, no audit trail, no dashboard. |
| Poiesis-style provider workflow engine | Useful contract/review/QA lessons, but too coupled to LLM execution. Fairway coordinates agents; it does not run them. |
| Plane / Jira / Linear / Notion | Planning and stakeholder tools, not execution stores for agent sessions, worktrees, evidence, handoffs, reviews, and merge readiness. Fairway integrates with them while keeping local execution state in its DB. |
| Temporal / Cadence | Massive overkill; not designed for human-paced coordination. |
| Custom Kanban app | Does not dispatch to worktrees, does not track sessions. |

Fairway sits between "shell scripts" and "issue tracker" — closer to the former in weight, closer to the latter in affordances.
