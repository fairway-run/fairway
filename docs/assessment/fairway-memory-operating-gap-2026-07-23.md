# Fairway Memory Operating Gap

Date: 2026-07-23

Fairway task: `FW-380`

## Gap

The GPUaaS pilot exposed two separate issues:

1. Fairway packets did not distinguish completed-task guidance from actionable
   guidance. `FW-379` fixed that product defect.
2. Normal team closeout did not surface memory disposition debt. Correctness
   still depended on remembering a separate memory command.

The second issue is both a Fairway integration gap and an adoption gap.

## Resolution

`workflow closeout` now reports same-id terminal task memory in `active` or
`promote` state as an advisory finding. It does not block closeout, mutate
memory, or create a review requirement.

The agent guide now defines a three-step routine:

- cold-start only when durable cross-burst memory is needed;
- update only for material objective, blocker, decision, or next-action change;
- use normal workflow closeout to surface the disposition decision.

This keeps the common path short. Single-burst tasks need no memory record.

## Measure

The desired maintenance cost is one existing closeout readback plus, when
needed, one explicit disposition command. A team should not need provider
conversation history, `tmp-ux`, or a process-only review task to resume or
close work.
