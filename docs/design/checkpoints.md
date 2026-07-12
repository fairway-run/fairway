# Checkpoints

Early consumer workflows used `track_checkpoints` to keep side tracks, watcher work, and
multi-day efforts from disappearing from the coordinator's working memory.
Fairway replaces separate track tables with task hierarchy, but the checkpoint
behavior is still useful.

In fairway, checkpoints attach to tasks, usually epics or stories. They are
append-only notes about operating state; they do not replace task status.

## Commands

```bash
fairway checkpoint record <task-id> --state <planned|active|awaiting_input|review|done|parked|abandoned> --summary <text> [--owner <role-or-lane>] [--target-close-by <date>] [--artifact <path-or-url>]
fairway checkpoint status [--include-done]
fairway checkpoint stale [--older-than <duration>]
```

## Schema Direction

Add `task_checkpoints`:

| Column | Notes |
|---|---|
| `project_id` | Project scope. |
| `id` | Integer primary key. |
| `task_id` | Task, story, or epic being checkpointed. |
| `state` | `planned`, `active`, `awaiting_input`, `review`, `done`, `parked`, `abandoned`. |
| `owner` | Role or lane responsible for the current checkpoint. |
| `target_close_by` | Optional date for stale-track detection. |
| `summary` | Human-readable state. |
| `artifact_path` | Optional evidence link. |
| `created_at` | Append-only timestamp. |

## Operating Rules

- Active side tracks need at least one fresh checkpoint per day.
- If no evidence appears for a day, the task should become `blocked` or
  `parked`; it should not remain vaguely active.
- `parked` means intentionally paused with enough context to resume.
- `abandoned` means intentionally stopped with a reason.
- The coordinator should review evidence and integrate, not rediscover, side
  track results.

## Relationship To Epics

An epic's explicit task status remains independent of descendants. Checkpoints
answer a different question: "what is the current operating decision for this
track?" The dashboard can show both:

- explicit status and descendant rollup from task hierarchy,
- latest checkpoint state and stale flags from `task_checkpoints`.
