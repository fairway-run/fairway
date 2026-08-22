# Schema

Fairway uses SQLite via `modernc.org/sqlite` (pure Go, no CGO). The database lives at the path configured by `[fairway] db_path` (default `.fairway/state.db`).

Fairway uses migration-managed tables plus `schema_migrations` for migration
tracking. Task hierarchy (epics, stories, subtasks) lives in
`task_definitions` via a self-referential `parent_id` — see
[hierarchy.md](hierarchy.md). Checkpoints attach append-only operating notes to
tasks; see [checkpoints.md](checkpoints.md).

## Project scope

Every table carries a `project_id TEXT NOT NULL` column. In v1 SQLite, each DB holds exactly one `project_id` value (set from `[fairway] project_name` at DB open). In a future Postgres adapter, one DB will hold many `project_id` values — no migration needed to add the column because it is already there.

PKs and FKs that include `project_id` are explicit per table below. The store layer threads `project_id` through every read and write; callers never pass it.

Multi-project visibility on a single user's machine is still provided at the dashboard layer via `ATTACH DATABASE` over a registry; see [multi-project.md](multi-project.md).

SQLite remains the default Fairway store. A future Postgres/server-backed store
is for shared team write coordination, not for dashboard caching or a second
read-model truth source. The Postgres path must preserve the same schema
ownership boundaries, command semantics, and `project_id` scoping described
here; see [postgres-adapter.md](postgres-adapter.md) for the assessed deployment
model, cutover requirements, and compatibility harness.

## Track memory lifecycle

`track_memory` is replace-by-key curated context with accountable `owner`,
`review_by`, `disposition`, `promotion_target`, `canonical_commit`, and
`superseded_by_track_id` projection fields. New active rows require at least
one existing checkpoint, evidence, or review source ID.

`track_memory_lifecycle` is append-only audit history for explicit disposition
changes. It records prior and next disposition, reason, promotion or
supersession references, actor, and timestamp. Reconciliation and dashboard
views derive lifecycle debt from these rows; there is no second memory or wait
store.

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
| `profile` | TEXT | Optional workstream profile name. Validated against `[[workstream_profiles]]` when configured. |
| `owning_domain` | TEXT | Optional architecture/domain owner label, e.g. `platform`, `billing`, `identity`. |
| `owning_layer` | TEXT | Optional layer label, e.g. `api`, `service`, `frontend`, `guard`, `release`. |
| `source_paths` | TEXT | JSON array of source paths relevant to the task. |
| `target_paths` | TEXT | JSON array of intended target paths or artifacts. |
| `review_domains` | TEXT | JSON array of review domains expected for this task. |
| `tags` | TEXT | JSON array of generic cross-cutting tags. Supports simple tags such as `production-readiness` and key:value tags such as `environment:staging`. |
| `risk_level` | TEXT | Optional risk label, e.g. `low`, `medium`, `high`. |
| `migration_type` | TEXT | Optional migration/refactor type, e.g. `facade`, `boundary-guard`, `ownership-map`. |
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
| `review_status` | TEXT | Denormalized latest review status: `not_required` / `pending` / `approved` / `changes_requested`. CLI and dashboard detail views may display `partial_approval` when this value is `approved` but required `review_domains` are still missing. |
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
| `actor` | TEXT NOT NULL | Active session ID when known, otherwise `<os_user>@<host>`. |
| `reason` | TEXT | Optional human note. |
| `at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_state(project_id, task_id)`.
Index: `(project_id, task_id, at)` for the task detail page.

### `task_commits`

Append-only task-to-commit provenance. A task may carry several implementation
commits while `task_state.commit_sha` remains the single canonical completion
commit.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | Stable row identity. |
| `project_id` | TEXT NOT NULL | Project scope. |
| `task_id` | TEXT NOT NULL | Owning Fairway task. |
| `commit_sha` | TEXT NOT NULL | Resolved Git commit SHA. |
| `association_kind` | TEXT NOT NULL | `work_base`, `work`, `completion`, or `manual`. |
| `source` | TEXT NOT NULL | Deterministic recording path such as `work_start`, `work_close`, or `record_commit`. |
| `actor` | TEXT NOT NULL | Fairway actor that recorded the association. |
| `created_at` | TEXT NOT NULL | UTC record time. |

