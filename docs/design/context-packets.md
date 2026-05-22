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

## Goal

## Non-Goals

## Current State

## Architecture Context

## Environment

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

## Relationship To Task Notes

Task `notes` are durable task context. A packet is an execution-time view of
that context plus current operating constraints. If a packet reveals durable
requirements, update the task notes or design docs rather than letting the
packet become the only source.
