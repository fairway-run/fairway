# Migration Execution Profile

## Status

Proposed design. This document defines an optional Fairway operating profile
for large code migrations. It does not authorize a core implementation, change
the default Fairway delivery model, or make Fairway a migration engine.

The profile has three distinct responsibilities:

```text
Migration execution profile
  Defines the phases, artifacts, decisions, and task/evidence shape.

Rule-pack completeness model
  Tests whether the migration rules constrain the work sufficiently.

Verifier qualification model
  Tests whether a green result is credible enough to support a decision.
```

These responsibilities must remain separate. A complete rule pack cannot
compensate for an unreliable verifier, and a qualified verifier cannot prove
that the migration rules captured every required behavior.

## Purpose

Large migrations are different from normal feature delivery. They often have:

- many repetitive work units;
- a source implementation that acts as a temporary behavioral baseline;
- rules that evolve as edge cases are discovered;
- expensive parity checks that cannot run after every edit;
- high risk of systematic error being copied across many translated units;
- a long interval between local correctness and whole-system confidence.

The profile makes those risks explicit and gives Fairway a bounded way to
coordinate them. Fairway remains the accountable record. Migration tools,
providers, compilers, test runners, CI systems, and live environments remain
external execution authorities.

## Applicability

The profile is opt-in. It should be selected only after an explicit migration
decision.

| Work shape | Profile use | Notes |
|---|---|---|
| Structure-preserving total migration | Full profile | Best fit. Source units, target units, and parity surfaces can be inventoried and compared. |
| Structure-preserving partial migration | Full or narrowed profile | Scope, coexistence boundary, and retirement criteria must be explicit. |
| Redesign during migration | Modified profile | Use subsystem or behavior slices. Do not claim translator bakeoff equivalence when source and target structures intentionally differ. |
| Incremental language adoption | Selected controls only | Verifier qualification and check economics may help; a migration-wide queue may not. |
| Ordinary feature, bug fix, or refactor | Not applicable | Use the normal risk-scaled Fairway delivery model. |
| Emergency repair | Not applicable during mitigation | Record the migration follow-up separately after service restoration. |

The decision not to migrate is a valid outcome of the first phase.

## Non-Goals

This profile does not:

- become Fairway's default development process;
- require two agents or two reviewers for every file;
- turn Fairway into a workflow or DAG engine;
- turn Fairway into a compiler, CI runner, test runner, or migration runtime;
- store raw prompts, transcripts, source code, secrets, or generated output;
- treat the old implementation as unquestionable product authority;
- make model output, rule-pack conformance, or a green checker an approval;
- authorize merge, deploy, release, credential, or production actions;
- prescribe one programming language, provider, or migration tool.

## Product Boundary

The existing [product boundary](product-boundaries.md) remains authoritative.

```text
Fairway owns
  task hierarchy
  accountable decisions
  source and configuration pins
  bounded evidence references
  gaps and exceptions
  review state
  promotion readiness

External execution owns
  source analysis
  translation
  compilation
  test execution
  parity execution
  CI
  deploy and live validation
```

On-disk work queues, migration manifests, and tool-specific settings are
adapter artifacts. They may be referenced by Fairway evidence, but they do not
replace Fairway task authority or state transitions.

## Profile Composition

A migration execution profile composes existing Fairway concepts:

```text
workstream profile
  + version-pinned rule pack
  + bounded task recipe
  + evidence and assurance mapping
  + migration-specific decision records
```

Rule packs remain project or domain knowledge. Recipes remain bounded reusable
packets. Assurance profiles remain read-only declarations of objectives and
evidence requirements; the separate assurance read model maps recorded
Fairway facts to those declarations. The migration profile adds phase semantics
and migration-specific evidence expectations without weakening those
contracts.

## Phase Model

### Phase 0: Feasibility And Migration Decision

Define:

