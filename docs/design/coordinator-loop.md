# Coordinator Loop

Fairway should support the coordinator operating loop that proved useful in
GPUaaS without making hidden decisions for the operator. The loop is a composed
status and preflight surface; it does not auto-claim, auto-merge, or mutate tasks
unless the user runs the explicit task commands.

## Commands

```bash
fairway coordinator preflight
fairway coordinator status
fairway coordinator plan
fairway coordinator tick
```

`preflight` validates whether it is safe to dispatch more work:

1. config validation,
2. task definition/state validation,
3. git consistency checks,
4. session reconciliation dry-run,
5. stale in-progress / blocked / watcher checks,
6. evidence and review gate health,
7. dirty or stale role worktree checks,
8. advisory work-coverage audit for commits, task path metadata, evidence, and
   required review domains,
9. advisory CI/deploy learning audit for failed pipeline, deploy, smoke, and
   UAT evidence without follow-up work,
10. lane closeout checks for done/reviewed work that still has unmerged,
    undeleted, dirty, or intentionally preserved branch/worktree state,
11. release-run verification for release attempts, including public release
    state, asset URLs, and Homebrew fetch evidence.

Use `fairway workflow closeout <task-id> --dry-run` for the lane-closeout check.
Use `fairway workflow check --mode close --task-id <task-id> --require-clean`
when the coordinator is deciding whether a lane may move to its next
implementation task. The closeout guard is advisory about cleanup actions; it
does not delete branches or worktrees automatically.

If a lane has a remote branch, closeout also checks for recorded push intent:

```bash
fairway record push-intent <task-id> --intent review --branch review/<task-id>
```

Provider/thread branches without push-intent evidence remain closeout debt
until the remote branch is deleted, a valid intent is recorded, or governance
records an explicit exception with reason.

`status` prints the current operating board:

1. role lane summary,
2. active tasks by age,
3. ready queue by role,
4. session table,
5. watcher and checkpoint summary,
6. health report.

`plan` is the deterministic dry-run orchestration controller surface. It reads
tasks, sessions, watchers, batches, reviews, worktrees, checkpoints, and active
reconciliation findings, then emits typed next actions without mutating task
state.

For approval-gated live operations, `plan` is also the control-room read model
described in `docs/design/live-operation-control-room.md`. It should show the
current live-window phase, next actor, deadline, authorization posture, exact
next action, and missed-deadline behavior from existing Fairway records. The
goal is to make approved-but-not-executed windows and operator closeout waits
visible without asking provider chats to remember or poll for the next handoff.

When all required review domains for a task have approved latest verdicts,
`plan` emits a `review-complete` handback action that tells the coordinator to
run `fairway merge-ready <task-id>` and perform the configured merge/push/release
step if it passes. The handback includes required domains, approved domains,
latest verdicts, the review signature used for notification acknowledgement,
merge-ready posture, and the recommended next action.

If reviews are complete but a store-visible non-review gate is still missing
such as required evidence or required handoff, `plan` emits
`review-complete-blocked` instead of a false merge-ready handback. Fairway does
not auto-merge, push, deploy, tag, or release from this signal.

If required reviews are missing and Fairway has only durable handoff state, a
failed notification, or no delivered reviewer notification, `plan` emits
`notification-blocked` instead of ordinary review wait. The action includes the
target review domain, latest handoff id/time, latest notification state/time,
provider/target when known, and a suggested retry or manual relay action.
Delivered notifications, direct `thread_steered` records, reviewer
acknowledgements, and recorded reviews move the task back to ordinary review
wait.

Review-complete handbacks are next-action signals, not historical audit rows.
Coordinator plan suppresses handbacks for terminal tasks that already have a
recorded completion commit, closure evidence such as `push-intent`,
`lane-closeout`, `release-run`, or `release-verify`, or an explicit
`review-handback-ack` evidence row. Task detail, reports, and audits can still
show the historical reviews and completion evidence without putting old work
back into the coordinator's action list.

For live review-complete notification delivery, record `notification_delivered`
or `thread_steered` with the current `review_signature` from `coordinator plan`
or task detail. Matching by commit alone is not enough because required review
domains can change on the same commit.

When the ready queue is empty but todo work remains, `plan` includes the same
readiness explanation used by `fairway ready`: non-ready todo count, blocker
categories, top task ids, dependency blocker ids, and suggested inspection
commands. This keeps empty ready queues operationally explainable without
direct SQLite inspection.

Checkpoint interpretation uses the latest checkpoint for each task as the
active operating decision. A later `done`, `parked`, or `abandoned` checkpoint
closes older `awaiting_input` and `review` checkpoints for coordinator-plan
purposes, so resolved approval waits do not keep completed work in the stop
condition list.

