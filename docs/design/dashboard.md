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
| `/reports` | Daily and date-range work reports for outcomes, CI/deploy activity, reviews, and follow-ups. |
| `/tasks/<task-id>` | Task detail page with metadata, history, evidence, sessions, reviews, and status controls. |
| `/wall` | Compatibility redirect to `/`. |

There is no dashboard version switch. `[dashboard] surface` is not part of the
active config contract; historical configs that still contain it load only
because unknown TOML keys are ignored.

## Why Web

- It can stay open on a second monitor while several agents work in parallel.
- Multiple people can watch the same local coordination state.
- Server-rendered HTML keeps the implementation small and inspectable.
- Browser affordances make filtering, sorting, export, and task drill-down
  easier than a terminal grid.

`fairway tui` covers headless basics for SSH sessions.

## Shared Read-Only Mode

Set `[dashboard] read_only = true` or start with `fairway dashboard
--read-only` when the dashboard is being shared outside the local operator
session. Read-only mode blocks dashboard mutation endpoints and hides mutation
controls; task changes still happen through the CLI from a trusted local
worktree.

For identity-aware proxy exposure, keep the Fairway origin bound to localhost
and put Cloudflare Access, Tailscale, an internal reverse proxy, or another
trusted access layer in front of it. See
[dashboard-sharing.md](dashboard-sharing.md) for the Cloudflare Tunnel +
Cloudflare Access One-Time PIN reference pattern and trust-boundary warnings.

## Flow

The dashboard is organized as a user flow, not independent pages:

1. Start at the wall (`/`) to see which lanes are active, idle, or overloaded.
2. Use `Open lane` to drill into `/board?role=<role>`.
3. Sort, search, or filter the board table.
4. Open a task from the table.
5. Use `Back` on task detail to return to the filtered board view.
6. Open Reports when the question is "what changed today, what finished, what
   failed, and what needs follow-up?"
7. Switch to diagnostics from the board when session/worktree/watcher state is
   the question.

## Wall

The wall view is intentionally compact. It answers: "What is each lane doing
right now?"

In multi-project mode, the wall groups lanes under collapsible project headers,
prefixes activity summaries with the project name, and shows a per-project
readiness rollup in the right rail.

Each role lane shows:

- current/idle state,
- representative backlog, claimed, working, review, and done task pills,
- overflow links that drill into the matching board filter,
- a lane-name toggle that opens one inline detail panel at a time. The panel
  shows queue, current working card, pending reviews, latest events, and an
  `Open full details for <role>` link to the board filtered by role.

Typed SSE events update the wall in place. Handoff events draw a short-lived arc
between source and target lanes and increment the handoff metric. Activity
events prepend verb-first ticker entries and maintain relative timestamps.
Session heartbeat events pulse the attached working task pill while fresh,
soften the pulse after one minute, and mute the pill after five minutes without
inventing synthetic heartbeat state.

Review wait SSE events are advisory updates derived from the Fairway DB review
wait projection. The dashboard may emit `review_wait.stale`,
`review_wait.notification_failed`, and `review_wait.resolved` so open clients
can refresh operator-visible state. These events are safe to lose because
Fairway DB remains authoritative and the next poll or page load reconstructs
the current wait rows.

The lane header distinguishes live provider attachments from task status. An
`active session` label means Fairway has a running session row attached to a
task. An `in_progress without session` label means the task state is active but
Fairway does not have a live provider attachment. That state is acceptable only
for short direct coordinator/orchestrator work with a fresh checkpoint and an
end-of-burst close/reset/block/handoff. For delegated, UAT, production-readiness,
tmux/Claude/Codex external, or multi-step work, register a provider session and
emit a `started` provider event so the wall can show who is actually attached.

The right rail shows gate readiness and recent activity for quick situational
awareness.

Accessibility expectations:

- interactive wall and board controls carry labels or visible text,
- board sort state is exposed on table headers with `aria-sort`,
- focus rings are visible for links, buttons, inputs, selects, textareas, and
  disclosure summaries,
- dashboard actions remain reachable by keyboard through links, forms, and
  dialogs rather than pointer-only controls.

When recent work-coverage or CI/deploy learning audits find high-risk advisory
issues, the wall shows a compact diagnostics banner with a link to the board
diagnostics tab. The banner does not change task state or merge gates; it is a
visibility surface for operators to triage uncovered commits, uncovered files,
missing evidence/review coverage, or failed CI/deploy evidence without follow-up
work.

## Board

