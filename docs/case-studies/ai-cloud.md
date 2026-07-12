# AI Cloud: an internal Fairway case study

## Scope

AI Cloud is Fairway's largest active internal consumer. This case study explains
how the team uses Fairway as an independent engineering record and control
layer while coding agents, reviewers, source control, CI, deployment tooling,
and operators retain their own responsibilities.

This is an internal operating case study, not customer evidence, a compliance
claim, or a controlled productivity experiment. Quantitative results below link
to reproducible Fairway or repository evidence. Internal activity volume is
reported only when it helps diagnose process or product behavior.

## The problem

AI Cloud work spans implementation, review, documentation, environment
rehearsal, release, and live-operation handoffs. Provider sessions are
replaceable and their context can be compacted or lost. Chat alone could not
reliably answer:

- what work was authorized and what remained outside scope;
- which material decisions changed the implementation;
- what evidence and independent review supported promotion;
- which actor had to act next after a wait or closeout; and
- whether a release or environment handoff completed under its stated boundary.

The team needed durable coordination without making provider transcripts,
generated rationale, or the shared dashboard authoritative.

## Operating model

Fairway holds the task contract, lifecycle state, curated decisions, evidence
references, reviews, waits, checkpoints, and provider-session attachments. Git
records the implementation. CI and environment tools produce their own proof.
Reviewers contribute independent judgment. Operators retain deployment and live
authority.

The resulting hierarchy is deliberate:

1. The task and policy define accountable intent and authority.
2. Git records what changed.
3. Curated decisions explain material choices and deviations.
4. Evidence and independent review support acceptance and promotion.
5. Memory packets and session records let another provider continue the work.

Raw provider transcripts are optional forensic context, not the engineering
record. The full hierarchy and privacy boundary are documented in
[Task decision memory](../design/task-decision-memory.md).

## Representative workflows

### Provider replacement and context recovery

An active lane records its provider session, checkpoint, bounded task memory,
and material decisions in Fairway. A replacement provider reads those records,
the current Git state, and referenced evidence rather than reconstructing intent
from chat. The `v0.1.12` release added first-class task decisions, track-memory
lifecycle history, atomic work start/status, and stable provider lifecycle
identity for this flow. The exact included tasks and boundaries are recorded in
the [v0.1.12 release preparation packet](../assessment/fairway-v0.1.12-release-prep-2026-07-11.md).

### Decision and evidence readback

Material scope or risk changes are recorded as bounded decision rows with fact
references and an independent quality state. `work verify`, task detail, and
context packets expose declared scope, accepted decisions, passing evidence,
and unexplained changes without inventing rationale from a transcript. The
[common-path pilot](../assessment/fairway-common-path-pilot-2026-07-11.md)
found two accepted material decisions out of two measured decision rows, with
no stale memory or promotion-debt finding in that cohort.

### Independent review

Review policy remains risk-scaled. Reversible work can iterate with evidence,
while security, live, deploy, release, credential, public-exposure, migration,
irreversible, and other consequential boundaries remain blocking. In the
common-path cohort, review found six changes across two tasks and those defects
were corrected before commit; seven other tasks had approval-only reviews. The
same assessment also found that review records and notifications per task rose,
which is treated as process overhead to improve rather than proof that more
review is always better.

### Release

Release preparation, publication, and dashboard restart are separate task
boundaries. The preparation packet records the candidate source, tests, vet,
configuration and workflow guards, release tooling checks, known limits, and
the actions explicitly deferred to publish and restart tasks. This made the
`v0.1.12` promotion auditable without allowing a documentation task, provider,
or dashboard to tag, publish, or restart by implication.

### Environment rehearsal

The small-team read-only pilot packaged configuration validation, doctor
diagnostics, SQLite backup and restored-store readback, managed lifecycle
start/status/log/stop, API and CLI status checks, endpoint timings, cleanup,
and a promote/repeat/block recommendation. A fresh GitHub Actions runner
exposed a `master` versus `main` initialization mismatch; the harness was fixed
to initialize the disposable repository explicitly, and the final clean run
passed. See the [repeat-pilot assessment](../assessment/fairway-small-team-repeat-pilot-2026-07-10.md).

## Measured outcomes

