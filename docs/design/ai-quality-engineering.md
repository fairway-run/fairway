# What Is AI Quality Engineering?

AI Quality Engineering is the practice of designing engineering systems that
consistently produce trustworthy AI-assisted software through controlled
intent, qualified verification, proportional human judgment, and
outcome-driven improvement.

Its central question is not whether an AI agent produced code, nor whether a
person reviewed every generated line. It asks:

> What evidence justifies trusting this engineering artifact for its next use
> or promotion boundary?

This is a quality-systems discipline. Agents, prompts, retrieval, memory,
workflows, and models are parts of the production process. They matter because
they affect quality, not because they define quality.

Tracked by Fairway task `FW-389`. This document defines principles only. It
does not specify a Fairway implementation, API, storage model, or roadmap.

## What Quality Means

Quality is fitness for an explicitly bounded engineering purpose. It includes:

- correctness against stated intent and contracts;
- safety, security, privacy, and authority compliance;
- operability, diagnosability, maintainability, and recoverability;
- reproducible evidence for consequential claims;
- acceptable behavior after integration and deployment;
- known residual risk appropriate to the next promotion boundary.

Quality is not equivalent to test passage, reviewer approval, model confidence,
or absence of a reported incident. Each is evidence with limits. Trust comes
from a coherent body of independently inspectable evidence.

Because AI-assisted production is probabilistic, the same input may not produce
the same artifact or reasoning path. The quality system therefore controls and
records the conditions needed to evaluate the result without pretending that
generation itself is deterministic.

## The Quality Record

The durable unit is a **Quality Record** for a bounded engineering change or
artifact:

```text
Intent
  -> material decisions
  -> production context
  -> collected evidence
  -> automatic verification
  -> human judgment where required
  -> promotion decision
  -> operational outcomes
  -> lessons and controlled improvement
```

A Quality Record is not a transcript and not a generated narrative. It is an
inspectable projection of durable facts and cited artifacts. At minimum, it
should make the following answerable:

- What was intended, for whom, and within what boundary?
- Which material decisions and assumptions shaped the work?
- What produced the artifact, using which relevant source and policy versions?
- Which checks ran, what did they establish, and what could they not establish?
- Which verifier or reviewer supplied each judgment, and was it qualified for
  that judgment?
- What promotion was requested, approved, or denied, and by whose authority?
- What happened after promotion?
- Which lessons changed the production or assurance process?

The underlying authorities remain where the facts originate: source control,
CI/CD, incident systems, deployment systems, reviewed policy, and accountable
human decisions. A Quality Record links and explains those facts; it does not
replace their authority.

## Evidence And Verification

Evidence should be matched to the claim it supports. Useful classes include:

1. **Deterministic evidence**: compilation, contract checks, type checks,
   invariant checks, signatures, policy evaluation, and reproducible tests.
2. **Empirical evidence**: integration tests, simulations, browser journeys,
   load tests, failure injection, deploy probes, and operational readback.
3. **Evaluative evidence**: rubric-based assessment of properties that cannot
   be reduced to a deterministic assertion.
4. **Human judgment**: accountable interpretation of ambiguity, trade-offs,
   residual risk, and consequential authority boundaries.
5. **Outcome evidence**: incidents, rollbacks, corrective work, reopens,
   regressions, user impact, and sustained operating behavior.

More evidence is not automatically better. Evidence must be relevant,
independent enough for the claim, current, attributable, reproducible where
possible, and bounded against leakage or gaming.

A verifier is itself part of the quality system. Tests, scanners, evaluators,
rubrics, model judges, and reviewers need known applicability and limitations.
Where practical, they should be qualified against known-good and deliberately
bad fixtures. A passing unqualified verifier is weak evidence.

## Automatic Checks And Human Judgment

AI Quality Engineering does not remove human review. It changes where scarce
human judgment is most valuable.

Automatic checks should establish properties that can be defined and repeated
reliably. Human review should focus on unclear intent, architecture, novel risk,
trade-offs, verifier adequacy, evidence gaps, and authority decisions. Critical
implementation details may still require direct inspection when risk or weak
verification warrants it.

The goal is proportional assurance, not universal line-by-line inspection:

