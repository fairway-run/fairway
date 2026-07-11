# Common-Path Automation And Progressive Disclosure

## Decision

Fairway keeps task, session, checkpoint, evidence, review, handoff, batch, and
promotion records as separate durable primitives. Routine users should not have
to operate every primitive manually.

The product provides a compact `fairway work` lifecycle for normal reversible
engineering and progressively reveals primitive-level controls when work is
ambiguous, blocked, consequential, or under audit.

```text
common path                     advanced path
fairway work start              task/session/checkpoint commands
fairway work status             task-detail/reconcile/review-policy
fairway work verify             evidence/batch/rule commands
fairway work close              merge-ready/status/session close

                         consequential boundary
                  explicit review, approval, live window,
                     deploy, release, or operator action
```

The common path is an atomic convenience layer over the same store. It is not a
second workflow model.

## Product Objective

A developer or agent making a small reversible change should need to answer
four questions:

1. What am I working on?
2. What proof did I produce?
3. Is anything blocking closeout?
4. What is the next action?

Fairway should infer mechanical context such as repository, task attachment,
provider session, existing active checkpoint, and applicable review profile
when exactly one safe answer exists. It must fail closed and show the competing
choices when inference is ambiguous.

An auditor must still be able to recover the complete chain of task ownership,
provider attachment, state transitions, evidence, reviews, handoffs, commit,
CI/deploy references, and promotion decisions.

## Three Disclosure Levels

### Common

The common surface is optimized for one task, one provider attachment, and one
reversible implementation slice:

```bash
fairway work start FW-292
fairway work status
fairway work verify --result pass --command-text "go test ./..."
fairway work close --commit <sha>
```

Output leads with task, lifecycle state, risk boundary, blockers, and one next
command. JSON output retains stable field names for agents and automation.

### Advanced

Primitive commands remain available for multiple sessions or providers,
explicit checkpoints, grouped batches, handoffs, waits, review routing, rule
packs, reconciliation, audit, and diagnostic detail. `work status --explain`
links each compact field to its primitive records and suggests the exact
advanced command for investigation.

### Consequential

The compact lifecycle stops and requires explicit controls for:

- live, production, deploy, release, credential, security, compliance,
  public-exposure, or irreversible markers;
- authority expansion or gate weakening;
- ambiguous task, project, provider, owner, or risk inference;
- required independent review or approval;
- destructive cleanup, migration, rollback, or external-system mutation.

No `fairway work` command creates a review, records an approval, accepts a live
window, pushes Git, triggers CI, deploys, releases, sends provider prompts,
uses credentials, or mutates an external system.

## Primitive Mapping

| Common command | Existing records used |
| --- | --- |
| `work start` | task definition/state/history, provider session, active lifecycle checkpoint |
| `work status` | task detail, effective review policy, session status, latest checkpoint, evidence summary, waits, reconciliation findings |
| `work verify` | task or batch evidence reference; optional UX media classification through the existing evidence model |
| `work close` | workflow/merge-ready/reconcile checks, explicit task status decision, provider session terminal state |

The commands call shared store/service functions used by primitive commands.
They do not shell out to the Fairway CLI or write a parallel lifecycle row.

## Start Contract

`work start` performs one atomic coordination operation:

1. resolve an explicit task id, `FAIRWAY_TASK_ID`, or exactly one claimable
   task for the requested role;
2. reject terminal tasks and ambiguous candidates;
3. create or refresh one provider session attachment;
4. claim or transition the task to `in_progress` through the configured state
   machine;
5. record one `active` lifecycle checkpoint tied to the task/session;
6. return compact status and the next command.

Task id, role, provider/backend, external session id, or session id become
required when they cannot be inferred safely. Repeated calls with the same
task/session identity are idempotent. Conflicting identity or task scope fails
closed.

## Verify Contract

