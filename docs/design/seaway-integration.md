# Optional Seaway Integration Contract

## Status And Scope

This document defines the Fairway-facing contract for an optional Seaway
adapter. It is an implementation prerequisite, not an implemented adapter or a
commitment to a transport. HTTP, gRPC, a local process, a queue, or another
mechanism may carry the contract after both products publish compatible public
interfaces.

Fairway remains usable with Codex, Claude Code, Gemini, Jcode, shell, CI, and
other execution surfaces without Seaway. Seaway remains usable by CLIs, IDEs,
CI jobs, gateways, and other callers without Fairway.

The integration has one purpose: correlate Fairway's durable cross-run work
record with sourced facts from zero or more Seaway runs without merging state,
policy, storage, or authority.

## Authority Invariants

The adapter must preserve these invariants in code, storage, tests, and public
copy:

1. Fairway owns task, lane, durable worktree assignment, session attachment,
   checkpoint, evidence acceptance, review, readiness, and promotion records.
2. Seaway owns run admission, effective runtime policy, run workspace,
   lifecycle, in-run approval, cancellation, reconnect, events, and terminal
   result.
3. One Fairway task may correlate with zero, one, or many Seaway runs. One run
   is one bounded attempt with exactly one terminal result. A retry that starts
   execution again is a new run, not another attempt inside the prior run.
4. Run success does not complete a task. Run failure, rejection, cancellation,
   timeout, or loss does not automatically block or close a task.
5. A Seaway approval authorizes only the named run operation. It never satisfies
   a Fairway review domain, risk acceptance, merge, release, deploy, or live
   approval.
6. Fairway records imported facts with their Seaway source, integrity reference,
   and uncertainty. It does not rewrite a runtime assertion as Fairway proof.
7. A Fairway lane worktree is durable coordination state. A Seaway run workspace
   is runtime state and may be disposable even when both reference the same
   repository revision.
8. Neither product reads or writes the other's database. Integration uses public
   contracts and explicit commands.

## Contract Parties

The contract separates four parties:

| Party | Responsibility |
|---|---|
| Fairway core | Produces bounded task context and records explicit coordination facts through existing validated commands and store methods. |
| Fairway-Seaway adapter | Negotiates capabilities, translates identifiers and events, maintains cursors and deduplication, and invokes only allowed Fairway commands. |
| Seaway | Admits and controls individual runs and returns truthful effective capabilities, events, results, usage, and cost facts. |
| Operator or calling controller | Holds any authority to submit or cancel runs, answer run approvals, change Fairway task state, record reviews, or promote work. |

The adapter should live at an edge, not inside Fairway's state machine or
Seaway's runtime core. A future in-tree reference adapter is acceptable only if
it depends on Seaway's public contract and can be disabled without weakening
normal Fairway behavior.

## Capability And Version Negotiation

Before submitting or reconnecting to a run, the adapter performs discovery.
Discovery returns:

- Seaway contract version and compatible version range;
- stable adapter/runtime identity;
- supported operations: admit, start, inspect, observe, cancel, and optional
  approval response or resume;
- supported event families and terminal result states;
- event stream identity and scope plus ordering, replay, cursor retention, and
  deduplication guarantees;
- persistence and resume modes;
- workspace, tool, credential, network, egress, provider/model, resource,
  budget, usage, and cost capabilities, including their enforcement source;
- artifact and evidence reference schemes; and
- redaction and content-retention behavior.

Version negotiation uses a major/minor contract version plus named capability
flags:

- an unsupported major version fails closed before admission or ingestion;
- a newer compatible minor version may be used only for fields and capabilities
  the adapter understands;
- unknown fields are retained only as opaque namespaced metadata when safe and
  are never interpreted as authority;
- a required capability that is absent, ambiguous, or only advisory rejects
  admission for that requested run; and
- reconnect repeats discovery when the server/runtime identity or advertised
  contract version changes.

No Fairway release currently advertises a supported Seaway integration version.
The first adapter task must select and test the initial version against an
implemented Seaway contract rather than treating this design document as a
wire schema.

## Correlation Envelope

Every request, event, and result exchanged for the integration carries a
correlation envelope. Fields absent from a particular Fairway configuration are
omitted rather than invented.

