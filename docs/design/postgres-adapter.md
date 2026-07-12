# Postgres Adapter

SQLite is fairway's v1 default because most users coordinate agents on one
machine. Consumer pilots still showed that shared, multi-machine coordination eventually
needs a network-reachable queue store. Fairway should preserve that path without
making operators learn a second workflow.

## Status

Planned v2 adapter. SQLite remains the supported default for local and
single-operator deployments. A server-backed store is justified when Fairway is
used as a shared team control plane, especially when several provider lanes,
reviewers, dashboards, and operators write to the same project from different
machines.

FW-255 assessed dashboard-performance and shared-team deployment pressure
points and landed this decision:

- keep SQLite as the local/single-user default,
- continue using multi-project dashboard attachment for local read-only rollups,
- do not use Postgres as a dashboard cache or second wait store,
- build Postgres as a first-class store adapter only after the compatibility
  harness, migration/cutover, locking, backup, and deployment topology gates are
  explicit.

The v1 schema is designed to be portable by carrying `project_id` on every table
and keeping mutable execution state behind the store layer. Portability does not
mean the embedded SQLite migrations can be applied directly to Postgres without
review.

## Deployment Modes

| Mode | Store | Intended use | Notes |
|---|---|---|---|
| Local operator | SQLite file | One developer or one local coordination host. | Default. Lowest operational cost. |
| Local multi-project dashboard | SQLite files registered by project | Read-only rollup across local configs or repos. | Uses the dashboard registry and attached DBs; it is not a shared write plane. |
| Shared team store | Postgres adapter | Multiple operators/providers writing the same Fairway project from different machines. | Future v2 mode. Requires locking, backup, migration, auth, and compatibility gates. |

Postgres should not be introduced merely because a dashboard route is slow.
Dashboard route cost is handled with projection fast paths, batching, snapshot
caching, and lazy panels. Postgres addresses cross-host write coordination,
durable shared access, and operational backup/restore posture.

## Compatibility Contract

The following command families must behave the same against SQLite and
Postgres:

- `ready`, `claim`, `set-status`, and `prune-stale`,
- `record evidence`, `record handoff`, and `record review`,
- `route review` and `merge-ready`,
- `session status` and `session reconcile`,
- `task-detail`, `status-report`, `timing-report`, and `health-report`,
- `git-check`,
- `checkpoint status` and `checkpoint stale`.

The adapter may use Postgres-native types such as `jsonb`, `timestamptz`, and
`boolean`, but the Go store must return the same domain structs and JSON output
as the SQLite store.

The current compatibility commands are:

```bash
fairway db compat --backend postgres [--print-ddl]
fairway db rehearsal --backend postgres --out .fairway/rehearsals/postgres-<timestamp>
fairway db rehearsal --backend postgres --out .fairway/rehearsals/postgres-<timestamp> --apply-dsn-env FAIRWAY_DISPOSABLE_POSTGRES_DSN
```

`--print-ddl` is a review artifact, not an applyable migration. `--apply-ddl`
is intentionally not implemented until the migration and cutover contract below
exists. By default, `db rehearsal` is non-mutating: it creates a SQLite backup,
exports source and backup read models, writes the Postgres compatibility
report/DDL, compares read-model counts, and records rollback instructions. The
default run does not apply Postgres DDL or switch any runtime store.

When `--apply-dsn-env` is supplied, `db rehearsal` runs an explicit disposable
proof instead of changing Fairway's runtime store. The command reads the DSN
from the named environment variable as a `postgres://` or `postgresql://` URL,
splits it into libpq environment variables, uses `psql`, drops and recreates
only the validated `--postgres-schema` inside that disposable database, applies
the compatibility DDL sketch, imports bounded task/state/history/evidence/
handoff/review/session rows from the SQLite snapshot, and records readback
counts. The schema must be a simple `fairway_`-prefixed name; reserved or common
schemas such as `public`, `pg_catalog`, `information_schema`, `pg_toast`, and
all `pg_` schemas are rejected before any drop statement is generated. The DSN
value is not passed in `psql` argv and is not written to the manifest. This is
still compatibility evidence: it does not prove a Postgres store adapter,
command parity, production migration, public/shared API exposure, dashboard
restart, release readiness, or cutover readiness.

## Transactions

State-changing commands remain single-transaction operations. In Postgres,
current-state transitions should lock the target task row with `SELECT ... FOR
UPDATE` before validating dependencies, role ownership, and state-machine
guards. This prevents two coordinators from claiming or completing the same task
at the same time.

The first adapter implementation must define transaction behavior for:

- task status transitions, including `claim`, `set-status`, and done/blocked
  closeout,
- append-only rows such as evidence, reviews, checkpoints, waits,
  notifications, and handbacks,
- denormalized review status updates on `task_state`,
- session heartbeat and stale-session reconciliation,
- imports that update task definitions without overwriting mutable execution
  state.

