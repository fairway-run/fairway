- Dashboard lifecycle readback is now process-verifiable: managed JSON records
  bind pid, listen address, binary, version, and mode flags, while legacy or
  mismatched records report unknown and block signaling.
- Larger Fairway stores are now easier to operate: route timing logs, batched
  review/evidence projections, the fast board path, snapshot caching, and
  lazy-loaded diagnostics turn previously multi-second dashboard paths into
  explicit fast/default and deep-diagnostics modes.
- Bounded read-only small-team Fairway is supported on an operator-controlled
  host: managed loopback lifecycle, explicit status/version/config/database/
  pid/log readback, backup/restore, read APIs, local CLI fallback, and a
  versioned clean-state CI rehearsal. Shared writes and non-loopback origins
  remain preview or unsupported.
- Team-store work is measurable but not overclaimed: disposable Postgres
  rehearsal packets and apply/import/readback proof validate compatibility
  boundaries without switching the runtime store or claiming production
  migration readiness.
