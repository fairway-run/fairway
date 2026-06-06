# Watchers

Watcher work is long-running observation and triage: CI, deploy, release,
smoke, environment recovery, visual regression, or drift monitoring. GPUaaS
showed that monitoring is real work and needs an owner, success condition,
failure condition, escalation rule, and evidence.

Watcher lanes are not new roles. They are modes attached to existing roles, such
as `ops/watch`, `ui/watch`, or `backend/watch`.

## Commands

```bash
fairway packet watcher <watch-id> --owner <role-or-lane> --process <text> --command <cmd> --success <text> --failure <text>
fairway watcher start <watch-id> --task <task-id>
fairway watcher finish <watch-id> --result <pass|fail|blocked> [--artifact <path-or-url>] [--duration-seconds <n>] [--notes <text>]
fairway watcher status [--include-done]
```

`packet watcher` prints a bounded watch brief. `start` and `finish` record
normal fairway evidence and checkpoint rows; they do not run the watched command
unless a future runner adapter is configured.

Starting a watcher is not enough to prove that something is actually watching.
For CI, deploy, smoke, UAT, or provider-session monitors, the session record
must include one of:

- a backing automation id,
- a local PID or tmux pane,
- an external run id plus a polling command,
- or an explicit manual checkpoint window with an expiry.

Use `session upsert` to record that provider-neutral proof:

```bash
fairway session upsert \
  --id ci-monitor-T-001 \
  --role ops/watch \
  --backend ci-monitor \
  --task-id T-001 \
  --monitor-kind ci \
  --automation-id gha-heartbeat-T-001

fairway session upsert \
  --id deploy-run-T-002 \
  --role ops/watch \
  --backend deploy-monitor \
  --task-id T-002 \
  --monitor-kind deploy \
  --external-run-id deploy-123 \
  --poll-command "gh run view deploy-123"

fairway checkpoint record T-003 \
  --state active \
  --owner ops/watch \
  --target-close-by 2026-06-07 \
  --summary "Manual smoke watch bounded until 2026-06-07"
```

If none of those exists, do not leave the task `in_progress` as a monitor.
Record a bounded checkpoint and close, reset, or block the task before ending
the work burst. `fairway reconcile active` should flag monitor sessions that
have no backing process/automation proof and no fresh bounded checkpoint as
`monitor_session_without_backing_proof`.

Watcher completion must hand control back to the coordinator loop. When the
last watched CI/deploy/UAT/provider task completes, the watcher should record
one of:

- the next ready task or branch action to resume,
- an explicit "no ready work" checkpoint,
- or a coordinator notification that a human decision is required.

Deleting the heartbeat or ending the monitor session is not enough when ready
work remains. Otherwise Fairway state becomes clean but the execution lane can
go idle with local branches or queued tasks waiting.

For provider-backed coordinators, the handback should be an explicit
continuation prompt, not only a Fairway checkpoint. The prompt should include:

- the completed monitor ids, task ids, source SHAs, and final result;
- the working-memory path and Fairway config path to reread;
- the instruction to check Fairway status/ready work and continue with the next
  non-conflicting task unless a documented stop condition applies;
- the required next checkpoint, such as selected next task, no ready work, or
  waiting on review/approval.

Recommended continuation prompt:

```text
The monitored CI/deploy/UAT window is complete.

Read the current working memory file and Fairway config, then check Fairway
status and ready tasks. If no stop condition applies, continue with the next
non-conflicting task. Do not wait for user input only because the monitor
finished. Record a checkpoint explaining the selected next action.
```

If ready work exists and the watcher cannot send the continuation prompt to the
owning coordinator session, it must record that as a resume-needed finding so
the dashboard and reconciliation output show that the lane is not truly idle.

## Packet Shape

```yaml
watch_id:
owner_lane:
process:
command:
expected_duration:
poll_interval:
success_condition:
failure_condition:
allowed_fixes:
must_not_touch:
escalation_rules:
evidence_required:
handoff_format:
```

## Authority Boundary

A watcher may:

- poll or subscribe to external job status,
- collect logs and traces,
- classify failures,
- record typed evidence,
- make a narrow pre-authorized fix,
- escalate when scope expands.

A watcher must not:

- silently expand into unrelated implementation work,
- change a deployment target outside the packet,
- retry indefinitely without a new hypothesis,
- mark work done without evidence,
- bypass review, merge, release, or environment-touch rules.

## Evidence Types

Recommended `artifact_type` values for watcher evidence:

- `ci_pipeline`,
- `deploy_validation`,
- `command_log`,
- `screenshot`,
- `browser_trace`,
- `trace`,
- `runbook`,
- `config`,
- `diff`,
- `other`.

## Failure Classification

| Class | Default action |
|---|---|
| Expected transient | Retry within packet limits. |
| Known flaky lane | Rerun once, then create or link a task if repeated. |
| Owning-layer defect | Make a narrow fix only if allowed by the packet. |
| Environment blocker | Mark blocked with reason and evidence. |
| Scope expansion | Return to coordinator with a blocker or handoff. |

## Dashboard

The dashboard should show active watchers separately from implementation tasks:

- watcher id,
- owner lane,
- process,
- elapsed time,
- success/failure condition,
- latest evidence,
- stale flag when no checkpoint/evidence appears within the expected interval.
- monitor backing proof and active reconciliation findings when a monitor
  session has no automation, PID/tmux, external polling proof, or fresh manual
  checkpoint window.
