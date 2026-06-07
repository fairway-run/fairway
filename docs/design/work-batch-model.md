# Work Batch Model

Status: proposed for `FW-127`.

Fairway tasks remain the accountability unit. A work batch is the execution and
validation unit for related tasks that share one branch, worktree, CI run,
review path, and deploy-run evidence.

The target shape is:

```text
Epic
  -> Fairway tasks
    -> Work batch
      -> branch/worktree
      -> shared validation commands
      -> one CI/deploy-run
      -> evidence mapped back to each task
```

## Problem

GPUaaS stabilization showed that treating every granular Fairway task as a
separate branch and CI run creates too many validation cycles. Watchers reduce
human idle time, but they do not reduce runner CPU, memory, IO, GitLab queue
pressure, duplicated codegen/frontend/build work, or reconciliation noise.

Granular tasks are still useful. The mistake is making each small task its own
implementation and CI unit when related tasks share the same validation surface.

## Batch Criteria

Batch tasks when they share:

- domain or product surface;
- touched files or contracts;
- validation commands;
- review domains;
- rollback behavior;
- no hard dependency requiring sequential proof.

Keep tasks separate when:

- owners or review domains differ materially;
- failure diagnosis would be unclear;
- one task is destructive, live-env, or approval-gated;
- one task depends on another task's result;
- rollback must be independent;
- unrelated files or contracts are touched.

## Required Batch Plan

Before implementation, the orchestrator should record:

- batch id;
- included task ids;
- branch and worktree;
- shared validation commands;
- review domains;
- expected CI/deploy-run;
- rollback or split criteria;
- known non-goals.

## Evidence Mapping

One CI/deploy-run can validate multiple tasks. Each task must still be closed
individually, with evidence explaining how the shared batch evidence satisfies
that task's acceptance criteria.

If a batch fails, split only the failing task or concern into a follow-up branch
when practical. Do not rerun unrelated passing work just because the original
batch contained it.

## Dashboard And Audit

The dashboard should show both task count and batch count. Reconciliation should
warn when many related tasks in the same domain create separate CI monitors
with the same validation commands, because that is a signal of over-splitting.
