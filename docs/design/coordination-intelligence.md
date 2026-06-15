# Coordination Intelligence

Fairway is the coordination system of record for governed agentic engineering.
LLM providers, tmux sessions, shell utilities, and human operators are
replaceable execution or advisory attachments. They must not be the only place
where task state, waiting conditions, handbacks, evidence, review status, or
next action exists.

This design captures the product requirements exposed by GPUaaS stabilization
and the June 2026 MFA drill loop: repeated coordination, retry, status polling,
and wait handling should be deterministic Fairway/tool work, not expensive LLM
chat work.

## Boundary

```text
Fairway = coordination system of record
LLMs = replaceable advisory/execution attachments
Humans = policy, priority, and approval authority
```

Fairway owns:

- task state, dependencies, ownership, and review gates;
- sessions, provider attachments, checkpoints, and handbacks;
- evidence, artifacts, usage metadata, and cost estimates;
- track memory and context packets;
- deterministic guards, reconciliation, and next-action reports;
- dashboard and CLI read models.

LLMs may:

- summarize current state from Fairway facts;
- classify blocked, stale, conflicting, or ready work;
- suggest a next action from an allowed enum;
- draft handoff prompts, context packets, and resume notes;
- refresh track memory from compact Fairway packets;
- estimate complexity, cost, and risk from recorded history;
- identify missing evidence or review gaps.

LLMs must not own:

- task state truth;
- review approval, merge approval, or deployment approval;
- private memory that is not written back to Fairway;
- destructive or production-impacting action selection;
- authority to ignore configured gates;
- repeated polling or retry mechanics that a controller can compute.

## Deterministic First

The coordinator should be deterministic before it is advisory. A controller tick
should be able to answer:

- Which tasks are active, blocked, stale, waiting, done, or ready?
- Which reviews or handbacks are missing, stale, delivered, or failed?
- Which provider sessions are active, stale, complete, or missing proof?
- Which live-operation phase is waiting for the next actor?
- Which task has evidence but still needs an explicit status decision?
- Which reviewed task is blocked only by a commit boundary?
- Which repeated failure class should create a scoped follow-up?

LLM advisory can then explain, prioritize, or draft a prompt, but Fairway should
validate the recommendation against task state, review gates, risk, and allowed
actions.

For approval-gated consumer flows, use the reusable critical-flow governance
template in [consumer-critical-flow-governance.md](consumer-critical-flow-governance.md).
It captures the rule learned from live drill loops: flow map before
implementation, non-live preflight before live window, bounded retry before
causal reset, and Fairway evidence before handoff.

## First-Class Track Memory

Local `tmp-ux` memory files are a product signal. Fairway should support
structured track memory as curated operating summaries backed by Fairway facts,
not transcript storage.

Suggested track-memory fields:

```text
track_id
title
purpose
operating_mode
active_scope
current_objective
active_tasks
active_sessions
decisions
blockers
open_questions
review_waits
handoff_prompt_refs
recent_milestones
next_recommended_actions
last_refreshed_at
source_checkpoint_ids
source_evidence_ids
source_review_ids
```

Track memory may keep references to handoff prompt templates, packet artifacts,
or delivery records. It should not store raw provider prompt bodies by default.
Prompts are rendered from current Fairway state and fixed templates when a
packet or wake is requested.

Suggested command surface:

```bash
fairway memory show --track architecture-control
fairway memory update --track architecture-control --from-checkpoints
fairway memory append --track architecture-control --decision "..."
fairway memory packet --track architecture-control --for codex
fairway memory stale --older-than 24h
```

The cold-start path for a new provider session should become:

```bash
fairway memory packet --track architecture-control
```

instead of requiring a long provider thread, a copied chat transcript, or a
manually maintained local file.

## Generic Wait And Wake Model

Review waits, completion handbacks, live-window control room rows, CI watches,
deploy monitors, UAT runs, and stale provider sessions are all variants of a
generic parked-work problem. A waiting track should be parked in Fairway, not in
provider chat memory.

Core rule:

```text
A wake is a durable delivery event, not just a dashboard status change.
```

Suggested wait record:

```text
wait_id
track_id
task_id
waiting_actor
waiting_on
wake_when
wake_target_provider
wake_target_id
wake_template
dedupe_signature
state
created_at
last_checked_at
last_wake_attempt_at
acknowledged_at
expires_at
```

Example wait conditions:

- `reviews_complete`
- `review_notification_failed`
- `completion_handback_ready`
- `provider_session_completed`
- `provider_session_blocked`
- `provider_session_stale`
- `task_unblocked`
- `ci_finished`
- `deploy_finished`
- `uat_finished`
- `approval_required`
- `live_window_deadline_missed`
- `track_memory_stale`

Suggested command surface:

```bash
fairway wait add --track architecture-control --on reviews-complete --task FW-192 --target 019e...
fairway wait list --stale
fairway wait tick
fairway wait wake --send
fairway wait ack <wait-id>
```

`fairway coordinator tick --wakes` can remain the high-level entry point that
evaluates wait records alongside review waits, completion handbacks,
live-operation phases, CI monitors, deploy monitors, UAT monitors, and memory
staleness.

Wait ticking is not a durable timer, DAG executor, or autonomous workflow
engine. `wait tick`, `wait wake --send`, and `coordinator tick --wakes` are
operator-invoked or bounded-adapter actions that render, record, or deliver
explicit wake events according to current Fairway state. They do not approve
work, claim tasks, execute live operations, or mutate environments.

