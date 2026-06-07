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
`fairway reconcile active` reports this condition as
`monitor_completion_resume_needed` when all watcher/monitor sessions are closed,
a recent monitor completion exists, and one or more ready tasks remain.

## Utility-First Monitoring

CI, deploy, smoke, and UAT polling should be performed by utility processes,
not by long-running agent conversations. The operating rule is:

```text
Agents do not poll CI. Watchers poll CI and emit Fairway handbacks.
Agents act on handbacks.
```

The watcher utility owns:

- polling the external run;
- recording heartbeat/checkpoint state;
- closing the monitor session when the run completes;
- attaching pass/fail evidence;
- creating or recommending follow-up tasks for actionable failures;
- emitting a continuation prompt or resume-needed finding.

The reference adapter is `examples/session-adapters/ci-monitor.sh`. It is
provider-neutral: wrappers provide a poll command and external run id, while the
adapter records Fairway state through existing commands.

```bash
examples/session-adapters/ci-monitor.sh \
  --task-id T-001 \
  --batch-id BATCH-001 \
  --monitor-kind ci \
  --external-run-id gha-123 \
  --poll-command "gh run view gha-123 --json conclusion --jq .conclusion" \
  --success-regex "success" \
  --failure-regex "failure|cancelled|timed_out" \
  --source-sha "$(git rev-parse HEAD)" \
  --manual-until 2026-06-07 \
  --artifact https://github.com/org/repo/actions/runs/123
```

The adapter upserts a monitor session with `monitor_kind`, external run id,
poll command, source SHA in notes, and optional automation/manual-window proof.
It starts a watcher, records active checkpoints while waiting, records pass,
fail, or blocked evidence at completion, closes the watcher/session, and runs
`fairway reconcile active --dry-run` as the handback. Failed and timed-out runs
recommend follow-up task prefixes instead of mutating the backlog implicitly.

The agent owns:

- deciding what work is safe to start before the result is known;
- making code, docs, review, or release decisions from the handback;
- batching related work so one CI run validates multiple tasks where safe;
- escalating when the handback requires approval, missing credentials, or a
  production-impacting decision.

This prevents token-heavy provider sessions from sitting idle during 15-20
minute CI windows, while still preserving a durable audit trail for what was
watched and what happened next.

CI monitor utilities are one example of the broader Fairway tool-first model.
The same shape should apply to deterministic operational work such as deploy
status checks, codegen drift checks, stale-branch scans, release asset checks,
registry/image freshness checks, and dashboard/report audits. Fairway support
needed for these utilities is:

- a stable adapter contract for tool state and handback events;
- session/checkpoint/evidence recording without an active provider chat;
- dashboard and reconcile output that distinguishes utility work from agent
  judgment work;
- next-action recommendations that an agent can consume without parsing raw
  logs;
- usage reporting that attributes agent tokens only to judgment or execution
  work, not to utility polling.

`examples/session-adapters/utility-event.sh` is the reusable event contract for
those utilities. It accepts:

- identity: task id, optional batch id, role, utility name, utility kind;
- execution proof: command, external run id, automation id, source SHA,
  expected manual window;
- state: `started`, `heartbeat`, `completed`, `failed`, `timeout`, or `stale`;
- output: artifact, artifact type, result, recommended next action, and
  `--decision-required`.

The adapter writes existing Fairway facts only: utility session rows,
watcher lifecycle, checkpoints, evidence, and the final
`reconcile active --dry-run` handback. It does not add provider dependencies or
make task status/review/merge decisions.

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
