# Consumer Critical-Flow Governance Template

Fairway can give consumer repositories a repeatable governance template for
approval-gated, user-visible, operator-visible, security, deploy, UAT, and live
environment flows. The template keeps coordination state in Fairway while the
consumer repo owns product scripts, fixtures, runbooks, and evidence contracts.

The durable rule is:

```text
Flow map before implementation.
Non-live preflight before live window.
Bounded retry before causal reset.
Fairway evidence before handoff.
```

## Boundary

Fairway owns reusable coordination primitives:

- task state, sessions, checkpoints, reviews, notifications, and evidence;
- context packets, track memory, wait/watch rows, and wake delivery records;
- reviewer packet shape, routability validation, retry packet shape, and
  causal-reset policy language;
- deterministic read models and dashboard projections.

The consumer repo owns product-specific material:

- flow maps and persona entry points;
- product scripts, fixtures, identities, browser flows, APIs, and CLI commands;
- environment, DNS, edge, deploy, rollback, and cleanup runbooks;
- evidence contracts and redaction tests for that product;
- live execution authorization and risk acceptance.

Fairway may recommend, render, validate, and record packets. It does not approve
reviews, accept risk, merge, deploy, mutate environments, authorize live
execution, or decide that a product flow is safe.

## Critical-Flow Row

Before implementation, broad UAT, deploy validation, or live drill scheduling,
the consumer repo should maintain a critical-flow row with these fields:

```text
flow_id:
title:
persona:
canonical_entry_point:
criticality: P0|P1|P2
owning_team:
task_or_epic_id:
happy_path:
empty_path:
blocked_path:
recovery_path:
negative_path:
cleanup_path:
contract_owner:
runtime_owner:
fixture_owner:
identity_owner:
permission_owner:
provider_owner:
dns_edge_owner:
browser_owner:
ci_cd_owner:
environment_owner:
non_live_preflight_command:
non_live_preflight_artifacts:
rollback_owner:
rollback_artifacts:
evidence_owner:
accepted_residual_gaps:
no_go_conditions:
```

The row should be product-owned and versioned with the consumer repo. Fairway
can link to it from task notes, context packets, evidence, or track memory, but
the flow row is not a substitute for Fairway task state.

## Non-Live Preflight

A non-live preflight must run before a live window or broad UAT can use the flow
as evidence. It should be checked in, reproducible, and reviewed.

Minimum shape:

- runs without source or production mutation unless the packet explicitly names
  an isolated disposable target;
- validates setup and readback evidence before browser, credential, token,
  break-glass, or sensitive-operation steps;
- fails closed with sanitized findings;
- proves rollback or cleanup for disposable resources;
- emits stable JSON or Markdown artifacts with redaction self-test evidence;
- avoids one-off scripts for credential submission, browser automation, or
  provider mutation.

Fairway evidence should cite the exact command, artifact paths, result, and
reviewer packet that allowed the evidence to be used for the next handoff.

## Reviewer Packet

Critical-flow reviewers need causal context, not only a narrow diff. A reviewer
packet should include:

```text
causal_goal:
last_blocker:
allowed_proof:
forbidden_actions:
reviewed_commands:
reviewed_artifacts:
preflight_artifacts:
rollback_artifacts:
redaction_artifacts:
residual_risk:
no_go_conditions:
next_owner:
next_action_if_pass:
next_action_if_fail:
fairway_task_id:
fairway_evidence_ids:
fairway_checkpoint_ids:
```

The packet should be generated or copied from current Fairway and consumer-repo
facts. It must not rely on provider chat memory as the only place where the
goal, last blocker, allowed proof, forbidden actions, artifact paths, or next
owner/action exist.

## Retry Budget

Approval-gated reruns must be bounded. A meaningful failure is a failure after
the approved preflight path reaches the behavior being tested. Coordination-only
failures, such as stale session cleanup, missing review packet metadata, or a
missing handoff notification, do not count against the product behavior retry
budget, but they still require Fairway evidence and may require a coordination
follow-up.

Default policy:

- after the first meaningful failure, create a scoped blocker task;
- after the second meaningful failure, verify the causal model before another
  rerun packet;
- after the third meaningful failure, stop narrow reruns and create a causal
  reset task before requesting another live or disposable rerun.

A causal reset should classify the likely owning layer:

```text
product
provider_semantics
harness
environment
execution_surface
review_packet
coordination_model
unknown
```

The reset must name the next proof required before another approved live or
disposable rerun.

## Fairway Evidence Before Handoff

Before a critical-flow task hands off to another owner, Fairway should contain:

- the current task status and active session or closeout checkpoint;
- the latest reviewer packet or context packet;
- preflight, rollback, redaction, and setup/readback evidence paths;
- review verdicts and notification delivery state;
- the current wait/watch or live-operation handoff row when parked;
- the next owner and next safe action.

The handoff should be durable enough that a replacement provider or human can
resume from Fairway without polling chat, reconstructing a missed handoff, or
spending most of a provider turn remembering scheduler state.

## Consumer Adoption Checklist

1. Add a product-owned critical-flow row file or section.
2. Link the row from the Fairway task, packet, or track memory.
3. Add a non-live preflight command and artifact contract.
4. Add rollback or cleanup proof for disposable resources.
5. Route reviewers with the reviewer packet fields above.
6. Record Fairway evidence before handing off to the next owner.
7. Use the retry budget before requesting another live or broad UAT window.

