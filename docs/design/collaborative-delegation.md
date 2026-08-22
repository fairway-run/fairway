# Collaborative Problem-Solving And Governed Delegation

Fairway supports an engineering team as work moves between two modes:

- **collaboration**, while the problem, constraints, acceptance boundary, or
  verification method still needs to be discovered; and
- **delegation**, when one bounded slice has a clear owner, known dependencies,
  explicit acceptance, and an independently checkable result.

This is not another phase in the software delivery lifecycle. Architecture,
implementation, review, CI/CD, deployment, and operations keep their existing
responsibilities. The model governs how humans, agents, and utilities work
within and across those responsibilities without losing context or transferring
authority by implication.

The short formulation is:

> Collaborate on the problem. Delegate the bounded work. Challenge with
> evidence. Promote through explicit authority.

## Choosing The Working Mode

Use collaboration when one or more of these remain unsettled:

- the user or operator problem and the outcome that matters;
- product, architecture, security, lifecycle, or operational constraints;
- the owning technical boundary;
- dependencies, failure modes, recovery, cleanup, or residual risk;
- the acceptance claim; or
- how another person can verify the result without trusting the execution
  transcript.

Use delegation when the slice is bounded enough that an executor can act
without silently deciding the product boundary and a reviewer can verify the
result at proportionate cost. A capable model does not make a task delegatable
when its intent is ambiguous or its output is expensive to verify.

These modes are not Fairway task states. Fairway does not infer a
`collaborative` or `delegatable` status, and the choice does not grant approval,
merge, deploy, release, credential, or live-operation authority. A team may
record a material mode or boundary change as a task decision when it affects
scope, risk, ownership, or validation.

## The Control Loop

```text
collaborate on intent and constraints
  -> define the acceptance and verification boundary
  -> delegate one bounded execution slice
  -> verify with reproducible evidence
  -> challenge through independent judgment when required
  -> collaborate again if evidence changes the diagnosis
  -> promote through the authority that owns the consequential boundary
  -> retain the outcome and next safe action
```

Fairway primitives support the loop without becoming the work:

| Working need | Fairway record | Boundary |
|---|---|---|
| Preserve an evolving problem | task, decision, checkpoint, handoff, memory | Curated facts replace neither product judgment nor repository documentation. |
| Delegate a bounded slice | task packet, owner, dependency, acceptance, session, lane/worktree | The provider attachment executes; it does not redefine the task or inherit promotion authority. |
| Test the claim | evidence and external artifact reference | Evidence preserves source, result, uncertainty, and external ownership. |
| Challenge the result | routed independent review and review wait | A separate thread or subagent is not reviewer independence by itself. |
| Correct the diagnosis | new decision, evidence, checkpoint, and continued execution | A failed hypothesis changes the record; it need not create a new task when ownership and delivery boundary remain coherent. |
| Cross a consequential boundary | workflow, merge-ready, release, deploy, or live-operation guard | Fairway reports readiness; Git, CI/CD, operators, and other authorities perform and authorize their actions. |
| Continue later | outcome, completion handback, track memory, cited Quality Record | Provider transcripts remain optional context rather than the system of record. |

## GPUaaS Consumer Requirements

GPUaaS is the largest current source of concrete pressure on this model. Its
requirements are classified by where they belong rather than copied into
Fairway core.

| GPUaaS requirement | Fairway disposition | Current mechanism or owner |
|---|---|---|
| Durable product and architecture decisions across provider replacement | Supported | Tasks, decisions, checkpoints, track memory, handoffs, and cited source references. |
| Material delegated work must be visible while it runs | Supported | Provider sessions plus active checkpoints; short direct coordinator work remains the bounded exception. |
| Independent review must be distinct from implementation | Supported | Reviewer identity, configured review domains, immutable verdicts, and no-self-review enforcement. |
| Evidence can disprove the initial diagnosis and redirect work | Supported | Conflicting/failing evidence, `changes` verdicts, superseding decisions, checkpoints, and continued work within one coherent task. |
| Completion must not imply merge, deploy, release, or live readiness | Supported | Workflow, merge-ready, release, deploy, and live-operation guards preserve separate authority. |
| Gates should be selected by changed surface and risk | Configurable | Workstream profiles, review policy profiles, rule packs, task metadata, and evidence expectations. Projects own the actual rules. |
| Feature completeness must cover applicable API, UX, CLI, SDK, lifecycle, operations, audit, recovery, and cleanup surfaces | Project-owned | GPUaaS owns its completeness model and matrix. Fairway can record the task boundary, rules, evidence, and review but does not define a universal feature checklist. |
| User and operator flows must map dependencies before broad UAT or live work | Project-owned | GPUaaS owns flow rows, environment prerequisites, and domain-specific preflight. Fairway records dependencies, waits, evidence, owners, and promotion guards. |
| API-first operational verification and domain error/event policy | Project-owned | Repository contracts and rule packs own these technical policies; Fairway must not encode one product's architecture as core grammar. |
| Coordination overhead must remain proportionate | Supported and measured | Coordination-budget guidance, compact `work` commands, delivery reports, structured friction, and control-effectiveness readbacks. |
| Exact prefixes such as `UAT-BUG-*`, `OPS-FIX-*`, and `HARNESS-FIX-*` | Project-owned | Workstream profile taxonomy; prefixes never acquire core state or authority semantics. |

No new Fairway implementation requirement follows from this first disposition.
The existing primitives cover the generic needs, while GPUaaS-specific policy
belongs in its repository and reusable rule packs. A future product task is
warranted only when another consumer or measured pilot shows that a recurring
fact cannot be represented or read back without prose conventions. Candidates
such as structured cleanup ownership, residual-gap ownership, or a
delegation-readiness advisory must meet that evidence threshold before becoming
core schema or workflow.

## Anti-Goals

This model must not:

- turn collaboration into mandatory meetings, permanent chats, or a new role;
- turn delegation into a claim that one prompt fully specifies the work;
- classify model capability as task readiness;
- make review volume or record volume a proxy for quality;
- create a workflow engine or universal SDLC methodology; or
- let execution output approve itself or grant promotion authority.

See [Concepts](concepts.md) for canonical record definitions and
[Product boundaries](product-boundaries.md) for the complete authority model.
