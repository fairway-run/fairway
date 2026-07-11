# Task decision memory

## Purpose

Fairway preserves the context required to continue, review, and maintain agent
work without treating a provider transcript as the engineering record. The
model captures material decisions and their proof at the point where they
affect scope, risk, contracts, or operations.

This is the middle ground between sparse task/evidence rows and complete session
capture. It reduces dependence on provider context windows and survives context
compaction, provider replacement, and long-running work.

## Authority hierarchy

| Record | Question answered | Authority |
|---|---|---|
| Task and contract | What was requested and allowed? | Declared intent and boundary |
| Git diff and commit | What changed? | Authoritative implementation fact |
| Task decision | Why did a material choice or deviation occur? | Accountable curated explanation |
| Evidence, review, and CI | Why is the result accepted? | Verification and independent judgment |
| Task memory | What must the next provider know to continue this task? | Current resumable context |
| Track memory | What must persist across related tasks? | Curated operating context |
| Project memory and canonical docs | What principles and contracts govern the system? | Stable project authority |
| Transcript reference | What process detail may help a forensic investigation? | Optional, non-authoritative context |

No transcript, memory entry, or decision grants approval, merge, deploy,
release, credential, public-exposure, or live-operation authority.

## Decision record

A material decision records:

```yaml
decision: Centralize session authorization while adding order authorization
trigger: Handler-only enforcement left an existing bypass path
alternatives:
  - Add authorization only in the orders handler
  - Centralize session lookup and authorization
chosen: Centralize session lookup and authorization
reason: All handlers then share the same authorization boundary
scope_added:
  - packages/platform/iam/session_store.go
risk: Shared session behavior changed
validation:
  - session regression tests
  - authorization negative-path tests
fact_refs:
  - evidence:1842
  - checkpoint:912
  - commit:abc123
supersedes: ""
```

The record contains the explanation a future maintainer needs. It does not store
chain-of-thought, raw prompts, provider-private transcripts, raw tool bodies,
credentials, tokens, or generated-content dumps.

Decisions are doer-drafted explanations. They do not become authoritative merely
because a required field is present. Their quality state is one of:

| State | Meaning |
|---|---|
| `draft` | Recorded by the doer and available for continuation; not independently accepted. |
| `accepted` | An applicable reviewer or grouped review found the explanation concrete and consistent with the diff and facts. |
| `insufficient` | Present but generic, incomplete, contradicted by the diff, or missing material alternatives, risk, or proof. |
| `superseded` | Replaced by a later linked decision while remaining in immutable history. |

Review checks the trigger, credible alternatives when they existed, concrete
reason, added scope, risk, and validation references. Generic statements such as
"for maintainability" do not become accepted without explaining the actual
boundary or tradeoff.

## When a decision is required

Record a decision when work:

- changes files or ownership domains outside declared source/target paths;
- changes an API, event, schema, state machine, security boundary, or policy;
- selects between materially different implementation or operational options;
- accepts risk, debt, compatibility impact, or a temporary exception;
- changes the rollback, migration, deployment, or evidence plan;
- rejects a plausible alternative that a future maintainer may reconsider;
- supersedes an earlier task or track decision.

A decision is not required for local variable names, routine formatting,
mechanical generated output, or implementation details already implied by the
task and canonical design.

## Task memory

Task memory is a compact resume packet, not a diary. It contains:

- current objective and active scope;
- accepted decisions and unresolved questions;
- material deviations and risks;
- blockers and stop conditions;
- latest accepted evidence references;
- next concrete actions;
- current session, branch, commit, and environment readback when relevant.

Task memory is refreshed at meaningful transitions: start, material decision,
block/wait, handoff, verification, and closeout. Repeated progress narration is
not retained.

## Track memory lifecycle

Track memory is the non-canonical middle tier and therefore requires a forcing
function. Every active record has:

```yaml
owner: architecture
review_by: 2026-08-01
disposition: active
promotion_target: docs/design/example.md
source_facts:
  - decision:123
  - evidence:456
```

Allowed disposition states are `active`, `promote`, `archived`, and
`superseded`. Reconciliation reports missing ownership or source facts, stale
review dates, repeated decisions that may belong in canonical documentation,
conflicting or superseded facts, memory referenced by no active work, and
promotion debt by owner and age.

Fairway recommends `refresh`, `promote`, `archive`, or `supersede`, but never
deletes or promotes memory silently. A human or reviewed automation records the
disposition and target. Promotion completes only when the canonical document is
committed and linked.

Track memory participates in Fairway backup, export, restore, and shared-store
rehearsal. A local SQLite copy without tested recovery is not sufficient for
irreplaceable cross-task context.

## Promotion between memory levels

```text
task decision or task memory
        |
        | repeated or cross-task relevance
        v
track memory
        |
        | stable principle, contract, or operating rule
        v
canonical project documentation
```

Promotion is explicit. A task decision does not silently become architecture.
Canonical documentation remains the authority after promotion and links back
to the supporting Fairway facts where useful.

## Intent-to-diff verification

`work verify` should compare changed paths and ownership domains with declared
task scope. A material addition must be one of:

1. explained by a current task decision;
2. classified as generated or mechanical with deterministic proof;
3. added to task scope through the normal reviewed task-definition update; or
4. rejected and removed before closeout.

`work close` must report unexplained material deviation as a blocker. It must
not infer a reason from a transcript or synthesize a decision after the fact.

## Proportional enforcement

The first release is advisory for reversible work. It reports missing or
insufficient decisions without blocking local iteration. Blocking applies only
when an existing consequential profile already requires it, including security,
live, production, deploy, release, credential, public-exposure, irreversible,
or migration boundaries.

A broader closeout gate is promoted only after the pilot shows useful decision
quality, acceptable false-positive rates, improved resume or defect outcomes,
and lower or neutral total process cost. If the pilot shows hollow records or
closeout delay without outcome improvement, Fairway keeps the signal advisory
and improves automation or trigger precision instead of adding ceremony.

## Transcript posture

Transcript capture is optional and exceptional. When retained, Fairway stores
only a reference plus classification, owner, retention, access boundary, and
content hash. Transcript contents remain outside normal task detail, dashboard,
review, recipe, and context-packet output.

The forensic reference is useful when curated records and the diff cannot
explain an incident. It is not required for routine governance and cannot
replace a missing decision or missing evidence.

## Privacy and retention

- Decision and memory text use the same secret/private-data rejection rules as
  other bounded Fairway text surfaces.
- References are preferred over artifact content.
- Superseded decisions remain immutable and are projected as historical.
- Active task memory is replace-by-key curated state backed by source facts.
- Transcript retention must be explicit, short by default, and controlled by
  the system that stores the transcript.

## Adoption sequence

1. Document and use the decision template manually through existing evidence,
   checkpoint, and memory references.
2. Add a first-class bounded task decision command and read model.
3. Include current decisions in task detail, work status, and context packets.
4. Add intent-to-diff deviation detection to work verification.
5. Make unexplained material deviation a closeout gate after a measured pilot.

The pilot must measure useful decisions captured, authoring time, false-positive
deviation findings, hollow-but-present decisions, stale memory, promotion debt,
defects discovered, context-resume quality, and closeout delay. If the model
adds ceremony without improving these outcomes, keep it advisory and refine the
trigger rules.
