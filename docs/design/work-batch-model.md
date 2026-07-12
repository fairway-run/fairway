# Work Batch Model

Status: implemented first slice for `FW-127`.

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

Consumer stabilization work showed that treating every granular Fairway task as a
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

The first CLI slice stores that plan directly:

```bash
fairway batch create BATCH-001 \
  --title "Platform validation slice" \
  --task T-001 \
  --task T-002 \
  --branch feature/platform \
  --worktree ../worktrees/platform \
  --validation-command "go test ./..." \
  --review-domain arch \
  --review-domain backend \
  --rollback-criteria "revert shared branch" \
  --split-criteria "split if failures need different owners" \
  --expected-ci "GitHub Actions platform workflow"
```

Use `fairway batch add` and `fairway batch remove` to maintain membership, and
`fairway batch link --pipeline-id <id> --deploy-run-id <id>` when the shared
CI/deploy-run exists.

## Evidence Mapping

One CI/deploy-run can validate multiple tasks. Each task must still be closed
individually, with evidence explaining how the shared batch evidence satisfies
that task's acceptance criteria.

If a batch fails, split only the failing task or concern into a follow-up branch
when practical. Do not rerun unrelated passing work just because the original
batch contained it.

Record shared evidence once:

```bash
fairway batch evidence BATCH-001 \
  --command-text "go test ./..." \
  --result pass \
  --artifact https://ci.example/runs/123 \
  --artifact-type ci
```

By default, batch evidence is also inserted as task evidence for every member
task with a `work_batch=<batch-id>` note. Use `--map-to-tasks=false` only for
batch notes that do not validate member task acceptance criteria.

## Dashboard And Audit

The dashboard should show both task count and batch count. Reconciliation should
warn when many related tasks in the same domain create separate CI monitors
with the same validation commands, because that is a signal of over-splitting.

Batching should be paired with utility-first CI/deploy monitoring. The expected
pattern is one coherent batch, one branch/worktree, one shared CI/deploy-run,
and one watcher utility that records handback evidence. Long-running agent
sessions should not spend provider tokens polling the same run.
Use `examples/session-adapters/ci-monitor.sh` for the first provider-neutral
utility shape: pass the batch id, task id, external run id, poll command,
source SHA, expected window, and artifact URL so the monitor evidence maps back
to the batch and member task.

The first implementation exposes:

- `fairway task-detail <task-id>` batch membership and mapped evidence;
- `/tasks/<task-id>` batch membership;
- `/reports` batch count and batched-task count;
- `fairway audit work-coverage` `work_batch_candidate` findings for related
  same-domain tasks with separate CI/deploy evidence and no existing batch.

## Remote Push Boundary

Batch branches may be local scratch branches until the batch is ready for
review, integration, or release validation. Remote CI should normally run on
the configured main branch after a coordinator or reviewer/merge lane has
verified and merged the batch locally.

Allowed remote push intents are:

- `main-validation`: push the configured main branch after local merge;
- `integration`: shared integration branch for multiple lanes;
- `review`: branch intentionally exposed for independent review;
- `release`: release or promotion branch;
- `backup`: explicit operator-approved mirror/backup;
- `exception`: scoped reason recorded in evidence or checkpoint.

Absent one of those intents, a provider thread should not push its scratch
branch or start remote CI. It should attach local validation evidence and hand
off to the coordinator or reviewer/merge lane.

Record push intent as Fairway evidence:

```bash
fairway record push-intent <task-id> --intent review --branch review/<task-id>
fairway record push-intent <task-id> --intent exception --branch scratch/<task-id> --reason "operator requested remote backup"
```

The closeout guard treats a remote branch without matching `push-intent`
evidence as closeout debt. `exception` intent is valid only with a reason.

## Batch And Lane Closeout

A completed task or batch is not enough to release a lane for new work. The
lane is available only after branch, worktree, review, CI/deploy, session, and
remote cleanup state are reconciled.

For a batch, closeout should prove:

- all member tasks have terminal or explicit follow-up status;
- shared evidence is mapped to every member task that it validates;
- required reviews for every member task are approved or explicitly waived;
- the batch branch is merged, or preserved with a recorded reason;
- configured remote branches are deleted after merge, or preserved with a
  recorded reason;
- the role worktree is clean and no longer carrying unmerged batch state;
- related sessions, watchers, and utility monitors are ended or have fresh
  intentional checkpoints.
- any remote branch push has a recorded push intent and final disposition:
  merged and deleted, preserved with reason, or blocked with owner/action.

This prevents a successful batch from leaving behind branch cleanup work that
is larger than the original implementation. The first closeout surface is
task-level:

```bash
fairway workflow closeout <task-id> --dry-run
fairway workflow check --mode close --task-id <task-id> --require-clean
```

Batch-level closeout can build on the same report model later. Until then,
run closeout for every task in the batch and keep the shared branch preserved
with an explicit reason until the batch is merged and cleaned.
