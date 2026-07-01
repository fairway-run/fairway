# Shared-Team Concurrency And Sync Model

FW-258 defines the concurrency, synchronization, offline, and conflict model for
future shared-team Fairway. It is a design model only. It does not implement a
server runtime, Postgres adapter, storage migration, event replication,
dashboard write authority, provider-send authority, approval authority,
merge/deploy/live-operation authority, release tag, or dashboard restart.

The model builds on:

- [shared-team-operating-model.md](shared-team-operating-model.md),
- [shared-team-server-api.md](shared-team-server-api.md),
- [postgres-adapter.md](postgres-adapter.md),
- [schema.md](schema.md).

## Goals

- Preserve Fairway's DB as the source of truth while allowing multiple trusted
  writers in a future shared-team mode.
- Make conflicts explicit and recoverable instead of silently merging stale
  provider or laptop state.
- Keep append-only facts append-only.
- Keep local SQLite useful for local/offline work without making offline writes
  authoritative until reviewed and promoted.
- Define what belongs in server/Postgres mode versus local export/import or
  event replication.

## Write Classes

Fairway writes fall into four concurrency classes:

| Class | Examples | Concurrency rule |
|---|---|---|
| Guarded mutable state | task status, owner, claimant, branch, commit SHA, review materialization, task definition updates | Validate expected current state in one transaction; fail with conflict on stale state. |
| Append-only facts | evidence, checkpoints, reviews, handoffs, notifications, usage, completion handbacks, waits, provenance events | Insert new rows with actor/session metadata; never overwrite prior facts. |
| Replace-by-key attachments | sessions, track memory, dashboard saved views, registry entries | Upsert by stable identity and reject unsafe identity changes. |
| Derived/read models | ready queue, review waits, reports, dashboard projections, coordinator plan, reconciliation | Recompute from source facts; cache only as a short-lived optimization. |

No shared-team mode may introduce a second wait store, review store,
notification store, dashboard cache store, or provider-memory truth source.

## Task Status And Ownership

Task status transitions are the most important guarded mutable state. A future
server/Postgres implementation must:

- lock the task state row for the target project/task during validation;
- validate allowed state transitions and dependency gates inside the same
  transaction;
- validate owner/claimant/branch/commit expectations when supplied;
- append a state-history row in the same transaction;
- return a conflict when the task changed since the client read it;
- return current task summary and suggested safe action with the conflict.

Suggested optimistic fields:

```text
project_id
task_id
expected_status
expected_owner
expected_updated_at
expected_commit_sha
idempotency_key
```

Clients that receive a conflict must reload `task-detail` or the relevant
read-model before retrying. Provider chat memory is not a retry source.

## Evidence, Reviews, Handoffs, Waits, And Notifications

Append-only facts are safe for concurrent inserts when each row carries enough
identity and context:

- authenticated actor or provider/session identity;
- task and project scope;
- command source or adapter name;
- artifact/reference metadata when applicable;
- result/verdict/state and timestamp;
- idempotency key or dedupe signature for retryable adapter paths.

Facts should not be edited to correct history. Corrections add a new checkpoint,
evidence row, review, supersede row, or follow-up task. Dashboard and CLI
projections must show current state by projecting from those facts, not by
rewriting history.

Review materialization is the exception: inserting a review also updates
`task_state.review_status` in the same transaction. That denormalized field is a
read optimization and must remain derivable from `task_reviews`.

## Sessions And Provider Attachments

Sessions are replace-by-key attachments. A provider session ID represents one
execution attachment; upsert is acceptable when it refreshes heartbeat,
transcript path, lane, or current task.

Fail-closed rules:

- a terminal session cannot be revived by a stale provider event;
- a session cannot silently switch task IDs without an explicit handoff or
  replacement event;
- a running task with no active provider lifecycle checkpoint is a
  reconciliation finding;
- concurrent heartbeat updates are last-writer-wins only for heartbeat fields,
  not for task ownership or status.

Provider lifecycle remains evidence of attachment, not proof of task
completion, review, merge, deploy, or live execution.

## Imports And Backlog Definitions

Backlog imports update stable task definitions. They must not overwrite mutable
execution state such as task status, owner, reviews, evidence, sessions, waits,
or checkpoints.

Shared-team imports need a definition-version boundary:

- import compares task definition fields against current definitions;
- updates are allowed for metadata fields already considered mutable by the
  schema;
- import records source file, actor, and timestamp;
- conflicting definition edits return a reviewable diff rather than silently
  choosing one source;