The unique key is `(project_id, task_id, commit_sha, association_kind)`.
`work_base` establishes the branch range and is never counted as delivered
work. Migrations do not infer or backfill historical links from paths or commit
messages.

### `control_friction_samples`

Attributable control-cost intervals and explicit unavailable facts. The record
is advisory measurement evidence; it does not approve, waive, or resolve the
owning control itself.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | Stable sample identity returned by `record friction start`. |
| `project_id` | TEXT NOT NULL | Project scope. |
| `task_id` | TEXT NOT NULL | Task where the control was applicable. |
| `control_id` | TEXT NOT NULL | Stable control identity used by analytics. |
| `status` | TEXT NOT NULL | `open`, `resolved`, or `unavailable`. |
| `started_at`, `resolved_at` | TEXT | RFC3339 interval bounds for measured samples. |
| `started_by`, `resolved_by` | TEXT | Actual Fairway actors for each lifecycle action. |
| `source_ref` | TEXT | Optional bounded external or Fairway source identity. |
| `reason` | TEXT | Resolution context or required unavailable reason. |
| `created_at`, `updated_at` | TEXT NOT NULL | UTC record timestamps. |

An `open` row has only start fields, a `resolved` row has both interval bounds
and actors, and an `unavailable` row has no fabricated interval and retains the
recording actor plus reason. Missing rows remain a fourth analytics state and
must not be interpreted as zero cost.

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

Evidence rows are append-only execution facts. Corrections should add a new
evidence row, checkpoint, review, or superseding task note rather than editing
or deleting historical evidence out of band. Fairway stores metadata and
references here, not artifact contents. Use `fairway provenance manifest` to
hash selected exported bundles or artifacts when a release/audit packet needs
tamper-evidence. `fairway audit export` projects these existing rows in stable
`id` order into `fairway.sovereign-audit-record.v1` JSONL. The export includes
actor, action, project, task, created-at, and a SHA-256 of `detail`; raw detail
content is not exported. Each row binds the previous row hash so the chain
remains stable when export policy, Fairway version, or trusted-time source
changes. A customer-signed
`fairway.sovereign-audit-export.v1` manifest binds the record file, chain head,
retention/legal-hold metadata, trusted-time evidence digest, source version,
and either a genesis marker or the previous externally retained checkpoint.
This is a derived export, not a second audit store, and does not mutate the
`audit_events` source of truth.

Offline distribution is also file-based rather than a database store.
`fairway.offline-distribution-manifest.v1` binds current and rollback release
identity, required platform archives and verifier binaries, typed local assets,
lifecycle scripts, file modes, sizes, and SHA-256 digests. A detached
`fairway.offline-distribution-signature.v1` Ed25519 signature binds the exact
manifest. `fairway.offline-distribution-verification.v1` is a derived,
read-only verification report. None of these schemas mutates task, release,
deployment, or certification state.

Harness interoperability adds append-only external-run, execution-observation,
and evaluator-result records. Their stable identities, replay/conflict rules,
privacy boundary, and authority invariants are defined in
[Harness interoperability and verified outcomes](harness-interoperability.md).
They extend the engineering record without replacing `task_evidence`,
`task_reviews`, `task_outcomes`, or task state. The implementation migration
must be additive and must leave existing projects with no harness records fully
functional.

### `harness_external_runs`, `harness_observations`, and `harness_evaluator_results`

Migration 018 implements the three append-only record families. All use a
source-scoped primary identity, retain the canonical payload and its SHA-256
idempotency digest, reference exactly one Fairway task, and sort readback by
source time plus stable identity. External runs additionally make
`submission_id` unique within a source. Observation and evaluator references
store both the referenced source and record ID; both halves are null for a
run-independent or observation-independent fact.

