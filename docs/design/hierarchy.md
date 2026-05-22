# Task hierarchy

Tasks form a tree via `task_definitions.parent_id`. There are no separate "epic" or "story" tables — epics, stories, tasks, bugs, and spikes are all rows in `task_definitions`, distinguished by their `kind` label and their position in the tree.

## Why one table

- New shapes (initiative → epic → story → task → subtask) cost nothing — they are just more levels.
- Aggregate progress is computed on the fly; no rollup table to keep in sync.
- The state machine applies uniformly to every node — an epic transitions to `done` the same way a leaf does.

## Schema shape

```
task_definitions
  project_id  TEXT NOT NULL
  id          TEXT NOT NULL
  parent_id   TEXT NULL          -- FK (project_id, parent_id) → task_definitions(project_id, id)
  kind        TEXT NULL          -- optional label
  ...
  PRIMARY KEY (project_id, id)
```

A NULL `parent_id` means the task is a root (often an epic, but not enforced).

Index: `(project_id, parent_id)` for descendant traversal.

## Kinds

Optional. Configure with:

```toml
[task_kinds]
allowed = ["epic", "story", "task", "bug", "spike"]
default = "task"
```

When `[task_kinds]` is absent, `kind` is free-text. When present, `kind` must be in `allowed`.

**Fairway never enforces what kinds may parent what.** An "epic" can have children of any kind, including other epics. Use your team's conventions; fairway does not constrain them.

## The `spawn` command

Solves the context-loss problem. While an agent is claimed on a task, work discovered during that task can be created with full epic context:

```bash
fairway spawn --title "race in worker.go" --kind bug
# Default: sibling of the current task (same parent).

fairway spawn --title "extract helper" --child
# Child of the current task.

fairway spawn --title "follow-up audit" --kind story --parent E-014
# Explicit parent override.
```

`spawn` inherits the current task's `priority` by default — so a P0 epic discovers P0 work. Override with `--priority <n>`.

The current task is resolved from the active session (`agent_sessions.id`). If no active session can be inferred, `spawn` requires `--parent` or `--root`.

## Returning to the epic

When `fairway set-status <task> done` transitions to a terminal state, the CLI prints the epic rollup automatically:

```
T-042 done. Epic E-007: 5/8 descendants done.
Open siblings: T-043 (ready), T-044 (blocked).
Next: fairway claim T-043
```

Opt-out via `--quiet`. The dashboard shows the same rollup live.

## Aggregate progress

Computed from descendants, never stored:

- `done_count` = descendants with `status` in `[states] terminal`.
- `total_count` = all descendants.
- `bottleneck` = descendant blocked the longest.

An epic's own `status` is independent of its descendants. You can mark an epic `done` while children remain — fairway prints a warning but does not refuse.

## Tree commands

```
fairway tree <id>                     # print the descendant tree under <id>
fairway tree <id> --depth 2           # limit depth
fairway claim --in <epic-id>          # claim next ready descendant of <epic-id>
fairway ready --in <epic-id>          # list ready descendants of <epic-id>
```

## Dashboard implications

- **Task cards** show a breadcrumb to root: `E-007 / S-013 / T-042`.
- **Epic page** (`/epics/E-007`): descendant tree, aggregate progress, bottleneck callout.
- **Filter chips**: "in epic …", "kind: epic / story / task / bug / spike".
- **Activity feed**: entries on descendant tasks are prefixed with the closest epic's ID.

## Task granularity (who decides)

**The orchestrator defines task granularity. Agents do not subdivide assigned tasks into fairway sub-tasks.**

| Kind of thing | Lives where |
|---|---|
| Assigned task | `task_definitions` (created by orchestrator) |
| Genuinely new work discovered mid-task | `task_definitions` via `fairway spawn --sibling` |
| Internal steps an agent uses to complete an assigned task | The agent's own scratch — todo file, Claude's internal task list, `WORKLOG.md` in the worktree. **Not** in fairway. |
| Visible progress markers on an assigned task | `task_evidence` rows on the same task |

### Why

If agents subdivide tasks into fairway sub-tasks:

- **Granularity sprawl.** Each task becomes a forest of 5–10 micro-rows; the dashboard becomes unreadable.
- **Rollup math breaks.** "Epic E-007: 12 of 47 done" becomes meaningless when 39 of those 47 are agent-spawned step rows.
- **Review boundaries muddle.** Reviews are at the task level; sub-steps don't carry verdicts.
- **State machine ambiguity.** "Parent is in_progress as long as any sub-step is in_progress" is a rollup rule with edge cases. Orchestrator-defined granularity keeps the explicit-status rule simple.

### Escape hatch: "this task is too big"

If an agent genuinely thinks the assigned task should have been multiple tasks, the right move is to **hand it back, not split it**:

```bash
fairway record handoff T-042 --to arch --payload "Suggest splitting: (a) data model, (b) API surface, (c) tests. The current task is ~2 days; each suggested split is ~half a day."
```

The orchestrator decides whether to split. If yes, they create the new tasks via `fairway add`. The orchestrator-defined-granularity invariant holds.

### Soft enforcement

`fairway spawn --child` prints a warning when the current task has a leaf kind (`task`, `bug`, `spike`):

```
Warning: T-042 is a leaf task (kind=task). Children of leaf tasks usually
indicate execution sub-steps, which belong in your own notes rather than
fairway. If T-099 is genuinely new work the orchestrator should see,
prefer --sibling. To suppress this warning, pass --child --force.
```

`--sibling` (the default) and `--child` from `epic` / `story` are not warned. This catches the common misuse without blocking legitimate "sub-epic discovery from within an epic."

## Replace `track_checkpoints`

The original standalone `track_checkpoints` identity model is removed. The work
it tried to represent is now covered by `kind = "epic"` / `kind = "story"`
tasks, while operating updates are recorded as append-only `task_checkpoints`
against those tasks.

If you were using track checkpoints to record durable per-track requirements,
those notes belong on the epic task's `notes` field. If you were recording
current operating state, such as active, parked, blocked, review, or target
close date, use `fairway checkpoint record`.

There is no automatic migration; week 1 has no live data to preserve.
