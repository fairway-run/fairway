# Harness Interoperability And Verified Outcomes

## Status And Purpose

This document defines Fairway's product contract for connecting replaceable
agent harnesses to the durable engineering record. It is the architecture
authority for `FW-408` and its implementation slices. It does not select a
transport, require Seaway, or turn Fairway into an agent runtime.

Harnesses increasingly own long-running execution loops, working memory,
tools, sandboxes, model selection, permission pauses, and runtime recovery.
Those implementations will change more quickly than a project's engineering
authority. Fairway therefore owns the stable cross-run record of what bounded
work was attempted, what was observed, how it was evaluated, and which
external authority must decide what happens next.

The product boundary is:

```text
Fairway
  intent / decisions / sourced observations / evaluator results / reviews / readiness
                                      ^
                                      | versioned, replay-safe facts
                                      v
Harness, provider, optional Seaway, CI, simulator, scanner, or human evaluator
  prompts / tools / working memory / execution loop / sandbox / runtime policy
```

## Authority Invariants

1. A Fairway task may correlate with zero, one, or many external runs.
2. An external run, experiment, or evaluator result never changes task status.
3. Runtime approval never satisfies a Fairway review, risk acceptance,
   promotion, merge, release, deploy, or live-operation requirement.
4. An evaluator result is a sourced observation. It becomes accepted evidence
   only through the project's existing evidence and review rules.
5. Fairway stores safe summaries, identities, classifications, and artifact
   references. It excludes raw prompts, private reasoning, transcripts, raw
   tool bodies, credentials, secrets, and generated-content dumps.
6. A Fairway lane worktree remains distinct from a harness workspace or
   sandbox even when both reference the same repository revision.
7. Fairway may recommend a redirect. It never silently sends a new prompt,
   cancels a run, changes policy, or approves the result.
8. The no-adapter path remains fully supported.

## Record Model

The first implementation adds three append-only record families. They may be
stored in separate tables or in one normalized harness-record store, provided
the public identities and invariants below remain stable.

### External Run

An external run links one provider or harness execution attempt to a Fairway
task and, when present, a Fairway session.

Required fields:

| Field | Meaning |
|---|---|
| `source_id` | Stable configured identity of the emitting harness or adapter. |
| `source_version` | Version of the source contract understood for this record. |
| `external_run_id` | Stable run identity within `source_id`. |
| `task_id` | Durable Fairway task identity. |
| `submission_id` | Caller-selected idempotency identity for one intended run. |
| `observed_at` | Source observation time in UTC. |

Optional fields include `session_id`, `caller_work_id`, `prior_run_id`,
provider, model, harness, repository identity, revision, branch, safe workspace
reference, trace ID, start/end time, terminal classification, usage reference,
and namespaced metadata.

The unique source identity is `(project_id, source_id, external_run_id)`. The
unique submission identity is `(project_id, source_id, submission_id)`.
Replaying the same identity and canonical payload returns the existing record.
Reusing either identity with a different canonical payload returns a conflict
and does not overwrite history. A deliberate retry receives a new
`submission_id` and `external_run_id`, retains the same `caller_work_id`, and
may name `prior_run_id`.

### Execution Observation

An observation reports a bounded experiment or material execution fact. It is
not a transcript or a replacement for task evidence.

Required fields:

| Field | Meaning |
|---|---|
| `observation_id` | Stable identity selected by the source. |
| `source_id`, `source_version` | Source and compatibility identity. |
| `task_id` | Fairway task receiving the sourced fact. |
| `kind` | `experiment`, `execution`, `artifact`, `policy`, `usage`, or `terminal`. |
| `subject_type`, `subject_ref` | Bounded subject such as task, commit, artifact, claim, policy, or environment and its safe reference. |
| `summary` | Bounded, redacted statement of what was observed. |
| `observed_at` | Source observation time. |
| `outcome` | `confirmed`, `rejected`, `inconclusive`, `blocked`, or `unavailable`. |
| `source_mode` | `measured`, `reported`, `derived`, or `human_judgment`. |

An experiment observation also carries a bounded `hypothesis` and
`expected_observation`. Optional fields include actual measurement, units,
`external_run_ref`, artifact reference and SHA-256 digest, evaluator references,
confidence, uncertainty, completeness, sequence, trace ID, and recommended
next action. `external_run_ref` is the fully qualified object
`{source_id, external_run_id}`; no unqualified cross-record run reference is
accepted. A run-independent CI, scanner, simulator, commit, artifact, or human
observation omits it and is not assigned a synthetic run.