`harness ingest` validates the full batch and all task, session, prior-run,
run, and observation relationships before committing any row. A matching
identity/digest is an existing replay. A matching identity with a different
digest is `ErrIdempotencyConflict`. Existing stores migrate additively, and a
project with no harness records receives empty readback rather than inferred
history.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `handoff_id` | INTEGER | Optional FK to `task_handoffs(id)`. |
| `command_text` | TEXT | Command or check that produced this evidence. |
| `result` | TEXT | `pass` / `fail` / `partial` / `skipped` / `blocked` / NULL. |
| `artifact_path` | TEXT | Screenshot, log, transcript, report, or other artifact path. |
| `artifact_type` | TEXT | Optional display hint, e.g. `log`, `screenshot`, `video`, `browser-trace`, `uat`, `coverage`, `report`. UX media types are stored as artifact references only and must be redacted before recording. |
| `duration_seconds` | INTEGER | Optional elapsed time for timing reports. |
| `notes` | TEXT | |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_state(project_id, task_id)`.

### `task_decisions` and `task_decision_assessments`

`task_decisions` is the append-only curated explanation for a material task
choice. It stores the decision, trigger, alternatives, chosen option, reason,
added scope, risk, validation references, supporting fact references, optional
superseded decision id, author, and creation time. Structured lists are JSON
text for SQLite/Postgres compatibility. A unique partial index permits only one
direct replacement for a superseded row.

`task_decision_assessments` appends independent `accepted` or `insufficient`
quality findings. The task owner or claimant cannot assess their own decision.
The read model derives `draft` when no assessment exists and `superseded` when a
later decision replaces the row. Earlier decisions and assessments are never
rewritten or deleted by the command surface.

Decision text is privacy-bounded and cannot grant approval, merge, deploy,
credential, release, public-exposure, or live-operation authority. An accepted
decision means the explanation is concrete and fact-consistent; normal Fairway
review, evidence, merge, deploy, and release gates remain separate.

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

`task_reviews` is the audit log. The review columns on `task_state`
(`review_required`, `review_status`, `reviewer`, `reviewed_at`,
`review_note`) are denormalized/materialized dashboard fields. Every review
insert updates `task_reviews` and the corresponding `task_state` review columns
in the same transaction. Readers may use the denormalized columns for current
state, but historical review questions must query `task_reviews`. Verdicts map
to current review status as `approve` → `approved`; `changes` or `reject` →
`changes_requested`.

Required review-domain completeness is evaluated from `task_reviews` plus
`task_definitions.review_domains`; it is not encoded directly in
`task_state.review_status`. User-facing CLI/dashboard detail views should not
summarize a latest `approved` review as domain-complete approval while required
domains are missing; they render that case as `partial_approval`.

### `agent_sessions`

Lifecycle for a single agent process attached to a lane.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | |
| `id` | TEXT NOT NULL | Session identifier (UUID or `role-pid-startts`). |
| `role` | TEXT NOT NULL | |
| `lane` | TEXT | Optional lane identifier when multiple execution slots share a role; see [concepts.md](concepts.md). |
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

### `task_checkpoints`

Append-only operating checkpoints for epics, stories, side tracks, and watcher
work.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `state` | TEXT NOT NULL | `planned` / `active` / `awaiting_input` / `review` / `done` / `parked` / `abandoned`. |
| `owner` | TEXT | Role or lane responsible for the checkpoint. |
| `target_close_by` | DATE | Optional date for stale-track checks. |
| `summary` | TEXT NOT NULL | Current operating summary. |
| `artifact_path` | TEXT | Optional evidence link. |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_definitions(project_id, id)`.
Index: `(project_id, task_id, created_at)`.
Index: `(project_id, state, target_close_by)`.

### `server_write_idempotency`

