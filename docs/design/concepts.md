# Concepts

This page owns Fairway's canonical vocabulary. Start with the common path and
open advanced references only when the work crosses their boundary.

## Common Path

Most reversible work needs six concepts.

### Task

A task is the bounded statement of intent: stable ID, title, owner role, status,
scope metadata, dependencies, and acceptance checks. The Fairway DB owns its
runtime state. A versioned backlog file may define task shape, but provider chat
does not.

### Session

A session is a replaceable execution attachment to a task. It identifies the
provider or utility currently doing work and its lifecycle state. Ending a
session does not erase the task; changing providers does not transfer hidden
authority.

### Decision

A task decision records a material choice, trigger, alternatives, chosen path,
rationale, risk, validation references, and cited Fairway facts. It explains
why work changed direction. It is not approval, evidence that an external fact
is true, or permission to merge, deploy, release, or act on a live system.

### Evidence

Evidence records a command or check, result, and optional safe artifact
reference. It supports or contradicts a claim. A generated summary, transcript,
or provider assertion is context unless it points to independently checkable
evidence.

### Review

A review is an attributable verdict for one configured domain. Required review
stays independent of the authoring lane. `approve`, `changes`, and `reject` are
judgments against the current task and evidence, not reactions in chat.

### Promotion

Promotion moves work from a cheap, reversible boundary to shared or
consequential state: remote branch, merge, release, deploy, public exposure, or
live execution. Fairway reports whether configured controls are satisfied; it
does not silently perform or authorize promotion.

## The Minimal Sequence

```text
task intent
  -> provider or utility session
  -> material decision when direction changes
  -> reproducible evidence
  -> independent review when policy requires it
  -> explicit promotion or local closeout
```

The compact CLI path is `work start`, `work verify`, and `work close`. The
[quickstart](../quickstart.md) proves that path before introducing advanced
coordination.

## Supporting Identity Terms

### Role

A role is a configured responsibility such as `backend`, `ui`, `ops`,
`architecture`, or `governance`. Tasks and review routes refer to roles.

### Lane

A lane is an optional active execution slot within a role. It is a coordination
and display label, not a permission model. A role can have an implementation
lane, review lane, or watcher lane without changing the role's authority.

### Actor

An actor is the identity recorded on state transitions and audit rows. It may be
a registered session identity or the local OS user and host. Actor attribution
does not grant additional authority.

### Task ID

A task ID is user supplied, stable, and validated by the configured pattern.
Fairway does not derive authority, risk, or state from a project-specific task
prefix.

## Advanced Concepts By Need

| Need | Concepts to add | Canonical reference |
|---|---|---|
| Resume long or interrupted work | checkpoint, context packet, track memory | [Checkpoints](checkpoints.md), [Context packets](context-packets.md) |
| Transfer or wait on ownership | handoff, notification, generic wait, completion handback | [Coordinator loop](coordinator-loop.md), [Provider notifications](provider-notifications.md) |
| Run related work together | hierarchy, work batch | [Hierarchy](hierarchy.md), [Work batch model](work-batch-model.md) |
| Apply project policy | workstream profile, gate, rule pack, review profile | [Workstream profiles](workstream-profiles.md), [Rule packs](rule-packs.md) |
| Govern a large code migration | migration execution profile, rule-pack completeness, verifier qualification | [Migration execution profile](migration-execution-profile.md) |
| Observe deterministic side work | watcher, utility session, deploy run | [Watchers](watchers.md), [Delivery resources](delivery-resources.md) |
| Coordinate a gated operation | live window, control room, closeout handback | [Live-operation control room](live-operation-control-room.md) |
| Share visibility across projects | registry, multi-project dashboard | [Multi-project mode](multi-project.md) |
| Coordinate shared writes | verified identity, command authorization, conflict guard | [Shared-team operating model](shared-team-operating-model.md) |
| Trace release or artifact lineage | provenance export, evidence retention | [Supply-chain provenance](supply-chain-provenance.md) |

These terms are not first-run requirements. Add them because a concrete
ownership, evidence, concurrency, or promotion boundary requires them.

## Authority Invariants

- Recording a task does not approve its execution.
- Recording a decision does not prove the decision is correct.
- Recording evidence does not make a failing result pass.
- Delivering a notification does not prove review or completion.
- A dashboard display does not gain mutation or approval authority.
- Advisory output does not accept risk or become provenance.
- Identity establishes accountability; command and policy checks establish
  permission.
- A task marked done is not automatically merge-, release-, deploy-, or
  live-ready.

The complete boundary is [Product boundaries](product-boundaries.md). Exact
storage and command contracts belong to [Schema](schema.md) and
[CLI](cli.md), not this concept map.