| Field | Owner | Rule |
|---|---|---|
| `integration_version` | negotiated | Contract version used for this exchange. |
| `project_id` | Fairway | Stable Fairway project identity; never inferred from a filesystem basename. |
| `task_id` | Fairway | Durable task identity. Required for task-bound submission. |
| `lane` and `role` | Fairway | Optional configured coordination labels; not Seaway policy roles. |
| `worktree_ref` | Fairway | Repository identity, revision, branch, and safe path reference for the assigned durable worktree. |
| `session_id` | Fairway | Provider-neutral execution attachment created or selected for this run. |
| `caller_work_id` | Fairway/adapter | Stable trace correlation shared by related runs for one Fairway work item. |
| `submission_id` | Fairway/adapter | Stable idempotency identity for admission of exactly one intended run. |
| `run_id` | Seaway | Stable identity of the admitted run. |
| `prior_run_id` | Fairway/adapter | Optional link from a deliberate retry/restart to the earlier run; not a shared lifecycle. |
| `stream_id` | Seaway | Stable identity and documented scope for one ordered event stream, normally a run-scoped stream. |
| `event_id` and `sequence` | Seaway | Stable deduplication identity and monotonic order within `stream_id`. |
| `cursor` | Seaway | Opaque reconnect position bound to source identity, `stream_id`, and negotiated version; never parsed or synthesized by Fairway. |

`run_id` is not used as a Fairway task ID or session ID. The adapter stores the
mapping and includes both identities in every imported record. Repeating a
submission with the same `submission_id` must inspect or return the existing
admission rather than silently creating another run. A deliberate retry or
restart uses a new `submission_id` and therefore a new `run_id`, retains the
same `caller_work_id` and Fairway task correlation, and may name the prior run.

## Record Mapping

| Fairway record | Seaway relationship | Allowed adapter behavior | Prohibited inference |
|---|---|---|---|
| Task | Correlates zero or more runs | Export bounded intent, acceptance, and requested execution constraints. | Run terminal state changing task status. |
| Lane and role | Caller metadata | Include as namespaced correlation and routing context. | Treating a Fairway role as Seaway identity or policy authority. |
| Worktree | Input repository/revision reference | Pass a reviewed reference or request a separate run workspace derived from it. | Assuming the run workspace is the durable lane worktree or may mutate it. |
| Session | Attachment to a run | Upsert a provider-neutral session with Seaway `run_id` as external identity and the actual backend/provider labels. | Making the session own the task or survive a conflicting task binding. |
| Checkpoint | Readback of material run state | Record `active`, `awaiting_input`, or `done` summaries with run/event references. | Treating a checkpoint as a task transition or approval. |
| Evidence | Sourced run fact or safe reference | Record command/result or reference metadata with source, digest, redaction, and uncertainty. | Treating an output artifact, transcript, or model statement as verified evidence. |
| Usage | Sourced measurement | Record available provider/model/token/request/runtime facts with source and completeness. | Using estimated cost or model choice as a completion gate. |
| Review and readiness | No automatic mapping | Display correlated runtime facts as reviewer inputs. | Converting run approvals, policy decisions, or success into review approval or promotion readiness. |

## Operations

The adapter needs the following transport-neutral operations. Operation names
are descriptive rather than a mandated API spelling.

1. `DiscoverCapabilities` returns version, identity, supported operations,
   controls, event guarantees, and degradation state.
2. `AdmitRun` evaluates an immutable requested run specification and returns
   accepted or rejected admission, effective configuration, policy decisions,
   and `run_id` when allocated.
3. `StartRun` idempotently starts an admitted run or returns its existing state.
4. `InspectRun` returns current lifecycle, effective configuration, resume mode,
   reconnect metadata, and the run's terminal result when present.
5. `ObserveEvents` names the stable stream identity and scope, then returns
   ordered events after an opaque stream-bound cursor and a new cursor, with
   explicit end-of-retention or gap signals.
6. `CancelRun` idempotently requests cancellation and returns accepted,
   already-terminal, unsupported, denied, or indeterminate status.
7. `RespondToApproval` is optional and sends a bounded response only when the
   caller has explicit run-level authority and the approval is current. Each
   response carries a stable `approval_response_id`. Replaying the same ID and
   payload returns the original result; replaying the ID with a different
   payload is rejected as a conflict. If the runtime cannot provide that
   guarantee, the adapter must inspect approval state before retry and report an
   indeterminate result rather than risk a second decision.

