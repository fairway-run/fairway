# Detailed Design Backlog Backfill - 2026-06-18

Task: `FW-217`

Purpose: map reusable Fairway product capabilities described in the
coordination-intelligence, review-wait, provider-notification, coordinator-loop,
live-operation control-room, and dashboard-sharing docs to durable backlog task
records.

## Inputs

Docs scanned:

- `docs/design/review-wait-notification-model.md`
- `docs/design/provider-notifications.md`
- `docs/design/coordinator-loop.md`
- `docs/design/coordination-intelligence.md`
- `docs/design/live-operation-control-room.md`
- `docs/design/dashboard-sharing.md`
- `tmp-ux/fairway-coordination-intelligence-2026-06-13.md`

Commands:

```bash
fairway audit docs-backlog \
  --doc docs/design/review-wait-notification-model.md \
  --doc docs/design/provider-notifications.md \
  --doc docs/design/coordinator-loop.md \
  --doc docs/design/coordination-intelligence.md \
  --doc docs/design/live-operation-control-room.md \
  --doc docs/design/dashboard-sharing.md \
  --doc tmp-ux/fairway-coordination-intelligence-2026-06-13.md

rg -n "future|later|defer|deferred|not implemented|out of scope|should|may add|needs|missing|gap|next phase|phase [0-9]|TODO|FIXME" \
  docs/design/review-wait-notification-model.md \
  docs/design/provider-notifications.md \
  docs/design/coordinator-loop.md \
  docs/design/coordination-intelligence.md \
  docs/design/live-operation-control-room.md \
  docs/design/dashboard-sharing.md \
  tmp-ux/fairway-coordination-intelligence-2026-06-13.md
```

## Existing Coverage

The core design stack is already represented by durable tasks:

- Review waits, dashboard projection, watcher, and blocked-task wake guard:
  `FW-179` through `FW-184`.
- Active evidence, live-window phases, completion handbacks, stale wakes,
  dashboard projection, and idle metrics: `FW-185` through `FW-191`.
- Live-operation control-room model: `FW-194`.
- Coordination intelligence model, track memory, generic waits, bounded wakes,
  failure routing, retry packets, advisory guards, and dashboard projection:
  `FW-195` through `FW-202`.
- Provider notification lifecycle, completion handback cleanup, wake
  routability, live-operation retry budget, docs-backlog audit, consumer
  critical-flow template, review profiles, delivery overhead, and automation
  candidates: `FW-203` through `FW-211`.
- Dashboard share naming/release/audit cleanup: `FW-212` through `FW-216`.
- Release and dashboard lifecycle work: `FW-218` through `FW-220`.

## New Backlog Rows

The scan found four reusable Fairway product gaps that were still described
only as future or suggested capabilities:

- `FW-221` - Add durable generic wait add and ack commands.
- `FW-222` - Add advisory provider adapter configuration.
- `FW-223` - Add optional external notifier interface.
- `FW-224` - Add trusted proxy identity verification model.

These rows are intentionally scoped as product primitives, not GPUaaS-specific
work. They preserve the existing boundaries:

- no dashboard send authority;
- no autonomous approval, merge, deploy, release, or live-operation authority;
- no mandatory Slack/email/Teams dependency;
- no provider credential storage;
- no trust in identity headers unless verification and origin reachability are
  explicitly configured.

## Residual Findings

`fairway audit docs-backlog` still reports `FW-181` as a stale completed task
when only the coordination-intelligence docs are scanned. Including its actual
source docs, such as `docs/design/backlog-sources.md`, `docs/agent-guide.md`,
and `docs/governance/review-guards.md`, clears that finding. This is a scan
scope issue, not a missing design-backlog task.

No GPUaaS-side backlog task is required from this backfill.
