# Issue Tracker Integrations

Fairway should integrate with existing planning systems without making them the
source of truth for agent execution. Jira and Linear are first-class targets:
Jira because many teams already use it for epics, stories, bugs, and release
tracking; Linear because its issues, projects, cycles, labels, and lightweight
status model are a strong fit for fast agent-driven execution. The same
boundary should also fit GitHub Issues and similar tools.

## Principle

External issue trackers own product planning and stakeholder visibility.
Fairway owns local agent coordination: task state, owner, evidence, handoffs,
reviews, sessions, checkpoints, and merge-readiness facts.

Integration must be explicit and operator-driven by default. No background sync
should overwrite DB-owned execution state.

## Planned Commands

```bash
fairway tracker configure jira --url <url> --project <key>
fairway tracker import jira --query <jql> [--parent <task-id>] [--dry-run]
fairway tracker configure linear --workspace <name> --team <key>
fairway tracker import linear --filter <text> [--parent <task-id>] [--dry-run]
fairway tracker link <task-id> --external <tracker-key-or-url>
fairway tracker export-status <task-id> [--dry-run]
fairway tracker reconcile [--dry-run]
```

The initial adapter can live behind a generic tracker interface:

| Fairway concept | Tracker mapping |
|---|---|
| `task_definitions.id` | Local ID; external key stored as metadata/link. |
| `title` | Issue summary/title. |
| `notes` | Issue description plus selected custom fields. |
| `kind` | Issue type, normalized by config. |
| `parent_id` | Epic/story/subtask relationship when supported. |
| `priority` | Configured priority mapping. |
| `dependencies` | Issue links such as blocks/depends-on. |
| `acceptance_checks` | Acceptance criteria field, checklist, or labels. |

Mutable fairway state should not be mirrored blindly. A status export should
write a concise comment or transition only when explicitly requested.

## Jira Adapter

Jira support should start with:

- one-way import from JQL into fairway task definitions,
- link storage between fairway task IDs and Jira issue keys,
- optional status/comment export for task completion, blockers, and evidence,
- config-driven mapping for project, issue types, priorities, labels, and custom
  fields,
- dry-run output before any remote write.

Jira credentials should come from the environment or OS credential store, not
from `.fairway/config.toml`.

## Linear Adapter

Linear support should start alongside Jira, not as a distant follow-up:

- one-way import from Linear team/project/cycle filters into fairway task
  definitions,
- link storage between fairway task IDs and Linear issue identifiers/URLs,
- mapping Linear projects or initiatives to fairway epics/stories,
- mapping Linear cycles to optional fairway import labels or metadata, not to
  required execution windows,
- optional status/comment export for task completion, blockers, evidence, and
  review outcomes,
- config-driven mapping for teams, states, priorities, labels, projects, and
  custom fields,
- dry-run output before any remote write.

Linear is especially useful for tracking discovered follow-up work. `fairway
spawn --sibling` or `fairway spawn --child` should be able to link a new local
task to a new or existing Linear issue when the operator asks for it, while
keeping the fairway DB authoritative for who is working, what evidence exists,
and whether the task is merge-ready.

Linear credentials should come from the environment or OS credential store, not
from `.fairway/config.toml`.

## Non-Goals

- Fairway does not become a replacement Jira UI.
- Fairway does not become a replacement Linear UI.
- Fairway does not continuously sync status in the background.
- Fairway does not require Jira, Linear, or any tracker for core workflow.
- Fairway does not treat tracker workflow states as the local state machine
  unless the operator intentionally maps them.
