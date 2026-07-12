# Governed Agentic Engineering

Agent-driven delivery increases implementation throughput. It does not remove
engineering accountability.

Fairway supports an operating model in which coding agents can implement,
test, document, investigate, and review, while durable records preserve intent,
material decisions, evidence, independent judgment, and explicit promotion.
Provider sessions remain replaceable; consequential authority remains with the
configured engineering process and responsible people.

The canonical terms are in [Concepts](design/concepts.md). This page explains
why the model exists rather than redefining those terms.

## The Problem

When agent work is coordinated only through provider chat, a team can lose:

- the current owner and bounded scope;
- the reason a material path was chosen;
- the exact proof behind a completion claim;
- the difference between author confidence and independent review;
- the point where local reversible work became shared or consequential.

More provider context does not solve this. Context compacts, sessions stop,
providers change, and chat is difficult to audit as structured engineering
state.

## The Operating Model

1. **Bound intent.** A task names owner, scope, acceptance, and dependencies.
2. **Attach execution.** A provider or utility session reports that work is
   active without becoming the durable owner.
3. **Record material decisions.** Alternatives and cited facts survive context
   loss; generated rationale does not become approval.
4. **Verify claims.** Evidence names the command, result, environment, and safe
   artifact reference needed to reproduce the check.
5. **Require useful independent judgment.** Review follows risk and ownership
   boundaries rather than repeating ceremony for every reversible edit.
6. **Promote explicitly.** Merge, release, deploy, public exposure, credential
   use, and live action remain distinct from local completion.

Fairway makes this state visible through its CLI, execution store, packets,
guards, and read-oriented dashboards. Source control, CI/CD, issue systems,
identity controls, coding agents, and orchestrators continue to own their own
responsibilities.

## Human Comprehension

The standard is not that one person reads every generated line. The standard is
that a responsible engineer can reconstruct why a subsystem is built that way,
inspect the proof for consequential claims, and challenge a confidently wrong
review verdict.

Attention belongs where judgment has leverage:

- schema, migration, and state semantics;
- authentication, authorization, privacy, and tenant boundaries;
- public interfaces and compatibility;
- release, deploy, rollback, and live-operation paths;
- ownership transfers and provider trust boundaries.

## Useful Process, Not Maximum Process

Process is justified when it improves speed, quality, or safety. Reversible
work should move through lightweight evidence-led loops. Full review and
blocking gates belong at explicit live, production, security, credential,
release, deploy, compliance, enforcement, or public-exposure boundaries.

New process rules should begin as bounded advisory pilots with a stated
hypothesis. Delivery reports and defect-source evidence should show whether a
rule catches defects, reduces rework, shortens blocked time, or avoids unsafe
actions. Rules that do not help should be narrowed or removed.

## Independent Judgment

A review is useful when it contributes a relevant defect class or risk-control
perspective. It is not useful merely because another provider repeated the same
checks. Fairway records domain, reviewer identity, verdict, and wait state so a
team can distinguish independent judgment from same-lane confirmation.

External reviews, advisory models, and generated explanations are inputs. They
do not become product truth, provenance, or authority by being recorded.

## Explicit Promotion

Local implementation is cheap to reverse. Shared-state transitions are not.
Fairway keeps these boundaries visible:

```text
local work -> evidence -> required review -> merge-ready
  -> remote/merge -> CI or release verification -> deploy/live closeout
```

The chain is not automatic. Each external system remains authoritative for the
action it performs, and each consequential step retains its own approval and
rollback requirements.

## What This Model Is Not

It is not autonomous software delivery, transcript-as-authority, agents
approving their own work, a substitute for architecture ownership, a claim that
process proves correctness, or a reason to impose release ceremony on every
small task.

Fairway records and explains engineering control state. It does not silently
claim, approve, accept risk, merge, push, deploy, release, hold credentials, or
mutate live systems.

## Next References

- [Product](product.md): product promise and capability status
- [Concepts](design/concepts.md): canonical vocabulary and progressive map
- [Agent guide](agent-guide.md): operational commands and lifecycle
- [Review policy profiles](design/review-policy-profiles.md): risk-scaled review
- [Product boundaries](design/product-boundaries.md): durable authority limits
