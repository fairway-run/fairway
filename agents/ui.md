# Agent: ui

**Branch:** `agent/ui`
**Worktree:** `<root>/{repo}-ui`
**Provider (informational):** Codex

## Scope

The dashboard frontend:

- `internal/dashboard/assets/` — CSS, HTMX, fonts, any JS.
- `internal/dashboard/templates/` — HTML templates.
- Layout, visual hierarchy, interaction patterns of the dashboard.
- Filter chips, kanban layout, lane card design, activity feed presentation.
- Presentation of coordinator, packet, watcher, checkpoint, tracker-link, and
  session state once backend exposes read models.
- Presentation of workstream profile metadata once backend exposes it:
  profile/task-kind grouping, named profile gates, review domains, and
  guard/evidence groupings.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| HTTP handlers, data fetching | `backend` |
| New SQL queries to feed views | `backend` |
| CSP / security headers | `backend` |
| Dashboard release packaging | `ops` |
| Product meaning of tracker/coordinator/checkpoint states | `arch` / `governance` |
| Product meaning of profile gates or dashboard grouping | `arch` / `governance` |

## Standards

- [Coding standards](../docs/governance/coding-standards.md) (CSS section)
- [Testing](../docs/governance/testing.md) (golden HTML files)
- [Review guards](../docs/governance/review-guards.md)

## Constraints

- No JS frameworks. HTMX + vanilla.
- No external CDN — everything served from the binary's embedded assets.
- Single stylesheet. CSS variables for theming.
- Layout assumes ≥ 1280px viewport (mobile is not a target).
- Dark mode via `prefers-color-scheme` from day 1.

## Typical handoffs out

- New view needs data not yet exposed → handoff to `backend` with the query shape.

## Typical handoffs in

- `backend` adds a new endpoint → `ui` builds the partial template + filter chip.
- `backend` exposes coordinator/checkpoint/tracker-link read models → `ui`
  renders badges, filters, and detail sections without adding write actions.
- `backend` exposes profile/dashboard grouping metadata → `ui` renders filters
  and sections using configured labels rather than hardcoded project names.
