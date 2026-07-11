# Fairway Common-Path Pilot

Date: 2026-07-11

Task: `FW-295`

## Decision

Keep the common `fairway work` lifecycle and continue the intent-to-diff
classifier as advisory measurement. Do not promote unexplained reversible-work
deviations into a blocking `work close` gate from this pilot.

The common cohort completed materially faster with fewer task-state CLI
transitions and no loss of passing evidence, active checkpoints, sessions, or
consequential review enforcement. The pilot does not yet contain the precision,
false-positive, or rollback-confidence data required to justify a new blocking
default. Review and notification records per task also increased.

## Boundary

This assessment changes no review, merge, release, deploy, credential, public,
live-operation, dashboard, provider-send, or task-state authority. It does not
use provider transcripts as measurement or provenance. The Fairway database,
Git commits, evidence, reviews, sessions, checkpoints, and lifecycle rows are
the source facts.

## Cohorts

The historical baseline is the four completed small-team tooling tasks
`FW-274`, `FW-278`, `FW-279`, and `FW-280`. They predate the completed common
path implementation.

The common-path cohort is the nine v0.1.12 train tasks completed through the
compact lifecycle: `FW-303`, `FW-293`, `FW-306`, `FW-294`, `FW-299`, `FW-297`,
`FW-298`, `FW-300`, and `FW-301`.

This is an observational pilot, not a randomized comparison. Task complexity,
review timing, and provider availability differ. Results support bounded
product decisions but not a claim that the command surface alone caused every
timing improvement.

## Results

| Measure | Historical baseline | Common path | Interpretation |
| --- | ---: | ---: | --- |
| Tasks | 4 | 9 | Cohorts above |
| Task-state CLI transitions per task | 3.75 | 3.00 | Common close/start removed extra task-state transitions |
| Checkpoint rows per task | 3.25 | 1.22 | Atomic start produced one clear active checkpoint |
| Tasks with active checkpoint | 4/4 | 9/9 | No visibility regression |
| Tasks with provider session | 4/4 | 9/9 | No attachment regression |
| Start to first evidence, average | 2,222.1 s | 581.7 s | Authoring/proof proxy improved |
| Active to done, average | 6,194.2 s | 713.5 s | Elapsed lifecycle proxy improved |
| First evidence to done, average | 3,972.1 s | 131.8 s | Closeout delay materially lower |
| Last validation to done, average | 3,965.0 s | 131.8 s | Closeout handback materially lower |
| Blocked seconds | 6,559 | 0 | No common-cohort blocked interval |
| Review-wait seconds projected | 0 | 0 | Reviews completed inside the acknowledgement window |
| Tasks with passing evidence | 4/4 | 9/9 | Evidence completeness preserved |
| Review records per task | 3.25 | 4.00 | Review ceremony increased |
| Changes/reject findings | 0 | 6 | Two common tasks had three-domain changes requests |
| Notifications per task | 1.75 | 3.33 | Coordination overhead increased |
| Reopen/retry count | 0 | 0 | No observed regression |
| Accepted material decisions | 0/0 | 2/2 | No hollow or insufficient decision in measured rows |
| Stale memory findings | 0 | 0 | Resume packet remained fresh |
| Promotion-debt findings | 0 | 0 | No unresolved promotion row |
| Rollback-referencing evidence rows | 0 | 1 | Too sparse and task-dependent for comparison |

The six review findings came from `FW-303` and `FW-293`; both were corrected
before commit. This is useful defect discovery, not a reason to require the
same review weight for every reversible task. The other seven common tasks had
approval-only review records.

## Measurement Limits

Fairway currently records durable primitives, not a complete command-invocation
ledger. Exact manual commands and human/provider authoring effort therefore
cannot be reconstructed without using transcripts, which this pilot rejects.
Task-state transitions and start-to-first-evidence time are bounded proxies.

Intent-to-diff classification has not yet run across a labeled set, so the
pilot has no measured precision, recall, or false-positive rate for deviations.
Rollback confidence is also not a normalized evidence field and varies by task
risk. These gaps prevent responsible promotion to a blocking reversible-work
gate.

## Reproducible Readback

The assessment used:

```bash
fairway --json delivery report --since 336h
fairway --json memory reconcile --older-than 24h
fairway --json decision list FW-303
fairway --json decision list FW-293
```

SQLite read-only queries grouped the named task IDs over `task_state_history`,
`task_evidence`, `task_checkpoints`, `agent_sessions`, `task_reviews`,
`task_notifications`, `task_decisions`, and `task_decision_assessments`. No row
was inserted, updated, deleted, or backfilled for the analysis.

The exact cohort queries are preserved in
[`fairway-common-path-pilot-2026-07-11.sql`](fairway-common-path-pilot-2026-07-11.sql):

```bash
sqlite3 .fairway/state.db < docs/assessment/fairway-common-path-pilot-2026-07-11.sql
```

Task-level blocked/review-wait/reopen and defect-source rows came from the
recorded delivery report. The exact cohort selection is reproducible with:

```bash
fairway --json delivery report --since 336h |
  jq '[.rows[] | select(.task_id as $id | ["FW-274","FW-278","FW-279","FW-280","FW-303","FW-293","FW-306","FW-294","FW-299","FW-297","FW-298","FW-300","FW-301"] | index($id))]'
```

## Follow-Through

1. Implement `FW-304` as an advisory/report-only intent-to-diff classifier for
   reversible work. Preserve existing blocking consequential boundaries.
2. Record classifier findings, accepted explanations, and false positives over
   a labeled pilot before considering promotion.
3. Keep review routing risk-scaled. Group approval-only reversible reviews when
   policy allows; retain direct review where a real defect class or
   consequential boundary justifies it.
4. Do not add a default rollback-confidence field or command ledger until a
   separate measured need establishes that the extra authoring cost improves
   safety or resume quality.

Promotion criteria for a later decision are: demonstrated classifier precision,
bounded false positives, lower common-path effort, no weaker consequential
boundary enforcement, and evidence that blocking catches material defects that
advisory reporting would otherwise miss.
