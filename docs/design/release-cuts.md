# Release Cuts

This file is the scope lock for early implementation. If a PR adds behavior
outside the active cut, it should update this file first or split the work.

## v0.1 Ships When

### CLI

- `fairway init`
- `fairway import <yaml-or-json-path>`
- `fairway ready`
- `fairway claim <task-id>`
- `fairway set-status <task-id> <state> [--reason <text>]`
- `fairway record evidence <task-id> ...`
- `fairway record handoff <task-id> ...`
- `fairway record review <task-id> ...`
- `fairway task-detail <task-id>`
- `fairway config validate`
- `fairway dashboard [--no-open] [--listen <addr>]`
- `fairway version`

### Tables

- `schema_migrations`
- `task_definitions`
- `task_state`
- `task_state_history`
- `task_handoffs`
- `task_evidence`
- `task_reviews`

`agent_sessions` and `task_checkpoints` may exist in migrations only if keeping
the schema forward-compatible is simpler than adding them later, but no v0.1 UI
or CLI should require them.

### Dashboard

- lane/role strip with current task or idle state,
- backlog grouped by role,
- task detail page,
- activity feed from state transitions, handoffs, evidence, and reviews,
- health badges for stale in-progress tasks and unacknowledged handoffs.

### Implementation Gates

- SQLite claim race test proves one winner and one `ErrAlreadyClaimed` loser.
- Review insert updates both `task_reviews` and denormalized `task_state`
  review columns in one transaction.
- `git diff --check`, unit tests, and focused store tests pass.

## v0.2 Ships When

### CLI

- `fairway worktree setup | status | prune`
- `fairway route review <task-id>`
- `fairway merge-ready <task-id>`
- `fairway review checkout <task-id>`
- `fairway session upsert | end | status | reconcile`
- `fairway status-report | health-report | timing-report | dispatch-plan`
- `fairway git-check`
- `fairway preflight`
- `fairway coordinator preflight | status | tick`
- `fairway packet context`
- `fairway packet bugfix`
- `fairway packet watcher`
- `fairway checkpoint record | status | stale`
- `fairway regression-pack list | show | validate`

### Tables

- `agent_sessions`
- `task_checkpoints`

### Dashboard

- session status,
- review routing/merge readiness,
- latest watcher packet and checkpoint summaries,
- stale checkpoint badges,
- epic rollup view.

## v0.3 Ships When

- Dashboard mutations have CSRF, audit, and parity with CLI behavior.
- `fairway tui` covers ready/claim/status/task-detail for SSH/headless use.
- Optional tracker import/link/export prototype exists behind dry-run defaults.

## v1.0 Ships When

- Schema migration policy is stable.
- Homebrew packaging is working.
- Release artifacts are signed or checksummed.
- Postgres compatibility harness exists, even if the runtime adapter remains
  v2.
