# Fairway AI Engineering Quality System Assessment

Date: 2026-08-02  
Fairway task: `FW-390`  
Assessment basis: current `main` after `f203e36`

## Purpose

This assessment asks how much of the quality-system model in
[Quality Engineering for AI-Assisted Software Delivery](../design/ai-quality-engineering.md)
Fairway implements today.

It is a capability and claim assessment, not an implementation plan. A gap in
this document does not create a roadmap commitment. Product architecture and
prioritization should follow only after the current capability boundary and the
empirical questions are accepted.

## Rating Method

| Rating | Meaning in this assessment |
|---|---|
| **Implemented** | Present in current source with repository validation. |
| **Validated practice** | Used successfully in bounded real execution with durable evidence, without a universal claim. |
| **Partial** | Useful foundations exist, but the quality-system capability is incomplete or not unified. |
| **Missing** | No supported current product capability establishes the principle. A design document or backlog task alone does not count. |
| **Deliberately external** | The authority correctly remains with another system or accountable person; Fairway should link the fact rather than own it. |

The assessment uses current product and release claims, command behavior,
repository tests, and dated assessments. It does not infer capability from
aspirational design text.

## Executive Assessment

Fairway already implements the durable accountability spine of an AI-assisted
engineering quality system:

- bounded intent and ownership;
- material decisions with cited facts;
- typed evidence and artifact references;
- attributable review records, configured review gates, and authority
  boundaries;
- explicit merge, release, deploy, and live-operation readiness;
- provider-independent continuity through sessions, checkpoints, memory, and
  engineering knowledge.

Several of these capabilities are also validated practices in Fairway's own
release work and in the GPUaaS consumer. The provider-replacement and
memory/knowledge assessments show that execution can resume from durable facts
without treating a provider transcript as authority.

Fairway does **not** yet implement the complete quality-system model. The
current product lacks:

- one supported Quality Record projection spanning intent through outcomes and
  lessons;
- a general evaluation-run and rubric model;
- a qualified-verifier registry with applicability, fixture, version, and
  error-characteristic records;
- mature outcome linkage and control-effectiveness measurement;
- comparison-class capability analysis for repeated engineering processes;
- a validated closed loop that changes production or assurance assets from
  measured outcomes.

The defensible current claim remains the canonical wording in
[Product](../product.md):

> **Fairway is the engineering control and accountability layer for
> agent-driven software delivery.**

The proposed category **AI Engineering Quality System** and the wording
“engineering quality and accountability system for AI-assisted software
delivery” remain a north star pending a separate product decision. It is not
yet justified to present Fairway as a complete, measurable, or continuously
improving AI engineering quality system.

## Quality Record Coverage

### 1. Intent

**Rating: Implemented; validated practice.**

Fairway tasks record stable identity, owner role, status, scope metadata,
dependencies, profile, risk, source and target paths, and acceptance checks.
Runtime state remains in the Fairway store while versioned backlog files define
intended task shape. The compact `work start` path and the full task commands
both preserve this boundary.