- objective and business reason;
- structure-preserving, partial, or redesign classification;
- included and excluded source surfaces;
- compatibility and coexistence boundary;
- rollback and retirement strategy;
- budget for engineering time, compute, model usage, and live environments;
- product contracts that supersede source behavior;
- stop conditions.

Required output:

- migration charter;
- source and target revision pins;
- explicit proceed, narrow, defer, or reject decision.

### Phase 1: Verifier Qualification

Qualify each verifier before it can support migration decisions. A verifier can
be a compiler, test suite, parity comparator, static analyzer, runtime probe, or
human review rubric.

Required output:

- verifier qualification report;
- verifier version and configuration pin;
- fixture or corpus revision pin;
- observed misses and false alarms;
- known blind spots;
- requalification triggers.

A result from an unqualified verifier is `unsupported`, not `pass`.

### Phase 2: Map, Inventory, And Rule Construction

Build a bounded inventory of:

- source units and target destinations;
- dependency direction and shared infrastructure;
- public contracts and compatibility surfaces;
- generated code and artifacts;
- platform-specific behavior;
- security and privacy constraints;
- intentionally unsupported behavior;
- validation commands and expected evidence.

Rules are written from the inventory, architecture decisions, contracts,
security requirements, and known failure modes. Source behavior is one input,
not the only authority.

Required output:

- dependency and ownership map;
- migration manifest;
- initial pinned rule pack;
- gap and exception inventory;
- ordered execution batches.

### Phase 3: Rule-Pack Completeness Bakeoff

Before broad execution, run a bounded pilot using independent contexts:

```text
rule-guided implementation
  versus
rule-blind implementation
  versus
qualified verifier and reviewer readback
```

The purpose is not to pick the best generated patch. It is to discover whether
the rule pack captures decisions that materially change the result.

Every divergence must be classified:

- `missing_rule`: the desired behavior was not represented;
- `ambiguous_rule`: the rule allowed materially different interpretations;
- `source_ambiguity`: the source or contract does not establish one answer;
- `intentional_freedom`: both results satisfy the bounded contract;
- `verifier_blind_spot`: the checker cannot distinguish an important defect;
- `reviewer_or_model_noise`: the difference is not a stable rule signal;
- `both_wrong`: neither result satisfies the target contract;
- `harmful_rule`: following the rule makes the result worse.

Rule changes require an attributable human decision. Agents may propose an
amendment but must not silently rewrite the active rule pack during a batch.

After amendment:

- publish a new rule-pack version or source pin;
- record the reason and affected rule IDs;
- identify completed work units affected by the change;
- reopen or revalidate those units before promotion.

Required output:

- bakeoff report;
- divergence classifications;
- rule-pack completeness decision;
- amendment and revalidation list.

### Phase 4: Deterministic Execution

Execute bounded work units in dependency-aware batches. Each unit should carry:

- source and target identifiers;
- rule-pack pin;
- verifier pins;
- accepted exceptions;
- commands or external job references;
- evidence expectations;
- completion and escalation criteria.

The execution adapter may use files, a database, or an external system for
efficient work distribution. Fairway records the accountable task, session,
evidence, and review facts.

The active rule pack is immutable during a batch. Recurring failures stop the
batch and return to Phase 3 rather than producing local workarounds in every
unit.

### Phase 5: Survey Build And Runtime Validation

Run checks that are too expensive or broad for every unit:

- whole-repository build;
- dependency and generated-artifact checks;
- representative runtime scenarios;
- performance and resource comparisons;
- security and privacy scans;
- platform and environment coverage;
- rollback or coexistence proof.

Failures are routed to their owning layer. Broad survey checks must not become
the first place a cheap local defect could have been detected.

### Phase 6: Parity, Exceptions, And Retirement

Prove the agreed target contract rather than claiming abstract equivalence.
Record:

- behavior and data parity results;
- accepted and rejected differences;
- performance and operational posture;
- unresolved gaps and owners;
- rollback readiness;
- source retirement criteria;
- cleanup and archival evidence.