FW-271 adds a small idempotency ledger for the shared-team write API pilot.
FW-272 extends that ledger to guarded status/review writes. It is not a second
evidence, checkpoint, status, or review store; it records retry metadata for
accepted API writes so network clients can safely replay the same request.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | |
| `command_family` | TEXT NOT NULL | `record:evidence`, `record:checkpoint`, `set:status`, or `record:review`. |
| `idempotency_key` | TEXT NOT NULL | Client-supplied retry key. |
| `actor` | TEXT NOT NULL | Redacted actor/fingerprint, never a raw token. |
| `role` | TEXT NOT NULL | Command-scoped role used for authorization. |
| `auth_source` | TEXT NOT NULL | Identity source, for example `api_token`. |
| `task_id` | TEXT NOT NULL | |
| `payload_digest` | TEXT NOT NULL | Digest of the accepted structured payload. |
| `result_kind` | TEXT NOT NULL | `evidence` or `checkpoint`. |
| `result_id` | INTEGER NOT NULL | Inserted append-only fact row ID. |
| `created_at` | DATETIME NOT NULL | |

Primary key: `(project_id, command_family, idempotency_key)`.
Index: `(project_id, task_id, created_at)`.

A replay is accepted only when actor, role, auth source, task, command family,
and payload digest match the original row. Mismatched replay fails closed. The
table stores payload digests and resulting row IDs, not raw request bodies,
prompts, transcripts, raw tool bodies, generated content, credentials, or
secrets.

### `provider_usage_events`

