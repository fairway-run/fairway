# Coordination Notification Backlog Audit - 2026-06-17

Task: `FW-214`

Purpose: verify that Fairway-owned review-wait, provider-notification,
coordinator-loop, live-operation, dashboard-share, and coordination-intelligence
gaps are represented in the Fairway backlog instead of remaining as consumer
project notes.

## Inputs

Docs scanned:

- `docs/design/review-wait-notification-model.md`
- `docs/design/provider-notifications.md`
- `docs/design/coordinator-loop.md`
- `docs/design/coordination-intelligence.md`
- `docs/design/live-operation-control-room.md`
- `docs/design/dashboard-sharing.md`
- `docs/design/dashboard-share-hostname-release.md`

Backlog range checked:

- Review-wait and provider-notification tasks: `FW-179` through `FW-184`
- Completion handback and live-operation tasks: `FW-187` through `FW-191`,
  `FW-194`
- Coordination-intelligence tasks: `FW-195` through `FW-211`
- Dashboard share hostname/release tasks: `FW-212`, `FW-213`

## Commands

```bash
fairway audit docs-backlog \
  --doc docs/design/review-wait-notification-model.md \
  --doc docs/design/provider-notifications.md \
  --doc docs/design/coordinator-loop.md \
  --doc docs/design/coordination-intelligence.md \
  --doc docs/design/live-operation-control-room.md \
  --doc docs/design/dashboard-sharing.md \
  --doc docs/design/dashboard-share-hostname-release.md

rg -n "TODO|FIXME|defer|deferred|future|missing|gap|follow-up|not implemented|out of scope|should add|later" \
  docs/design/review-wait-notification-model.md \
  docs/design/provider-notifications.md \
  docs/design/coordinator-loop.md \
  docs/design/coordination-intelligence.md \
  docs/design/live-operation-control-room.md \
  docs/design/dashboard-sharing.md \
  docs/design/dashboard-share-hostname-release.md

rg -n "notification|review-wait|review wait|provider notification|completion handback|live-operation|coordination intelligence|dashboard share|hostname|AI Cloud" \
  docs/roadmap/fairway-product-backlog.yaml
```

## Findings

The docs-backlog audit reported:

- all 7 scanned docs had backlog coverage;
- 0 doc-only capabilities;
- 0 uncovered command examples;
- 4 stale completed-task documentation references;
- 1 historical consumer-project lesson mention.

The historical consumer-project mention is already generalized into Fairway
coordination and dashboard-share tasks. Product-facing new docs use AI Cloud
naming; the older consumer-specific hostname remains only as a compatibility
reference.

No new implementation primitive is missing for review waits, provider
notifications, completion handbacks, live-operation handoffs, generic waits,
dashboard read-only projection, or dashboard-share hostname planning.

## Follow-Up

Created `FW-215` to reconcile the remaining stale completed-task documentation
references for `FW-181`, `FW-184`, `FW-187`, and `FW-188`. This is a
documentation/backlog governance cleanup, not a product implementation blocker.

No GPUaaS-side task is required from this audit. DNS, Cloudflare Tunnel,
Cloudflare Access policy, and viewer migration remain deployment-owned outside
the Fairway repo.

## FW-215 Closeout

`FW-215` corrected the stale completed-task findings by:

- adding implemented references for `FW-184` near status-aware
  `review-waits wake` guidance;
- adding implemented references for `FW-187` and `FW-188` near completion
  handback delivery proof and closeout wait surfacing;
- updating runtime Fairway task metadata for `FW-184`, `FW-187`, and `FW-188`
  from missing or broad paths to exact coordination docs and command paths;
- rerunning the docs-backlog audit with the affected task source docs included.

Final audit result:

- `docs_backlog_ok: true`
- `docs_scanned=11`
- `docs_with_backlog_coverage=11`
- `doc_only_capabilities=0`
- `command_examples_uncovered=0`
- `stale_completed_tasks=0`

The remaining historical consumer-project lesson mentions are intentional
history/internal references, not new product-facing naming.
