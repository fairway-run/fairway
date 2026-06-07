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

## FWRD-129 Remeasurement

After implementing client-side board windowing at the 200-row threshold, the
same 1000-task fixture was regenerated at:

```text
/private/tmp/fairway-perf-v129-zC9N7t
```

The board rendered with:

- `data-virtual-window="200"`,
- 1000 rows in the HTML DOM,
- 200 visible rows after client-side windowing,
- footer text `showing 1-200 of 1000 filtered tasks`.

Post-change measurements:

| Check | Budget | Measured | Result |
|---|---:|---:|---|
| Wall first response/total proxy | < 200 ms | 300 / 306 ms | Fail |
| Board first response/total proxy | < 200 ms | 292 / 301 ms | Fail |
| Board sorted response/total proxy | p95 < 100 ms | 336 ms p95 total | Fail |
| Diagnostics response/total proxy | Informational | 298 / 298 ms | Watch |
| JSON export response/total | Informational | 8 / 8 ms | Pass |
| Dashboard RSS | < 50 MB | 51,536 KB | Fail |

Conclusion: client-side row windowing improves visible DOM work but does not
meet the first-paint or sort budget because the server still computes and sends
all 1000 board rows. The next performance task needs server-side/windowed data
delivery or a leaner dashboard read model, not only client-side virtualization.

## FWRD-161 Remeasurement

After implementing server-side board windowed delivery, the same 1000-task
fixture was regenerated at:

```text
/private/tmp/fairway-perf-v161-osAMYj
```

This pass also stopped running work-coverage and CI-learning diagnostics on
normal wall/board requests; those audits now run when the board diagnostics tab
is requested.

The board rendered with:

- `data-server-window="200"`,
- 200 task rows in the HTML table for the first page,
- URL-backed pagination for additional windows,
- footer text `showing 1-200 of 1000 filtered tasks`,
- full filtered JSON export still returning all matching tasks.

Post-change measurements:

| Check | Budget | Measured | Result |
|---|---:|---:|---|
| Wall first response/total proxy | < 200 ms | 116 ms total | Pass |
| Board first response/total proxy | < 200 ms | 43 ms total | Pass |
| Board sorted response/total proxy | p95 < 100 ms | 45 ms p95 total | Pass |
| Diagnostics response/total proxy | Informational | 239 ms total | Watch |
| JSON export response/total | Informational | 8 ms total | Pass |
| Dashboard RSS | < 50 MB | 50,544 KB | Fail |

Conclusion: server-side board windowing resolves the board first-response and
sort budget failures. RSS remains slightly above the recorded budget and should
be resolved or accepted with explicit release evidence before retiring the old
dashboard surface.

## Interpretation

The dashboard is usable for current Fairway dogfooding, and the board path now
meets the 1000-task response/sort budget after FWRD-161.

The remaining open budget question is process RSS. Exporting the full filtered
JSON view remains fast, and diagnostics latency is acceptable as an explicit
diagnostics-tab cost rather than a normal board render cost.

The SSE budget passes with the current one-second polling model.

## Blocking Gaps

Before using the 1000-task budget as complete release evidence, Fairway needs a
focused RSS decision.

Recommended follow-up:

- resolve the dashboard RSS overage or document an accepted exception with
  target hardware/process assumptions;
- record exact browser paint timing from a surface that exposes the Paint
  Timing API before final dashboard-v2 release signoff.

## Decision

Go/no-go: no-go for claiming the complete 1000-task performance budget is met
until RSS is resolved or explicitly accepted.

Owner: ops.

Next action: keep `FWRD-151` blocked on the remaining RSS budget decision and
do not remove legacy dashboard compatibility until the RSS overage is resolved
or accepted.
