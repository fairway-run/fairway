# Dashboard Performance Budget Verification

Date: 2026-06-07
Owner: ops
Task: FWRD-151

## Scope

Verify the dashboard v2 performance budget against a synthetic 1000-task
Fairway project.

Budget:

- wall first paint under 200 ms,
- board first paint under 200 ms,
- board sort p95 under 100 ms,
- SSE activity latency under 1.5 s,
- dashboard process RSS under 50 MB.

## Fixture

Temporary project:

```text
/private/tmp/fairway-perf-2GwBRU
```

Fixture shape:

- 1000 imported tasks,
- roles: `backend`, `ui`, `arch`, `ops`, `governance`,
- profile: `dashboard-v2`,
- mixed task kinds and risk levels,
- git repository initialized so dashboard git checks run normally.

Dashboard command:

```bash
/private/tmp/fairway-perf-2GwBRU/fairway dashboard \
  --listen 127.0.0.1:7887 \
  --no-open
```

## Measurements

Server response measurements used `curl` against the local dashboard. Browser
paint timing could not be collected from the in-app browser automation surface
because the page `performance` API was not exposed in that evaluation context,
so the wall and board timings below should be treated as response/render proxy
measurements rather than precise paint entries.

| Check | Budget | Measured | Result |
|---|---:|---:|---|
| Wall first response/total proxy | < 200 ms | 284 / 290 ms | Fail |
| Board first response/total proxy | < 200 ms | 316 / 316 ms | Fail |
| Board sorted response/total proxy | p95 < 100 ms | 327 ms p95 total | Fail |
| Diagnostics response/total proxy | Informational | 306 / 306 ms | Watch |
| JSON export response/total | Informational | 8 / 8 ms | Pass |
| SSE evidence event latency | < 1.5 s | 748 ms | Pass |
| Dashboard RSS | < 50 MB | 53,264 KB | Fail |

## Interpretation

The dashboard is usable for current Fairway dogfooding, but the 1000-task
performance budget does not pass yet.

The failures appear to be server/read-model/rendering cost rather than export
cost. Exporting the full filtered JSON view was fast, while wall, board, and
sorted board page responses were all above budget.

The SSE budget passes with the current one-second polling model.

## Blocking Gaps

Before using the 1000-task budget as release evidence, Fairway needs a focused
performance optimization pass.

Recommended follow-up:

- keep `FWRD-129` as the board virtualization task, but do not assume it is
  sufficient by itself;
- add or use a follow-up for server-side dashboard read-model/render latency if
  `FWRD-129` does not bring wall and board first response under 200 ms;
- re-run this fixture after virtualization/read-model changes and record exact
  browser paint timing from a surface that exposes the Paint Timing API.

## Decision

Go/no-go: no-go for claiming the 1000-task performance budget is met.

Owner: ops.

Next action: keep `FWRD-151` blocked on dashboard performance budget failures
and prioritize `FWRD-129` plus any server-side read-model optimization that the
next measurement pass identifies.
