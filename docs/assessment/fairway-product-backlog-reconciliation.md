# Fairway Product Backlog Reconciliation

Date: 2026-06-07

## Context

The dashboard redesign queue was retired as the active Fairway queue and
`docs/roadmap/fairway-product-backlog.yaml` was promoted as the product backlog.
The live `.fairway/state.db` still contained the dashboard task history, so the
product backlog was imported and reconciled before starting new `FW-*` work.

The import added 31 product-backlog tasks to the live Fairway DB. Several were
already implemented in recent commits and were marked `done` with reconciliation
evidence rather than reworked.

## Reconciled As Done

These tasks are implemented in code/docs/tests and were recorded as historical
reconciliation completions in Fairway state:

| Task | Evidence |
|---|---|
| `FW-107` | Dashboard ready metric uses dependency-aware readiness and has regression coverage in `internal/dashboard/server_test.go`. |
| `FW-108` | Provider-neutral tmux session/transcript bridge exists through `session upsert`, `examples/session-adapters/tmux.sh`, docs, and tests. |
| `FW-109` | Provider-neutral delegated session event adapter exists at `examples/session-adapters/provider-event.sh` with docs and tests. |
| `FW-111` | `fairway reconcile active` reports stale/unattended active work and monitor/session ambiguity through `internal/reconcile/active.go`. |
| `FW-112` | Grouped subcommand help is implemented through `subcommandUsage` and covered by CLI help alias tests. |
| `FW-113` | `fairway audit work-coverage` exists with since-ref, since-duration, task, JSON, and dry-run support. |
| `FW-114` | `fairway audit ci-learning` exists with failure classification, template output, JSON, and tests. |
| `FW-115` | Dashboard diagnostics surfaces active reconcile, work-coverage, CI-learning, monitor proof, and resume-needed findings. |
| `FW-119` | `/reports` is implemented as the daily/date-range retrospective dashboard with summaries, timeline, table, filters, and exports. |

## Left Open

These tasks remain open because the implementation is absent, partial, or needs a
fresh product decision:

| Task | Decision |
|---|---|
| `FW-101` | Prompt-file session launch path remains a concrete launcher feature, not just session metadata. |
| `FW-102` | Gate-readiness drill-down needs explicit verification against current dashboard behavior before closure. |
| `FW-103` | SQLite busy retry is still needed for burst multi-agent writes. |
| `FW-104` | Store-level activity filters are still distinct from dashboard-side filtering. |
| `FW-105` | Workstream pagination/expandable groups remain separate from current dashboard grouping. |
| `FW-106` | Repeated metadata flag preservation remains a targeted CLI backlog item. |
| `FW-110` | Provider-event adapters emit `started` checkpoints, but the separate readiness guard for active sessions missing that lifecycle event remains open. |
| `FW-130` | Orchestration controller loop stays last; the primitives should land first. |
| `FW-131` | Lane closeout guard is the next recommended implementation task. |

## Dependency Adjustment

`FW-131` no longer depends on `FW-130` in the active backlog. Lane closeout is a
workflow guard around branch, worktree, session, and task lifecycle. It can be
implemented before the broader orchestration controller loop, and should be used
to prevent `task done != lane done` coordination drift before more automation is
added.

## Next Order

1. `FW-131` lane closeout guard.
2. `FW-103` SQLite busy retry.
3. `FW-112` only if further help coverage gaps appear after reconciliation.
4. `FW-106` repeated metadata flags.
5. Session/provider lifecycle follow-ups, starting with remaining `FW-110`.
6. Dashboard/product polish after operational correctness.
7. `FW-130` orchestration controller loop last.
