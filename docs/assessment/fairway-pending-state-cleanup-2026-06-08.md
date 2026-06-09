# Fairway Pending-State Cleanup - 2026-06-08

This assessment closes noisy pending state without rewriting audit truth.

## Current Open Items

### FW-135: GPUaaS Operator Signoff

`FW-135` remains the active external-signoff blocker. It requires named GPUaaS
operator evidence:

- reviewer/operator name,
- date and time,
- dashboard URL or commit/version,
- personas and scenarios walked through,
- pass, changes-requested, or waived outcome,
- follow-up tasks for any gaps.

Do not mark `FW-135` done from local dogfood evidence alone.

### FWRD-154: Superseded By FW-135

`FWRD-154` is the historical dashboard-redesign walkthrough task. It captured
the local dogfood walkthrough artifact, but it did not receive named GPUaaS
operator signoff.

Decision: leave the original blocked history intact and treat new operator
signoff tracking as consolidated under `FW-135`. `FWRD-154` should not be used
as an active queue item.

### FWRD-129 And FWRD-151: Historical Performance Failures

`FWRD-129` and `FWRD-151` remain historical blocked evidence:

- `FWRD-129` proved client-side board virtualization did not satisfy the
  1000-task first-paint/sort acceptance because the server still shipped the
  full filtered row set.
- `FWRD-151` proved the original strict performance budget failed, including
  RSS over the stated budget.

Later work changed the current dashboard posture:

- `FWRD-161` added server-side board windowed delivery.
- `FWRD-162` recorded a narrow accepted RSS exception for the documented
  1000-task local single-project operator dashboard fixture.

Decision: do not mark the original `FWRD-129` or `FWRD-151` acceptance as
passed. Keep them as blocked/deferred historical evidence and point current
release posture to `FWRD-161`/`FWRD-162`.

## Review Debt Strategy

Coordinator planning now distinguishes active review gates from historical
review debt:

- tasks in explicit `review` state with missing required domains remain
  `review-gated` stop conditions,
- terminal `done` tasks with missing domains are reported as `review-debt`
  actions and do not block unrelated current dispatch by themselves.

Historical review debt must be cleared only by one of these paths:

1. A required reviewer performs a real review and records it.
2. A reviewer records an approval that references an existing artifact-backed
   review.
3. Governance records an explicit waiver with scope, reason, residual risk, and
   expiry or revisit criteria.

Do not backfill approvals from chat summaries, commit history, validation logs,
or role ownership alone.

The separate product gap remains:
`FAIRWAY-REVIEW-DOMAIN-OWNER-MISMATCH-001`. That work should define how Fairway
represents a required domain review when the domain name is also the task owner
role, without weakening the no-self-review rule. This cleanup does not implement
that behavior and does not use it to backfill approvals.

## Coordinator Checkpoint Semantics

Coordinator planning treats the latest checkpoint for each task as the active
operating decision. A later `done`, `parked`, or `abandoned` checkpoint closes
older `awaiting_input` and `review` checkpoints for plan purposes.

This prevents a stale approval checkpoint from keeping a completed and
merge-ready task, such as `FW-141`, approval-gated after the task has a later
done checkpoint and clean reconciliation evidence.