Fairway core does not need to expose all operations as CLI commands. A future
adapter design must identify which actor can invoke each mutation, its dry-run
or confirmation boundary, and how the result becomes durable evidence.

## Submission And Session Attachment

The normal combined path is:

1. Fairway resolves a ready, explicitly claimed task and its lane/worktree.
2. The caller creates or selects a Fairway session attachment before material
   execution.
3. The adapter discovers capabilities and compares requested constraints with
   supported and enforced capabilities.
4. The caller reviews any degradation. Required unmet capabilities stop before
   submission.
5. The adapter submits a bounded run specification with the correlation
   envelope and per-run `submission_id`.
6. Seaway returns admission and effective configuration.
7. The adapter records the external `run_id`, effective capability summary, and
   an `active` checkpoint only after admission/start is confirmed.
8. Event ingestion updates checkpoints and records sourced facts while normal
   Fairway commands continue to own task, review, and closeout state.

Rejected admission is evidence about an attempted execution path. The adapter
records an `awaiting_input` checkpoint when owner action is required, ends or
updates the session honestly, and leaves task status unchanged unless the owner
explicitly chooses a normal Fairway transition.

## Event And Terminal-Result Ingestion

Events use at-least-once ingestion. The adapter must:

- deduplicate by stable Seaway source identity, negotiated version,
  `stream_id`, `run_id`, and `event_id`;
- reject or quarantine an event whose task/session mapping conflicts with the
  durable mapping;
- retain Seaway stream identity, ordering fields, and opaque cursor without
  renumbering events;
- persist each cursor under Seaway source identity, negotiated version, and
  `stream_id` (plus `run_id` when the stream scope is not inherently run-bound);
- advance only through the highest contiguous sequence for which every returned
  event has been durably recorded, safely ignored as a known duplicate, or
  durably quarantined with a visible error;
- commit the imported Fairway records and advanced cursor atomically where
  possible, or leave the cursor at the prior contiguous position and replay
  safely when the sink cannot provide atomicity;
- expose gaps, expired cursors, invalid signatures/digests, and unsupported
  event types rather than skipping them silently; and
- redact or omit raw prompts, transcripts, secrets, credentials, private tool
  bodies, and generated-content dumps by default.

Normalized lifecycle mapping is deliberately narrow:

| Seaway fact | Fairway readback |
|---|---|
| admitted or started | Session external identity plus `active` checkpoint. |
| running or heartbeat | Session heartbeat; checkpoint only when the material next action changes. |
| approval required or caller input required | `awaiting_input` checkpoint with approval/request reference, expiry, and responsible caller. |
| artifact available | Safe artifact reference; not evidence unless separately typed and sourced as evidence. |
| evidence available | Evidence reference with source, integrity, redaction, and uncertainty metadata. |
| usage or cost available | Usage record with measurement source, currency/pricing source when applicable, and exact/estimated/incomplete/unavailable status. |
| terminal success or partial success | `done` session checkpoint and terminal-result evidence; no task transition. |
| rejection, failure, cancellation, timeout, or infrastructure loss | Terminal or `awaiting_input` session checkpoint with classified reason and retained partial facts; no automatic task transition. |

A terminal result is accepted only once per `run_id`. A
later contradictory result is recorded as a conflict requiring owner action;
it does not overwrite the earlier record.

## Approval Handoff

Seaway approval requests are run-scoped facts. The adapter records:

- approval request identity and run correlation;
- requested operation and bounded subject;
- effective policy revision and enforcement source;
- expiry and consequences of no response;
- the caller identity or role expected to decide; and
- a safe reference to supporting context.

Fairway may surface the request through an `awaiting_input` checkpoint,
handoff, wait, notification, or dashboard readback. Those records prove routing,
not approval. A response may be sent only through an explicit caller action
that names the request, decision, and stable `approval_response_id`. The adapter
records delivery and Seaway's acceptance, rejection, prior identical result, or
idempotency conflict.

A Fairway review verdict cannot be reused as a Seaway approval response unless
an authorized human explicitly performs a distinct run-approval action. A
Seaway approval response can never be imported as a Fairway review verdict.

## Cancellation And Reconnect