The unique observation identity is
`(project_id, source_id, observation_id)`. `task_id`, `source_version`, the
optional fully qualified external-run reference, kind, source mode, observed
time, and subject
are immutable. An observation belongs to exactly one task and zero or one
external run. Related evaluator results form a zero-or-many relationship.

Confidence is optional and never synthesized. When supplied it is a decimal in
`[0,1]` with a named method or source. Uncertainty is a short retained statement
or `unavailable`; absence does not mean certainty.

### Evaluator Result

An evaluator result states how a named evaluator assessed one observation,
artifact, commit, run, or explicit claim.

Required fields:

| Field | Meaning |
|---|---|
| `evaluation_id` | Stable source-selected identity. |
| `source_id`, `source_version`, `task_id` | Source, compatibility, and task identity. |
| `evaluator_id`, `evaluator_version` | Stable evaluator implementation or rubric identity. |
| `subject_type`, `subject_ref` | Bounded evaluated subject and safe reference. |
| `result` | `pass`, `fail`, `partial`, `inconclusive`, `error`, or `unavailable`. |
| `mode` | `deterministic`, `statistical`, or `human_judgment`. |
| `evaluated_at` | Evaluation time in UTC. |

Optional fields include environment identity, repository revision, rubric or
case-set version, `external_run_ref`, `observation_ref`, sample and exclusion
counts, measurement, threshold,
artifact reference and digest, confidence, uncertainty, completeness, and a
bounded summary. A model judge is `statistical`, not deterministic. A human
reviewer remains `human_judgment`; importing the result does not create a
Fairway review verdict.

`external_run_ref` is `{source_id, external_run_id}` and `observation_ref` is
`{source_id, observation_id}`. Both are fully qualified because an evaluator
may be emitted by a different source than the run or observation it assesses.
The referenced records must belong to the same Fairway task; an unqualified or
cross-task reference is rejected before persistence.

The unique evaluator-result identity is
`(project_id, source_id, evaluation_id)`. `task_id`, `source_version`,
evaluator identity/version, subject, mode, evaluation time, and the optional
fully qualified external-run and observation references are immutable. A
result belongs to exactly one task, zero or one external run, and zero or one
observation. The subject reference remains required when no observation is
named.

## Compatibility And Ingestion

The public schemas are:

```text
fairway.harness.external-run.v1
fairway.harness.execution-observation.v1
fairway.harness.evaluator-result.v1
fairway.harness-record-batch.v1
```

`fairway contract harness-record` advertises these input schemas and their
authority and privacy limits. The existing `contract agent-output` catalog
remains limited to read models emitted by Fairway. A producer submits one
record or an ordered batch through
a file/stdin CLI in the first slice. HTTP, gRPC, A2A, ACP, MCP, OpenTelemetry,
Seaway, or vendor adapters translate into the same contracts later; none is a
required core transport.

Compatibility rules:

- a producer and Fairway must agree on the schema major version;
- additive optional fields are compatible within a major version;
- unknown fields may be retained only as bounded namespaced metadata and are
  never interpreted as authority;
- unsupported schema major versions fail before any record is written;
- a batch validates fully before writing and commits atomically;
- duplicate identical records are reported as existing, not duplicated;
- conflicting duplicate identities fail visibly and preserve the first row;
- task, session, source, run, subject, and prior-run references are checked
  before commit;
- future timestamps outside the configured clock-skew allowance are rejected;
  and
- raw/private or secret-like retained fields are rejected before persistence.

Replay equality uses a canonical SHA-256 payload digest retained with every
row. Fairway decodes the supported schema, rejects duplicate JSON object keys,
normalizes field order through its typed representation, omits transport-only
batch position, and encodes one compact UTF-8 JSON object with no insignificant
whitespace. Explicit JSON values remain distinct from omitted optional fields.
Namespaced metadata is recursively key-sorted, depth/size bounded, and included
in the digest. A matching identity and digest is an idempotent replay; a
matching identity and different digest is a conflict. The digest is an
idempotency aid, not an artifact signature or evidence-integrity claim.

The first CLI shape is transport-neutral:

