- Same-repo multi-config dashboards now keep distinct Fairway project lanes by
  DB/config identity, so platform and docs work under one repository can be
  displayed side by side without replacement.
- `fairway notify send` adds explicitly configured `log` and `webhook`
  delivery adapters with env-only destinations/tokens, attempt-based rate
  limits, and durable delivered/failed notification evidence.
- Environment deploy preflight guidance gives operators a reusable readiness
  packet for route readback, worker access, smoke, rollback, blockers,
  next-owner, and next-action handoffs without granting deploy or dashboard send
  authority.
