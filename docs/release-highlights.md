- Larger Fairway stores are now easier to operate: route timing logs, batched
  review/evidence projections, the fast board path, snapshot caching, and
  lazy-loaded diagnostics turn previously multi-second dashboard paths into
  explicit fast/default and deep-diagnostics modes.
- Shared-team Fairway has a bounded pilot surface: loopback-only read APIs,
  API-token identity/authz, append-only evidence/checkpoint writes, guarded
  status/review writes, Mac mini lab packaging, and a recorded small-team pilot
  while keeping public exposure and dashboard authority out of scope.
- Team-store work is measurable but not overclaimed: disposable Postgres
  rehearsal packets and apply/import/readback proof validate compatibility
  boundaries without switching the runtime store or claiming production
  migration readiness.