This is the strongest current Quality Record stage. It is defined in
[Concepts](../design/concepts.md), covered by
[CLI tests](https://github.com/fairway-run/fairway/blob/main/cmd/fairway/main_test.go) and
[store tests](https://github.com/fairway-run/fairway/blob/main/internal/store/store_test.go), and exercised in the
[common-path pilot](fairway-common-path-pilot-2026-07-11.md).

**Limit:** intent quality is recorded, not automatically established. Fairway
can detect missing configured fields and acceptance checks, but it cannot prove
that a requirement is correct, complete, or valuable.

### 2. Material Decisions

**Rating: Implemented.**

`fairway decision record` stores the trigger, alternatives, chosen path,
rationale, risk, validation references, and citations to durable Fairway facts.
Task decision-memory packets preserve these decisions across provider changes.
Decision-quality assessment remains separate from approval and evidence.

The current contract is documented in
[Task decision memory](../design/task-decision-memory.md) and the authority
limits are explicit in [Concepts](../design/concepts.md). Persistence and CLI
behavior are covered in [store tests](https://github.com/fairway-run/fairway/blob/main/internal/store/store_test.go) and
[CLI tests](https://github.com/fairway-run/fairway/blob/main/cmd/fairway/main_test.go).

**Limit:** Fairway checks record shape and citations; it does not prove that the
decision was technically correct. Unrecorded decisions remain a coverage gap.

### 3. Production Context

**Rating: Partial.**

Sessions, provider identity, checkpoints, handoffs, task packets, source paths,
commit references, track memory, engineering-knowledge snapshots, rule packs,
and configuration digests capture important production context. The GPUaaS
[memory and knowledge adoption assessment](gpuaas-memory-knowledge-adoption-2026-07-23.md)
validated useful cold starts and source ranking without hosted retrieval or
provider-private memory. The
[provider-replacement pilot](fairway-provider-replacement-pilot-2026-07-14.md)
separately exercised continuity across provider attachments.

**Missing from a complete quality record:** there is no unified, bounded model
for relevant generation conditions such as model/tool versions, retrieved
source snapshot, prompt or instruction revision, evaluator revision, and
material environment facts. Fairway correctly rejects unrestricted transcripts
as authority; closing this gap should not turn it into a prompt-surveillance or
secret-retention system.

### 4. Collected Evidence

**Rating: Implemented; validated practice.**

`fairway record evidence` records commands or checks, pass/fail results, and
artifact references. Evidence is exposed in task detail, packets,
readiness checks, assurance mapping, and signed/offline release packages.
Rule packs can declare evidence expectations and closeout can report missing
required evidence.

Fairway's `v0.2.4` and `v0.2.5` release rehearsals validated exact-commit,
artifact-digest, signing, notarization, smoke, and promotion evidence across a
real release boundary. The release evidence is summarized in
[Release notes](../release-notes.md) and the
[v0.2.4 release verification](fairway-v0.2.4-release-verification-2026-07-23.md).
Record behavior is covered by [store tests](https://github.com/fairway-run/fairway/blob/main/internal/store/store_test.go)
and [CLI tests](https://github.com/fairway-run/fairway/blob/main/cmd/fairway/main_test.go).

**Limit:** evidence records remain general facts. Claim-to-evidence semantics
exist in bounded rule-pack and assurance-profile forms, but there is no general
claim graph describing what a result establishes, contradicts, or cannot
establish.

### 5. Automatic Verification

**Rating: Partial.**

Fairway implements deterministic configuration and workflow checks, rule-pack
validation and matching, evidence expectations, merge readiness, assurance
profile validation, evidence mapping, readiness reports, and release-bundle
verification. These are useful automatic-verification primitives.

The limits are material:

- verifier qualification is design-only in the current release;
- there is no general evaluation-run model for rubric, model-judge, simulation,
  or sampled assessment results;
- verifier applicability, fixture coverage, false-positive behavior, and
  false-negative behavior are not first-class runtime records;
- a passing command is recorded as evidence but is not automatically a
  qualified verifier for every claim it might be cited against.

The current boundary is stated in [Release notes](../release-notes.md),
[Rule packs](../design/rule-packs.md), and
[Assurance profiles](../design/assurance-profiles.md). Current implemented
behavior is covered by [rule tests](https://github.com/fairway-run/fairway/tree/main/internal/rules),
[assurance profile tests](https://github.com/fairway-run/fairway/blob/main/internal/assurance/profile_test.go), and
[assurance verification tests](https://github.com/fairway-run/fairway/blob/main/internal/assurance/verify_test.go).

### 6. Human Judgment

**Rating: Partial.**

Fairway records attributable review verdicts by configured domain, detects
missing review, preserves changes-requested state, and separates configured
review gates from evidence and promotion readiness. This record and gate
behavior is covered by
[review-policy tests](https://github.com/fairway-run/fairway/tree/main/internal/reviewpolicy),
[review-state tests](https://github.com/fairway-run/fairway/tree/main/internal/reviewstate), and
[coordinator tests](https://github.com/fairway-run/fairway/blob/main/internal/coordinator/plan_test.go).

**Limit:** current reviewer identity and domain are asserted at the CLI
boundary. Fairway prevents some direct self-review cases and reports required
domains, but it does not generally authenticate reviewer identity, authorize a
person for the asserted domain, establish competence, or verify organizational
decision authority. Those facts remain external unless supplied by a trusted
identity/authorization boundary. Fairway therefore implements review records
and configured gates, not qualified human judgment as defined by the
principles.

### 7. Promotion Decision

**Rating: Implemented for readiness; deliberately external for execution and
authority.**

`merge-ready`, `workflow check`, review-wait projections, push intent,
release/deploy preflight, live-operation controls, and closeout handbacks make
promotion state explicit. They distinguish local reversible work from merge,
release, deploy, public exposure, credentials, and live mutation.

Git, CI/CD, deployment systems, and accountable people correctly retain
execution and approval authority. Fairway reports whether its configured facts
support promotion; it does not silently merge, deploy, accept risk, or grant
credentials.

The readiness behavior is covered by
[coordinator tests](https://github.com/fairway-run/fairway/blob/main/internal/coordinator/plan_test.go) and
[CLI tests](https://github.com/fairway-run/fairway/blob/main/cmd/fairway/main_test.go). Exact-commit release promotion was
exercised in the
[v0.2.4 release verification](fairway-v0.2.4-release-verification-2026-07-23.md).

**Limit:** readiness is profile- and evidence-dependent. It is not a universal
proof that the promoted artifact is correct.

### 8. Operational Outcomes

**Rating: Partial.**

Fairway records task reopens, blocked/unblocked transitions, CI learning,
delivery timing, rough edges, follow-up tasks, deploy/run evidence, and some
release and consumer outcomes. `fairway delivery report`,
`fairway audit work-coverage`, and `fairway audit ci-learning` provide useful
observational facts.

The reviewed [Control effectiveness](../design/control-effectiveness.md) model
defines stronger coverage, outcome, friction, cohort, and confound semantics,
but its outcome linkage and analytics tasks are not implemented. Explicit
incident, rollback, corrective-work, superseding-task, and post-promotion
rework links are incomplete. Fairway therefore cannot yet establish whether a
specific control discriminates better outcomes.

Current audit and delivery facts are covered by
[audit tests](https://github.com/fairway-run/fairway/tree/main/internal/audit) and repository command tests; no dated
assessment is treated as validation of control effectiveness.

### 9. Lessons And Controlled Improvement

**Rating: Partial.**

Task decisions, project working memory, engineering knowledge, rule-pack
maintenance, CI learning, rough-edge capture, and common-path automation
candidates preserve lessons and can inform reviewed changes. The
memory/knowledge pilot demonstrated that source-grounded learning can improve
subsequent cold starts without replacing canonical documentation.

**Missing:** Fairway has not validated a closed outcome-to-change loop that
measures a bounded weakness, proposes a versioned change to a prompt, retrieval
policy, rule, verifier, review profile, or test suite, independently verifies
it, and compares subsequent outcomes. Current mechanisms support controlled
improvement; they do not yet prove continuous improvement.

### 10. Unified Quality Record

**Rating: Missing as a supported product projection.**

Most early-stage facts exist, but users must currently assemble them from task
detail, decision packets, memory/knowledge packets, evidence, reviews,
readiness, assurance reports, delivery reports, Git, CI/CD, and external
operational systems. No single versioned projection answers the full chain from
intent through outcomes and learned process changes.

This is a product-coherence gap, not evidence that the underlying records are
absent. Whether Fairway should add a first-class Quality Record projection is a
separate architecture decision.

## Cross-Cutting Capabilities

| Capability | Current rating | Assessment |
|---|---|---|
| Coverage measurement | **Partial** | Work-coverage audit can compare recent commits and Fairway records, but a sustained representative coverage baseline has not been established across consumers. |
| Risk-proportional assurance | **Implemented** | Profiles, risk metadata, review routes, rule severity, and promotion-specific controls support proportional checks while retaining mandatory invariants; current behavior is covered by [config](https://github.com/fairway-run/fairway/tree/main/internal/config), [rules](https://github.com/fairway-run/fairway/tree/main/internal/rules), and [assurance](https://github.com/fairway-run/fairway/tree/main/internal/assurance) tests. |
| Evaluations | **Missing as a general subsystem** | Tests and assurance checks exist; versioned eval cases, rubrics, evaluators, samples, and evaluation regressions do not form a supported general product model. |
| Machine verifier qualification | **Missing; design-only** | Migration design names completeness and qualification, but current releases do not record verifier applicability or measured error behavior. |
| Human reviewer qualification | **Missing in Fairway / deliberately external today** | Fairway records asserted reviewer and domain values and blocks some exact self-review cases; it does not authenticate identity, establish competence, authorize the asserted role, or grant organizational authority. |
| Comparison classes and process capability | **Missing** | Delivery reports are descriptive. No supported runtime defines comparable repeated work and computes bounded capability with revision and uncertainty controls. |
| Control effectiveness | **Planned** | The metric contract is reviewed; outcome linkage, reports, dashboard, and GPUaaS calibration remain backlog work under `FW-388`. |
| Audit and provenance | **Implemented; validated practice** | Actor attribution, transitions, evidence, review, source/commit references, and signed release packets provide a strong provenance base, covered by [audit](https://github.com/fairway-run/fairway/tree/main/internal/audit), [provenance](https://github.com/fairway-run/fairway/tree/main/internal/provenance), and [offline-bundle](https://github.com/fairway-run/fairway/tree/main/internal/offlinebundle) tests and the [v0.2.4 release verification](fairway-v0.2.4-release-verification-2026-07-23.md). External facts remain externally authoritative. |
| Bounded export and package custody | **Implemented** | Assurance, provenance, audit, and offline-package paths constrain exported artifacts and are covered by [assurance](https://github.com/fairway-run/fairway/tree/main/internal/assurance), [provenance](https://github.com/fairway-run/fairway/tree/main/internal/provenance), and [offline-bundle](https://github.com/fairway-run/fairway/tree/main/internal/offlinebundle) tests. |
| General record-ingest privacy and retention | **Partial** | Fairway discourages transcript and credential custody, but general evidence input can retain command text, artifact paths, types, and notes without universal secret scanning or policy-based retention enforcement. |
| Quality-system dashboard | **Missing** | Current dashboards are read-oriented execution projections. They do not present a unified Quality Record or qualified outcome/control-effectiveness view. |

## Claim Boundary

### Defensible now: canonical current claim

> Fairway is the engineering control and accountability layer for agent-driven
> software delivery. It keeps accountable intent, material decisions, evidence,
> attributable review state, continuity, and promotion readiness durable while
> external engineering tools execute the work.

This claim matches current implemented capability and bounded consumer/release
validation.

### Proposed category, pending product decision

> Fairway is an engineering quality and accountability system for AI-assisted
> software delivery.

The assessment shows substantial foundations for this category, but the wording
remains a north star rather than the current public claim until the missing
quality-record, verifier, evaluation, and outcome capabilities are weighed in a
separate product decision.

### Not yet defensible

Fairway should not yet claim that it:

- measures software quality comprehensively;
- proves that AI-assisted work is trustworthy;
- continuously improves prompts, models, retrieval, or engineering policy;
- qualifies arbitrary tests, scanners, evaluators, or human reviewers;
- predicts or prevents escaped defects;
- establishes statistical process capability for heterogeneous engineering
  work;
- certifies compliance or autonomously approves promotion.

## Empirical Questions Before Repositioning

The following questions should be answered with real execution data before a
broader product claim is adopted:

1. **Coverage:** What fraction of actual commits and consequential operational
   changes are linked to a Fairway task, including urgent and failed work?
2. **Control signal:** Within contemporaneous risk and diff-size cohorts, which
   evidence and review controls correlate with less rework, fewer reopens, or
   fewer failed promotions?
3. **Friction and bypass:** Which controls add delay or repeated coordination,
   and does that friction reduce coverage or push work outside the system?
4. **Outcome linkage:** Can incidents, rollbacks, corrective commits, reopens,
   and superseding tasks be linked reliably enough to interpret control value?
5. **Verifier adequacy:** For representative verifiers, what claims are they
   applicable to and what do known-good and deliberately bad fixtures reveal
   about false results?
6. **Record usefulness:** Can a reviewer or replacement provider use a bounded
   Quality Record projection to reach the correct next action faster and with
   fewer unsupported assumptions than current separate packets?
7. **Comparison classes:** Which recurring work types are similar enough for
   capability measurement without hiding model, process, or policy revisions?
8. **Learning:** When a measured weakness changes a rule, verifier, prompt,
   retrieval policy, or review profile, do subsequent outcomes improve without
   unacceptable new friction?

The existing control-effectiveness work under `FW-388` is the nearest planned
source of evidence for questions 1 through 4. It should be treated as an
empirical input, not as proof of the larger positioning.

## Recommendation

1. Retain the canonical current public product claim and the explicit implemented,
   validated-practice, experimental, planned, and non-goal labels.
2. Accept the Quality Record as the assessment lens, but do not claim it as an
   implemented unified Fairway object.
3. Use current control-effectiveness work and consumer execution to answer the
   empirical questions before changing the product category or roadmap.
4. After that evidence is reviewed, make a separate product/architecture
   decision about a unified Quality Record, evaluation records, and verifier
   qualification. Do not infer those implementation commitments from this
   assessment alone.

## Evidence Sources

- [Product definition and claim inventory](../product.md)
- [Quality Engineering for AI-Assisted Software Delivery](../design/ai-quality-engineering.md)
- [Concepts and authority invariants](../design/concepts.md)
- [Task decision memory](../design/task-decision-memory.md)
- [Project working memory](../design/project-working-memory.md)
- [Engineering knowledge](../design/engineering-knowledge.md)
- [Rule packs](../design/rule-packs.md)
- [Assurance profiles](../design/assurance-profiles.md)
- [Control effectiveness](../design/control-effectiveness.md)
- [GPUaaS memory and knowledge adoption assessment](gpuaas-memory-knowledge-adoption-2026-07-23.md)
- [Provider-replacement pilot](fairway-provider-replacement-pilot-2026-07-14.md)
- [Common-path pilot](fairway-common-path-pilot-2026-07-11.md)
- [v0.2.4 release verification](fairway-v0.2.4-release-verification-2026-07-23.md)
- [Release notes and current known limits](../release-notes.md)
- [CLI implementation tests](https://github.com/fairway-run/fairway/blob/main/cmd/fairway/main_test.go)
- [Store implementation tests](https://github.com/fairway-run/fairway/blob/main/internal/store/store_test.go)
- [Coordinator implementation tests](https://github.com/fairway-run/fairway/blob/main/internal/coordinator/plan_test.go)
- [Assurance implementation tests](https://github.com/fairway-run/fairway/tree/main/internal/assurance)
- [Audit implementation tests](https://github.com/fairway-run/fairway/tree/main/internal/audit)
- [Provenance implementation tests](https://github.com/fairway-run/fairway/tree/main/internal/provenance)