Read-only dashboards may show waits, targets, stale state, last wake attempt,
and suggested commands. They must not send prompts, approve work, mutate task
state, or carry provider credentials.

## Advisory Provider Plugins

Fairway should allow optional advisory providers as plugins or configured
adapters. The provider is replaceable and advisory; Fairway validates the
output.

Initial provider types:

```text
noop / rules-only
local_ollama
local_llamacpp
openai-compatible
codex
claude
gemini
```

Good advisory tasks:

- classify task state;
- summarize evidence;
- detect stale sessions;
- suggest next action from a fixed enum;
- draft a handoff prompt;
- rank ready tasks;
- label blocked reasons;
- refresh track memory from a compact packet.

Poor advisory tasks:

- final architecture authority;
- security approval;
- merge or deploy authorization;
- ambiguous root-cause ownership without human review;
- destructive action selection;
- unbounded cross-repo planning.

Suggested output shape:

```json
{
  "action": "resume",
  "task_id": "FW-192",
  "target_role": "governance",
  "confidence": "medium",
  "requires_human": false,
  "rationale": "Task has approved reviews and needs merge-ready verification.",
  "risk_flags": []
}
```

Fairway validation must reject:

- invalid enum values;
- actions not allowed by current task state;
- self-review or self-approval;
- uncited claims about evidence, reviews, or approvals;
- high-risk actions from low-trust advisory providers;
- destructive, credential, production-impacting, or approval-gated actions
  without explicit human authority.

Accepted advisory output should be recorded as advisory evidence or a
checkpoint, not silently applied.

## Live Operation Control

The live-operation control room is a specialization of the generic wait model.
It should track:

- packet prepared;
- approvals ready;
- execution authorized;
- operator running;
- closeout required;
- done or blocked.

For each phase, Fairway should show:

- next actor;
- deadline;
- authorization state;
- exact next action;
- evidence path;
- missed-deadline action;
- forbidden actions until the next boundary.

The control room reduces token burn because providers do not have to
reconstruct who acts next, which window was approved, or whether a handoff was
missed. Fairway owns that routine coordination state.

## Known Failure Routing

Repeated failure classes should map to scoped follow-up tasks without requiring
an LLM to rediscover the route.

Examples:

| Failure class | Suggested route |
|---|---|
| artifact missing or schema mismatch | harness/artifact-contract task |
| provider 4xx or unknown provider behavior | provider API proof task |
| browser launch or permission failure | provider-surface readiness task |
| setup gate failed | setup/readback task |
| callback missing | browser-flow contract task |
| redaction finding | redaction guard task |
| reviewed files uncommitted | commit-boundary lane |
| review handoff not delivered | wait/wake notification task |

Failure routing should recommend scoped tasks from templates by default. A task
may be created only through an explicit operator command, dry-run/apply
workflow, or configured policy that names the task id, owner, evidence path,
and allowed mutation. Any live or production action stays blocked until the new
causal model is reviewed.

## Token-Burn Reduction Rule

Provider/LLM turns are valuable for judgment and exception handling. They
should not be spent on routine polling or repeated coordination mechanics.

Fairway and scripts should handle:

- polling and waiting;
- stale-session checks;
- review and handback wakeups;
- retry packet scaffolding;
- evidence collection;
- known failure routing;
- commit/deploy/CI/UAT watches;
- artifact contract validation.

LLMs should handle:

- architecture tradeoffs;
- ambiguous root-cause analysis;
- review synthesis;
- new workflow design;
- exception analysis;
- risk option summaries;
- approval-packet drafting for human or governance decision.

If a workflow consumes multiple LLM turns repeating the same status, retry,
packet, or handoff loop, that is a Fairway product gap.

## Candidate Product Tasks

```text
FW-MEMORY-001
Add first-class track memory records and CLI packet generation.

FW-ADVISORY-001
Define advisory provider config and structured recommendation contract.

FW-ADVISORY-LOCAL-001
Add local OpenAI-compatible/Ollama advisory provider support.

FW-ADVISORY-GUARDS-001
Validate advisory recommendations against task state, review gates, and risk.

FW-DASHBOARD-MEMORY-001
Show track memory, stale context, blockers, and next action on the dashboard.

FW-COORDINATION-REPORT-001
Add next-action, stale-session, review-wait, and memory-staleness reports.

FW-WAIT-001
Define first-class wait/watch records for parked tracks and provider sessions.

FW-WAIT-WAKE-001
Add bounded wake rendering, dedupe signatures, send/failure recording, and ack.

FW-DASHBOARD-WAITS-001
Show open/stale waits, wake target, last wake attempt, and suggested command.

FW-FAILURE-ROUTER-001
Map known evidence failure classes to scoped task templates and next actions.

FW-RETRY-PACKET-GENERATOR-001
Generate `fairway packet retry` packets from task, SHA, operator surface,
artifact directory, evidence contract, allowed actions, forbidden actions,
expiry, and prior-failure closure. Rendering a retry packet is not execution
authorization.
```

## Product Principle

LLMs should increase Fairway's coordination intelligence without becoming the
coordination authority.

The target operating model is:

```text
deterministic Fairway facts
-> compact context packet
-> optional advisory LLM
-> schema validation and policy checks
-> recorded recommendation
-> human or configured workflow action
```

Fairway should remain useful without LLMs, better with local advisory models,
and capable of escalating to stronger models or humans when judgment matters.