Append-only provider usage attribution. This table stores normalized counts and
metadata only. It must not store prompts, transcripts, secrets, provider inputs,
provider outputs, messages, or generated content. Missing numeric values stay
`NULL`; Fairway treats them as unknown rather than zero.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `provider` | TEXT NOT NULL | Provider label such as `codex`, `claude`, `gemini`, `tmux`, or `shell`. |
| `external_session_id` | TEXT | Provider-side session/thread/run id, when known. |
| `session_id` | TEXT | Fairway `agent_sessions.id`, when known. |
| `task_id` | TEXT | Fairway task receiving attribution. Nullable for provider runs not yet mapped to a task. |
| `role` | TEXT | Role or lane receiving attribution. |
| `phase` | TEXT | Optional work phase such as `implementation`, `review`, `ci`, `deploy`, or `uat`. |
| `source` | TEXT NOT NULL | `provider_reported`, `derived_snapshot`, `manual`, or `unknown`. |
| `confidence` | TEXT NOT NULL | `exact`, `estimated`, or `unknown`. |
| `started_at` | DATETIME | Measured usage window start. |
| `completed_at` | DATETIME | Measured usage window end. |
| `started_token_snapshot` | INTEGER | Optional provider running total at start. |
| `completed_token_snapshot` | INTEGER | Optional provider running total at completion. |
| `input_tokens` | INTEGER | Optional provider-reported input tokens. |
| `cached_input_tokens` | INTEGER | Optional provider-reported cached input tokens. |
| `uncached_input_tokens` | INTEGER | Optional provider-reported or derived uncached input tokens. |
| `output_tokens` | INTEGER | Optional provider-reported output tokens. |
| `reasoning_tokens` | INTEGER | Optional provider-reported reasoning tokens. |
| `total_tokens` | INTEGER | Optional provider-reported or snapshot-derived total tokens. |
| `elapsed_seconds` | INTEGER | Optional elapsed time. |
| `model` | TEXT | Optional provider model label. |
| `metadata_json` | TEXT | Optional small JSON object for non-sensitive metadata. |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, task_id) → task_definitions(project_id, id)`.

Indices:
- `(project_id, task_id, created_at)` — task detail usage timeline.
- `(project_id, provider, created_at)` — provider rollups.
- `(project_id, role, created_at)` — lane rollups.
- `(project_id, created_at)` — daily reports.

### `work_batches`

Execution and validation plans for related tasks that share one branch,
worktree, CI/deploy-run, review path, and evidence set. Tasks remain the
accountability unit; batches are the implementation and validation unit.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | |
| `id` | TEXT NOT NULL | Stable batch identifier, e.g. `BATCH-001`. |
| `title` | TEXT NOT NULL | Human-readable batch title. |
| `branch` | TEXT | Shared implementation branch. |
| `worktree_path` | TEXT | Shared worktree path. |
| `validation_commands` | TEXT | JSON array of commands expected to validate the batch. |
| `review_domains` | TEXT | JSON array of review domains expected for the shared work. |
| `rollback_criteria` | TEXT | Criteria for reverting or backing out the batch. |
| `split_criteria` | TEXT | Criteria for splitting the batch when failure diagnosis or ownership diverges. |
| `expected_ci` | TEXT | Expected CI/deploy-run description. |
| `deploy_run_id` | TEXT | Linked deploy-run id, when known. |
| `pipeline_id` | TEXT | Linked CI pipeline/run id, when known. |
| `created_at` | DATETIME NOT NULL | |
| `updated_at` | DATETIME NOT NULL | |

Primary key: `(project_id, id)`.

### `work_batch_tasks`

Membership join table from work batches to granular Fairway tasks.

| Column | Type | Notes |
|---|---|---|
| `project_id` | TEXT NOT NULL | |
| `batch_id` | TEXT NOT NULL | |
| `task_id` | TEXT NOT NULL | |
| `created_at` | DATETIME NOT NULL | |

Primary key: `(project_id, batch_id, task_id)`.
FKs:
- `(project_id, batch_id) → work_batches(project_id, id)`.
- `(project_id, task_id) → task_definitions(project_id, id)`.

Index: `(project_id, task_id)` — task detail batch lookup.

### `work_batch_evidence`

Batch-level evidence records. `fairway batch evidence` can also map evidence to
each member task by inserting corresponding `task_evidence` rows with a
`work_batch=<batch-id>` note.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `project_id` | TEXT NOT NULL | |
| `batch_id` | TEXT NOT NULL | |
| `command_text` | TEXT | Shared validation command. |
| `result` | TEXT | `pass` / `fail` / `partial` / `skipped` / `blocked` / NULL. |
| `artifact_path` | TEXT | Pipeline URL, deploy-run, log, report, or local artifact reference. |
| `artifact_type` | TEXT | Optional display hint such as `ci`, `deploy`, `uat`, or `work-batch`. |
| `notes` | TEXT | |
| `created_at` | DATETIME NOT NULL | |

FK: `(project_id, batch_id) → work_batches(project_id, id)`.
Index: `(project_id, batch_id, created_at)`.

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

**Why evidence has both command text and artifact path.** Consumer use showed that
completed work needs command-level proof even when there is no durable file
artifact. Artifact paths remain optional references; large logs, screenshots,
and transcripts stay out of the DB.

**Why keep checkpoints after dropping `track_checkpoints`.** Fairway does not
need a separate track identity table because epics/stories already represent
bounded work. It still needs append-only operating decisions for active, parked,
awaiting-input, and watcher-style work; `task_checkpoints` provides that without
creating a second task hierarchy.

## Write Semantics

### Claim Concurrency

SQLite claim must be atomic and deterministic:

1. Open `BEGIN IMMEDIATE` so the writer lock is acquired before reading
   claimable state.
2. Validate the task is claimable in the same transaction.
3. Run a guarded update, for example:

   ```sql
   UPDATE task_state
      SET status = 'in_progress',
          owner = ?,
          claimant = ?,
          branch = ?,
          claimed_at = ?,
          updated_at = ?
    WHERE project_id = ?
      AND task_id = ?
      AND status IN ('todo', 'blocked')
      AND claimant IS NULL;
   ```

4. If zero rows were updated, rollback and return `ErrAlreadyClaimed` or the
   more specific validation error.
5. Insert the `task_state_history` row in the same transaction as the successful
   update.
6. Commit.

Tests must prove two concurrent claim attempts produce exactly one winner and
one loser.
