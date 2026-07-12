# Backlog Sources

Fairway separates immutable planning inputs from mutable execution state. This
page defines where work lives and how backlog files move between active,
example, and archive roles.

## Source Of Truth

The active project config decides which backlog file is active:

```toml
[fairway]
queue_source = "yaml:docs/roadmap/fairway-product-backlog.yaml"
```

For the Fairway repository, the current product backlog source is:

```text
docs/roadmap/fairway-product-backlog.yaml
```

That file is the durable backlog definition: task ids, titles, ownership,
metadata, dependencies, acceptance checks, and required review domains. The
Fairway DB remains the mutable runtime state for claims, task status, evidence,
handoffs, reviews, sessions, watchers, checkpoints, batches, usage, and tracker
links.

The dashboard and CLI read runtime state from the DB. Updating a YAML file does
not change live task state until the task is imported or reconciled through
Fairway commands.

## DB Visibility And YAML Search Misses

Fairway reviewers must verify task existence through Fairway commands before
using text search in a configured YAML queue as evidence that a task is
missing. A task can be present in the DB and valid runtime state even when the
configured `queue_source` file has not yet been exported or refreshed. That is
not, by itself, an implementation blocker.

Use this order when reviewing or coordinating a task id:

```bash
fairway task-detail <task-id>
fairway list --status todo --status in_progress --status blocked --status done
fairway reconcile active --dry-run
fairway db export .fairway/fairway-state-snapshot.json
```

`task-detail` is the first source for task definition metadata, status,
evidence, handoffs, notifications, reviews, sessions, and batches. `list` and
`ready` answer queue questions from the DB. `reconcile active --dry-run`
reports runtime inconsistencies that need operator action. `db export` provides
a review artifact when a reviewer needs a durable snapshot of DB-visible tasks.

A YAML text-search miss is actionable only when it points to real definition or
import drift, for example:

- the active config points at the wrong `queue_source`,
- a task should be promoted into the active backlog definition but only exists
  in chat, an example file, or an archived queue,
- a task definition changed in YAML but was never imported or reconciled,
- a reviewer needs a portable queue artifact and no DB export or task detail
  artifact was provided.

When a DB-visible task should also be represented in the configured YAML queue,
record that as a follow-up or governance/export task. Do not block an otherwise
reviewable implementation solely because `rg <task-id> <queue-source.yaml>`
does not find the id.

## Cross-Program Mirror Tasks

Product gaps may be discovered while Fairway is being used by another program,
such as a consumer stabilization program. In that case, the program backlog can carry a
mirror or dependency task explaining why the program needs the Fairway
capability, but Fairway product implementation still belongs in the Fairway
active backlog with an `FW-*` id.

Use this boundary:

- the program backlog records impact, dependency, and coordination evidence;
- the Fairway backlog records product implementation scope, reviews, merge
  readiness, commits, and release state;
- the program task links to the Fairway task or commit as dependency evidence;
- Fairway repo closeout uses Fairway repo validation and Fairway repo git state,
  not the program worktree's dirty files or merge-ready checks.

Do not use a program-specific Fairway config as the code merge gate for Fairway
repo implementation. If cross-program visibility is useful, record evidence in
both places, but keep the implementation authority on the Fairway-owned `FW-*`
task.

## Current Roles

| Path | Role |
|---|---|
| `.fairway/config.toml` | Selects the active backlog file through `[fairway].queue_source`. |
| `docs/roadmap/fairway-product-backlog.yaml` | Active Fairway product backlog definition. |
| `examples/fairway-adoption-improvements.yaml` | Example/source material for adoption demos and candidate task ideas. |
| `docs/archive/dashboard-redesign-backlog.yaml` | Historical dashboard redesign queue retained for provenance; not active. |
| `.fairway/*.db` | Mutable execution state; do not edit by hand. |
| `tmp-ux/*.md` | Local untracked working memory for active provider sessions. |

## Promotion Rules

Promote an example, archive, assessment, or chat-derived task into the active
backlog only when:

- it still matches the current product direction,
- its acceptance checks are concrete enough to review,
- its `source_paths`, `target_paths`, `review_domains`, and `risk_level` are
  accurate,
- the active Fairway config points at the file that will contain it,
- dependencies are valid in the active backlog,
- any already-completed implementation has evidence or a reconciliation note,
- the task is imported or reconciled into the DB after the file update.

Do not update an archived or example queue and assume the dashboard will change.
The dashboard reads the active config and DB state, not every YAML file in the
repo.

Recommended promotion flow:

1. Copy or rewrite the candidate task into
   `docs/roadmap/fairway-product-backlog.yaml`.
2. Normalize metadata for the active product profile.
3. Run `fairway config validate`.
4. Import or reconcile the active backlog into the live DB.
5. Record evidence for any task marked done by reconciliation rather than fresh
   implementation.
6. Leave the original example/archive file unchanged unless the task was
   intentionally removed as source material.

Do not use examples or archived queues as the active implementation queue. They
can inform new tasks, but the active backlog file and DB state are the only
coordination authority.

## Runtime State Rules

The backlog file and DB have different responsibilities:

- edit the active backlog file when task definition metadata changes,
- use Fairway commands when execution state changes,
- use `record evidence`, `record review`, `checkpoint record`, and session
  commands for runtime facts,
- avoid direct DB edits except through migrations or explicit recovery work,
- reconcile imported tasks before claiming new work after a queue switch.

If a task exists in the active backlog but not in the DB, import it before
claiming. If a task exists in the DB but no longer belongs in the active backlog,
use reconciliation or archival notes; do not silently delete history.

## Archive Rules

Archive a backlog file when:

- its work is complete,
- its remaining tasks were promoted elsewhere,
- it describes a superseded design path,
- it exists only for provenance.

Archived queues should live under `docs/archive/` and have an accompanying note
explaining the active replacement, if any.

Archived queues must not be referenced by `.fairway/config.toml` after they are
retired. If an archived task becomes relevant again, promote a fresh task into
the active backlog instead of editing the archived file in place.

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