Historical review debt is segmented from active review gates. Tasks in explicit
`review` state with missing required domains remain review-gated stop
conditions. Terminal tasks that predate the current review-domain policy are
reported as review-debt actions and should be cleared only by real review,
artifact-backed approval, or explicit governance waiver.

`tick` prints the same plan in daily operating form. It answers: "what should
the coordinator do next?" A tick may recommend a claim, review, unblock,
checkpoint, utility handback, batch, or merge-ready check, but it does not
perform those actions automatically.

For approval-gated live operations, `plan` and `tick` also surface
live-operation control-room checkpoints recorded through `fairway live-window`.
Phases such as `approvals_ready`, `execution_authorized`, and
`closeout_required` are treated as control stop conditions with the next actor,
deadline, authorization state, exact action/prompt/command, and
missed-deadline action in the plan reason. This is the control-channel layer for
approved-but-not-executed windows: the coordinator should see missed execution
handoffs as stale Fairway state instead of relying on provider chat polling.

The live-operation control room is also a token-burn reduction mechanism. LLM
providers should not spend most of their context reconstructing who acts next,
what the approved window was, or whether a handoff was missed. Fairway holds
that routine coordination state; provider turns are reserved for judgment,
implementation, review, and exception handling. See
[Live Operation Control Room](live-operation-control-room.md).

For stale completion handbacks, `tick` can render a bounded provider wake
surface:

```bash
fairway coordinator tick --completion-handback-wake --task <task-id>
fairway coordinator tick --completion-handback-wake --task <task-id> --send --state thread_steered
```

The wake is selected from existing coordinator plan rows, completion handbacks,
live-window checkpoints, and `task_notifications`; it does not introduce a
second wait store. Without `--send`, it prints a fixed prompt naming the task,
task status, stale handback or live-window closeout, next owner/action, and the
suggested Fairway command. With `--send`, it records a coordinator-domain
notification using a stable wake signature and suppresses duplicate successful
signatures. If no provider target exists for the next owner, dry-run output
marks the wake `mapping_required`; `--send` records `notification_failed` with
`action=mapping_required` instead of claiming delivery. Fresh waits and terminal
tasks are not woken; they remain visible through plan/task detail or historical
task evidence. This path is a coordinator/provider-adapter action, not dashboard
send authority.

`fairway audit notifications [--task <task-id>] [--all]` is the matching
read-only lifecycle report for notification delivery facts. It lets the
coordinator inspect unresolved or historical provider notification rows across
review waits, completion handbacks, generic waits, live-operation handoffs, and
coordinator plan waits without polling provider chats. The audit reports the
task, source, target role/domain, provider target, handoff id, latest
notification id, stale age, expected next action, mapping-required target gaps,
and recovery command. It does
not send wakes, approve work, mutate task state, or create a second wait store.

## Orchestration Controller Direction

The coordinator loop is becoming an orchestration surface. Fairway core remains
the state, evidence, and review authority; orchestration is a bounded controller
that reads Fairway state, applies documented policy, and proposes or triggers
the next safe action.

The controller should not become a general autonomous agent. It should behave
like an operations controller:

- reconcile active sessions, watchers, batches, reviews, and handbacks;
- classify work as active, waiting, blocked, stale, complete, or ready;
- surface live-operation control-room waits so provider/LLM turns are spent on
  judgment, implementation, review, and exception handling rather than
  scheduler bookkeeping;
- use configured review profiles to recommend grouped review and continued
  safe-boundary iteration for small non-live/docs/harness slices, while
  reserving full matrices for epic, launch, live-window, deploy,
  production-readiness, compliance, and enforcement boundaries;
- recommend removing or narrowing advisory review/gate pilots when recorded
  Fairway outcomes do not show speed, quality, or safety value;
- detect looping review/retry patterns from Fairway evidence and review facts,
  then recommend a causal reset with failure chain, real unknowns, required
  proof before retry, and a lighter safe-boundary review plan;
- recommend or continue configured utility monitors for deterministic/pollable
  work when `--allow-utility-monitor` is set;
- recommend batching when related tasks share validation surfaces;
- block or warn before dispatching new implementation work on a lane whose
  previous task is done but whose branch/worktree/session cleanup is not
  closed;
- emit continuation prompts only when provider judgment or implementation is
  needed;
- report Fairway handoffs that have no delivered provider/thread notification,
  so a handoff recorded in the DB is not confused with an actual message sent to
  a reviewer or provider target;
- report review-complete handbacks so a coordinator does not need to poll
  provider chat to discover that required reviewers approved a task;
- enforce stop conditions before destructive, production-impacting, credential,
  approval-gated, or review-gated actions;