- deterministic invariants for non-negotiable boundaries;
- qualified automated verification for repeatable properties;
- risk-based human judgment for ambiguity and consequence;
- statistical sampling for sufficiently repeated processes;
- longitudinal monitoring for outcomes that appear only after promotion.

No model, dashboard, aggregate score, or generated recommendation silently
gains authority to approve, merge, deploy, release, accept risk, use
credentials, or mutate a live environment.

## Process Capability, Not Process Theater

Manufacturing quality systems provide a useful principle: reliable output comes
from capable processes, not inspection alone. Software differs from repetitive
manufacturing, however. Engineering tasks vary widely, requirements evolve,
interactions are complex, and defects may remain latent.

AI Quality Engineering therefore uses process evidence without assuming every
artifact is interchangeable. It asks whether a specific control discriminates
useful outcomes within comparable work, not whether process volume proves
quality.

Useful measures include:

- coverage: how much real work is represented by the quality system;
- capability: how reliably a repeated process meets defined expectations;
- control signal: whether a control detects or prevents its intended defect;
- friction: time, cost, delay, and rework attributable to the control;
- escapes: defects or unsafe behavior discovered after promotion;
- learning: whether outcome evidence improves future production and assurance.

These measures are observational unless a stronger study design exists. They
must expose denominators, exclusions, uncertainty, selection effects, and
changes in models or process. Sparse incidents never justify weakening a
mandatory safety invariant.

## Continuous Improvement

A quality system should improve both its outputs and the process producing
them. Relevant controlled assets may include requirements templates, prompts,
retrieval policy, engineering knowledge, architecture rules, test suites,
evaluation rubrics, verifier versions, review policy, and deployment checks.

Improvement follows a governed loop:

```text
Observe outcomes
  -> identify a bounded weakness or opportunity
  -> propose a versioned change
  -> verify the proposal independently
  -> approve through the owning authority
  -> compare subsequent evidence and outcomes
```

The system may detect patterns and propose changes. It must not silently modify
its own prompts, policies, verifiers, evidence requirements, or authority
boundaries. Learning is reviewed evolution, not autonomous self-authorization.

## Principles

1. **Quality begins with bounded intent.** Unclear requirements cannot be
   repaired by accumulating downstream evidence.
2. **Claims require relevant evidence.** Confidence and fluent explanation are
   not proof.
3. **Verifiers must be trusted deliberately.** A check has value only within
   demonstrated applicability and known limits.
4. **Assurance is proportional to risk.** Reversible routine work and
   consequential live work do not need the same treatment.
5. **Mandatory invariants remain authoritative.** Analytics may improve their
   implementation but cannot optimize them away.
6. **Human judgment remains accountable.** Models may advise; responsible
   people and external authorities decide where required.
7. **Outcomes close the loop.** Promotion is not the end of quality evaluation.
8. **Improvement is versioned and reviewable.** The quality system changes by
   the same evidence discipline it expects from engineering work.
9. **Coverage precedes conclusions.** Unrepresented work creates selection bias
   that must be visible.
10. **Quality records outlive providers.** Trust must not depend on recovering a
    particular chat, model session, or generated explanation.

## Relationship To Fairway

This definition suggests a possible north star:

> Fairway is an AI Engineering Quality System that makes AI-assisted software
> delivery measurable, evidence-based, and accountable.

That statement is a direction to evaluate, not an implemented-capability claim.
Fairway's current task, evidence, review, provenance, memory, knowledge,
assurance, and delivery-measurement capabilities may contribute to such a
system. Whether they form a complete Quality Record, and what additional
product changes are justified, requires separate architecture and product
decisions after this principles document is reviewed.

## Non-Goals

AI Quality Engineering is not:

- a promise that AI-generated software is defect-free;
- a generic model leaderboard or prompt benchmark;
- replacement of engineering judgment with one aggregate score;
- universal inspection of every generated line;
- autonomous approval, risk acceptance, deployment, or policy mutation;
- compliance certification by assertion;
- collection of unrestricted prompts, transcripts, secrets, or private source
  as a substitute for bounded evidence.

The discipline succeeds when teams can explain why an artifact is trustworthy
for its next bounded use, identify what remains uncertain, observe what happens
after promotion, and improve the system using durable evidence.