The board is the working surface for operators.

It includes:

- a control-room header with total filtered scope,
- current orchestration recommendation and reason from the same dry-run plan
  evaluator used by `fairway coordinator plan`,
- a read-only coordination intelligence rail with open/stale wait counts,
  wake targets, last wake attempt, suggested CLI command, track-memory
  freshness, and next deterministic actions from existing coordinator and
  memory read models,
- gate readiness above the table, grouped by profile/gate group with
  blocking, advisory, and report-only misses called out,
- expandable missing-task detail for each gate so exceptions are visible before
  opening individual tasks,
- workstream progress above the table for quick backlog and completion scans.
  The board renders an actionable compact subset by default when many
  workstreams match, preserves filter/sort URL state in show-all/compact links,
  and orders active, ready, and blocked workstreams ahead of passive backlog,
- search across task ID, title, metadata, tags, status, owner, source paths,
  target paths, and review domains, with the query mirrored in URL state,
- clearable role/status/project/profile/kind/domain/risk/review-domain/tag
  filter chips,
- in multi-project mode, the same `/board` toolbar and table are used; project
  is a normal filter chip and visible/sortable/exportable table column,
- sortable task columns with URL state; shift-click adds a secondary sort key,
- a column chooser with URL-backed visibility and up/down ordering controls,
  including an optional `tags` column for cross-cutting work buckets,
- saved views for named filter, column, and sort combinations. Personal views
  are persisted to `~/.fairway/views.json`; team views are read from
  `.fairway/views.json` and are read-only in the dashboard. The first nine
  personal views are available with Cmd/Ctrl+1..9 shortcuts,
- keyboard navigation for board operators: `j`/`k` moves the row cursor,
  Enter opens task detail, `/` focuses search, `c` and `v` toggle columns and
  saved views, `x` toggles selection, `s` and `h` open status and handoff
  dialogs, `t` toggles theme, `g w` goes to the wall, `?` opens help, and Esc
  closes open menus/dialogs,
- bulk selection with CSRF-backed Claim, Hand off, Set status, and Record
  evidence dialogs; terminal status changes remain CLI-gated,
- CSV and JSON export for the current board view. Exports use the active
  filters, sort order, and visible column set, and include all filtered rows
  rather than silently truncating at the current page,
- server-side table windows above 200 filtered rows, with sort/filter applied
  before slicing, total filtered count shown separately, and URL-backed
  pagination for additional windows,
- operational health badges,
- activity kind and row-count filtering. Activity kind is applied in the store
  query before the dashboard trims visible rail rows, rather than by fetching a
  large mixed activity feed first.

The task table is the drill-down layer after the gate and workstream highlights.
It links to task detail. The board preserves role/status/project/profile/kind/
domain/risk/review/search/sort/column state when switching between task and
diagnostics tabs.

Task detail shows review waits from the same read model as
`fairway review-waits list`, including state, blocking flag, target
provider/target, expected response time, suggested action, and reason. This is
a read-only visibility surface; it does not approve reviews, send provider
wake prompts, merge, deploy, or create notification subscriptions.

Task detail also shows completion-handback projection rows and coordinator
completion-handback waits. The panel includes next owner/action, completion
state, task/live-window context, delivery state, provider target, stale age, and
suggested CLI action from the existing completion-handback/coordinator plan read
models. This includes live-window closeout or next-decision waits that do not
yet have a recorded completion handback. The dashboard remains display-only; it
does not send completion-handback wake prompts or write notification rows.

For approval-gated live operations, dashboard control-room state follows
`docs/design/live-operation-control-room.md`. The dashboard may show live-window
phase, next actor, deadline, authorization posture, stale/missed handoff state,
provider-surface capability readiness, retired/no-go surfaces, replacement
surface requirements, command/prompt, missed-deadline behavior, and suggested
commands derived from existing Fairway read models. It must remain read-only:
provider prompting belongs to coordinator/watch/provider adapters, and live
execution belongs to the accountable operator surface after explicit
authorization. The dashboard must not authorize execution, mutate production,
or send provider messages.

## Diagnostics

`/board?tab=diagnostics` shows operational tables for:

- coordination intelligence waits, wake targets, track memory freshness, and
  deterministic next actions,
- active reconciliation findings,
- work coverage and CI/deploy learning findings,
- sessions,
- worktrees,
- watchers,
- checkpoints.

