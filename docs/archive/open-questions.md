# Open questions — week 1 decisions

These are unresolved in GPUaaS and are settled here to avoid carrying ambiguity into fairway.

## 1. State machine cardinality

**Decision:** Ship 4 states by default (`todo`, `in_progress`, `blocked`, `done`). Allow extension via `[states] allowed` and optional `[states] transitions`. See [state-machine.md](../design/state-machine.md).

**Rationale:** The 11-state v2 model is aspirational. Forcing it on users bakes in unvalidated distinctions. Config-driven states cost ~20 LOC of validation and let dogfooding decide whether the richer flow is worth the operator overhead.

## 2. Task source of truth

**Decision:** SQLite DB is authoritative. Optional YAML / JSON import (`fairway import <path>`) is for bootstrapping only — one-shot, never overwrites mutable state.

**Rationale:** In GPUaaS, YAML re-sync overwrites SQLite mutable fields, causing data loss. Fairway makes the DB the only mutable source.

## 3. Evidence enforcement

**Decision:** Config-driven gate.

```toml
[gates]
require_evidence_before_done = false   # default
require_review_before_done = false     # default
require_handoff_before_merge_ready = false
require_blocked_reason = true
```

Teams opt in. Defaults are permissive so a new user can get value before configuring policy.

## 4. Multi-project visibility & tenancy

**Decision:** Per-project DBs in v1, but every table carries `project_id` from day 1. Multi-project visibility on one user's machine uses SQLite `ATTACH DATABASE` over a registry. The `project_id` column makes the schema portable to a shared backend (Postgres) without a row-rewrite migration. See [multi-project.md](../design/multi-project.md) and [schema.md](../design/schema.md).

**Rationale:** The shared-server scenario (multiple humans collaborating on one project, or a SaaS hosted offering) is plausibly near-term. Pre-paying the column cost (~8 short string columns) is cheaper than a future ALTER + backfill migration. The store layer threads `project_id` so callers never see it.

## 5. Postgres backend

**Decision:** Out of scope for v1. Schema is portable; adapter is v2+ work.
Capture the adapter compatibility contract now so v2 can prove SQLite/Postgres
parity command by command. See [postgres-adapter.md](../design/postgres-adapter.md).

## 6. Dashboard mutations

**Decision:** Observation-only in v0.1. Mutations remain CLI-only. May expand in v0.2 once auth and audit are designed.

**Rationale:** Keeps week 1 scope tight; avoids CSRF / auth / audit work; forces the CLI to be feature-complete.

## 7. Task hierarchy (epics / stories / subtasks)

**Decision:** Self-referential `parent_id` on `task_definitions`. N-level tree. Optional `kind` label (`epic`, `story`, `task`, `bug`, `spike`) configured via `[task_kinds]`. Drop the separate `track_checkpoints` identity model, but keep append-only task checkpoints for operating decisions. New `fairway spawn` command auto-parents under the current claimed task's parent (sibling) or the current task (`--child`) to prevent context loss when agents discover work mid-task. See [hierarchy.md](../design/hierarchy.md) and [checkpoints.md](../design/checkpoints.md).

**Rationale:** GPUaaS's `track_checkpoints` was a separate concept that never quite tracked the work; the real need is "epics with multiple tasks, checkpointed operating decisions, and when an agent discovers a new bug, it stays connected to the epic." A self-referential tree handles the work model, while `task_checkpoints` handles active/parked/awaiting-input side-track state. Epics are tasks; the state machine applies uniformly.

## 8. Ordering: priority and sequence

**Decision:** Three orthogonal ordering signals.

- `dependencies` (existing JSON array) — hard gate; task is not `ready` until all deps reach a terminal state.
- `priority INTEGER NULL` — soft, cross-cutting urgency. Lower rank = more urgent. Config-driven labels via `[task_priorities]`. NULL = unprioritized, sorts last.
- `sequence INTEGER NULL` — soft, within-siblings order. Lower = earlier. Sparse values recommended (10, 20, 30). NULL = unsequenced, sorts last.

