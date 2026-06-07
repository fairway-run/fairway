# Dashboard Performance Reconciliation

Date: 2026-06-07
Task: FW-138
Owner: ops

## Scope

Reconcile the remaining blocked dashboard performance tasks after dashboard v2
retirement work:

- `FWRD-129` board virtualization at the 200-row threshold,
- `FWRD-151` original performance budget verification.

This pass does not invent a new benchmark harness. It reconciles the blocked
state against the already-recorded FWRD-161 and FWRD-162 evidence in
[`dashboard-performance-budget.md`](dashboard-performance-budget.md).

## Evidence Reviewed

| Evidence | Result |
|---|---|
| FWRD-129 client-side windowing remeasurement | Failed wall/board/sort/RSS because the server still emitted the full 1000-row payload. |
| FWRD-161 server-side board windowing remeasurement | Passed wall, board, and sort response budgets; RSS remained above the original strict target. |
| FWRD-162 RSS decision | Accepted a narrow `<=52 MiB` RSS exception for the documented 1000-task local single-project macOS ARM64 dashboard fixture. |
| Dashboard closeout | Retired the dashboard redesign queue while leaving FWRD-129/FWRD-151 as intentional historical/deferred blocked evidence. |

## Reconciliation Decision

`FWRD-129` remains `blocked`.

Reason: its original acceptance required the client-side virtualization slice to
meet the 1000-task first-paint/sort budget. It did not. The operational board
latency problem was later resolved by `FWRD-161` through server-side board
windows, so `FWRD-129` is retained as historical evidence of a failed approach
rather than reopened or marked done.

`FWRD-151` remains `blocked`.

Reason: the original strict performance budget failed. Later work passed the
latency budgets and accepted a bounded RSS exception through `FWRD-162`, but
that does not make the original strict verification task true as written.
`FWRD-151` stays blocked as historical evidence of the initial failed budget.

## Current Release Posture

Dashboard v2 is acceptable for the documented local single-project operator
fixture because:

- wall response budget passed after server-side windowing,
- board response budget passed after server-side windowing,
- board sort p95 budget passed after server-side windowing,
- SSE latency passed with the current polling model,
- RSS has an explicit accepted exception at `<=52 MiB` for the documented
  fixture.

This is not a blanket performance guarantee for multi-project dashboards,
larger task sets, long-lived dashboard processes, or repeated diagnostics runs.
Those need separate measurement before inheriting the exception.

## Remaining Follow-Up

- Record exact browser paint timing from an automation surface that exposes the
  Paint Timing API before a formal dashboard-v2 release signoff.
- Create a separate task if multi-project or larger-than-1000-task dashboard
  measurements become release criteria.
- Keep `FWRD-129` and `FWRD-151` blocked unless their original acceptance is
  explicitly rewritten or waived.
