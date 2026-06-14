# Context Packets

Context packets are structured prompts for bounded agent work. They came from
GPUaaS side-lane work where vague thread summaries caused scope drift,
forgotten constraints, and weak handoffs.

Fairway should support context packets as generated text artifacts. They do not
replace task definitions, state, evidence, or handoff records.

## Command

```bash
fairway packet context <task-id> \
  --goal <text> \
  --owner <role-or-lane> \
  --acceptance <text> \
  [--track <id>] \
  [--non-goal <text>] \
  [--current-state <text>] \
  [--architecture-context <text>] \
  [--environment <text>] \
  [--workflow-regression-pack <id>] \
  [--owned <path-or-system>] \
  [--must-not-touch <path-or-system>] \
  [--command <command>] \
  [--risk <text>] \
  [--stop-condition <text>] \
  [--deviation-rule <text>] \
  [--handoff-required=true]
```

The command prints Markdown by default and supports `--output <path>` when the
operator wants to persist the packet as an artifact.

## Packet Shape

```markdown
# Context Packet: T-042

task_id:
track_id:
owner_lane:
handoff_required:
workflow_regression_pack:

## Goal

## Non-Goals

## Current State

## Architecture Context

## Environment

## Workflow Regression Pack

## Owned Files / Systems

## Must Not Touch

## Commands To Run

## Acceptance

## Known Risks

## Stop Conditions

## Deviation Rules

## Handoff Format
```

## Rules

- A packet narrows scope; it does not expand the task.
- `must_not_touch` is a hard boundary unless the coordinator updates the task or
  records a handoff.
- Drift that blocks the task becomes a blocker or follow-up task.
- Useful but non-blocking drift becomes a follow-up task.
- Handoff should include files changed, commands run, pass/fail/skipped
  results, blockers, residual risk, and the next recommended slice.
- If a workflow regression pack is named, proof must either update/run that
  pack or record a release-blocking reason why it could not be covered.

## Relationship To Task Notes

Task `notes` are durable task context. A packet is an execution-time view of
that context plus current operating constraints. If a packet reveals durable
requirements, update the task notes or design docs rather than letting the
packet become the only source.

## Track Memory Packets

`fairway memory show|update|append|packet|stale` is the durable resume surface
for long-running tracks. It stores curated operating summaries and references to
Fairway source facts, not copied provider transcripts or raw prompt bodies.

Track memory records can include:

- current objective and active scope;
- decisions, blockers, open questions, and next actions;
- source checkpoint, evidence, and review ids.

`fairway memory packet --track <track-id>` renders that curated memory together
with current task, session, and checkpoint facts. The packet is a compact
provider-independent resume view. It does not approve work, expand scope, send
provider prompts, or mutate task state.

## Related Packets

Bug fixes use a narrower review packet because reviewers need root cause,
owning layer, proof, and regression coverage more than full execution context.
See [regression-packets.md](regression-packets.md).

Approval-gated consumer critical flows need a reviewer packet that includes the
causal goal, last blocker, allowed proof, forbidden actions, reviewed commands,
artifact paths, and next owner/action. See
[consumer-critical-flow-governance.md](consumer-critical-flow-governance.md) for
the reusable Fairway template.
