# Fairway Review Wait Notification Model

Status: design accepted; first read-model/CLI/static-routability slice
implemented

## Problem

Fairway records task state, reviews, handoffs, notifications, and provider
checkpoints durably. That audit model works, but the current workflow still
lets provider threads sit idle when a review wait needs action.

The recurring failure mode is:

1. an implementation task routes reviews;
2. one or more review domains are pending, stale, or not routable;
3. the orchestrator records a blocked or waiting state;
4. no live surface wakes the orchestrator when the wait resolves or becomes
   actionable.

The result is a coordination gap: a control or coordinator surface has to
notice idle threads and manually inspect task detail, reviewer chats,
notification records, and active reconciliation.

## Goals

- Make review waits first-class Fairway state.
- Let orchestrator/provider sessions park on a specific wait condition without
  losing the next action.
- Wake or nudge the orchestrator when the wait resolves, becomes stale, or
  becomes a routing/mapping problem.
- Keep Fairway DB as the source of truth.
- Keep the first implementation small enough to help current stabilization
  work without turning Fairway into a general notification product.

## Non-Goals

- No arbitrary read-only dashboard user subscriptions.
- No Slack, email, Teams, or other external delivery requirement in the first
  slice.
- No in-memory-only workflow authority.
- No automatic approval, review waiver, merge, deploy, or production-impacting
  action.
- No replacement for Fairway task status, review records, provider sessions, or
  checkpoints.
- No second read model. Review waits must be a projection over the existing
  review-notification state, not a parallel subsystem with its own vocabulary.

## Existing Fairway Surfaces

Most of the durable state this model needs already exists:

- `task_notifications` records delivery facts with the states defined in
  `docs/design/provider-notifications.md`.
- `internal/reviewstate` derives a deterministic per-domain status
  (`missing_notification`, `handoff_recorded`, `sent_awaiting_ack`,
  `notification_failed`, `notification_delivered`, `review_acknowledged`,
  `review_recorded`) with `blocking` and `suggested_action`.
- Coordinator plan emits `notification-blocked`, escalates `stale-sent` using
  `[coordinator].notification_ack_timeout`, and emits review-complete
  handbacks with signature-based suppression.
- `[[provider_targets]]` maps review domains to notification destinations.
- Watchers and utility adapters already implement the poll-then-handback
  pattern for CI monitors.

The review-wait model is therefore a projection and a small set of additions,
not a new subsystem. Anything in this document that duplicates the surfaces
above should be implemented by extending them.

## Durable State

Derive a review-wait read model from existing review, notification, handoff,
and task records. The derived state must be deterministic and exposed through
one command. A dedicated table is only justified later if derivation proves
insufficient.

| Field | Meaning |
|---|---|
| `wait_id` | Stable id for this task/domain wait, derived as `<task_id>/<domain>`. |
| `task_id` | Fairway task id. |
| `domain` | Review domain or mapped review intent. |
| `state` | `pending`, `stale`, `notification_failed`, `resolved`, or `cancelled`. |
| `blocking` | Whether this wait currently blocks merge-ready or task closeout. |
| `reason` | Human-readable reason. |
| `action` | Suggested next action such as `deliver_notification`, `record_delivery_proof`, `nudge_reviewer`, `reroute`, `mapping_required`, or `run_merge_ready`. |
| `target_provider` | `codex-thread`, `fairway`, `manual`, or future provider type. |
| `target_id` | Thread id, reviewer role, queue name, or manual target. |
| `last_notified_at` | Last notification attempt time. |
| `expected_response_at` | Time when a pending wait becomes stale. |
| `resolved_at` | Resolution time. |
| `resolved_by` | Reviewer, coordinator, operator, control surface, or automation. |
| `wake_thread_id` | Optional provider thread to nudge when the state changes. |

Wait states project from `reviewstate` statuses rather than being computed
independently:

| Wait state | Derived from |
|---|---|
| `pending` | `sent_awaiting_ack`, `notification_delivered`, `review_acknowledged`, `handoff_recorded`, or `missing_notification` on a routable domain, within the response window |
| `stale` | any pending-class status past `expected_response_at` |
| `notification_failed` | `notification_failed`, or `missing_notification` on an unroutable domain |
| `resolved` | `review_recorded` |
| `cancelled` | task left review state or domain removed from required set |

