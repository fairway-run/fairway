# Dashboard Redesign Decision Log

Status: superseded by the single dashboard described in
[dashboard.md](../design/dashboard.md).

This document is retained as the decision log for the redesign that introduced
the wall, board, diagnostics, and v2-style task detail flows. The redesign
consolidated v1's overloaded dashboard into a single dashboard system with
purpose-built views: wall, board, diagnostics, and task detail.

## Goal

The current dashboard at `/` answers two distinct operator questions on one screen:

1. *What is the state of coordination right now?* — lane card strip + sessions table + workstream progress + activity feed.
2. *Where is the work I'm looking for?* — 9 filter dropdowns, a 42-option Domain filter, gate-readiness drill-down, per-workstream tables, per-role tables.

Both are legitimate questions. Mixing them in one screen makes both jobs worse: the coordination story is buried under tables, and the data manipulation is constrained by header layout. The redesign splits them.

| Surface | Question it answers | Visual language |
|---------|---------------------|-----------------|
| `/` (wall) | *What is the state of coordination right now?* | Swim lanes, motion, color, animated handoffs, gate gauges |
| `/board` | *Where is the work I'm looking for, and how do I act on it?* | Sortable table, column chooser, saved views, bulk actions, keyboard nav |
| `/tasks/<id>` | *What is the full history of this task?* | Unchanged from v1 |

## URL plan

| URL | v1 | v2 |
|-----|----|----|
| `/` | operator dashboard (info-dense) | wall view |
| `/board` | — | new operator surface (replaces v1 `/`) |
| `/wall` | — | redirects to `/` (so direct links work) |
| `/tasks/<id>` | task detail | unchanged |
| `/events` | SSE: generic `refresh` | SSE: typed events (see taxonomy below) |
| `/actions/claim` | dashboard claim | unchanged |
| `/actions/set-status` | dashboard status set | unchanged |
| `/partials/*` | HTMX partials | unchanged or refactored as needed |
| `/epics/<id>` | epic page | unchanged |

A header-level **view toggle** flips between wall and board. The toggle is intentional — the two answer different questions and the user opts in. There is no "hybrid" view.

## Retirement Outcome

The legacy dashboard has been retired. `/` always serves the wall view, `/board`
serves the operator board, `/board?tab=diagnostics` serves diagnostics, and
`/tasks/<id>` serves task detail. Older configs that still contain
`[dashboard] surface` are tolerated by TOML decoding but the key is ignored.

## Wall view (`/`)

**Audience:** anyone watching coordination — lead engineer, second monitor, demo viewer, GPUaaS operator scanning state.

**Layout (top to bottom):**

```
┌───────────────────────────────────────────────────────────────────────────┐
│ brand           N agents · M moving · K handoffs · J done   providers · ●●● │
├───────────────────────────────────────────────────────────────────────────┤
│ ┌─ Lane: orchestrator ───────────────────────────────────────────────────┐ │
│ │ idle                       [backlog] [claimed] [working] [review] [done] │ │
│ ├─ Lane: backend ────────────────────────────────────────────────────────┤ │
│ │ ● Claude · 12s ago         [+12]    []   [T-042●]  [T-041]   [87 ✓]   │ │
│ │   (click expands: queue · working detail · pending review · open board) │ │
│ ├─ Lane: frontend ───────────────────────────────────────────────────────┤ │
│ │ ● Codex · 4s ago           [T-051]  [T-046][T-043●][T-039]    [51 ✓]   │ │
│ └─ ... (one row per configured role) ──────────────────────────────────────┘ │
│                                                                           │
│           ──── arc overlay: animated handoffs draw between lanes ────     │
└───────────────────────────────────────────────────────────────────────────┘
```

**Right rail:** gate-readiness rings (one per gate group, fills as evidence accumulates), recent activity ticker (verb-first SSE events with provider color).

**Lane rows are sourced from `[[roles]]` in `.fairway/config.toml`.** A 3-lane config renders 3 lanes; a 7-lane config renders 7. No hardcoded role list.

**Interactions:**

- Click lane name → expand inline panel (queue list · working card with gate progress · pending review list). Click again or click another lane to swap. Only one expanded at a time.
- Click `Open full details for <role>` inside the expanded panel → navigate to `/board?role=<role>` with the role filter pre-applied. This eliminates the need for a separate per-lane modal.
- Click task pill → navigate to `/tasks/<id>`.

