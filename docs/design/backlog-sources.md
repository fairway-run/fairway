# Backlog Sources

Fairway separates immutable planning inputs from mutable execution state. This
page defines where work lives and how backlog files move between active,
example, and archive roles.

## Source Of Truth

The active project config decides the active queue file:

```toml
[queue]
source = "yaml:docs/roadmap/fairway-product-backlog.yaml"
```

For the Fairway repository, the current product backlog source is:

```text
docs/roadmap/fairway-product-backlog.yaml
```

The Fairway DB remains the mutable runtime state for claims, task status,
evidence, handoffs, reviews, sessions, watchers, checkpoints, batches, usage,
and tracker links.

## Current Roles

| Path | Role |
|---|---|
| `docs/roadmap/fairway-product-backlog.yaml` | Active product backlog imported by the local Fairway config. |
| `examples/fairway-adoption-improvements.yaml` | Example/source material for adoption demos and historical task ideas. |
| `docs/archive/dashboard-redesign-backlog.yaml` | Historical dashboard redesign queue; not active. |
| `.fairway/*.db` | Mutable execution state; do not edit by hand. |
| `tmp-ux/*.md` | Local untracked working memory for active provider sessions. |

## Promotion Rules

Promote an example or archived task into the active backlog only when:

- it still matches the current product direction,
- its acceptance checks are concrete enough to review,
- its `source_paths`, `target_paths`, `review_domains`, and `risk_level` are
  accurate,
- the active Fairway config points at the file that will contain it,
- the task is imported or reconciled into the DB after the file update.

Do not update an archived or example queue and assume the dashboard will change.
The dashboard reads the active config and DB state, not every YAML file in the
repo.

## Archive Rules

Archive a backlog file when:

- its work is complete,
- its remaining tasks were promoted elsewhere,
- it describes a superseded design path,
- it exists only for provenance.

Archived queues should live under `docs/archive/` and have an accompanying note
explaining the active replacement, if any.

## Memory File Rules

Provider sessions may keep a local working memory file for long-running tracks.
Use `tmp-ux/` for those files.

Memory files should record:

- current objective,
- ordered task list,
- active task,
- last completed task and commit,
- validation commands,
- required reviews,
- next action after CI, review, wait, or provider handback.

These files are local execution aids. They should not be committed unless a
specific file is intentionally converted into a public assessment or runbook.