Where SQLite currently relies on one local writer plus a busy timeout, Postgres
must use row-level locks, explicit unique constraints, and retryable transaction
errors. Retried operations must stay idempotent or fail closed with a clear
operator message.

## Source Of Truth

Postgres does not change the ownership boundary:

- task imports own stable task definitions,
- fairway DB rows own mutable status, owner, evidence, reviews, sessions,
  checkpoints, and timing facts,
- imports never continuously overwrite execution state.

Dashboard projections, review waits, completion handbacks, generic waits,
rough-edge rows, and delivery reports remain projections from Fairway state.
Postgres must not introduce a second wait table, dashboard cache store, review
store, or notification truth source.

## Compatibility Harness

```bash
fairway db compat --backend postgres [--print-ddl | --apply-ddl]
fairway db rehearsal --backend postgres --out <artifact-dir> [--apply-dsn-env <env>] [--postgres-schema <schema>]
```

The harness is for adapter development and disposable experiments. DDL printed
by the harness is a sketch until a real migration path and cutover runbook
exist. Rehearsal output is a provenance packet, not a production cutover:

- `sqlite-backup.db` is rollback/import input;
- `source-export.json` captures the current SQLite read model;
- `rehearsal-export.json` captures the read model from the disposable backup;
- `postgres-compat-report.json` and `postgres-compat-ddl.sql` are review
  artifacts;
- `readmodel-equivalence.json` compares deterministic task, transition,
  evidence, handoff, review, and session counts;
- `postgres-apply.sql`, `postgres-import.sql`, and `postgres-readback.json`
  are written only when `--apply-dsn-env` is supplied; they prove disposable
  schema apply/import/readback against the DSN named by the environment
  variable and must not be treated as a runtime backend switch. The apply file
  contains `DROP SCHEMA IF EXISTS <schema> CASCADE`, so the command rejects
  reserved schemas and requires a `fairway_`-prefixed rehearsal schema;
- `manifest.json` records paths, counts, compatibility/equivalence status, and
  boundaries;
- `rollback.md` states the manual rollback/readback steps.

The harness should grow in this order:

1. static migration token checks that flag SQLite-specific SQL,
2. generated Postgres DDL review output,
3. disposable backup/export read-model equivalence rehearsal,
4. disposable Postgres schema apply in CI or local containers,
5. command parity tests that run the same store test cases against SQLite and
   Postgres,
6. cutover rehearsal that imports a SQLite backup/export into Postgres and
   proves read-model equivalence.

## Migration And Cutover

A supported cutover must be reversible and rehearsed before team deployments:

1. stop writers or place Fairway into a documented read-only/drain mode,
2. back up the SQLite DB and record a provenance manifest,
3. apply reviewed Postgres migrations,
4. import task definitions, mutable state, append-only logs, sessions, waits,
   notifications, and reviews with stable IDs,
5. run read-model equivalence checks for `ready`, `task-detail`, `review-waits`,
   completion handbacks, generic waits, dashboard board, reports, and
   reconciliation,
6. switch config to the Postgres DSN through an explicit store backend setting,
7. keep the SQLite backup as rollback input until the team-store deployment has
   passed a documented observation window.

Rollback must not merge divergent writes automatically. If both SQLite and
Postgres accept writes after cutover, recovery requires an operator-reviewed
reconciliation packet.

## Configuration Boundary

Future configuration should make the backend explicit:

```toml
[fairway.store]
backend = "sqlite" # sqlite | postgres
path = ".fairway/state.db"
postgres_dsn_env = "FAIRWAY_POSTGRES_DSN"
```

Credentials must come from the environment or an OS credential store, not from
committed config. Dashboard read-only mode remains a dashboard trust boundary;
the store backend does not grant dashboard mutation, send, approval, merge,
deploy, or live-operation authority.

## Operational Requirements

A server-backed store needs operational controls before release:

- TLS or private-network connectivity suitable for the deployment environment,
- least-privilege DB user for Fairway writes,
- backup, restore, and point-in-time recovery guidance,
- migration lock or advisory lock so only one migrator runs,
- health/readiness checks that include DB connectivity and schema version,
- clear latency expectations for dashboard routes and provider write commands,
- observability for retryable transaction conflicts and failed writes,
- compatibility tests that prove duplicate task IDs remain scoped by
  `project_id`.

SQLite remains preferred for laptops, local demos, airgap single-host work, and
small temporary drills where its file backup/restore model is simpler than a DB
service.

## Implementation Slices

FW-255 closes as an assessment. The implementation should be split only at
reviewed architecture boundaries:

- store interface extraction and adapter contract tests,
- Postgres DDL/migration generation plus disposable apply check,
- Postgres-backed store implementation for core task/state/evidence/review
  commands,
- session/checkpoint/wait/notification parity,
- backup/import/cutover rehearsal,
- dashboard/report parity and performance validation,
- deployment docs and release gate.

Each slice must preserve SQLite behavior and run the shared command parity
suite against both backends before Postgres is considered production-capable.