- produce a deterministic next-action plan in dry-run mode. Unknown or unsafe
  actions remain recommendations or stop conditions; Fairway does not infer
  destructive, production-impacting, credential, approval-gated, or review-gated
  mutations.

This is the missing layer between Fairway as a dashboard and Fairway as a
reliable multi-agent operating system. Utility adapters reduce token churn, but
the orchestration controller decides what should happen after those utilities
hand control back.

## Notification Boundary

Fairway handoffs are durable coordination records. They are not notifications by
themselves. Provider notification is a separate state transition recorded with:

```bash
fairway record notification T-001 \
  --domain security \
  --provider codex \
  --target 019e... \
  --state notification_delivered
```

Valid states are `intent`, `handoff_recorded`, `sent`,
`notification_delivered`, `thread_steered`, `acknowledged`,
`review_recorded`, and `failed`.

Use `handoff_recorded` when Fairway recorded the durable handoff but the
provider or app surface could not prove delivery to the target. Use
`notification_delivered` when an adapter or provider reports successful
delivery. Use `thread_steered` only when the active provider surface has
verified `send_message_to_thread` / `read_thread`-style capability and actually
posted to the target thread. A notification state does not approve review,
close a task, merge, push, deploy, or release. Review authority still comes
from `fairway record review`; task authority still comes from normal status,
evidence, and merge-ready gates.

`sent` is not permanent quieting. If no acknowledgement, `review_recorded`
notification, or real matching review arrives within
`[coordinator].notification_ack_timeout` (default `24h`), coordinator plan
surfaces the handoff as `stale-sent` and recommends escalation. Fresh sent
notifications remain `sent-awaiting-ack` and do not create coordinator noise.

When all required reviews are approved, coordinator plan may emit a
`review_complete_next_action` recommendation. That is a resume signal for the
coordinator/control lane to run `fairway merge-ready <task-id>` and then make
the configured merge/push/CI decision. It is not approval, merge, push, deploy,
or release authority.

Coordinator notification-gap reporting is scoped to actionable work. It ignores
handoffs that were explicitly acknowledged and suppresses historical handoffs
on terminal tasks unless the task still has pending review routing or an
unresolved notification attempt such as `intent` or `failed`. This keeps
upgraded projects from flooding the coordinator plan with old done-task
handoffs while preserving gaps that still need human delivery or review action.

## Why This Exists

GPUaaS used separate `queue-*`, `agent-*`, and `orchestrator-*` helpers. The
combined tick kept the coordinator from missing stale sessions, stale tracks, or
missing evidence while dispatching parallel agents.

The useful lesson is composition, not automation. Fairway should make the next
operator action obvious while preserving explicit human control over mutation.

## Output Contract

Human output should be compact and ordered by urgency:

```text
Preflight
  config: ok
  git: ok
  sessions: 1 stale candidate
  gates: 2 tasks missing evidence

Active
  backend T-042 in_progress 1h12m session=running
  ui      T-043 blocked 3h04m reason="needs API shape"

Ready
  ops     T-044 P1 setup release workflow

Watchers
  W-CI-1594 active 32m success="pipeline success"

Next
  - reconcile stale sessions dry-run before dispatch
  - route review for T-041
  - claim T-044 as ops
```

JSON output should expose the same sections with stable keys for scripts.

## Relationship To Reports

The coordinator commands compose lower-level commands:

- `fairway status-report`,
- `fairway health-report`,
- `fairway timing-report`,
- `fairway completion-handback-report`,
- `fairway dispatch-plan`,
- `fairway git-check`,
- `fairway audit work-coverage --dry-run`,
- `fairway audit ci-learning`,
- `fairway release verify`,
- `fairway session status`,
- `fairway session reconcile --dry-run`,
- `fairway reconcile active --dry-run`,
- `fairway watcher status`,
- `fairway checkpoint status`.

The lower-level reports remain useful for focused debugging. The coordinator
loop is the daily operating view.

`fairway completion-handback-report` is the retrospective closeout-latency view
for completion handbacks. It derives rows from existing handbacks,
notifications, checkpoints, task status, and the configured notification ack
timeout. It reports idle seconds from handback delivery or recording to the next
architecture, orchestrator, coordinator, or target-owner checkpoint, plus stale
and open counts by task and workstream. It deliberately avoids per-person
scoring and excludes terminal tasks by default; pass `--include-closed` only
when a closeout retrospective needs closed-task history.

When an older completion handback is obsolete, `fairway record
completion-handback-supersede` records the cleanup decision as immutable
evidence. Coordinator plan keeps the superseded handback in historical
projections but does not emit it as an active notification-gated stop
condition. Non-terminal tasks need a replacement handback or explicit blocked
status before the old handback can be superseded.