The migration closes only when the agreed acceptance boundary is met or an
explicit decision narrows or stops it.

## Rule-Pack Completeness Model

Rule-pack completeness is a measured claim about the selected migration scope.
It is not a claim that every possible behavior is documented.

### Completeness Dimensions

| Dimension | Question |
|---|---|
| Contract | Are public behavior, compatibility, and error semantics represented? |
| Architecture | Are ownership, dependency direction, and target boundaries represented? |
| Data | Are schema, migration, ordering, precision, and durability rules represented? |
| Security | Are trust, authorization, secret custody, audit, and negative paths represented? |
| Runtime | Are concurrency, lifecycle, retry, cleanup, and failure semantics represented? |
| Operations | Are build, deploy, observability, rollback, and incident surfaces represented? |
| Platform | Are OS, architecture, packaging, generated code, and environment differences represented? |
| Exceptions | Are deliberate non-parity decisions attributable and bounded? |

### Completeness Decision

The bakeoff may produce:

- `sufficient_for_pilot`;
- `sufficient_for_bounded_execution`;
- `insufficient`;
- `not_applicable` for a redesign where rule-guided translation is not a
  meaningful comparison.

No percentage alone proves completeness. The decision must cite the sampled
surfaces, divergence classes, unresolved gaps, verifier posture, and reviewer
judgment.

## Verifier Qualification Model

Verifier qualification asks whether a checker can detect the defects it is
expected to govern.

### Qualification Corpus

Each material verifier should be exercised against:

- known-good fixtures;
- deliberately-bad fixtures for each claimed defect class;
- boundary and empty-state cases;
- malformed or adversarial inputs where applicable;
- at least one realistic migrated unit;
- known source defects that should not be preserved, when relevant.

Deliberately-bad fixtures may be hand-authored or generated by bounded mutation.
The corpus must not be generated solely by the same context that implements the
verifier.

### Qualification Evidence

Record:

- verifier name, version, configuration, and source revision;
- fixture/corpus revision;
- expected and observed outcomes;
- detected defects;
- missed defects;
- false positives;
- normalization or nondeterminism controls;
- environment requirements;
- qualification owner and date.

Exact statistical thresholds are verifier-specific. At minimum, every claimed
defect class must have a deliberately-bad fixture that the verifier detects,
and known-good fixtures must not produce unexplained blocking results.

### Requalification Triggers

Requalify when:

- verifier code, dependencies, configuration, or normalization changes;
- the target language, compiler, runtime, or platform changes materially;
- a production or migration defect escapes a claimed verifier class;
- rule amendments introduce a new behavior class;
- the qualification corpus changes materially;
- the verifier has exceeded its recorded freshness window.

## Check Economics And Placement

Check placement is an engineering decision. Running every check on every unit
can make the migration slower without improving signal.

Price a check using:

- wall-clock latency;
- compute or provider cost;
- scarce environment use;
- setup and cleanup cost;
- flake or nondeterminism risk;
- diagnostic precision;
- defect escape impact;
- blast radius if delayed.

| Placement | Typical cost and purpose | Examples |
|---|---|---|
| Inline | Seconds, deterministic, high diagnostic value | syntax, formatting, type checks, local schema validation |
| Work-unit close | Low minutes, scoped behavior | focused unit tests, translated-module comparator |
| Batch close | Moderate cost or shared setup | package integration, generated-artifact drift, cross-unit checks |
| Merge/parity gate | Broad and expensive | full build, corpus parity, performance envelope |
| Live or approval-gated | Scarce, destructive, credentialed, or externally visible | production-like migration, failover, load, rollback |

A check should run at the earliest level where it is affordable, reliable, and
diagnostically useful. Delayed checks need an explicit reason and a bounded
batch size so systematic defects do not spread unchecked.

## Review Model

Review remains risk-scaled.

- Migration charter, target architecture, rule-pack completeness decisions,
  verifier qualification, material rule amendments, parity exceptions, and
  retirement require independent judgment.
