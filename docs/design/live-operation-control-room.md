# Live Operation Control Room

Fairway supports approval-gated live operations such as production deploys,
security drills, rollback tests, and exact-window UAT runs. These operations
fail operationally when the next actor, deadline, or authorization state lives
only in provider chat. The live operation control-room model keeps that state in
Fairway and makes provider sessions replaceable execution attachments.

This model is intentionally small. It reuses tasks, checkpoints,
notifications, completion handbacks, live-window records, coordinator plan/tick,
sessions, and dashboard projections. It must not introduce a second wait store
or make the dashboard an acting surface.

## Goals

- Make every approval-gated live operation show the current phase, next actor,
  deadline, authorization state, exact next action, and missed-deadline
  behavior.
- Keep routine scheduling, wake visibility, and missed-handoff detection out of
  LLM chat memory.
- Spend provider/LLM turns on judgment, implementation, review, and exception
  handling instead of polling, remembering who is next, or reconstructing a
  missed handoff from chat.
- Allow a tmux or zellij control room to make the same Fairway state visible to
  humans and providers without becoming the source of truth.
- Preserve trust boundaries: read-only dashboard views never send prompts,
  approve reviews, authorize live execution, mutate production, or carry
  provider credentials.

## Non-Goals

- No independent live-operation database, queue, or wait store.
- No general chat system, chat subscription product, or dashboard user
  notification application.
- No automatic approval, merge, deploy, rollback, Keycloak mutation, credential
  use, or live execution.
- No send/wake authority in shared or read-only dashboard mode.
- No requirement that every project run tmux or zellij. The control-room layout
  is an operator convenience, not the state authority.

## State Model

The durable unit remains the Fairway task. The live-operation state is projected
from typed checkpoints and related rows.

| Concept | Fairway source |
| --- | --- |
| Current task and ownership | task row, owner, status, session attachment |
| Current live-operation phase | latest `live-window` checkpoint |
| Next actor | live-window `next_owner`, completion handback `next_owner`, or coordinator plan action |
| Exact next action | live-window `next_action`, completion handback `next_action`, suggested command |
| Deadline | live-window target close time, checkpoint `target_close_by`, review/notification ack timeout, or explicit packet window |
| Authorization state | packet/review evidence, live-window phase, recorded approvals, and explicit execution authorization checkpoint |
| Provider delivery proof | `task_notifications` rows and provider-event checkpoints |
| Missed handoff | coordinator plan/tick stale row derived from the same records |
| Human visibility | dashboard read-only projection, CLI status, tmux/zellij panes |

The expected approval-gated operation phases are:

| Phase | Meaning | Next required action |
| --- | --- | --- |
| `packet_ready` | Packet exists and is ready to route or review. | Route reviews or record why routing is blocked. |
| `approvals_ready` | Required approvals are recorded for the exact packet/window. | Architecture/control or accountable operator authorizes execution. |
| `execution_authorized` | Execution is explicitly authorized for a bounded window. | Operator starts the approved command or records why it cannot start. |
| `operator_running` | Operator/provider is inside the approved window. | Continue gates and record bounded evidence until closeout. |
| `closeout_required` | Operator stopped with pass, blocked, partial, or fail evidence. | Record completion handback and next decision owner. |
| `done` | Operation closed successfully and required evidence is recorded. | Normal task closeout, merge, deploy, or next backlog action. |
| `blocked` | Operation cannot proceed safely. | Create/fix follow-up, reroute review, or roll a new window. |

Implementations may use compatibility phase names such as
`packet-prepared`, `approvals-readback`, `gate-authorized`, `gate-running`,
`closeout`, and `next-decision`, but coordinator and dashboard projections must
map them to the same control-room semantics. A project should not have two
parallel phase vocabularies for the same operation.

## Required Handoff Fields

Every live-operation handoff that can stop progress must name:

