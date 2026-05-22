# Postgres Adapter

SQLite is fairway's v1 default because most users coordinate agents on one
machine. GPUaaS still proved that shared, multi-machine orchestration eventually
needs a network-reachable queue store. Fairway should preserve that path without
making operators learn a second workflow.

## Status

Planned v2 adapter. The v1 schema is designed to be portable by carrying
`project_id` on every table and keeping mutable execution state behind the store
layer.

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

## Transactions

State-changing commands remain single-transaction operations. In Postgres,
current-state transitions should lock the target task row with `SELECT ... FOR
UPDATE` before validating dependencies, role ownership, and state-machine
guards. This prevents two coordinators from claiming or completing the same task
at the same time.

## Source Of Truth

Postgres does not change the ownership boundary:

- task imports own stable task definitions,
- fairway DB rows own mutable status, owner, evidence, reviews, sessions,
  checkpoints, and timing facts,
- imports never continuously overwrite execution state.

## Compatibility Harness

```bash
fairway db compat --backend postgres [--print-ddl | --apply-ddl]
```

The harness is for adapter development and disposable experiments. DDL printed
by the harness is a sketch until a real migration path and cutover runbook
exist.