**Animations are tied to real events, not timed.** Arcs draw when a handoff is recorded. Ticker updates when an SSE event arrives. The pulsing dot on the working pill ties to heartbeat freshness. Nothing fakes liveness.

**Idle lanes look intentional**, not apologetic — muted role label with `○ idle · waiting` instead of hidden or empty.

## Board view (`/board`)

**Audience:** operators doing work — claiming, recording evidence, handing off, reviewing, marking done, hunting for state, exporting reports.

**Layout (top to bottom):**

1. Header (same as wall, view toggle shows `Board` active)
2. Toolbar: search · active filter chips · column chooser · saved views · export
3. Selection bar (visible only when rows selected): selection count · clear · bulk actions
4. Sortable table
5. Footer: pagination / row range / virtualized count

**Default columns:** ID · Title · Role · Status · Kind · Started · Last activity · Gates · Owner

**Toggleable columns:** Profile · Owning domain · Risk · Review domain · Created · Workstream

**Filter chips:** every applied filter is a chip with `×` to remove. Chips reflect query-string state.

**Saved views:** named filter + column + sort combos persisted to `~/.fairway/views.json`. A team-shared view file at `.fairway/views.json` is also loaded, marked "team". Views have keyboard shortcuts (`⌘1`–`⌘9`).

**URL state:** every filter, sort, column-set, page, and saved-view selection encoded in the query string. Links are shareable. Reload preserves state.