`work verify` records a bounded fact. It does not become a CI runner. The caller
supplies command text or a named gate, result, optional artifact/type, duration,
and privacy-safe notes. Fairway stores references and summarized facts, not raw
command output by default. Secrets, tokens, prompt bodies, transcripts, raw
tool bodies, and unredacted user data remain forbidden.

## Close Contract

`work close` is guarded composition, not automatic approval:

1. show effective review and evidence requirements;
2. run existing merge-ready, workflow, and reconcile checks;
3. reject closeout when required review, evidence, handoff, source state, or
   active reconciliation remains;
4. record the explicit terminal task decision and commit when supplied;
5. end the matching provider session;
6. return the next ready task or a clean-idle result.

The command never synthesizes missing evidence or review. Consequential tasks
must use their explicit boundary workflow before `work close` can succeed.

## Risk Inference

Inference is deterministic and explainable. Inputs are existing task profile,
risk, kind, tags, paths, ownership, parent/batch relationship, effective review
policy, and rule-pack matches. Configured and built-in boundary profiles always
override convenience. Changed-file inference may recommend policy but must not
mutate it silently.

## Output Contract

Default output is compact:

```text
FW-292  in_progress  reversible
session: codex-fw292 (running)
proof: 1 pass, 0 fail
review: not blocking
next: fairway work verify ...
```

Blocked output leads with the blocker, boundary, remedy, and an explain link.
JSON includes task/status, profile/risk, session, checkpoint, evidence summary,
effective review rows, blockers, boundary markers, next action, and command.

## Recovery And Idempotency

Partial operations must not leave hidden active work. Start uses one store
transaction for task/session/checkpoint writes. If an existing primitive cannot
participate, the first implementation fails before mutation. Recovery uses
`work status --explain`, `reconcile active --dry-run`, and explicit primitive
repair commands. `work start --resume` succeeds only for matching identity.

## Measurement And Promotion

The common path starts opt-in. Compare it with the primitive path:

| Measure | Desired result |
| --- | --- |
| Manual Fairway commands per reversible task | materially lower |
| Time from claim to visible active work | lower |
| Unattended active tasks or missing checkpoints | no increase |
| Evidence completeness and rollback confidence | equal or better |
| Defects caught and reopen/retry count | no regression |
| Review wait on reversible work | lower |
| Consequential-boundary bypasses | zero |
| Hollow-but-present decisions | low and decreasing |
| Stale track memory and promotion debt | bounded with named owners |
| Decision authoring and closeout delay | lower than the value of defects or resume gaps prevented |

Promotion requires evidence that effort falls without hiding state or weakening
review, release, security, or operational boundaries.

The first measured pilot is recorded in
[`../assessment/fairway-common-path-pilot-2026-07-11.md`](../assessment/fairway-common-path-pilot-2026-07-11.md).
It found lower lifecycle and closeout time with complete session/checkpoint/
evidence coverage, but higher review/notification overhead and no labeled
intent-to-diff false-positive sample. The resulting policy is to keep
reversible-work deviation findings advisory and collect classifier outcomes
before considering a blocking promotion.

Decision-memory and intent-to-diff findings remain advisory for reversible work
during the pilot. They become blocking only at existing consequential boundaries
until measured evidence justifies broader promotion.

## Decision Memory

The common path uses the task decision and memory model in
[`task-decision-memory.md`](task-decision-memory.md). Routine work does not need
a narrated transcript. Material scope, risk, contract, security, migration, or
operational choices require a concise decision linked to supporting facts.

`work verify` identifies material changed scope that is not covered by the task
or a decision. `work close` reports unexplained deviations as blockers after the
decision-memory pilot establishes acceptable precision. Git remains the
authority for what changed; the decision explains why.

## Anti-Goals

This work does not turn Fairway into an LLM proxy, CI/deployment engine, IAM or
credential system, issue tracker replacement, generic workflow engine,
autonomous approval authority, or second simplified lifecycle store.
