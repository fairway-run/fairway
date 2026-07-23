# GPUaaS Memory Maintenance Assessment

Date: 2026-07-23

Fairway task: `FW-379`

GPUaaS track: `ARCH-FAILURE-UPGRADE-DOMAINS-001`

## Purpose

This assessment measures what happens after a real task represented by Fairway
working memory is completed. The test uses the failure-and-upgrade-domain track
from the first GPUaaS memory and knowledge pilot. The owning architecture task
completed at GPUaaS commit `5d0be6cac`.

The evaluator uses only the retained cold-start packet and Fairway
reconciliation output. It does not use provider conversation history,
repository inspection, or `tmp-ux` memory.

## Before

Fairway correctly reported `track_task_status=done`, but the active memory still
presented the old objective and next actions without an actionability label.
`fairway memory reconcile` returned no findings.

This created a gear-shift risk: a replacement provider could correctly identify
the task as complete while still treating its old execution guidance as the
next work to perform.

Before evidence:

```text
docs/assessment/evidence/fw-379/before-summary.json
```

## Product Change

Cold-start and memory packets now:

- preserve prior objective and next actions for historical traceability;
- mark both non-actionable when the same-id Fairway task is terminal;
- emit `actionability=historical_terminal_task`;
- emit a bounded closeout warning.

Memory reconciliation now:

- accepts `--track <track-id>` for targeted maintenance;
- reports `terminal_task_active_memory` when active memory belongs to a
  terminal task;
- remains read-only and requires an accountable promote-or-archive decision.

## After

The same GPUaaS track was evaluated again with the updated Fairway binary. The
result:

- task status remains `done`;
- objective and next actions remain visible but are explicitly non-actionable;
- targeted reconciliation reports exactly the closeout condition;
- the accountable archive disposition records canonical commit `5d0be6cac`;
- post-closeout reconciliation returns no findings;
- cold-start rejects the archived track instead of reviving completed work.

The completed track's durable architecture result already exists in
`doc/architecture/Failure_And_Upgrade_Domain_Model_v1.md` at GPUaaS commit
`5d0be6cac`. Therefore the correct closeout for this track is archive with that
canonical commit, not creation of a second knowledge authority.

Maintenance required one targeted reconcile readback and one disposition
command. It required no clarification, provider conversation history,
repository investigation, or legacy `tmp-ux` file. The compiled cold-start
readback completed in under one second.

After evidence:

```text
docs/assessment/evidence/fw-379/after-summary.json
```

## Decision

Keep deterministic retrieval and add lifecycle maintenance before considering
embeddings or hosted retrieval. This test found a concrete limitation in
actionability and closeout, not in lexical retrieval quality.

The maintenance target is small: after task completion, one targeted reconcile
readback and one accountable disposition should be sufficient. If maintenance
requires reconstructing provider conversation history, the model has failed.