- `task_id`
- `phase`
- `next_actor`
- `deadline` or the reason no deadline applies
- `authorization_state`
- exact next prompt, command, or manual action
- expected evidence path or command output
- missed-deadline action
- forbidden actions until the next authorization boundary

Examples of missed-deadline actions:

- roll exact window to nearest approved 5 or 10 minute boundary,
- wake the operator provider session,
- route missing review domain,
- mark blocked and create a scoped follow-up task,
- ask Architecture Control for a new execution authorization.

## Coordinator Behavior

`fairway coordinator plan` is the deterministic read model for live-operation
handoffs. It should expose approved-but-not-executed windows as stop conditions,
not let them become silent idle time.

`fairway coordinator tick` may render or record bounded wake prompts through
provider adapters. Wake prompts are derived from fixed templates and current
Fairway state. The prompt body is not stored as free-form authority on the
task, and sending a wake does not approve or execute the live operation.

The coordinator should prefer polling/tick degradation over silence. If a
provider target is unavailable, the state should become visible as a failed or
undelivered notification, not as an invisible chat gap.

## Retry Budget and Causal Reset

Approval-gated live-operation reruns must be bounded. Fairway records the
current budget through `live-window retry-budget` checkpoints rather than a
separate retry store. The checkpoint distinguishes:

- meaningful failures, where the approved path reached the behavior being
  tested and failed;
- coordination-only failures, such as missing notification delivery, stale
  provider handoff, or packet metadata gaps, which are visible but do not count
  against the product behavior retry budget;
- the current budget;
- existing Fairway reset task and reset reason when the causal model has been
  refreshed.

When meaningful failures reach the budget and no existing reset task plus reset
reason is recorded, `packet retry` should fail closed and require a causal-reset
task before another narrow preflight or live-operation packet is rendered. After
reset evidence is recorded, the retry packet includes the iteration count, prior
failure closure, reset task, reset reason, and the authorization boundary.
Rendering that packet does not approve or execute the live operation.

## Optional tmux/zellij Layout

A live control room can be a tmux or zellij workspace with panes such as:

- Architecture Control: decision log, Fairway status, and exact authorization
  readback.
- Orchestrator: packet preparation, review routing, CI/deploy/UAT monitor, and
  next-action planning.
- Operator: the approved command surface for the bounded live operation.
- Reviewer Watch: review-wait, approval, and notification status.
- Fairway Dashboard: read-only local or shared dashboard.
- Logs/Tails: CI, deploy, cluster, browser-smoke, or drill evidence streams.

This layout reduces coordination token usage because the state is visible and
durable. Provider sessions do not need to ask what happened, reread long chat
history, or infer the next actor from memory. They read the current Fairway
state and operate only inside their authority boundary.

## Trust Boundary

The read-only dashboard may show control-room state, stale handoffs, next
actors, deadlines, and suggested commands. It must not:

- send provider prompts,
- hold provider credentials,
- approve reviews,
- authorize execution,
- perform live or production mutations,
- change task status.

Provider prompting belongs to coordinator/watch/provider-adapter surfaces. Live
execution belongs to the accountable operator surface after explicit
authorization is recorded.

## Acceptance Checks

- The model derives from existing Fairway state and introduces no second wait
  store.
- A live packet that is approved but not executed by its deadline is visible in
  coordinator plan/tick and dashboard read-only projections.
- A provider/operator closeout that needs Architecture Control or orchestrator
  action is visible as a completion handback or closeout wait.
- Missing provider delivery becomes a visible notification failure or stale
  wait.
- Retry budget state distinguishes meaningful rerun failures from
  coordination-only failures and requires a causal reset before more narrow
  retry packets when the meaningful-failure budget is exhausted.
- Dashboard sharing remains read-only and has no wake/send authority.
- Tests or examples cover a missed approved exact-window handoff where all
  provider chats were idle but Fairway should still expose the next actor and
  missed-deadline action.