- YAML text-search misses are not runtime-state failures when the DB task
  exists and `task-detail` is authoritative.

Runtime task state remains changed only through Fairway commands.

## Recipes, Packets, And Local Artifacts

Recipes and packet templates are source-controlled or configured artifacts, not
mutable execution state. Shared-team APIs may list or render them when privacy
checks pass, but they should not accept arbitrary prompt bodies as durable
server state.

Local evidence artifacts are references. If a provider records an artifact path
from a laptop, the shared server records the reference and the actor/context.
Safe artifact viewing requires configured local roots on the serving host. A
path that is safe on one laptop is not automatically renderable on another.

## Idempotency

Retryable API and adapter writes should accept an idempotency key. The server
stores enough metadata to detect safe replay:

```text
actor_id
project_id
task_id
command_family
idempotency_key
payload_digest
result_row_ids
created_at
```

Safe replay returns the original row IDs/result. Mismatched replay fails closed
with `idempotency_key_conflict`. Commands without an idempotency key may still
succeed, but provider adapters and external notifiers should use one for any
network retry path.

## Conflict Response Shape

Conflict responses should be structured and safe to display:

```json
{
  "error": "task_state_conflict",
  "project_id": "fairway",
  "task_id": "FW-123",
  "expected_status": "in_progress",
  "actual_status": "blocked",
  "actual_owner": "ops",
  "actual_updated_at": "2026-07-01T12:00:00Z",
  "suggested_command": "fairway task-detail FW-123"
}
```

Conflict responses must not include raw private payloads, secrets, prompts,
transcripts, raw tool bodies, or generated content dumps.

## Offline Local Work

Offline local writes are not automatically authoritative. Supported patterns:

- export a read-only context packet before going offline;
- perform local scratch work in a local SQLite DB or source branch;
- record local evidence with timestamps and actor identity;
- reconnect and generate a promotion/reconciliation packet;
- have an operator review conflicts before importing or recording facts in the
  shared store.

Forbidden patterns:

- silent replay of offline status changes over newer shared state;
- automatic merge of two mutable task-state histories;
- backdated approvals without explicit actor/time evidence;
- treating provider chat transcript as the source of truth;
- bypassing review, merge-ready, release, deploy, or live-operation gates.

Offline promotion should prefer append-only evidence/checkpoints and explicit
follow-up tasks over mutable state replay.

## Event Replication And Export/Import

Event export/import may help observability, backup, or airgap transfer, but it
must not become hidden workflow sync.

Allowed:

- deterministic exports of tasks, evidence summaries, reviews, checkpoints,
  waits, sessions, and provenance references;
- import dry-runs that show proposed rows and conflicts;
- append-only fact import with idempotency keys;
- read-model equivalence checks after import.

Not allowed without separate review:

- bidirectional background sync between two writable Fairway stores;
- automatic conflict resolution for task state;
- cross-project writes based only on matching task IDs;
- private transcript/prompt replication;
- hidden dashboard-originated mutation replay.

## Server/Postgres Versus Local SQLite

Use server/Postgres mode for:

- multiple simultaneous trusted writers;
- row locking and transactional conflict detection;
- central audit and backup;
- remote provider/session adapter writes;
- team control-room read/write consistency.

Use local SQLite plus export/import for:

- single-host or single-operator work;
- airgap scratch work;
- local demos and drills;
- bounded offline work that will be promoted through a review packet;
- read-only context transfer.

Postgres is not required for read-only multi-project dashboard rollups, local
performance tuning, or replacing deterministic read-model design.

## Release Gates Before Implementation

A future implementation slice must include:

- store/interface tests that run the same command semantics under SQLite and
  the server-backed adapter;
- transaction tests for status transition conflicts and review materialization;
- append-only fact insertion tests under concurrent writers;
- idempotency replay and mismatch tests;
- offline promotion dry-run tests;
- import conflict tests for task definition drift;
- reconciliation tests for stale sessions and active work after conflicts;
- documentation for operator conflict handling.

## Acceptance Checklist

- Multi-writer task state uses guarded transactional semantics.
- Append-only facts remain append-only and carry actor/session context.
- Sessions are replace-by-key attachments with lifecycle guardrails.
- Imports do not overwrite mutable execution state.
- Offline/local writes require explicit promotion and operator-reviewed
  reconciliation.
- Event export/import is bounded and not hidden workflow sync.
- The model adds no runtime server, storage migration, dashboard write,
  provider-send, approval, merge, deploy, release, or live-operation authority.