The coverage section is backed by the same advisory audit read models as
`fairway audit work-coverage` and `fairway audit ci-learning`. It summarizes
uncovered commits, uncovered changed files, orphan evidence count, done tasks
without required evidence, missing review domains, and failed CI/deploy evidence
without follow-up tasks. Findings link to task detail when a task is known and
show the relevant commit, file, evidence artifact, reproduction command, or
suggested follow-up task command when available.

The active reconciliation section includes monitor lifecycle findings such as
`monitor_session_without_backing_proof`, which means a CI/deploy/UAT/watch
session is recorded as active but has no backing automation id, PID/tmux pane,
external run plus poll command, or fresh bounded manual checkpoint.

The coordination intelligence section is derived from existing Fairway facts:
track memory rows, coordinator plan actions, review waits, completion handbacks,
and notification metadata. It may show stale memory, open waits, notification
mapping gaps, wake target provider/thread, last wake attempt, and fixed CLI
recovery suggestions. It does not add dashboard wake authority or write
notification, review, task, merge, deploy, or execution state.

Diagnostics tables are sortable. The task-table export action is hidden on the
diagnostics tab because there is no task table there.

## Reports

`/reports` is the retrospective surface for coordinators and operators. It is
not a raw task table. It answers:

- what work moved during a day or date range,
- what finished versus what only closed monitor/deploy-run bookkeeping,
- which lanes produced product/code/docs/ops outcomes,
- which CI/deploy/UAT runs passed, failed, retried, or still need follow-up,
- which reviews, evidence rows, and handoffs changed merge readiness,
- which tasks were created as findings from CI, deploy, UAT, ops, or security
  work.

The default report opens to the local current day. A date picker and date-range
control allow yesterday, last seven days, and custom ranges. Reports preserve
filters in the URL so a coordinator can share or reopen the same view.
Report activity and review summaries use the store activity API with the report
date window applied at query time.

The page layout should be scannable:

- a compact summary band for completed tasks, moving tasks, active lanes,
  follow-ups created, CI/deploy results, and review outcomes,
- a lane outcome section grouped by role/domain/kind with short task links,
- a CI/deploy timeline that separates monitor closures from implementation
  work,
- a finding section grouped by `CI-FIX`, `CD-FIX`, `UAT-BUG`, `OPS-FIX`,
  `HARNESS-FIX`, and `DOC-FIX`,
- a review/evidence section showing newly satisfied gates and missing required
  review domains,
- a rule-pack signal section showing selected task-rule matches, tasks with
  missing rule evidence, blocking gaps, advisory gaps, and non-applicable
  rules,
- a bounded task table for drill-down, with pagination and export.

Visual density should match the board: restrained cards, clear section rhythm,
small status chips, progress bars only when they summarize a real denominator,
and no nested cards. The default view should show the highest-signal sections
above the fold before any large table.

Reports support Markdown, JSON, and CSV export for the selected date range.
Markdown export is intended for daily summaries and handoffs. JSON and CSV are
intended for audit, analysis, and external planning tools.

## Task Detail

Task detail uses the same dashboard shell with a compact detail header. It
shows:

- flow breadcrumbs back to Wall and Board,
- direct actions back to Board and Board Diagnostics,
- task status, owner, review status, and descendant rollup,
- CSRF-protected claim and non-terminal status update controls,
- metadata and source/target paths,
- task-scoped profile gate readiness, including matching evidence counts and
  missing evidence reasons from the same evaluator used by `merge-ready` and
  board gate rollups,
- applicable rule-pack checks, including rule mode, match status, required and
  missing evidence types, review domains, stop conditions, and non-applicable
  rationale,
- notes, dependencies, and acceptance checks,
- transition history,
- evidence,
- task-scoped work coverage and CI/deploy learning diagnostics,
- task-bound sessions,
- handoffs and reviews,
- missing required review domains before merge-ready.

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

The multi-project board is a read-oriented operator surface. It uses the same
template, filter query state, saved-view links, pagination, and export routes as
single-project mode. Each row carries a dashboard-only project label from the
registry entry, exposed through the `project` query parameter and Project
table/export column. Task identity, ownership, evidence, reviews, and gates
remain authoritative inside the source project store.

The multi-project wall is also read-oriented. `/` renders collapsible project
sections, each with the standard role lanes. `/projects` remains available as a
compact registry summary. `/wall` redirects to `/`.

## Security Posture

- Binds to `127.0.0.1` by default.
- If `[dashboard] listen` is changed to a non-loopback address, Fairway prints a
  startup warning.
- Mutations use CSRF tokens and write audit events.
- No external resources are loaded.
