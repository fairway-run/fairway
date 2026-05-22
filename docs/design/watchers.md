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
