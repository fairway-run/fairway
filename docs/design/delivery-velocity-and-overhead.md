# Delivery Velocity And Process Overhead

Fairway should measure whether process improves secure delivery speed, quality,
or safety. The first implementation is a read-only report over existing Fairway
state; it does not create a second metrics store and does not make process
metrics into approval, merge, deploy, or release gates.

## Command

```bash
fairway delivery report --since 168h [--profile <name>] [--format text|json]
```

The report uses:

- task status transitions for completed, blocked, and unblocked counts;
- evidence rows for first-evidence-to-done timing and outcome-source buckets;
- review rows for approvals, changes requested, same-lane mappings, and review
  usefulness ratio;
- handoffs, notifications, and review-wait projections for coordination
  overhead and wake activity;
- repeated failure/review-change signals for advisory loop summaries.

The command is advisory. Operators can use it before process reviews, release
retrospectives, or review-profile tuning to decide whether a review/gate is
helping or should be narrowed. It must not approve reviews, mutate task status,
mark merge-ready, deploy, or authorize live work.

## Dashboard Follow-Up

The same report model can feed a compact read-only dashboard panel with trends
for completed tasks, blocked/review-wait time, review usefulness ratio, and top
sources of follow-up tasks. The dashboard remains display-only and must not send
provider messages or change workflow state.
