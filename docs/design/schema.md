# Schema

Fairway uses SQLite via `modernc.org/sqlite` (pure Go, no CGO). The database lives at the path configured by `[fairway] db_path` (default `.fairway/state.db`).

Seven tables, plus `schema_migrations` for migration tracking. Task hierarchy (epics, stories, subtasks) lives in `task_definitions` via a self-referential `parent_id` — see [hierarchy.md](hierarchy.md).

## Project scope

Every table carries a `project_id TEXT NOT NULL` column. In v1 SQLite, each DB holds exactly one `project_id` value (set from `[fairway] project_name` at DB open). In a future Postgres adapter, one DB will hold many `project_id` values — no migration needed to add the column because it is already there.

PKs and FKs that include `project_id` are explicit per table below. The store layer threads `project_id` through every read and write; callers never pass it.

Multi-project visibility on a single user's machine is still provided at the dashboard layer via `ATTACH DATABASE` over a registry; see [multi-project.md](multi-project.md).

## Tables

### `task_definitions`

Slowly-changing task metadata. One row per task. Most fields are mutable via `fairway update` — see "Mutability" below.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | Project this task belongs to. Immutable. |
| `id` | TEXT NOT NULL | Stable task identifier, e.g. `T-042`. Unique within a project. Immutable. |
| `parent_id` | TEXT | Self-referential FK for hierarchy (epics → stories → tasks). NULL = root. Mutable (reparenting). |
| `kind` | TEXT | Optional label (`epic`, `story`, `task`, `bug`, `spike`). Validated against `[task_kinds] allowed` when configured. Mutable. |
| `title` | TEXT NOT NULL | Short human-readable title. Mutable. |
| `role` | TEXT NOT NULL | Role that owns this task. Mutable (handoff updates `task_state.owner`, not this). |
| `notes` | TEXT | Long-form description, acceptance criteria, links. Mutable. |
| `acceptance_checks` | TEXT | JSON array of opaque strings. Mutable. |
| `dependencies` | TEXT | JSON array of task IDs that must reach a terminal state before this task is `ready`. Mutable. |
| `priority` | INTEGER | Urgency. Lower = more urgent. NULL = unprioritized. Validated against `[task_priorities]` when configured. Cross-cutting (overrides epic boundaries in sort). |
| `sequence` | INTEGER | Suggested order among siblings (same `parent_id`). Lower = earlier. NULL = unsequenced. Soft signal, not a gate. |
| `created_at` | DATETIME NOT NULL | Immutable. |
| `created_by` | TEXT | OS user or agent identifier. Immutable. |
| `updated_at` | DATETIME NOT NULL | Touched on any mutable-field change. |

Primary key: `(project_id, id)`.
FK: `(project_id, parent_id) → task_definitions(project_id, id)`.

Indices:
- `(project_id, parent_id)` — descendant traversal.
- `(project_id, status, priority, sequence, created_at)` via join with `task_state` — backlog sort hot path (see [hierarchy.md](hierarchy.md) and [dashboard.md](dashboard.md) for the sort order).

#### Mutability

Three orthogonal ordering signals: **`dependencies`** (hard gate — task not `ready` until deps terminal), **`priority`** (soft, cross-cutting urgency), **`sequence`** (soft, within-siblings order). All three are mutable.

An audit table `task_definitions_changes` may come in v0.2 if drift becomes a debugging pain. For v0.1, the audit trail is `updated_at` plus the git history of any YAML/JSON imports.

See [hierarchy.md](hierarchy.md) for the tree model, the `spawn` command, granularity rules, and epic rollup semantics.

### `task_state`

Mutable per-task execution state. One row per task.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `status` | TEXT NOT NULL | Must be in the configured `[states] allowed`. |
| `owner` | TEXT | Role currently responsible. |
| `claimant` | TEXT | OS user or session identifier holding the claim. |
| `branch` | TEXT | Branch where work is happening. |
| `claimed_at` | DATETIME | |
| `completed_at` | DATETIME | |
| `commit_sha` | TEXT | Commit that satisfied the task, when done. |
| `review_required` | BOOLEAN NOT NULL DEFAULT 0 | Set by `fairway route review`. |
| `review_status` | TEXT | `not_required` / `pending` / `approved` / `changes_requested` / `blocked`. |
| `reviewer` | TEXT | Latest routed or recorded reviewer. |
| `reviewed_at` | DATETIME | Latest review timestamp, when any. |
| `review_note` | TEXT | Latest review summary, when any. |
| `updated_at` | DATETIME NOT NULL | |

Primary key: `(project_id, task_id)`.
FK: `(project_id, task_id) → task_definitions(project_id, id)`.

Indices:
- `(project_id, owner, status)` — hot path for "what is each role doing?" on the dashboard.
- `(project_id, status)` — backlog views.
- `(project_id, claimant)` — session reconciliation.

### `task_state_history`

