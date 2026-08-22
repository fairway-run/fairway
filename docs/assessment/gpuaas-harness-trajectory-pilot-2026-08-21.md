# GPUaaS Harness-Record And Trajectory Pilot

Date: 2026-08-21

Fairway task: `FW-413`

Consumer task: `FAIRWAY-TRAJECTORY-SUPERVISION-PILOT-001`

Fairway implementation: `d74d025`

## Decision

**Keep and narrow.** Keep the versioned harness records, task-local named
cohort, evaluator-backed efficiency projection, and cited repeated-pattern
advisory. Narrow the current claim to an experimental, manually integrated
read-only capability. Do not yet add automatic provider redirects, cross-task
or cross-provider comparisons, model routing, or product-availability claims.

The pilot demonstrates that a consumer can preserve a failed strategy, detect
its repetition from durable facts, record a materially different redirect, and
show the resulting evaluator-backed observation without granting the evaluator
or advisory any Fairway workflow authority. One pilot is not evidence that the
detector generally improves engineering outcomes.

## Consumer Workflow

GPUaaS had already configured a `trajectory-redirect` packet and measured its
fail-closed behavior. The public harness contract records that real workflow as
three attempts:

1. render with objective-only context; deterministic evaluator fails because
   `current_strategy` is missing;
2. deliberately repeat the under-specified strategy as a detector-calibration
   control; the exact command again fails closed on missing `current_strategy`;
3. redirect to complete context containing the old strategy, observations, new
   bounded experiment, expected observation, stop condition, and authority
   boundary; the evaluator passes.

The fixture is
[`gpuaas-trajectory-supervision-harness-records.json`](fixtures/gpuaas-trajectory-supervision-harness-records.json).
It uses safe summaries and artifact references, not prompts, private reasoning,
transcripts, raw tool bodies, generated-content dumps, or credentials.

## Measured Readback

The exact batch was ingested into the GPUaaS Fairway store through the public
`harness ingest` command. The first ingest inserted three runs, three
observations, and three evaluator results. An identical second ingest inserted
nothing and reported all nine source-scoped identities as existing, confirming
canonical replay behavior.

`harness report` then returned:

| Measure | Result |
|---|---|
| External-run attempts | 3 |
| Explicit recorded actions | 3 |
| Evaluator results | 3 |
| Evaluator-backed outcomes | 3 |
| Named cohort | `configured-packet-render@1.0/configured-packet:trajectory-redirect/GPUasService-local/complete` |
| Attempts per evaluator-backed outcome | 1 |
| Actions per evaluator-backed outcome | 1 |
| Usage/cost efficiency | unavailable; no task usage event or exact comparable cost denominator |
| Repeated-action finding | two rejected/inconclusive objective-only actions; recommend `change_execution_profile` |
| Repeated-evaluator finding | two failed/inconclusive results without changed revision or hypothesis; recommend `reframe_hypothesis` |

The second baseline is a controlled calibration input, not a claim that the
consumer naturally became stuck twice. It provides a known repeated pattern so
the detector's citations and false-positive warning can be inspected. The
later passing result remains in the same named evaluator cohort and cites a
new hypothesis and action fingerprint. The findings preserve the historical
reason for redirect; they do not disappear merely because the next experiment
passes.

## False Positives And Missing Data

The detector cannot know whether two identical fingerprints hid materially
different inputs or environments. It also cannot see a meaningful strategy
change that the producer failed to express through revision, hypothesis, or
action identity. A supervisor must inspect the cited rows before redirecting.

This pilot intentionally had no provider-usage event correlated through a
session referenced by the external runs. Fairway therefore withheld token,
elapsed-time, and cost ratios rather than assigning zero or borrowing unrelated
task usage. That is the correct result and a concrete integration requirement
for any future efficiency study.

The no-new-evidence advisory did not fire because the consumer task had no live
provider session. A stale checkpoint alone is insufficient. This avoids
presenting historical task metadata as current execution stagnation.

## Authority Readback

Ingestion, replay, reporting, and the dashboard projection did not:

- change either task's state;
- create or accept Fairway evidence;
- create or satisfy a review;
- contact, cancel, pause, resume, or redirect a provider;
- approve a merge, deployment, release, or live operation; or
- make the GPUaaS packet or this integration generally available.

The stored recommendation is a next question for a supervisor. Normal task,
review, source-control, environment, and promotion authorities remain external.

## Next Validation

Before broadening the feature, run it on several naturally occurring stalled
tasks with producer-emitted session correlation and exact or explicitly
estimated usage. Measure whether a cited redirect produces new evaluator-backed
information with less repeated work and acceptable reviewer effort. Add more
cohort support only when separate populations can remain visibly separate; do
not average incompatible tasks, evaluators, subjects, environments, or
completeness levels.