A routable domain with no notification attempt is `pending` with action
`deliver_notification`; a routable handoff with no provider proof is `pending`
with action `deliver_notification` or `record_delivery_proof`; a delivered
notification with no review is `pending` with action `nudge_reviewer` after the
response window. Only unroutable domains map directly to
`notification_failed`. UI and CLI surfaces must not render all `pending` waits
as "reviewer has been contacted"; the `action` field carries that distinction.

`expected_response_at` defaults to `last_notified_at` plus
`[coordinator].notification_ack_timeout`. When no notification attempt exists,
the clock starts from the latest handoff time, then from review routing time.
A per-route override may be added to `[[provider_targets]]` later; there must
not be two independent staleness clocks for the same wait.

There is no stored `wake_prompt` field. Wake prompts are rendered from a fixed
template at send time (see Wake Adapter). Storing per-wait prompt text in the
DB creates stale instructions and widens the provider event trust surface.

## Static Routability Validation

The motivating incident is detectable before any wait exists. When a task
declares required review domains, Fairway can check each domain against
configured roles, `[[review_routes]]`, and `[[provider_targets]]` at routing
time.

- `fairway route review` warns or fails when a required domain has no routable
  reviewer role and no provider target.
- `fairway coordinator preflight` reports declared-but-unroutable domains
  across active tasks, since config can drift after routing.

Unroutable domains surface as `notification_failed` waits with
`mapping_required`, but the static check is the primary defense.
Discovering a missing reviewer mapping as a failed wait days later is the
failure mode this design exists to remove.

## Example

A task can have valid approvals for configured domains but remain blocked on a
declared domain that has no reviewer route. For example, a task might be
approved by `ops` and `backend`, but still require `product` review while the
active Fairway config has no `product` reviewer role and no provider target for
that domain.

The intended system behavior is:

```text
review_wait:
  task_id: EXAMPLE-REVIEW-ROUTING-001
  domain: product
  state: notification_failed
  action: mapping_required
  wake_target: coordinator thread

review_wait:
  task_id: EXAMPLE-REVIEW-ROUTING-001
  domain: compliance
  state: notification_failed
  action: mapping_required
  wake_target: coordinator thread
```

The orchestrator should not continue waiting for nonexistent reviewers. It
should be woken or should see a structured blocking wait that says reviewer
mapping is required. With static routability validation, this state is also
reported at routing time and in preflight, before a provider thread parks on it.

## CLI Surface

Implemented first-slice command surface:

```bash
fairway review-waits list --blocking
fairway review-waits list --task <task-id>
fairway review-waits list --stale
fairway review-waits wake --task <task-id>
```

Useful JSON fields:

```json
{
  "task_id": "EXAMPLE-REVIEW-ROUTING-001",
  "domain": "product",
  "state": "notification_failed",
  "blocking": true,
  "action": "mapping_required",
  "target_provider": "codex-thread",
  "target_id": "codex-review-thread",
  "expected_response_at": "2026-06-12T18:55:00Z"
}
```

`review-waits list` is a read-only presentation of the `reviewstate` projection. It
does not approve reviews, send notifications, wake providers, merge, push, or
close work. The same rows are also visible through coordinator plan actions so
the orchestrator active-wait loop can use the same state as the CLI before
idling.

`review-waits wake` is the bounded acting companion for coordinator/watch
loops. Without `--send`, it renders fixed wake prompts from current
review-wait rows and exits without writing notification state. With `--send`,
it records a provider notification row on the `coordinator` domain using the
current prompt signature. Duplicate signatures are suppressed. A missing wake
target records `notification_failed` instead of pretending delivery occurred.
The prompt text is rendered at send time and is not stored as arbitrary
operator-authored wake text for future replay.

The first slice also validates static routability before ambiguous wait states:
`fairway route review` fails when a task declares a required review domain that
has no configured role, review route, or provider target, and
`fairway coordinator preflight` reports the same issue across non-terminal
tasks.

## Dashboard Surface

Implemented follow-up: `OPS-FAIRWAY-REVIEW-WAIT-DASHBOARD-SSE-001`.

The dashboard server remains read-only. Its responsibilities for review waits
are:

1. poll Fairway DB for review-wait state changes;
2. derive advisory events such as `review_wait.stale`,
   `review_wait.notification_failed`, and `review_wait.resolved`;
