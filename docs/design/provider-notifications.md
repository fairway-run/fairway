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
- `review_recorded`: the notification is superseded by a matching Fairway
  review.
- `failed`: delivery failed; a reason is required.

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