The [common-path observational pilot](../assessment/fairway-common-path-pilot-2026-07-11.md)
compared four historical small-team tooling tasks with nine `v0.1.12` train
tasks. The cohorts differed in complexity and availability, so the results do
not establish causation. They did show:

| Measure | Historical baseline | Common path |
| --- | ---: | ---: |
| Task-state CLI transitions per task | 3.75 | 3.00 |
| Tasks with active checkpoint | 4/4 | 9/9 |
| Tasks with provider session | 4/4 | 9/9 |
| Tasks with passing evidence | 4/4 | 9/9 |
| Active to done, average | 6,194.2 s | 713.5 s |
| Last validation to done, average | 3,965.0 s | 131.8 s |
| Review records per task | 3.25 | 4.00 |
| Notifications per task | 1.75 | 3.33 |

The useful result is not the number of rows created. It is that the measured
cohort retained sessions, checkpoints, passing evidence, and consequential
review enforcement while reducing lifecycle and closeout delay. Increased
review and notification overhead remains a cost. The assessment therefore kept
reversible intent-to-diff findings advisory because it lacked labeled precision
and false-positive data.

The independent read-only repeat pilot also established a supported bounded
shape: one operator-controlled host, loopback origin, explicit binary/config/
database/pid/log readback, backup/restore rehearsal, and local CLI fallback.
It did not promote shared writes or public origin binding.

## Defects and process gaps found

Using Fairway against the real AI Cloud data set found issues that smaller
fixtures did not expose:

- the clean environment rehearsal found a default-branch mismatch and converted
  the prose sequence into one reusable local/CI harness;
- consumer review domains could become unroutable too late, historical waits
  were noisy, failure routing overstated actionable debt, managed binaries
  dirtied consumer repositories, and binary capability drift lacked one
  readback surface; these were converted into bounded product work in the
  [consumer gap audit](../assessment/ai-cloud-consumer-gap-audit-2026-07-11.md);
- the released dashboard's real-data assessment found an idle SSE connection
  consuming about one CPU core, uncached task detail around 9.6 seconds on the
  local full-access instance, and cold reports around 11.8 seconds. It also
  showed that the snapshot cache could make warm routes appear healthy while
  cold projections remained slow. Exact routes, row counts, timings, and
  follow-up IDs are in the
  [dashboard performance assessment](../assessment/fairway-v0.1.12-ai-cloud-dashboard-performance-2026-07-11.md).

These findings are separated into product defects, data/archive hygiene, and
deployment/runtime behavior. Historical task volume is not used to excuse
product defects.

## Assurance package pilot

A bounded three-task node-security set was exported with the NIST SSDF 1.1
starter profile. Export took 0.02 seconds and pinned-key offline verification
took 0.01 seconds. One narrow accountability control was supported, two were
partial, and release provenance was missing. The package contained metadata and
Fairway references only; it did not import AI Cloud source or artifact bodies.

The result supports continued internal assessment preparation because the tool
made the missing decision, provenance, release, and vulnerability evidence
explicit. It does not establish full SSDF coverage or any certification,
compliance, procurement, legal, or authorization outcome. See the
[assurance package pilot](../assessment/fairway-assurance-package-pilot-2026-07-12.md).

## Known limitations

- The timing comparison is observational and small; it does not isolate Fairway
  as the cause of every improvement.
- Fairway does not retain a complete command ledger or provider transcript, so
  exact authoring effort cannot be reconstructed from the durable record.
- Reversible intent-to-diff classification remains advisory pending measured
  precision and false-positive results.
- Shared-team write APIs, trusted-proxy runtime verification, non-loopback
  Fairway origins, and Postgres runtime storage remain preview or unsupported.
- The dashboard remains read-only for shared visibility and has no provider
  send, approval, merge, deploy, release, credential, or live-operation
  authority.
- The AI Cloud case is evidence for Fairway's internal use, not evidence of
  adoption or outcomes in unrelated organizations.

## What the team retained

The durable result is an engineering record that survives provider replacement
and separates intent, implementation, explanation, evidence, independent
judgment, and promotion. Fairway reduced chat-only reconstruction while making
its own overhead and defects measurable. It did not replace source control,
CI/CD, issue systems, identity controls, deployment tooling, or accountable
human authority.
