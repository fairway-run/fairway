# Fairway dashboard contention assessment

Date: 2026-07-11

Task: `FW-323`

## Scope

This assessment reran the dual-dashboard contention case after the FW-316 SSE
fix and the FW-317 through FW-321 read-model changes. It used two temporary
loopback processes built from the current source, one read-only SSE client, and
the real AI Cloud SQLite database. It did not restart shared dashboards, mutate
task state, checkpoint the WAL, switch stores, or change public exposure.

Artifact directory: `/tmp/fairway-fw323-contention`

## Provenance

- binary: `/tmp/fairway-fw323`
- binary version: `0.1.0-dev` (local source build without release ldflags)
- config: `/Users/subash/dev/GPUasService/.fairway/platform-foundation-config.toml`
- database: `/Users/subash/dev/GPUasService/.fairway/platform-foundation.db`
- temporary listeners: read-only `127.0.0.1:7896`, full `127.0.0.1:7897`
- both temporary processes and the bounded SSE client were stopped by the
  harness cleanup trap

## Dataset and SQLite state

- tasks: 1,747
- evidence: 4,976
- state history: 5,873
- checkpoints: 3,777
- sessions: 1,194
- reviews: 1,379
- notifications: 1,229
- database bytes: 13,410,304
- WAL bytes: 14,980,352
- journal mode: `wal`
- WAL autocheckpoint: `1000`
- integrity check: `ok`

## Results

Five unique-key `/board` samples on the full dashboard averaged:

- baseline without SSE client: `0.067499s`
- with one idle read-only SSE client: `0.063819s`
- calculated degradation: `-5.45%`

The negative value is normal sample noise. It is well inside the task's maximum
20 percent degradation budget and shows no measurable cross-process SQLite
contention from one idle SSE client.

During ten one-second samples with the client connected:

- read-only process CPU stayed between `0.4%` and `1.0%`;
- read-only RSS grew gradually from `47,024` to `47,792` KiB;
- full process CPU returned to `0.0%` after the initial measured request burst;
- full process RSS remained `155,456` KiB.

The SSE output contained the bounded connected comment and did not hold a CPU
core. Identical board requests demonstrated independent process-local caches:

- read-only process: `0.059002s` cold, `0.001881s` warm;
- full process: `0.055213s` cold, `0.001931s` warm.

## Conclusion

SQLite remains suitable for the current local two-dashboard read topology. The
previous severe contention was a product defect in SSE polling, not evidence
that the store must be switched. One idle client now has negligible CPU cost and
does not materially degrade cold routes on the other process.

The full dashboard's approximately 152 MiB RSS remains above the earlier
synthetic memory ceiling. That is a product/dataset projection budget concern,
not a lock or WAL failure. Existing performance-budget work should track it
separately; shrinking or deleting canonical facts is not an acceptable fix.

The harness in `scripts/dashboard-contention-benchmark.sh` is the repeatable
operator packet for future releases. Re-run it after material SSE, cache, store,
or dashboard projection changes.
