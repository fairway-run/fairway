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

## FWRD-162 RSS Decision

FWRD-162 re-ran the same 1000-task fixture at:

```text
/private/tmp/fairway-perf-v162-N7Fp6k
```

Measured results:

| Check | Budget | Measured | Result |
|---|---:|---:|---|
| Wall first response/total proxy | < 200 ms | 126 ms total | Pass |
| Board first response/total proxy | < 200 ms | 44 ms total | Pass |
| Board sorted response/total proxy | p95 < 100 ms | 46 ms p95 total | Pass |
| Diagnostics response/total proxy | Informational | 227 ms total | Watch |
| JSON export response/total | Informational | 8 ms total | Pass |
| Dashboard RSS | Original target: < 50 MB | 51,424 KiB / 50.22 MiB | Accepted exception |

Accepted exception:

- The dashboard v2 1000-task RSS budget is accepted at `<=52 MiB` for the
  local single-project operator dashboard on macOS ARM64 with the standard Go
  build, embedded dashboard assets, SQLite store, and explicit diagnostics-tab
  audit execution.
- The measured 50.22 MiB RSS is 0.44% above a strict 50 MiB binary threshold
  and 2.8% below the accepted 52 MiB ceiling.
- The latency budgets now pass with margin, and further RSS reduction would
  require broader read-model or runtime tuning that is not necessary before
  retiring the legacy dashboard surface.
- Multi-project dashboards, long-lived dashboard processes with many repeated
  diagnostics runs, and significantly larger task sets still need separate
  measurement before they inherit this exception.

Release implication: FWRD-160 may proceed after FWRD-162 receives the required
ops and architecture reviews. The release note should mention that the v2
dashboard budget is `<=52 MiB RSS` for the documented 1000-task local operator
fixture, not a general memory guarantee for every deployment shape.

## Interpretation

The dashboard is usable for current Fairway dogfooding, and the board path now
meets the 1000-task response/sort budget after FWRD-161.

The remaining process RSS question is resolved as a documented FWRD-162
exception. Exporting the full filtered JSON view remains fast, and diagnostics
latency is acceptable as an explicit diagnostics-tab cost rather than a normal
board render cost.

The SSE budget passes with the current one-second polling model.

## Blocking Gaps

Before using the 1000-task budget as complete release evidence, Fairway needs
reviews on the FWRD-162 RSS exception.

Recommended follow-up:

- record exact browser paint timing from a surface that exposes the Paint
  Timing API before final dashboard-v2 release signoff.

## Decision

Go/no-go: go for FWRD-160 after FWRD-162 receives ops and architecture review.

Owner: ops.

Next action: record FWRD-162 ops/architecture reviews, then allow FWRD-160 to
remove legacy dashboard compatibility if no walkthrough blocker is raised.
