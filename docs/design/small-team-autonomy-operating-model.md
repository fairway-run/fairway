# Small-Team Autonomy Operating Model

Fairway is meant to shorten the build-use-learn loop for small autonomous
lanes without moving real risk out of explicit control. The product capability
is generic: task state, sessions, evidence, review profiles, waits, handbacks,
reports, and dashboard projections. A small-team consumer is one operating practice that
uses those capabilities for MFA drills, demos, docs portal updates, VM restore
work, airgap/staging rehearsals, and release coordination.

Material choices and scope deviations follow the
[`task decision memory`](task-decision-memory.md) model. This keeps replacement
providers productive after context compaction without requiring full transcript
retention. Small reversible implementation details remain lightweight; shared
contracts, security boundaries, operational choices, risk acceptance, and
unexpected changed scope receive a concise decision plus supporting facts.

## Operating Principle

Move reversible work quickly with evidence. Slow down only at boundaries where
extra process improves speed, quality, or safety.

Use lightweight Fairway state for:

- docs, local dashboard, command help, report, fixture, harness, readback,
  setup, classifier, and provider-shape fixes;
- prototype-first product or UX work where the fastest way to learn is to use a
  thin slice;
- grouped child slices that share one risk boundary, branch, validation packet,
  or release review;
- rough edges found while using the product.

Use explicit blocking controls for:

- live windows, deploy, release, public exposure, production-readiness,
  compliance, enforcement, credential handling, security boundary changes,
  safety-gate changes, and irreversible migration semantics;
- any slice that expands authority or weakens a configured gate;
- any boundary where review is expected to catch a concrete defect class or
  prevent an unsafe action.

Review ceremony without defect discovery is process overhead. `fairway delivery
report` and `fairway review-policy report` should be used to challenge policies
that add wait time without improving outcomes.

## Fairway Capability Versus Consumer Practice

Fairway product capabilities are reusable:

- review policy profiles classify reversible, prototype-first, grouped,
  irreversible, live-boundary, and release-boundary work;
- task evidence records proof without storing raw secrets, transcripts,
  provider-private data, or artifact contents;
- `task-detail` and dashboard detail show UX media evidence references;
- `rough-edge add` and `rough-edge list` keep found-while-using gaps visible;
- `delivery report` measures review/gate overhead, defect source, reopen/retry
  counts, handoffs, notification counts, and blocked/review-wait time;
- workflow and reconcile guards catch dirty source, stale sessions, active
  evidence without closeout, and deploy-boundary gaps.

Consumer practice is the local usage pattern:

- MFA drill packets use Fairway state for approval, execution handoff,
  rollback proof, closeout, follow-up assignment, and retry decisions;
- demo and UAT loops record owner usage proof, screenshots, browser traces, and
  rough edges before deciding whether to harden or defer;
- docs portal and dashboard changes move as reversible product slices until a
  release or public-exposure boundary is reached;
- VM-104 restore and airgap/staging rehearsal work use packets and evidence to
  prove readiness before user handoff;
- tmux or SSH execution lanes are used when Desktop cannot control git,
  long-running servers, stale dashboard listeners, or sandboxed Go caches.

The practice can change without changing Fairway's trust boundary. Fairway
does not authorize live execution, approve risk, merge, deploy, push, or send
dashboard-originated provider messages.

## Normal Lane Loop

1. Claim one task or a grouped batch.
2. Register a provider session and record an active checkpoint.
3. Implement a reversible slice or bounded packet.
4. Record evidence as the source of truth, including commands, artifacts, UX
   media references, rough edges, and validation output.
5. Run focused tests plus workflow/reconcile guards appropriate to the slice.
6. Use review profiles to decide whether review is advisory, grouped,
   inherited, waived, deferred, or directly blocking.
7. Commit a coherent slice after evidence and required review pass.
8. Mark the task done or blocked with a status decision and close the session.

This loop keeps provider chat replaceable. A lane can restart from Fairway
task detail, evidence, checkpoints, reviews, sessions, and reports instead of
reconstructing state from chat memory.

## Reversible Work

Use the `reversible` profile for small non-live changes where the cost of a
mistake is low and rollback is straightforward. Examples:

- command help and docs wording;
- local dashboard display-only polish;
- fixture, mock, and harness improvements;
- readback/report formatting;
- setup scripts that do not touch credentials or live environments.

Expected proof is evidence-led:

- command run and result;
- artifact reference if useful;
- owner or operator readback when user-facing;
- workflow/reconcile guard output at closeout.

Do not require a full matrix merely because the task has a small code diff.
Ask what defect class an extra reviewer is expected to catch.

## Prototype-First Work

Use `prototype-first` when the unknown is product shape, UX flow, workflow fit,
or integration feel. Build a thin reversible slice, use it, record what
happened, and then choose one of:

- stabilize it;
- keep iterating in the safe boundary;
- discard it;
- escalate to a stricter profile because the next step exits the boundary.

Record evidence types such as:

- `prototype-artifact`;
- `owner-usage-proof`;
- `prototype-gap-list`;
- `stabilization-decision`;
- `screenshot`, `video`, `browser-trace`, or `uat` artifact references when
  the proof is visual or owner-facing.

