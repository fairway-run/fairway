# Dashboard contention benchmark

Use `scripts/dashboard-contention-benchmark.sh` to measure two temporary Fairway
dashboard processes against one SQLite database. The harness starts a read-only
instance and a local full-access instance on configurable loopback ports, opens
one bounded SSE client against the read-only process, and measures unique-key
board requests against the full process before and during that client.

```bash
scripts/dashboard-contention-benchmark.sh \
  --binary /path/to/fairway \
  --config /path/to/config.toml \
  --db /path/to/state.db \
  --out /tmp/fairway-dashboard-contention
```

The output contains latency samples, CPU/RSS samples for both processes, process
logs, SQLite journal/WAL metadata, row counts, binary/config provenance, and a
calculated SSE degradation percentage. Same-URI requests are sent to both
processes to demonstrate that the short snapshot caches are process-local.

The harness uses only temporary loopback listeners and kills only the PIDs it
starts. It does not restart shared dashboards, checkpoint the WAL, mutate task
state, switch stores, expose a public endpoint, or change dashboard authority.
Run it from a stable source/binary boundary; do not interpret a failing product
projection as a reason to delete or archive canonical Fairway facts.