Sort order for `fairway ready` and dashboard backlog: priority asc → sequence asc → created_at asc.

`task_definitions` becomes slowly-changing; `priority`, `sequence`, `title`, `notes`, `kind`, `parent_id`, `dependencies`, `acceptance_checks` are mutable via `fairway update`. New `updated_at` column. An audit table is deferred to v0.2.

**Rationale:** Conflating priority and sequence forces invented urgency or loses one signal. Poiesis used filename prefixes (`iam-001-...`) for sequence and had no priority — fairway closes both gaps.

## 9. Task granularity

**Decision:** Orchestrator defines granularity. Agents do not subdivide assigned tasks into fairway sub-tasks. Two outlets for agent progress tracking: `task_evidence` rows (visible) or the agent's own scratch (invisible). `fairway spawn --child` prints a warning when invoked from a leaf-kind task; `--sibling` (default) and `--child` from epic/story are silent. If an agent thinks a task is too big, they hand it back to `arch` rather than splitting it themselves. See [hierarchy.md](../design/hierarchy.md#task-granularity-who-decides).

**Rationale:** Agent-driven decomposition causes granularity sprawl, breaks epic rollup math, muddles review boundaries, and creates state machine ambiguity. Keeping every fairway row at orchestrator-defined granularity preserves the dashboard's signal-to-noise ratio.

## 10. Caller-role determination

**Decision:** Resolve in this order: `--as <role>` flag, `FAIRWAY_ROLE` env var, current worktree's configured role (looked up by path), prompt if still ambiguous.

**Rationale:** Worktree-based resolution is the ergonomic happy path; flag and env var let scripts and CI override.

## 11. GPUaaS extraction boundary

**Decision:** Port the queue model, not the GPUaaS implementation surface. See
[gpuaas-extraction.md](gpuaas-extraction.md).

**Rationale:** GPUaaS proved the source-of-truth split, evidence records,
handoffs, reviews, sessions, merge readiness, and reports. Its role names,
review regexes, branch names, provider commands, DB path, and acceptance-check
examples are project policy, so they become config or adapters.

## 12. Coordinator operating model

**Decision:** Add first-class coordinator, packet, watcher, checkpoint, and review-lane command surfaces. See [coordinator-loop.md](../design/coordinator-loop.md), [context-packets.md](../design/context-packets.md), [watchers.md](../design/watchers.md), [checkpoints.md](../design/checkpoints.md), and [review-lanes.md](../design/review-lanes.md).

**Rationale:** GPUaaS used these operational wrappers to keep parallel work
bounded and observable. They are generic enough to belong in fairway, while
provider launch commands remain adapters.

## 13. Regression and bugfix packets

**Decision:** Keep workflow regression packs and bug-fix review packets as
generic catalog/rendering commands. See [regression-packets.md](../design/regression-packets.md).

**Rationale:** GPUaaS added these after live workflow bugs. They are not core
state-machine concepts, but they are important quality gates: every bug fix
should explain root cause and regression coverage, and workflow-impacting tasks
should point at a reusable pack.

## 14. Issue tracker integrations

**Decision:** Add Jira, Linear, and other issue trackers as adapters, not as core
dependencies. See [issue-tracker-integrations.md](../design/issue-tracker-integrations.md).

**Rationale:** Existing tools should keep owning product planning and external
stakeholder visibility. Fairway should import/link/export deliberately while
keeping local execution facts in its own DB.

## 15. Actor identity and task IDs

**Decision:** Actor is the active `agent_sessions.id` when a command can infer
one, otherwise `<os_user>@<host>`. Task IDs are user-supplied, stable, and match
`^[A-Z]+-[0-9]+$` by default. Fairway v1 does not auto-generate task IDs. See
[concepts.md](../design/concepts.md).

**Rationale:** Both choices unblock the first migration and transition tests.
Session IDs make agent activity traceable; the OS fallback keeps manual use
simple. User-supplied IDs preserve compatibility with imported Jira/Linear/Git
issue keys without inventing another allocator.