Prototype-first is not a shortcut for live, deploy, release, credential,
security, public-exposure, production, or irreversible work.

## Grouped Review

Use grouped review when several child tasks share the same branch, proof
surface, defect class, or release boundary. Small child tasks should not each
trigger full ceremony when the actual risk is the grouped slice.

Good grouped-review candidates:

- docs plus dashboard wording for the same user-visible behavior;
- harness and fixture fixes validated by one command set;
- stale-cleanup or report formatting changes with shared screenshots;
- one feature slice split across CLI, dashboard template, tests, and docs.

Do not inherit grouped approval for boundary children. Live, release,
irreversible, credential, deploy, security, production, and public-exposure
markers force direct review even if the child carries a grouped tag.

## Rough-Edge Capture

Use `fairway rough-edge add` when real use finds a product gap that should not
disappear into chat:

```bash
fairway rough-edge add \
  --task FW-244 \
  --owner ui \
  --severity high \
  --decision fix-now \
  --summary "Owner could not find release status" \
  --expires 2026-07-01 \
  --artifact artifacts/walkthrough.png
```

The queue is projected from evidence, not a second store:

```bash
fairway rough-edge list
fairway rough-edge list --expired
```

Use `fix-now` when the rough edge blocks the current flow or would make the
next user handoff unsafe or misleading. Use `defer` when the rough edge is
real but can wait without losing the learning. Expiry keeps deferred feedback
from becoming invisible stale debt.

## Measurement

Use metrics to remove or narrow process that is not helping:

```bash
fairway delivery report --since 168h
fairway review-policy report
fairway automation candidates --since 168h
```

Useful outcomes include defects caught, rework reduced, blocked time reduced,
cycle time improved, repeated manual checks automated, and unsafe actions
avoided. If a review or gate repeatedly adds delay without these outcomes, run
it as advisory, narrow it to the defect class it actually catches, or remove it
from the default path.

Loop signals are step-back triggers. Repeated meaningful failures after
near-ready claims, repeated same-layer fixes, or approvals that do not move the
end-to-end flow forward should trigger a causal reset with:

- failure chain;
- real unknowns;
- proof required before retry;
- a lighter safe-boundary review plan.

## Shared-Team Pilot Evidence

The first shared-team pilot is recorded in
[fairway-small-team-shared-pilot-2026-07-06.md](../assessment/fairway-small-team-shared-pilot-2026-07-06.md).
It proves the loopback read-only shared surface can expose status, task, and
report readback without write authority. It does not promote shared-team
support by itself; the recommendation is to repeat the pilot with a
non-authoring operator on the Mac mini GitLab lab host before treating the mode
as supported.

## Consumer Examples

MFA loop:
: Use Fairway packets for exact live windows, approval readback, operator
  active state, rollback proof, final closeout, and next follow-up. Do not use
  live drill/UAT as the first dependency discovery surface. Run non-live
  preflight first, record proof, then enter the live boundary with explicit
  authorization.

Demo and UAT:
: Use prototype-first or reversible profiles for the display/readback slice.
  Record screenshots, browser traces, owner usage proof, and rough edges. Move
  to stricter review only when the slice changes public exposure, release
  behavior, credentials, or production data.

Docs portal:
: Treat docs wording, navigation, screenshots, and release-note corrections as
  reversible until publication or release tagging becomes the boundary. Capture
  screenshots and stale portal findings as evidence instead of relying on chat
  memory.

VM-104 restore:
: Use environment rehearsal packets and evidence before handoff. Record the
  restore command, readback proof, blocker owner, and next safe action. Escalate
  only when the restore crosses production, credential, or user-impacting
  boundaries.

Airgap and staging rehearsal:
: Use packetized preflight and rehearsal results before user handoff. Late
  route/runtime/worker-access failures should become evidence and follow-up
  tasks, not ad hoc chat instructions.

tmux/CLI execution lanes:
: Use tmux or SSH lanes when Desktop cannot kill stale dashboard listeners,
  control git index locks, maintain long-running processes, or use writable Go
  caches. Record the command, lane, evidence, and status decision back in
  Fairway so the execution surface is replaceable.

## Anti-Patterns

- Asking for full review on every small reversible child task without a defect
  class or risk-control value.
- Waiting on every task when a grouped slice or advisory profile would preserve
  the real boundary.
- Treating a live drill or UAT window as first dependency discovery instead of
  running non-live preflight.
- Letting rough edges, screenshots, rollback proof, or owner feedback live only
  in chat.
- Recording evidence while active and then leaving the task without a status
  decision.
- Using dashboard visibility as if it were send, approval, merge, deploy, or
  live-operation authority.
- Letting an LLM reconstruct next actor, deadline, and closeout state when
  Fairway can hold that durable coordination state.

## Escalation Boundaries

Escalate to direct review and explicit authorization when a slice:

- mutates live, production, staging-with-users, or credentialed environments;
- changes release, deploy, public exposure, or trusted proxy behavior;
- weakens safety gates, review gates, audit boundaries, or redaction policy;
- introduces persistence/schema compatibility risk;
- expands provider-send, dashboard-write, approval, merge, or deploy authority;
- repeats failures without a causal model.

These boundaries are where process earns its keep. Everything else should move
as fast as evidence, tests, owner use, and clean closeout allow.