**Bulk actions:** when one or more rows are selected, a sticky action bar offers: Claim · Hand off to… · Set status… · Record evidence… (each opens a modal with the action's required fields; same CSRF + audit path as v1 `/actions/*`).

**Keyboard:**

| Key | Action |
|-----|--------|
| `j` / `k` | Move row cursor down / up |
| `↵` | Open task detail |
| `/` | Focus search |
| `c` | Open column chooser |
| `v` | Open saved views |
| `x` | Toggle row selection |
| `s` | Open status setter for cursor row |
| `h` | Open handoff for cursor row |
| `t` | Toggle theme |
| `g` then `w` | Go to wall view |
| `?` | Toggle keyboard cheatsheet |
| `esc` | Close any open menu or modal |

**Export:** CSV (default) and JSON of the current view (visible columns × applied sort × applied filters). No silent "first N rows" truncation — if a row is excluded, the count and reason are surfaced.

**Performance budget:** virtualized rendering once row count exceeds 200; otherwise full DOM. First paint < 200ms on the 1000-task case, p95 sort < 100ms.

## Shared design language

Both surfaces consume the same design tokens and component primitives.

**Tokens** (`assets/css/tokens.css`):

```css
:root {
  --bg, --panel, --panel-2, --line, --text, --muted,
  --good, --warn, --bad, --accent,
  --claude, --codex, --compass,
  /* type scale, spacing scale, radii, transitions */
}
[data-theme="light"] { /* overrides */ }
```

Both surfaces support `[data-theme="dark"]` and `[data-theme="light"]`. Default is **light** — operators run the tool 8 hours a day. Dark is a one-click opt-in (and the demo default for projection).

**Components** (`assets/css/components.css`): pill · status pill · button · card · modal · gauge ring · ticker entry · theme toggle · view toggle · dropdown menu.

Both wall.css and board.css extend the tokens and components.

## Embed migration

Templates and assets move out of `internal/dashboard/server.go` into a versioned directory tree loaded via `//go:embed`. The `internal/dashboard/assets/` directory already exists (with a `.gitkeep`) — this work uses it.

Target layout:

```
internal/dashboard/
  server.go                      Go logic (routes, data prep, SSE)
  assets/
    templates/
      layout.html                shared shell (head, header, theme toggle)
      wall.html                  extends layout
      board.html                 extends layout
      task-detail.html
      partials/
        lane-card.html
        gate-gauge.html
        activity-tick.html
        provider-chip.html
    css/
      tokens.css                 design tokens (light + dark)
      components.css             shared primitives
      wall.css                   wall-specific styles
      board.css                  board-specific styles
    js/
      common.js                  theme toggle, SSE client, helpers
      wall.js                    arc animation, lane expansion, ticker
      board.js                   sort, filter, column chooser, keyboard nav
```

The embed migration is **step one** of the implementation work and contains **no functional changes** — only the move from string literals to files. Existing dashboard regression tests cover correctness.

## SSE event taxonomy

`/events` continues to be the live update channel but emits typed events rather than a generic `refresh`. Wall view subscribes to all; board view subscribes to events relevant to its current filter.

| Event | Payload (JSON) | Fired when |
|-------|----------------|------------|
| `claim` | `{task_id, role, owner, provider?, at}` | `fairway claim` succeeds |
| `status_change` | `{task_id, role, from, to, actor, at}` | non-terminal status change (CLI or board) |
| `handoff` | `{task_id, from_role, to_role, actor, reason?, at}` | `fairway record handoff` |
| `evidence` | `{task_id, role, kind, count, at}` | `fairway record evidence` |
| `review_verdict` | `{task_id, role, verdict, reviewer_domain, at}` | review row added |
| `done` | `{task_id, role, owner, at}` | task transitions to `done` |
| `session_attach` | `{role, provider, session_id, at}` | session registers via provider-event |
| `session_heartbeat` | `{role, provider, session_id, at}` | heartbeat tick |
| `session_detach` | `{role, provider, session_id, reason, at}` | stale or completed |
| `gate_change` | `{profile, gate, satisfied, total, at}` | recompute after evidence/status change |

Backward compatibility: `refresh` is preserved for one release as a synthetic event derived from any of the above, so v1 clients keep working through the cutover.

## What v1 loses

The redesign removes ergonomics from v1 only where v2 covers them better:

- 9-filter strip → toolbar with chips + saved views + search
- 42-option Domain dropdown → searchable column filter inside the chooser
- Per-workstream tables → single sortable table filtered by `workstream`
- Per-role tables → single sortable table filtered by `role`
- Health badge row → headline metrics in wall header + dedicated saved views ("Blocked >24h", "Stale claims")
- Sessions / Worktrees / Watchers / Checkpoints tables → moved to a Diagnostics tab on `/board`
- Lane card strip → wall is the lane card strip, just rebuilt

v1 features preserved verbatim: `/tasks/<id>`, `/epics/<id>`, CSRF on mutations, audit trail on writes, terminal status changes routed through CLI gates.

## Resolved And Remaining Questions

1. **Diagnostics surface.** Resolved: sessions, worktrees, watchers, and checkpoints live at `/board?tab=diagnostics` with separate sortable tables.
2. **Multi-project mode.** v1 has `fairway dashboard --multi` with project-prefixed tables. v2 needs the same — probably as a project filter chip on `/board` and a project group header on `/wall`.
3. **Saved view sharing.** Per-user views in `~/.fairway/views.json` is uncontroversial. Team-shared views in `.fairway/views.json` (versioned with the project) is a stronger commitment. Needs an explicit go/no-go before we ship.
4. **Inline edit on the board.** Setting status from a row dropdown is in scope. Editing acceptance checks or other definition fields is not — those continue through CLI or the task detail page.
5. **Pagination vs. virtualization cutover.** Threshold of 200 rows is a starting guess. Confirm with a measurement on the 1000-task case before lock-in.
6. **Activity feed filtering.** v1 lets you filter activity by kind and row count. v2's wall ticker is intentionally narrow (last 5–6 entries, no filter). The board activity feed could grow filters — needs a call on whether to bother.

## References

- Mockups:
  - [fairway-wall-mockup.html](https://fairway.run/mockups/fairway-wall-mockup.html)
  - [fairway-board-mockup.html](https://fairway.run/mockups/fairway-board-mockup.html)
- v1 design doc: [dashboard.md](../design/dashboard.md)
- v1 multi-project: [multi-project.md](../design/multi-project.md)
- v1 hierarchy: [hierarchy.md](../design/hierarchy.md)
- Architecture change protocol: [AGENTS.md](https://github.com/fairway-run/fairway/blob/main/AGENTS.md) (operating principle 10, "Update docs with code")
- Backlog: [dashboard-redesign-backlog.yaml](dashboard-redesign-backlog.yaml)
