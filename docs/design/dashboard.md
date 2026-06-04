# Dashboard

Fairway ships with a local web dashboard from v0.1. It is the primary surface for observing what your lanes are doing.

## Why web (not TUI)

- You want it open on a second monitor while several agents work in parallel.
- Multiple watchers can connect (you + a teammate looking over your shoulder).
- Server-sent events give live updates with no client framework.
- Richer affordances for filtering and drill-down than a TUI grid.

`fairway tui` covers the same headless basics for SSH sessions.

## Mutation Scope

The CLI remains the complete write surface. The dashboard has a small audited
mutation surface for day-to-day coordination:

- claim a task,
- move a task among non-terminal states.

Every dashboard write uses a CSRF token and records an audit event. Terminal
status changes still go through the CLI so evidence, handoff, review, and
profile gates cannot be bypassed from the browser.

## Stack

- Go `net/http` for the server.
- `html/template` for server-rendered HTML.
- HTMX for partial updates.
- Server-Sent Events for live push to the activity feed.
- Embedded templates under `internal/dashboard/assets/templates`.
- `internal/dashboard/assets/css` and `internal/dashboard/assets/js` are
  embedded scaffolds for dashboard-v2 work. The current v1 pages keep their
  existing inline styles/scripts until the visual redesign tasks extract them.
- No build step, no node_modules.
- All dashboard assets are embedded via `//go:embed`.

`fairway dashboard` starts the server in the foreground (default
`127.0.0.1:7878`) and opens the system browser unless `--no-open` or
`[dashboard] auto_open = false`.

For long-running local coordination, use the lifecycle subcommands:

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
agent runtime restart. `--pid-file` and `--log-file` can override those paths
when multiple dashboards are needed. Multi-project lifecycle commands use
`.fairway/dashboard-multi.pid` and `.fairway/dashboard-multi.log` by default.

## Views

### Lanes strip (header)

One card per configured role. Each card shows:

- Role name and branch.
- Current task ID + title, or "idle".
- Session heartbeat age. Green < 1 min, amber < 5 min, red older or session missing.
- Last commit short SHA + age.
- Active watcher or latest checkpoint badge when present.
- tmux pane (when present) rendered as `attach: agents:0.2`. Clickable to copy the attach command to clipboard.

### Backlog sort order

Within each column (or row, in by-role layout), tasks sort by:

1. `priority` ascending (lower rank = more urgent; NULLs last).
2. `sequence` ascending within siblings (NULLs last).
3. `created_at` ascending (older first).

This sort applies to `fairway ready` output as well.

### Backlog (main pane)

Two layouts, toggleable:

- **By state** — kanban columns matching `[states] allowed`. Cards are click-to-detail; no drag in v0.1.
- **By role** — rows are roles, columns are states. Better for spotting role imbalance.

Filter chips above:
- Role (one or more).
- State (one or more).
- Kind (when `[task_kinds]` is configured).
- Profile and workstream kind when task metadata is present.
- Owning domain, risk level, and review domain when task metadata is present.
- Priority (min rank, e.g. "P1 and above").
- In epic (search by epic ID or title).
- "Has unrouted review" — tasks where a review verdict needs route resolution.
- "Stale" — task in `in_progress` with owner session heartbeat older than 2h.

### Gate readiness

When workstream profiles define gates, the dashboard evaluates them across all
matching tasks and renders a live readiness panel. Gates are first rolled up by
profile and gate group, then expanded into detailed gate rows. The default
screen is exception-first: groups with missing blocking evidence appear before
advisory/report-only misses, no-task groups, and fully satisfied groups. This
keeps large readiness portfolios readable when a track has dozens of gates.

Each gate group shows:

- profile and group label,
- gate count,
- satisfied / total task-check count,
- blocking, advisory, and report-only missing counts,
- collapsed gate details, opened automatically when the group has misses.

Each detailed gate row shows:

- profile and gate name,
- group,
- mode and evidence type,
- satisfied / total task count,
- missing task links with the first matching-evidence reason.

This is the operator-facing view of the same evidence contract used by
`fairway readiness report` and `fairway merge-ready`.

### Activity feed (right rail)

Reverse-chronological merge of `task_state_history`, `task_handoffs`,
`task_evidence`, `task_reviews`. The feed is filterable by activity kind and
bounded by a configurable row limit so busy tracks do not turn into an
unbounded scroll.

Each entry is one line: `15:42  T-042  in_progress -> done  by backend`.

### Data-volume controls

The dashboard treats task and activity data as operational tables, not static
pages. Main task tables support a configurable row limit, and the activity feed
supports kind and row-count filters. The intent is to keep the page fast and
scan-friendly once a track has hundreds of tasks or many evidence rows.

### Health badges (header)

Counters that double as filter links:
- Stale claims (heartbeat > 2h).
- Sessions without heartbeats in the last 5m.
- Tasks blocked > 24h.
- Unrouted reviews.
- Handoffs not acknowledged after 1h.
- Stale checkpoints.
- Active watchers past expected duration.

### Task detail page

Single scrollable page per task with:
- Breadcrumb to root (`E-007 / S-013 / T-042`).
- Definition (notes, acceptance checks, dependencies, kind).
- Profile-aware task metadata: profile, owning domain/layer, source/target
  paths, review domains, risk, and migration type.
- Current state.
- Full history (transitions, handoffs, evidence, reviews) merged into one timeline.
- Latest checkpoints and watcher packets.
- CSRF-protected claim and non-terminal status update controls.

### Epic page

Route `/epics/<id>` (any task with descendants):
- Descendant tree.
- Aggregate progress (X of Y descendants done).
- Bottleneck callout: descendant blocked the longest.
- Latest checkpoint state, owner, target close date, and stale flag.
- Per-kind counts when `[task_kinds]` is configured.

See [hierarchy.md](hierarchy.md) for the underlying model.

## Multi-project mode

`fairway dashboard --multi` aggregates across all registered projects via SQLite `ATTACH DATABASE`. The lanes strip groups lanes under project headers, the activity feed prefixes entries with `[project]`, and a project filter chip appears above the backlog. See [multi-project.md](multi-project.md) for the registry and command surface.

## SSE implementation

For v0.1, SSE is driven by 1Hz polling of `task_state_history` for new rows since the connection opened. Cheap because the rate of transitions is low (single-digit per minute even on a busy day) and the indices are tight. v0.2 may upgrade to SQLite `sqlite3_update_hook` for push-style notification.

## Performance budgets

- First paint < 200ms on a 1000-task DB.
- Activity feed latency < 1.5s from CLI transition to SSE delivery.
- Memory footprint < 50MB.

These are budgets, not benchmarks — they are what we measure during week 1.

## Security posture

- Binds to `127.0.0.1` by default.
- No authentication in v0.1. If a user changes `[dashboard] listen` to a non-loopback address, fairway prints a startup warning.
- Mutations use CSRF tokens and write audit events.
- No external resources loaded — HTMX and CSS are embedded in the binary.