One row per state transition. Append-only.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `from_status` | TEXT | NULL on initial insert. |
| `to_status` | TEXT NOT NULL | |
| `from_owner` | TEXT | Previous owner / responsible role. |
| `to_owner` | TEXT | New owner / responsible role. |
| `from_branch` | TEXT | Previous branch. |
| `to_branch` | TEXT | New branch. |
| `from_commit_sha` | TEXT | Previous commit SHA. |
| `to_commit_sha` | TEXT | New commit SHA. |
| `command_source` | TEXT | CLI command or integration that created the row. |
| `actor` | TEXT NOT NULL | OS user or session ID. |
| `reason` | TEXT | Optional human note. |
| `at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_state(project_id, task_id)`.
Index: `(project_id, task_id, at)` for the task detail page.

### `task_handoffs`

Directed handoff between roles.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `from_role` | TEXT NOT NULL | |
| `to_role` | TEXT NOT NULL | |
| `payload` | TEXT | Inline text or path to a file. |
| `commit_sha` | TEXT | Commit being handed off, if any. |
| `changed_files` | TEXT | Human summary or newline-separated file list. |
| `commands` | TEXT | Acceptance commands run before handoff. |
| `results` | TEXT | Summary of command results. |
| `risks` | TEXT | Residual risks. |
| `blockers` | TEXT | Known blockers. |
| `next_step` | TEXT | Recommended next slice of work. |
| `acknowledged_at` | DATETIME | When `to_role` acknowledged. |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_state(project_id, task_id)`.
Index: `(project_id, to_role, acknowledged_at)`.

### `task_evidence`

Artifact paths and result classifications.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `handoff_id` | INTEGER | Optional FK to `task_handoffs(id)`. |
| `command_text` | TEXT | Command or check that produced this evidence. |
| `result` | TEXT | `pass` / `fail` / `partial` / `skipped` / `blocked` / NULL. |
| `artifact_path` | TEXT | Screenshot, log, transcript, report, or other artifact path. |
| `artifact_type` | TEXT | Optional display hint, e.g. `log`, `screenshot`, `playwright`, `coverage`, `report`. |
| `duration_seconds` | INTEGER | Optional elapsed time for timing reports. |
| `notes` | TEXT | |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_state(project_id, task_id)`.

### `task_reviews`

Review records.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `reviewer` | TEXT NOT NULL | |
| `verdict` | TEXT NOT NULL | `approve` / `changes` / `reject`. |
| `reviewed_commit_sha` | TEXT | Commit reviewed, if applicable. |
| `route_reason` | TEXT | |
| `notes` | TEXT | |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_state(project_id, task_id)`.

Constraint (enforced in code): `reviewer != task_state.claimant`.

### `agent_sessions`

Lifecycle for a single agent process attached to a lane.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | |
| `id` | TEXT NOT NULL | Session identifier (UUID or `role-pid-startts`). |
| `role` | TEXT NOT NULL | |
| `lane` | TEXT | Optional lane identifier when multiple lanes share a role. |
| `worktree_path` | TEXT | Worktree path for status and attach affordances. |
| `branch` | TEXT | Branch active when the session was recorded. |
| `session_backend` | TEXT | `tmux`, `zellij`, `shell`, or another adapter label. |
| `provider` | TEXT | Informational provider label, e.g. `codex`, `claude`, `gemini`, `shell`. |
| `session_name` | TEXT | Human-readable backend session name. |
| `task_id` | TEXT | Task associated with the session, when known. |
| `pid` | INTEGER | OS PID. |
| `tmux_pane` | TEXT | e.g. `agents:0.2`. |
| `transcript_path` | TEXT | Optional path reference; transcript contents are not stored in DB. |
| `status` | TEXT NOT NULL | `starting` / `running` / `ended` / `failed` / `stale`. |
| `started_at` | DATETIME NOT NULL | |
| `last_heartbeat_at` | DATETIME | |
| `ended_at` | DATETIME | |
| `exit_code` | INTEGER | Process exit code, when known. |
| `end_reason` | TEXT | `normal` / `reconciled` / `crashed` / NULL. |

Primary key: `(project_id, id)`.
Index: `(project_id, role, ended_at)` — find the live session for a role.

## Migration strategy

- One SQL file per migration in `internal/store/migrations/`, named `001_init.sql`, `002_*.sql`, ...
- Embedded via `//go:embed`.
- A `schema_migrations(version INTEGER PK, applied_at DATETIME)` table tracks applied versions. (No `project_id` — migrations are per-DB, not per-project.)
- Migrations are forward-only in v1. `fairway db backup` runs automatically before any migration beyond `001_init.sql`.

## Design notes

**Why `project_id` everywhere even in single-project SQLite?** So the schema is portable to a shared backend (Postgres) without a row-rewrite migration. The marginal cost in v1 is ~8 short string columns and a `WHERE project_id = ?` clause on every read — both hidden behind the store layer.

**Why split definitions from state?** Same reason a user table is split from a session table: definitions are referenced by foreign keys and rarely change; state churns.

**Why an explicit `task_state_history`?** SQLite has no built-in temporal tables. The audit trail is a first-class queryable surface for the dashboard's activity feed.

**Why does `agent_sessions` carry tmux pane?** So the dashboard can render a "click to attach" affordance. NULL when tmux is not in use.

**Why evidence has both command text and artifact path.** GPUaaS showed that
completed work needs command-level proof even when there is no durable file
artifact. Artifact paths remain optional references; large logs, screenshots,
and transcripts stay out of the DB.
