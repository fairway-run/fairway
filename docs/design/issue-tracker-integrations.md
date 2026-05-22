# Issue Tracker Integrations

Fairway should integrate with existing planning systems without making them the
source of truth for agent execution. Jira is the first target because many teams
already use it for epics, stories, bugs, and release tracking, but the boundary
should also fit Linear, GitHub Issues, and similar tools.

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

## Non-Goals

- Fairway does not become a replacement Jira UI.
- Fairway does not continuously sync status in the background.
- Fairway does not require Jira for any core workflow.
- Fairway does not treat Jira workflow states as the local state machine unless
  the operator intentionally maps them.
