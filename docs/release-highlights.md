- Generic wait coordination now supports durable `fairway wait add` and
  `fairway wait ack` commands for parked work, repeated handoffs, live-window
  waits, and non-review waits without adding a second wait store.
- Advisory provider adapter configuration and validation are now first-class, so
  teams can declare bounded non-authoritative adapter surfaces without granting
  approval, merge, deploy, wake, or provider-private data access.
- External notifier configuration starts with a dry-run/logging interface and
  fixed templates only, and trusted proxy identity verification is documented as
  a dashboard-security model without enabling runtime verifier middleware yet.