- Mechanical work units may use a qualified verifier and one accountable
  reviewer when project policy permits.
- High-risk security, data, compatibility, live, or irreversible boundaries
  retain their normal review domains.
- Review repetition is not a substitute for verifier quality or rule
  completeness.

## Fairway Record Shape

A migration should normally use:

```text
migration epic
  feasibility and charter task
  verifier qualification tasks
  rule and inventory task
  completeness bakeoff task
  execution batch tasks
  survey validation tasks
  parity and retirement task
```

Recommended bounded evidence types:

- `migration-charter`;
- `source-target-pin`;
- `migration-inventory`;
- `rule-pack-pin`;
- `rule-pack-bakeoff`;
- `rule-amendment`;
- `verifier-qualification`;
- `work-unit-result`;
- `batch-survey`;
- `parity-report`;
- `migration-exception`;
- `retirement-proof`.

These names are design vocabulary until a pilot proves that first-class schema
or CLI support is warranted. A pilot may use normal Fairway evidence records
with these values as project conventions.

## Security And Privacy

- Store source pins, hashes, summaries, counts, and safe artifact references;
  do not store raw source, prompts, transcripts, credentials, or private tool
  output in Fairway.
- Treat provider deny lists and tool settings as adapter controls, not Fairway
  authorization.
- Sanitize comparator output before retaining it.
- Do not preserve a source behavior that conflicts with an approved security,
  legal, product, or architecture decision.
- Live, destructive, credential, or production checks keep their existing
  approval and stop conditions.

## Bounded Pilot

No Fairway core implementation should begin before a pilot.

Choose a migration that is:

- structure-preserving;
- small enough to complete in days, not months;
- large enough to contain repeated units and at least three rule dimensions;
- supported by an existing test or comparator surface;
- non-production-critical or easily reversible;
- free of sensitive source or credentials in retained evidence.

Run two bounded approaches on comparable slices:

1. normal Fairway delivery with existing project rules;
2. this migration profile with verifier qualification and a completeness
   bakeoff.

Before execution, declare:

- primary success measures;
- non-regression thresholds for escaped defects, total engineering cost, and
  elapsed delivery time;
- the economic dimensions that will be measured;
- any dimension that is not applicable or cannot be measured, with a reason;
- sample limits and conditions that make the comparison invalid.

Measure:

- escaped defects;
- rework after batch or parity checks;
- rule amendments and affected work reopened;
- verifier misses and false positives;
- elapsed engineering time;
- compute and provider cost;
- scarce environment occupancy;
- setup and cleanup effort;
- flake and rerun cost;
- provider/model usage where available;
- review effort;
- diagnostic time;
- operational or migration blast radius;
- operator confidence in rollback and retirement.

The pilot closes with one decision:

- `adopt`: the profile meets the predeclared success measures without
  violating any non-regression threshold;
- `narrow`: retain only selected controls such as verifier qualification or
  check economics when the retained controls meet their predeclared thresholds;
- `reject`: the added process does not justify its cost.

Only an `adopt` or explicitly scoped `narrow` decision may create core
implementation tasks.

## Deferred Implementation

Until the pilot closes:

- do not add a migration-specific Fairway state machine;
- do not add a migration queue executor;
- do not add automatic rule mutation;
- do not add a universal verifier schema;
- do not make migration evidence types core vocabulary;
- do not change default review policy;
- do not market Fairway as a migration engine.

## External Reference

The phase model is informed by Anthropic's
[Code Migration Kit with Claude Code](https://github.com/anthropics/code-migration-kit-with-claude-code),
especially its feasibility-first approach, explicit rulebook, pre-migration
judge, bounded pilot, deterministic work queues, survey build, and parity
phases.

That repository is reference code and a methodology example, not a Fairway
dependency or product authority. Fairway adopts only the controls that survive
the bounded pilot and fit the product boundary above.
