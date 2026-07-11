# Fairway v0.1.12 AI Cloud Dashboard Performance Assessment

Date: 2026-07-11
Task: FW-315
Release: v0.1.12
Source: `e60617871f5f10ac1a4f73474854e2ba17c9a56e`

## Scope

This assessment measures the released Fairway dashboard against the real AI
Cloud platform-foundation database. It is read-only. It does not change
dashboard authority, public exposure, task state, or the consumer data set.

Instances:

- `127.0.0.1:7878`: read-only origin behind the existing Cloudflare Access
  boundary;
- `127.0.0.1:7879`: local full-access dashboard;
- both use the released
  `/Users/subash/dev/GPUasService/.fairway/bin/v0.1.12/fairway` binary and the
  same SQLite database.

The documented synthetic budget is wall and board under 200 ms, board sort p95
under 100 ms, SSE latency under 1.5 seconds, and RSS at or below the accepted
52 MiB ceiling for the 1000-task fixture. Those numbers are comparison points,
not an assumption that a larger historical data set should be hidden.

## Data Shape

| Table or file | Count or size |
|---|---:|
| task definitions / state | 1,747 / 1,747 |
| evidence | 4,976 |
| state history | 5,873 |
| checkpoints | 3,777 |
| sessions | 1,194 |
| reviews | 1,379 |
| notifications | 1,229 |
| handoffs | 245 |
| watchers | 91 |
| SQLite database | 13 MiB |
| SQLite WAL | 14 MiB |

The board readback showed 1,702 done tasks, 37 blocked tasks, 216 stale
checkpoints, and 245 handoffs older than one hour. These are data-hygiene inputs,
not a reason to dismiss product read-model defects.

## Route Measurements

Cold measurements waited longer than the two-second snapshot-cache TTL. Warm
measurements immediately repeated the same request. Values are TTFB / total in
seconds.

| Route | 7878 cold | 7878 warm | 7879 cold | 7879 warm |
|---|---:|---:|---:|---:|
| `/` | 24.274 / 24.277 | 0.001 / 0.003 | 10.700 / 10.703 | 0.001 / 0.005 |
| `/board` | 2.713 / 2.715 | 0.001 / 0.002 | 0.058 / 0.059 | 0.001 / 0.002 |
| `/board?tab=diagnostics` | 3.407 / 3.407 | 0.001 / 0.001 | 0.055 / 0.055 | 0.001 / 0.001 |
| `/reports` | 26.232 / 26.241 | 0.001 / 0.009 | 11.825 / 11.835 | 0.001 / 0.009 |
| task detail | 23.450 / 23.450 | 19.861 / 19.861 | 9.582 / 9.582 | 9.773 / 9.773 |
| diagnostics panel | 41.648 / 41.662 | 0.000 / 0.014 | 20.519 / 20.533 | 0.000 / 0.014 |

Task detail used
`/tasks/IAM-FIX-MFA-RECOVERY-MANAGE-UAT-001`, a real high-evidence AI Cloud
task. Task detail does not use the route snapshot cache, so its second request
is still slow.

The default board fast path meets the strict 200 ms budget on 7879 without an
active SSE polling load. The 7878 result is degraded by an existing connected
SSE client. The diagnostics shell is also fast on 7879; the heavy panel is not.

## Projection Breakdown

Representative no-SSE 7879 cold results:

- wall, 10.702 s: coordinator plan 9.106 s and closeout reports 1.493 s;
- reports, 11.834 s: delivery 3.980 s, rough edges 3.856 s, and report facts
  3.876 s with `task_detail_calls=1747`;
- diagnostics panel, 20.533 s: coordinator plan 10.940 s, audit diagnostics
  7.620 s, and closeout reports 1.860 s.

Representative 7878 results with one long-lived SSE client:

- wall, 24.276 s: coordinator plan 18.855 s and closeout reports 5.324 s;
- reports, 26.240 s: delivery 9.997 s, report facts 8.041 s, and rough edges
  4.749 s;
- diagnostics panel, 41.662 s: coordinator plan 19.627 s, audit diagnostics
  16.853 s, and task loading 3.381 s.

Template rendering is milliseconds, so HTML templates are not the primary
server-side bottleneck. Task-detail requests currently lack equivalent named
`dashboard_timing` blocks, which prevents attribution below the route level.

## Cache And Contention

The two-second in-process cache is effective after a successful build: warm
wall, board, report, and diagnostics responses had sub-millisecond TTFB and
small body-transfer totals. Four simultaneous cold `/reports` requests on 7879
all completed in about 10.626 seconds. Timing logs showed one cache miss and
three `status=wait` requests, confirming singleflight coalescing.

The cache does not solve the cold path, is isolated per dashboard process, and
does not cover task detail. A browser also requests `/favicon.ico`; the current
catch-all route rendered the complete wall model for that request and logged a
10.915-second wall projection.

## SSE Idle Cost

One bounded `curl /events` connection to otherwise idle 7879 reproduced the
largest runtime defect:

- dashboard CPU rose from idle to 99.3%;
- eight wall-clock seconds consumed about 8.08 CPU seconds;
- the stream emitted zero bytes because no event changed;
- CPU returned to idle after disconnect.

The long-lived client on 7878 kept that process near one full CPU core. The
current event loop polls every second and recomputes event sources plus review
waits even when no fact changes. This is a product defect, not SQLite network
latency or browser rendering.

After the route sweep, RSS was about 160 MiB for 7878 and 144 MiB for 7879,
well above the synthetic 52 MiB accepted ceiling. The larger data set, heavy
projection allocations, per-process caches, and live SSE work all contribute;
the assessment does not assign the entire delta to one cause.

## Browser Usability

The in-app browser confirmed:

- a cached/full-dashboard `/board` became usable in about 171 ms and correctly
  displayed 1,747 tasks with diagnostics marked deferred;
- `/board?tab=diagnostics` returned a usable shell in about 151 ms, but the
  heavy diagnostics content remained loading until roughly 20-25 seconds;
- `/reports` exceeded the browser navigation wait threshold of 10 seconds and
  only became usable afterward;
- once loaded, board, diagnostics, and reports content rendered coherently.

The progressive diagnostics shell is a useful improvement from FW-254, but a
20-second all-or-nothing panel still leaves the operator without incremental
diagnostic results. Warm-cache measurements should not be treated as first-use
usability.

## Findings And Follow-Ups

Product defects:

- FW-316: bound SSE polling and idle CPU;
- FW-317: remove coordinator/closeout work from wall first response;
- FW-318: batch delivery reports and eliminate full task-detail loops;
- FW-319: split diagnostics into independently bounded progressive panels;
- FW-320: instrument and optimize task detail;
- FW-321: prevent static and unknown routes from running wall projections.

Data/archive hygiene:

- FW-322: define a reversible archive, retention, checkpoint, and WAL hygiene
  packet without deleting evidence or using cleanup to close product defects.

Deployment/runtime behavior:

- FW-323: benchmark two dashboard processes, live clients, cache isolation,
  WAL state, CPU, and RSS after FW-316, without switching stores or changing
  exposure or dashboard authority.

No implementation is included in FW-315. The follow-ups carry their own review,
tests, and authority boundaries.
