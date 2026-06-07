# Multi-project mode

Fairway's project boundary equals the git repo boundary. Each project has its own `.fairway/state.db`. Multi-project aggregation is opt-in via a registry — there is no single shared database.

## Why per-project DBs

- The DB moves with the repo. Clone, archive, or `rm -rf` and state stays consistent with the working tree.
- Schema migrations affect one project at a time.
- A project's data is portable — copy `.fairway/` to share state, or commit it for fully reproducible team setups.

Every row in every table carries a `project_id` column (see [schema.md](schema.md)) — in v1 SQLite each DB has a single value, set from `[fairway] project_name` at DB open. This makes the schema portable to a future shared backend (Postgres) without a row-rewrite migration.

## The registry

`~/.fairway/registry.toml` lists known projects:

```toml
[[project]]
name = "gpuaas"
path = "/path/to/GPUasService"
# db_path optional; defaults to <path>/.fairway/state.db

[[project]]
name = "fairway"
path = "/path/to/fairway"
```

The registry is the only file outside of a project's own directory that fairway writes. Plain TOML; editing by hand is fine.

## Commands

- `fairway register [--name <n>]` — add the current project. Name defaults to `[fairway] project_name` or the repo basename. Idempotent.
- `fairway unregister [<name>]` — remove. Defaults to the current project.
- `fairway projects` — list registered projects with last-activity timestamp.

## Project name

Each project carries a `project_name`, used to label rows in the multi-project dashboard:

```toml
[fairway]
project_name = "gpuaas"      # default: basename of the repo root
```

Names must be unique across the registry. `fairway register` refuses duplicates and suggests a suffix.

## Dashboard modes

`fairway dashboard` runs **single-project** mode against the current repo's DB. This is the common case.

`fairway dashboard --multi` runs **multi-project** mode:

- Opens the registry.
- `ATTACH DATABASE` each registered DB into a single SQLite session.
- Read views `UNION ALL` across attached DBs. Each row already carries `project_id` — no need to project it from config.
- The wall lanes strip groups lanes under project headers.
- `/board` exposes the project selector as URL-backed project filter state and
  an active project filter chip.
- The activity feed prefixes entries with `[project] `.

Single-project and multi-project queries share the same Go view layer; the multi-project path swaps the data source.

## Why per-DB instead of one shared local DB

`project_id` is in the schema, so a single shared local SQLite would be technically possible. We choose per-DB anyway because:

- Schema migrations stay per-project; one project's upgrade does not block another's.
- The DB moves with the repo (clone, archive, `rm -rf` are all consistent).
- Backup is per-project; you can hand a colleague one `.fairway/` directory without leaking other projects.
- Bug blast radius is bounded — a corrupt DB takes down one project, not all of them.

The shared-DB story is for a *server-hosted* (Postgres) future, not a shared local SQLite file.

## Path to a shared server

When (if) fairway grows a Postgres adapter:

- One Postgres DB holds many `project_id` values.
- Existing local SQLite DBs are imported with `fairway export | fairway import --to postgres://...` — rows carry their `project_id` intact.
- The CLI's `[fairway] backend = "postgres://..."` setting switches the store driver. No schema change, no row rewrite.

## Performance

A registry of 20 projects with 1000 tasks each (~20MB of attached state, 20k task rows) is well within multi-project dashboard budgets. Beyond that, multi-project mode degrades to slower first-paint but stays usable. Single-project mode is unaffected by registry size.

## Limits

- Multi-project mode is read-only. Mutations always target one project's DB via the CLI inside that project's directory.
- The registry does not enforce that all attached DBs share a schema version; mismatched versions are flagged in the dashboard header so the user can run `fairway db migrate` per project.
