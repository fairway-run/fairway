# State machine

## Default states

```toml
[states]
allowed = ["todo", "in_progress", "blocked", "done"]
terminal = ["done"]
```

A task starts in `todo` when created. The first claim transitions it to `in_progress`. `blocked` is for work paused awaiting external input. `done` is terminal.

## Configurable extension

Teams that want a richer flow can extend the state list:

```toml
[states]
allowed = [
  "todo", "ready", "claimed", "in_progress",
  "needs_review", "changes_requested", "verified", "merge_ready",
  "done", "blocked", "failed",
]
terminal = ["done", "failed"]
transitions = [
  ["todo", "ready"],
  ["ready", "claimed"],
  ["claimed", "in_progress"],
  ["in_progress", "needs_review"],
  ["needs_review", "changes_requested"],
  ["changes_requested", "in_progress"],
  ["needs_review", "verified"],
  ["verified", "merge_ready"],
  ["merge_ready", "done"],
  ["in_progress", "blocked"],
  ["blocked", "in_progress"],
  ["*", "failed"],
]
```

`transitions` is optional. When omitted, any-to-any transitions are allowed (subject to the invariants below). When present, only listed transitions are accepted; `*` on the `from` side is a wildcard.

## Invariants (always enforced)

1. The initial transition for a new task must have `from_status = NULL`, and `to_status` must be in `allowed`.
2. A task in a terminal state cannot be transitioned out unless `fairway set-status --reopen` is used (writes an explicit history row with `reason = "reopen"`).
3. Every transition writes one `task_state_history` row in the same SQL transaction as the `task_state` update.
4. The `actor` column on history is populated from the current OS user or active session — never NULL.

## Hierarchy

The state machine applies uniformly to every node in the task tree (see [hierarchy.md](hierarchy.md)). An epic transitions to `done` the same way a leaf does. An epic's status is **independent** of its descendants — you can mark an epic `done` with open children; fairway warns but does not refuse. The dashboard shows both the explicit status and the X/Y descendants-done rollup.

## Rationale for shipping 4 states by default

The 11-state model in GPUaaS's `Agent_Orchestrator_v2` doc is aspirational — never load-bearing in the Ruby store. Hardcoding it bakes in unvalidated distinctions (`merge_ready` vs `verified` vs `done` have no enforcement today). Config-driven states cost ~20 LOC of validation and let dogfooding decide whether the richer flow is worth the operator overhead.

If three months of usage shows everyone configuring the same 11 states, those will be promoted to a built-in `[states] preset = "verified-merge"` so users do not have to copy the same TOML block.
