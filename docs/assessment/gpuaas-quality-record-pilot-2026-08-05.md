# GPUaaS Quality Record Pilot

Date: 2026-08-05  
Fairway task: `FW-395`  
Fairway source: `1582601bf02bc38142468736dff1b44aafc9e555`  
GPUaaS source: `8000199e21b968c7ba3370d876acc1ae67472302`

## Purpose

This second GPUaaS quality-system pilot tests whether the versioned
`fairway.quality-record.v1` projection can reconstruct a bounded, cited record
from a large consumer's existing Fairway history. It also establishes the
post-instrumentation baseline for task-to-commit links, structured outcomes,
and attributable control friction.

The pilot does not score task quality, qualify verifiers or reviewers, infer
missing facts, establish causal control effectiveness, or backfill historical
records.

## Method

The committed Fairway binary was run against the GPUaaS platform-foundation
store after a consistent SQLite backup. The population was every task that was
still `done` and had a `done` transition in the preceding 30 days: 288 tasks.
Each task was projected independently with:

```text
fairway --config .fairway/platform-foundation-config.toml \
  quality-record <task-id> --format json
```

Six recent cross-domain tasks were retained as drill-down examples. The same
binary also generated a 30-day control-effectiveness report. Generated raw
records remain in the GPUaaS `tmp-ux` workspace; only bounded aggregates and
digests are committed here.

## Population Result

| Quality Record stage | Present | Other state | Interpretation |
|---|---:|---:|---|
| Intent | 218 (75.7%) | 70 missing | Titles and roles existed, but 70 records lacked at least one required intent element, usually acceptance checks. |
| Material decisions | 5 (1.7%) | 283 missing | Decision capture was exceptional rather than normal. |
| Production context | 246 (85.4%) | 42 missing | Sessions, checkpoints, or commit context existed for most tasks. |
| Collected evidence | 288 (100%) | 0 missing | The configured completion evidence gate is visible in the historical population. |
| Automatic verification | 287 present | 1 conflicting | The projection retained one blocked-then-passed verifier history as conflicting instead of silently selecting a winner. |
| Human judgment | 195 (67.7%) | 93 missing | Attributable review rows existed for two thirds of the population. Missing review is not treated as failed review. |
| Promotion decision | 0 owned | 288 externally owned | Fairway reports readiness and completion facts but does not claim Git, CI/CD, deployment, or accountable-person authority. |
| Operational outcomes | 0 present | 288 unavailable | The new structured outcome table had no historical rows and was not backfilled. |
| Lessons | 0 present | 288 missing | No explicit corrective/superseding outcome or lesson handoff met the bounded projection rule. |

Across 2,592 stage records, the projection reported 1,239 `present`, 776
`missing`, 288 `unavailable`, one `conflicting`, and 288 `externally_owned`.
The distinction is useful: missing configuration, unavailable outcome
instrumentation, contradictory verifier history, and external promotion
authority are not collapsed into one generic incomplete state.

## Conflict Drill-Down

`UAT-HANDOFF-END-USER-LIFECYCLE-001` retained the same Playwright invocation as
blocked twice and later passing. The projection reports the verification stage
as `conflicting` and cites all three evidence rows. It does not infer that the
later pass supersedes the earlier environmental or product failure. A durable
supersession or interpretation record is still required.

This is evidence that the projection can expose a review question. It is not
evidence that the verifier was qualified or that the final artifact was
correct.

## Control Measurement Baseline

The contemporaneous 30-day control report contained 449 eligible commits and
154 covered commits: 34.3% commit-to-task coverage. Changed-file coverage was
99.0%, but broad task path ownership cannot substitute for durable commit
association. The report found zero explicit `task_commits` links, zero
structured outcomes, and zero attributable friction samples in GPUaaS history.

The small difference from the 34.6% commit coverage measured on 2026-08-02 is
not interpreted as a trend because the windows have different endpoints. Both
measurements remain below the 80% interpretation threshold.

The new Fairway features establish a forward measurement boundary:

- `work start` and `work close` can now retain baseline, work, and completion
  commit associations;
- structured outcomes can distinguish incidents, rollbacks, reopens,
  corrective work, and superseding work from Git touch proxies;
- attributable friction can distinguish measured, open, unavailable, and
  missing control cost;
- the Quality Record exposes these facts without filling historical gaps.

No historical association, outcome, or friction row was manufactured for this
pilot.

## Operational Characteristics

The complete 288-task projection took 7.49 seconds wall time on the local
consumer environment, approximately 26 milliseconds per task when invoked as
separate CLI processes. Repeating a representative projection produced
byte-identical JSON after removing `generated_at`.

The projection remained read-only apart from applying the product's normal
schema migrations. A consistent pre-migration database backup was retained in
the project-local temporary evidence directory.

## Decisions

1. **Keep the Quality Record as a supported read-only projection.** It makes
   evidence boundaries and missing instrumentation visible without creating a
   score or new authority.
2. **Do not reposition Fairway as a complete AI Quality System yet.** The pilot
   validates record reconstruction and data-quality diagnosis, not trustworthy
   outcomes or process capability.
3. **Use forward instrumentation instead of historical backfill.** New GPUaaS
   work should use normal `work start`/`work close`, structured outcomes when an
   outcome occurs, and friction records when control cost is discussed.
4. **Retain coverage-first suppression.** Control rankings remain invalid until
   representative commit association and outcome coverage meet the reviewed
   thresholds.
5. **Add explicit verification supersession before interpreting conflicts.**
   Chronology alone is insufficient to decide whether a later pass closes an
   earlier failure.

## Next Measurement Gate

Run the next GPUaaS measurement only after at least one natural 30-day window
uses the new association and outcome/friction commands. Interpret a control
only when the existing coverage, sample, risk, size, and authority gates pass.
Until then, report adoption coverage, missing data, conflict yield, and measured
friction without effectiveness rankings.

## Evidence

The bounded committed summary is
[`evidence/fw-395/gpuaas-quality-record-pilot-summary.json`](evidence/fw-395/gpuaas-quality-record-pilot-summary.json).
Raw project-local artifacts are retained outside version control under:

```text
GPUasService/tmp-ux/fairway-quality-record-pilot-2026-08-05/
```