3. fan events out to connected dashboard clients with Server-Sent Events.
4. show task-level review wait rows with state, blocking flag, target
   domain/provider, expected response time, suggested action, and reason.

The dashboard server does not send wake prompts. Sending messages to provider
threads is an acting capability and belongs in the coordinator/watcher surface,
not in a server that is also exposed through read-only dashboard sharing. The
dashboard does not become the source of truth: if it restarts, it rebuilds
live state from Fairway DB.

In-process pub/sub may be used inside the dashboard server as a fanout helper:

```text
Fairway DB -> dashboard poller -> in-process pubsub -> SSE clients
```

The pub/sub layer is not authoritative. It must be safe to lose any in-memory
event because the next dashboard poll can reconstruct state from the DB. A
small local implementation is sufficient; it should remain an internal
dashboard dependency rather than a Fairway workflow boundary.

## Wake Adapter

Implemented follow-up: `FW-183`.

Waking a parked provider thread reuses the existing watcher/utility-adapter
pattern. A review-wait wake is structurally the same as a CI monitor: poll a
condition, then hand a bounded prompt back to a lane.

- A `review-wait` watcher (or the coordinator tick loop) polls the review-wait
  projection for a task's blocking waits.
- The watcher wakes immediately when any blocking wait transitions to `stale`
  or `notification_failed`; those states require operator action even if other
  domains remain pending.
- The watcher also wakes when all blocking waits are `resolved`, or when the
  task becomes merge-ready actionable after review completion.
- Each wake prompt is rendered from a fixed template at send time and delivered
  or recorded through the configured `wake_thread_id`/provider adapter target.
- Delivery is recorded with `fairway record notification` like any other
  provider send, subject to the provider event trust boundary.

Wake prompt template:

```text
Review wait update for <task-id>:
- <domain>: <state>
- <domain>: <state>

Next action:
1. Re-run fairway review-waits list --task <task-id>.
2. Re-run fairway merge-ready <task-id>.
3. If gates pass, continue reviewed-lane closeout.
```

The orchestrator must re-check Fairway after wake before acting. A wake prompt
is advisory, not approval.

## External Notifier Boundary

External notification libraries, Slack/email/Teams adapters, or packages such
as `nikoksr/notify` belong behind a future notifier interface:

```go
type ReviewWaitNotifier interface {
    Publish(ctx context.Context, event ReviewWaitEvent) error
}
```

That interface can send a stale wait to Slack or email later, but external
delivery must remain optional. Missing external credentials or provider outages
must not make Fairway lose workflow state.

## Orchestrator Wait-Blocking Model

When review waits block closeout, orchestrator should record a parked state:

```text
provider_state: wait_blocking
task_id: <task>
wait_ids: <wait ids>
wake_condition: any wait stale/notification_failed, or all blocking waits resolved
next_action: re-run fairway review-waits list, then merge-ready, reroute, or ask control
```

The park-and-wake path is an optimization. The robust baseline remains the
coordinator loop: `coordinator tick` on an interval surfaces the same waits as
typed actions whether or not a wake prompt was delivered.

## Dashboard UX

Read-only dashboard should show:

- blocking review waits;
- stale waits;
- notification failures;
- target reviewer/domain;
- expected response time;
- suggested action.

Read-only dashboard users must not create notification subscriptions in this
slice. User-requested notification preferences would require a separate
authenticated writable operator capability with audit, expiry, unsubscribe,
rate limits, and permission checks.

## Acceptance For Implementation Task

- Fairway exposes blocking/stale/failed review waits through a deterministic
  CLI/read model projected from `reviewstate`; no parallel wait store is
  introduced.
- Review waits distinguish `pending`, `stale`, `notification_failed`,
  `resolved`, and `cancelled`, with documented derivation from existing
  notification and review states.
- `fairway route review` and `coordinator preflight` report declared review
  domains with no routable reviewer role or provider target.
- Unconfigured review domains surface as actionable `notification_failed`
  waits with `mapping_required`.
- Staleness derives from `notification_ack_timeout`; no second timeout
  mechanism is added.
- Orchestrator instructions are updated to check review waits before idling.
- Dashboard server shows review wait state and may stream SSE events; it does
  not send wake prompts.
- Wake delivery, when implemented, goes through the watcher/coordinator
  surface and records its sends as notifications.
- No external notification provider is required.
- No read-only dashboard user subscription feature is added.
