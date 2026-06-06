# Coordinator Loop

Fairway should support the coordinator operating loop that proved useful in
GPUaaS without making hidden decisions for the operator. The loop is a composed
status and preflight surface; it does not auto-claim, auto-merge, or mutate tasks
unless the user runs the explicit task commands.

## Commands

```bash
fairway coordinator preflight
fairway coordinator status
fairway coordinator tick
```

`preflight` validates whether it is safe to dispatch more work:

1. config validation,
2. task definition/state validation,
3. git consistency checks,
4. session reconciliation dry-run,
5. stale in-progress / blocked / watcher checks,
6. evidence and review gate health,
7. dirty or stale role worktree checks,
8. advisory work-coverage audit for commits, task path metadata, evidence, and
   required review domains.

`status` prints the current operating board:

1. role lane summary,
2. active tasks by age,
3. ready queue by role,
4. session table,
5. watcher and checkpoint summary,
6. health report.

`tick` is `status` plus dispatch recommendations. It answers: "what should the
coordinator do next?" A tick may recommend a claim, review, unblock, checkpoint,
or merge-ready check, but it does not perform those actions automatically.

## Why This Exists

GPUaaS used separate `queue-*`, `agent-*`, and `orchestrator-*` helpers. The
combined tick kept the coordinator from missing stale sessions, stale tracks, or
missing evidence while dispatching parallel agents.

The useful lesson is composition, not automation. Fairway should make the next
operator action obvious while preserving explicit human control over mutation.

## Output Contract

Human output should be compact and ordered by urgency:

```text
Preflight
  config: ok
  git: ok
  sessions: 1 stale candidate
  gates: 2 tasks missing evidence

Active
  backend T-042 in_progress 1h12m session=running
  ui      T-043 blocked 3h04m reason="needs API shape"

Ready
  ops     T-044 P1 setup release workflow

Watchers
  W-CI-1594 active 32m success="pipeline success"

Next
  - reconcile stale sessions dry-run before dispatch
  - route review for T-041
  - claim T-044 as ops
```

JSON output should expose the same sections with stable keys for scripts.

## Relationship To Reports

The coordinator commands compose lower-level commands:

- `fairway status-report`,
- `fairway health-report`,
- `fairway timing-report`,
- `fairway dispatch-plan`,
- `fairway git-check`,
- `fairway audit work-coverage --dry-run`,
- `fairway session status`,
- `fairway session reconcile --dry-run`,
- `fairway watcher status`,
- `fairway checkpoint status`.

The lower-level reports remain useful for focused debugging. The coordinator
loop is the daily operating view.