```bash
fairway harness ingest --file <records.json>
fairway harness ingest --stdin
fairway harness runs --task <task-id> [--format text|json]
fairway harness record <external-run-id> --source <source-id> [--format text|json]
```

Ingestion is the only new mutation. Readback never changes task, session,
review, evidence, outcome, or policy state. Existing `session.external_run_id`
remains a compatibility shortcut for monitor attachments; the new external-run
record is the durable many-run relationship and may link to that session.
`harness record` requires the source because external-run IDs are only unique
inside a source. It returns the external-run row plus all directly correlated
observations and evaluator results in stable time/identity order. Task-scoped
records without a run appear in `harness runs --task` under a separate
`run_independent_records` collection and are never attached heuristically.

## Protocol Mapping

Adapters map protocol facts into Fairway without importing protocol authority:

| External surface | Fairway mapping | Explicit non-mapping |
|---|---|---|
| MCP | Tool/resource identity may appear as namespaced observation metadata. | MCP tool success is not task success or evidence acceptance. |
| ACP | Session/run identity and typed updates may correlate to an external run. | ACP session state is not Fairway task state. |
| A2A | Agent task, status, and artifacts may correlate to a run and observations. | An A2A task is not created as a Fairway task automatically. |
| OpenTelemetry | Trace/span identity and measurements may support observations and usage. | A successful span is not verification or approval. |
| Seaway | Uses the richer correlation and runtime semantics in `seaway-integration.md`. | Seaway admission, approval, and terminal state retain their existing boundary. |

## Verified-Outcome Efficiency

After records exist, one read-only report may join:

- external-run attempts and elapsed time;
- existing provider usage and cost records;
- observation outcomes;
- evaluator results;
- attributable task outcomes; and
- explicit missing, estimated, partial, or unavailable denominators.

The supported units are cost, elapsed time, requests/tokens when reported,
attempts, and recorded actions per evaluator-backed outcome. Results are
bounded to named cohorts and source versions. The report must not rank people,
models, or providers as generally better, infer unobserved activity, or label a
task complete. Cross-provider comparison requires compatible evaluator,
subject, environment, and completeness dimensions; otherwise it reports the
comparison as unavailable.

## Trajectory Advisory

The first trajectory detector is deterministic and report-only. It may report
a candidate when durable records show one or more of:

- the same evaluator and subject failing repeatedly without a changed
  hypothesis or repository revision;
- repeated rejected or inconclusive observations with the same action
  fingerprint;
- a configured interval with no new observation, evidence, decision, or
  repository revision while a run/session remains active; or
- an exhausted retry budget already represented by Fairway's coordinator
  model.

Every finding cites the rows and thresholds that produced it, reports missing
telemetry and false-positive limits, and recommends one of `continue`,
`reframe_hypothesis`, `change_evaluator`, `change_execution_profile`,
`request_input`, or `causal_reset`. It cannot emit a provider command, cancel a
run, create or close a task, satisfy a review, or waive a mandatory control.

The reusable redirect packet contains current task intent, latest decisions,
observations, evaluator results, failed/rejected approaches, remaining
uncertainty, and the next bounded experiment. It excludes raw provider context.

## Provider Capability Profile

A later additive schema may describe whether a harness supports resume,
snapshot, structured events, cancellation, approval pause, evaluator hooks,
artifact references, policy attestation, and complete usage reporting. This is
compatibility metadata, not a quality score or provider endorsement. It is not
required for the first implementation slice.

## Delivery Sequence

1. Add the four schemas, append-only storage, CLI ingestion/readback, and one
   file/stdin adapter example.
2. Prove replay, conflict, privacy, migration, no-adapter, and zero/one/many-run
   behavior with focused tests.
3. Add verified-outcome efficiency and deterministic trajectory advisory using
   only recorded facts.
4. Pilot the redirect packet against one bounded consumer workflow and measure
   false positives, missing data, and whether the redirect led to a new
   evaluator-backed observation.
5. Update product and public pages from the exact implemented boundary.
6. Consider protocol-specific or Seaway adapters only after a versioned
   external contract and fixtures exist.

## Non-Goals

This increment does not add a model gateway, provider router, prompt manager,
context optimizer, agent loop, sandbox, credential store, network/egress
enforcer, transcript store, autonomous supervisor, universal evaluator,
quality score, or automatic task/review/promotion transition.
