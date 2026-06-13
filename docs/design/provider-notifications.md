# Provider Notifications

Provider notifications are delivery facts around Fairway handoffs and review
completion signals. They are not reviews, task status changes, merge decisions,
push decisions, deploy decisions, or release approvals.

## Capability Boundary

Provider capabilities are session and surface scoped. One Codex app session may
have `send_message_to_thread` and `read_thread`; another project/session may
not. Agents must verify the tool surface they actually have before claiming a
reviewer or coordinator thread was steered.

If direct thread tooling is unavailable, record the Fairway handoff or
notification state and route through the coordinator/control track. Do not
claim direct delivery from chat memory alone.

## Notification States

- `intent`: notification is planned but not yet attempted.
- `handoff_recorded`: Fairway recorded a durable handoff, but no provider
  delivery proof exists.
- `sent`: a provider send was attempted and is waiting for acknowledgement.
- `notification_delivered`: adapter/provider delivery proof exists.
- `thread_steered`: direct thread tooling was available and a message was
  posted to the target thread.
- `acknowledged`: the target acknowledged receipt.
- `review_acknowledged`: the reviewer/control lane acknowledged receipt.
- `review_recorded`: the notification is superseded by a matching Fairway
  review.
- `failed`: delivery failed; a reason is required.
- `notification_failed`: delivery failed; a reason is required.

For required review domains, Fairway treats `handoff_recorded`, missing
notification rows, `sent` without acknowledgement, `failed`, and
`notification_failed` as notification-blocked until the notification is
delivered, thread-steered, acknowledged, or the review is recorded. A Fairway
handoff is durable queue state; it is not proof that a reviewer or control
thread was contacted.

## Completion Handback Signal

When a delegated provider closes a slice and the next required action belongs
to another actor, use a completion handback instead of relying on chat state:

```bash
fairway record completion-handback FW-123 \
  --to coordinator \
  --next-action "decide whether to schedule the next live window" \
  --completion-state blocked-with-follow-up \
  --evidence .fairway/artifacts/FW-123/closeout.md \
  --approval-boundary "implementation handback only" \
  --provider codex \
  --target 019e... \
  --state thread_steered
```

The helper writes a normal handoff plus a linked `task_notifications` row. Task
detail and coordinator plan show whether the handback is still pending,
delivered, or failed. Pending cross-role completion handbacks block terminal
closeout until a delivered state or an explicit failure state is recorded; a
clean Fairway task status alone is not proof that the next actor was informed.
The handback record does not grant approval, merge, deploy, provider wake, or
dashboard send authority.

`--completion-state` records the outcome being handed back, separate from
notification delivery. Supported outcomes include `done`, `reviewed`,
`merge-ready`, `blocked-with-follow-up`, `monitor-completed`,
`live-window-closeout`, and `live-window-next-decision`. Coordinator plan uses
the same `[coordinator].notification_ack_timeout` clock as review/provider
notification waits to mark pending completion handbacks stale. It also projects a
`live-window closeout` or `next-decision` checkpoint with no completion handback
as a closeout-to-next-owner wait, so operator closeout cannot disappear into
silent idle when the control thread was not woken.

`fairway coordinator tick --completion-handback-wake` is the bounded wake
surface for these stale waits. It renders fixed prompts from the current
completion-handback/coordinator-plan rows. With `--send`, it records a
coordinator-domain notification with a stable `completion_handback_wake`
signature; a prior successful signature suppresses duplicates. If the next owner
has no provider target, the tick records `notification_failed` with the same
signature. The dashboard does not call this path and remains display-only.

## Review Completion Resume Signal

When all required review domains for a task are approved, Fairway can surface a
review-complete next action. The coordinator plan includes the task id, commit,
approved domains, missing domains, and suggested `fairway merge-ready <task-id>`
command.

The signal is bounded. It is suppressed after matching coordinator delivery or
acknowledgement is recorded, and it appears again if the task commit or review
set changes. Use the `review_signature` value printed by `fairway coordinator
plan` or `fairway task-detail <task-id>` in the notification reason:

```bash
fairway record notification FW-123 \
  --domain coordinator \
  --provider codex \
  --target 019e... \
  --state thread_steered \
  --reason "review_complete review_signature=<current-review-signature>"
```

The commit alone is not sufficient acknowledgement because a task can gain or
change required review domains without changing the implementation commit.

The signal does not authorize merge, push, CI, deploy, release, or task
completion. Those remain explicit coordinator/operator actions guarded by
Fairway evidence, review, workflow, and merge-ready checks.

## Review Wait Wake Signals

`fairway review-waits wake` is a bounded provider-notification surface for
parked review waits. It renders a fixed prompt from current review-wait rows:

```text
Review wait update for <task-id>:
- Task status: <status>
- <domain>: <state>

Next action:
1. Re-run fairway review-waits list --task <task-id>.
2. Follow the status-aware review-wait guidance in the prompt.
3. Do not treat review-wait resolution as task closeout unless task-level gates
   support it.
```

Without `--send`, the command is dry-run output for a coordinator or provider
adapter. With `--send`, it records a `task_notifications` row on the
`coordinator` domain using the current review-wait signature. A matching
previous successful notification suppresses duplicates. If no wake target is
configured, Fairway records `notification_failed` with the signature instead of
claiming delivery. Resolved review waits on a blocked, in-progress, todo, or
otherwise non-review task produce review-wait-only guidance naming the task
status; they do not tell operators to run `merge-ready` or continue closeout.
Blocking `stale` or `notification_failed` waits instruct operators to address
the review-wait blocker before closeout.

The dashboard does not call this path. Dashboard review-wait state and SSE
events remain read-only visibility surfaces.
