# Dashboard

Fairway has one local web dashboard. It is the primary surface for observing
active lanes, drilling into a lane, inspecting diagnostics, and opening task
detail.

## Routes

| Route | Purpose |
|---|---|
| `/` | Wall view. High-level role lanes for live coordination. |
| `/board` | Operator board. Filterable/sortable task table, workstreams, gates, and activity. |
| `/board?tab=diagnostics` | Diagnostics tab for sessions, worktrees, watchers, and checkpoints. |
| `/tasks/<task-id>` | Task detail page with metadata, history, evidence, sessions, reviews, and status controls. |
| `/wall` | Compatibility redirect to `/`. |

There is no dashboard version switch. Older project configs may still contain
`[dashboard] surface`; Fairway ignores it and new configs should omit it.

## Why Web

- It can stay open on a second monitor while several agents work in parallel.
- Multiple people can watch the same local coordination state.
- Server-rendered HTML keeps the implementation small and inspectable.
- Browser affordances make filtering, sorting, export, and task drill-down
  easier than a terminal grid.

`fairway tui` covers headless basics for SSH sessions.

## Flow

The dashboard is organized as a user flow, not independent pages:

1. Start at the wall (`/`) to see which lanes are active, idle, or overloaded.
2. Use `Open lane` to drill into `/board?role=<role>`.
3. Sort, search, or filter the board table.
4. Open a task from the table.
5. Use `Back` on task detail to return to the filtered board view.
6. Switch to diagnostics from the board when session/worktree/watcher state is
   the question.

## Wall

The wall view is intentionally compact. It answers: "What is each lane doing
right now?"

Each role lane shows:

- current/idle state,
- representative backlog, claimed, working, review, and done task pills,
- overflow links that drill into the matching board filter,
- an `Open lane` action that opens the full board filtered by role.

The right rail shows gate readiness and recent activity for quick situational
awareness.

## Board

The board is the working surface for operators.

It includes:

- a control-room header with total filtered scope,
- gate readiness above the table, grouped by profile/gate group with
  blocking, advisory, and report-only misses called out,
- expandable missing-task detail for each gate so exceptions are visible before
  opening individual tasks,
- workstream progress above the table for quick backlog and completion scans,
- search across task ID, title, metadata, status, owner, source paths, target
  paths, and review domains,
- role/status/profile/kind/domain/risk/review-domain filter chips,
- sortable task columns,
- CSV export for the current task table,
- operational health badges,
- activity kind and row-count filtering.

The task table is the drill-down layer after the gate and workstream highlights.
It links to task detail. The board preserves role/status/search state when
switching between task and diagnostics tabs.

## Diagnostics

`/board?tab=diagnostics` shows operational tables for:

- sessions,
- worktrees,
- watchers,
- checkpoints.

Diagnostics tables are sortable. The task-table export action is hidden on the
diagnostics tab because there is no task table there.

## Task Detail

Task detail uses the same dashboard shell with a compact detail header. It
shows:

- flow breadcrumbs back to Wall and Board,
- direct actions back to Board and Board Diagnostics,
- task status, owner, review status, and descendant rollup,
- CSRF-protected claim and non-terminal status update controls,
- metadata and source/target paths,
- notes, dependencies, and acceptance checks,
- transition history,
- evidence,
- task-bound sessions,
- handoffs and reviews.

Task detail is intentionally task-scoped. Global sessions, worktrees, watchers,
and checkpoints remain in `/board?tab=diagnostics` so the detail page does not
become the old mixed dashboard.

The `Back` action returns to the referring board view when the referrer is
local. If the task is opened directly, it falls back to the board filtered by
the task role.

## Mutation Scope

The CLI remains the complete write surface. The dashboard exposes only small,
audited coordination mutations:

- claim a task,
- move a task among non-terminal states.

Every dashboard write uses a CSRF token and records an audit event. Terminal
status changes still go through the CLI so evidence, handoff, review, and
profile gates cannot be bypassed from the browser.

## Stack

- Go `net/http` for the server.
- `html/template` for server-rendered HTML.
- Server-Sent Events for live activity updates.
- Embedded templates under `internal/dashboard/assets/templates`.
- Embedded CSS and JS under `internal/dashboard/assets`.
- No build step, no node modules, no external browser resources.

`fairway dashboard` starts the server in the foreground at
`127.0.0.1:7878` by default and opens the system browser unless `--no-open` or
`[dashboard] auto_open = false` is set.

For long-running local coordination, use:

```bash
fairway dashboard start
fairway dashboard status
fairway dashboard restart
fairway dashboard stop
```

`start` runs the dashboard detached, records the child process in
`.fairway/dashboard.pid`, and appends logs to `.fairway/dashboard.log`.
Detached lifecycle commands do not open a browser unless `--open` is passed.
`restart` is the preferred recovery command after a host app, terminal, or
agent runtime restart.

## Multi-Project Mode

`fairway dashboard --multi` aggregates registered projects via SQLite
`ATTACH DATABASE`. See [multi-project.md](multi-project.md) for the registry
and command surface.

## Security Posture

- Binds to `127.0.0.1` by default.
- If `[dashboard] listen` is changed to a non-loopback address, Fairway prints a
  startup warning.
- Mutations use CSRF tokens and write audit events.
- No external resources are loaded.