Cancellation is a consequential runtime mutation owned by Seaway and the
authorized caller. A Fairway status change does not silently cancel a run, and
cancelling a run does not transition its task. A future convenience command
must name the exact `run_id`, show current state, and report whether cancellation
was accepted, already terminal, unsupported, denied, or indeterminate.

After adapter, caller, process, or network interruption:

1. rediscover capabilities when identity or version may have changed;
2. inspect the run by stable `run_id` and submission correlation;
3. resume the same `stream_id` after the last opaque cursor committed for the
   same Seaway source identity and negotiated version;
4. deduplicate replayed events;
5. detect cursor expiry or sequence gaps and use `InspectRun` to obtain a
   bounded current snapshot;
6. mark missing history as unavailable rather than manufacturing events; and
7. continue only if the Fairway session mapping still matches the task and the
   run is not terminal or superseded.

Runtime resume modes remain Seaway facts: native continuation of the same run,
Seaway-managed continuation of the same run, restart from checkpoint as a new
run, or unavailable. Fairway reports the mode and new-run boundary without
describing a restart as exact continuation.

## Failure And Degradation Semantics

| Condition | Required behavior |
|---|---|
| Seaway is not configured | Use Fairway's normal direct provider/session adapters. No warning or reduced Fairway readiness solely because Seaway is absent. |
| Discovery unavailable | Do not submit a new Seaway run. Record integration unavailability only when a requested Seaway path depends on it; allow another execution surface. |
| Incompatible major version | Fail closed before submission or ingestion and report supported/observed versions. |
| Required capability missing or advisory-only | Reject that requested run path before execution; never silently weaken the constraint. |
| Admission rejected | Record sourced rejection and requested/effective policy facts; leave Fairway task state to its owner. |
| Event connection lost | Keep the last cursor, mark observation degraded, reconnect with backoff, and avoid inferring run failure. |
| Cursor expired or event gap | Inspect current state, mark unavailable history explicitly, and require owner judgment if the missing interval affects an evidence claim. |
| Duplicate event/result | Return the prior ingestion result without creating duplicate Fairway facts. |
| Conflicting mapping or terminal result | Quarantine the new fact, expose the conflict, and require explicit reconciliation. |
| Invalid integrity or identity proof | Reject ingestion and record the verification failure without exposing unsafe content. |
| Seaway terminal result unavailable | End observation only when justified; record result unavailable and retain partial events/evidence. |
| Fairway unavailable during a run | Seaway continues according to its own caller/runtime policy. The adapter later reconnects and ingests from the last durable cursor. |

Integration failure is not proof of runtime failure, and runtime failure is not
proof that the Fairway task cannot proceed through another execution surface.

## Security, Privacy, And Evidence

The adapter stores metadata and safe references by default, not run content.
Credential values, provider tokens, raw prompts, raw transcripts, private tool
bodies, secret environment values, and unrestricted generated output must not
enter the Fairway store.

Every imported evidence or artifact reference should identify:

- source product and source identity;
- run and event or terminal-result identity;
- media/type and safe locator;
- integrity digest or signature status when available;
- redaction status and content classification;
- collection time and observed time;
- exact, estimated, incomplete, unavailable, or conflicting status; and
- the claim it may support, without asserting that the claim is accepted.

The adapter applies Fairway's existing sanitization, structured-output, and
store validation boundaries before recording data. Seaway remains responsible
for redaction before disallowed content leaves its run boundary.

## Implementation Gate

No adapter implementation should be scheduled until Seaway publishes a stable
or explicitly versioned experimental contract for discovery, admission, run
identity, ordered/replayable events, approval requests, cancellation, terminal
results, and evidence/usage references.

The first implementation task must include contract fixtures for:

- no-Seaway operation;
- compatible and incompatible version negotiation;
- one task with multiple runs, including a retry linked to its prior run;
- admission rejection and missing required capability;
- replay, duplicate, gap, expired cursor, and reconnect behavior;
- approval request routing without review inheritance;
- cancellation outcomes;
- success, partial success, failure, rejection, cancellation, timeout, and
  infrastructure-loss terminal states;
- unavailable and conflicting evidence; and
- proof that no runtime event changes Fairway task, review, or promotion state.

Until those prerequisites exist, Fairway should improve its generic provider
session and event adapter surfaces rather than introduce Seaway-specific core
state.
