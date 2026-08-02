# Control Effectiveness

Tracked by the `FW-388` control-effectiveness epic and its implementation
children `FW-386`, `FW-379`, `FW-380`, `FW-381`, and `FW-387`.

Fairway should be able to test its own governance claims with the same standard
it applies to engineering work: durable facts, explicit limits, and no promotion
of generated interpretation into authority.

The useful question is not whether Fairway, or process in general, "helps."
The useful question is whether a specific control distinguishes work with a
different observable outcome after accounting for coverage, time, and task
size. The first product surface is an advisory, read-only analysis over Fairway
and Git facts. It does not establish causality and cannot change workflow state.

## Product Boundary

Control-effectiveness analytics may:

- measure whether commits and changed paths are represented by Fairway tasks;
- compare contemporaneous task cohorts where a control was applicable and was
  or was not observed;
- report outcome rates, process cost, sample size, exclusions, and uncertainty;
- recommend keeping, narrowing, instrumenting, redesigning, or investigating a
  control.

It must not:

- approve or waive a review, gate, merge, deploy, release, or live operation;
- describe observational association as causal impact;
- recommend removing a mandatory security, legal, release, credential, or
  production-safety invariant solely because observed defects are rare;
- infer incidents, corrective work, or control application from unconstrained
  prose when a structured fact is available or required;
- create a second analytics store when the result can be derived from Fairway,
  Git, CI/CD, and linked incident facts.

## Measurement Contract

Every reported control row uses the following explicit states.

| Field | Meaning |
|---|---|
| `control_id` | Stable configured rule, gate, review, evidence expectation, preflight, or process-control identity. |
| `family` | Quality gate, security invariant, evidence, process, preflight, or compliance. |
| `applicable` | The task matched the control's configured applicability rules. |
| `observed` | The expected control action or evidence exists for the task. |
| `triggered` | The control found a defect, blocked unsafe progress, requested a change, or otherwise produced its defined positive signal. |
| `passed` | The control executed and found no issue. Passing is not equivalent to not applicable. |
| `bypassed` | The control was applicable but explicitly waived or deferred. The reason, authority, and stable source identity must remain visible; legacy skipped evidence without those facts remains unknown. |
| `unknown` | Available records cannot distinguish observed, bypassed, and not applicable. Unknown rows do not enter effectiveness comparisons. |

"Fired" is not used as a persisted state because it can mean either "ran" or
"found something." Reports use `observed` and `triggered` separately.

## Coverage Before Outcomes

The first result for every window is commit-to-task coverage:

```text
covered commits / eligible commits
covered changed files / eligible changed files
```

Eligible commits exclude merge-only commits and configured generated or
high-churn paths. The report lists every exclusion. Coverage is also segmented
by risk, profile, repository, and task size where those facts exist.

If urgent, failed, or unusually complex work bypasses Fairway, the recorded
population is selected and downstream comparisons are biased. A report with
insufficient coverage may describe the missing population but must not rank
control effectiveness.

## Outcome Model

The initial Git-derived proxy is **post-promotion touch rate**, not "defect
rate." For each promoted task, Fairway records whether another eligible commit
touches the same owned files within 7, 14, and 30 days. Generated files,
lockfiles, release metadata, and configured high-churn paths are excluded.

Same-file touches are noisy: planned follow-up, adjacent feature work, and
refactoring are not necessarily rework. Interpretation becomes stronger when a
touch is linked to one or more structured outcomes:

- task reopen or retry;
- corrective or superseding task;
- rollback or revert;
- failed CI, deploy, smoke, UAT, or live evidence;
- incident or escaped defect.

Near-term reports expose the Git proxy and structured links separately. They do
not silently label every subsequent edit as a defect.

Outcome links are integrity checked. Incident and rollback rows require an
explicit external reference; corrective and superseding rows require a
different existing Fairway task; and reopen rows require the ID of an existing
terminal-to-active task transition. Bounded notes and references pass the same
content-free secret detector used by other retained Fairway records.

For an outcome horizon `H`, a task is mature only when promotion occurred at
least `H` days before the report's `as_of` time. Right-censored tasks remain in
the raw export but are excluded from that horizon's denominator. For one
control, profile, time window, risk band, and size band:

Git-derived windows use the task completion timestamp as the promotion clock
and Git committer time (`%cI`) as the integration clock for descendant commits.
Author time is retained as metadata but does not place a touch into a window.

```text
eligible cohort = applicable + mature + known control state
observed cohort = eligible cohort where observed=true
bypassed cohort = eligible cohort where bypassed=true
outcome rate = unique tasks with the named outcome / tasks in that cohort
outcome delta = observed outcome rate - bypassed outcome rate
trigger yield = observed tasks where triggered=true / observed tasks
```

`passed` and `triggered` partition the observed cohort for descriptive yield;
they are not substituted for the observed/bypassed comparison. Not-applicable
and unknown tasks are excluded and counted separately. Each outcome category
has its own rate. An `any_outcome` rate counts a task once even when it has
multiple linked outcomes.

Control friction is reported only from attributable facts. Its initial measures
are control-specific review or wait records per applicable task and elapsed time
from a recorded control-required/start fact to its resolution. If those facts
do not exist, friction is `unavailable`, not zero. Aggregates report median and
p90 with sample size; they do not mix notification, handoff, or total task time
into a control-specific cost without an explicit attribution.

Generated and high-churn exclusions are versioned configuration, not report
arguments. Every exclusion requires a path pattern, category, and rationale.
The report records the configuration revision and digest used for the cohort.
An exclusion changed after the window began creates separate cohorts; it cannot
retroactively remove unfavorable outcomes from an earlier cohort.

Different control families use different primary outcomes:

| Control family | Primary outcomes |
|---|---|
| Quality gate | Corrective touch, reopen, failed validation, escaped defect |
| Security invariant | Violation detected, unsafe action prevented, incident |
| Evidence | Verifiability, completeness, reuse, missing-fact rate |
| Process | Coverage, delay, bypass, coordination cost |
| Preflight | Failed-run avoidance, recovery time, repeated attempt rate |
| Compliance | Required coverage and auditability; not optimized away by sparse incidents |

## Cohorts And Confounds

Comparisons are observational and use tasks from the same bounded time window.
This limits model, team, product, and process drift. Before comparing outcomes,
rows are stratified by the best available task-size proxy:

- eligible changed lines;
- eligible file count;
- configured risk level;
- profile and owning domain.

The minimum implementation uses deterministic size bands and reports results
inside each band. It does not combine incomparable bands into one persuasive
number. Later matching or regression is allowed only if the raw cohort rows and
formula remain exportable.

Required confound readback includes:

- coverage and missing-population rate;
- model/provider and source revision when recorded;
- control selection or applicability rules;
- task-size and risk distribution;
- observation-window completeness;
- excluded paths and missing outcome links.

## Classification

Fairway may classify a control as:

- `discriminating`: a sufficiently covered cohort shows a material outcome
  difference in the control's expected direction;
- `high_friction`: measured cost is material and no matching outcome signal is
  currently observed;
- `insufficient_sample`: too few applicable tasks or outcomes;
- `insufficient_coverage`: commit/task or control-state coverage is too low;
- `mandatory_invariant`: effectiveness is not the basis for removal;
- `redesign_candidate`: the control is measurable but its signal, placement, or
  cost does not support the current form.

The report language is deliberately bounded:

```text
No measurable incremental signal under the current sample, coverage,
risk controls, and outcome definition.
```

It must never shorten that statement to "the control does nothing."

`mandatory_invariant` is authoritative input from reviewed project policy,
rule-pack metadata, or built-in Fairway safety policy. Analytics can never infer
or remove that status. It overrides `high_friction` and `redesign_candidate`:
the report may recommend better instrumentation or a less costly implementation
only when the same invariant remains enforced. It may not recommend waiving,
narrowing, relaxing, or redesigning away the protected behavior.

## CLI And Dashboard

The CLI is the canonical advisory report surface:

```bash
fairway control report --since 30d [--profile <name>] [--control <id>] \
  [--format text|json]
```

The report includes coverage, cohort definitions, raw counts, outcome rates,
friction, exclusions, classifications, and limitations. JSON preserves the
task and Git fact references needed to reproduce every aggregate.

Authority remains with Fairway records, reviewed policy and rule-pack
configuration, Git, CI/CD, and linked incident systems. The report only
projects those sources.

The dashboard is a read-only projection of the same report model. Its useful
views are:

1. coverage and data-quality summary;
2. control table with cohort size, outcome delta, friction, and classification;
3. filters for window, profile, risk, size band, and control family;
4. drill-down to the tasks, commits, evidence, and structured outcome links
   behind an aggregate.

The dashboard does not create policy changes. A person may use the report to
propose a reviewed config or rule-pack change through the normal Git workflow.

## Delivery Order

1. Define this metric contract and add the versioned epic.
2. Add structured outcome links and Git-derived coverage/touch facts.
3. Add the advisory CLI report and deterministic cohort classification.
4. Add the dashboard projection without duplicating analytics logic.
5. Run a GPUaaS pilot, beginning with coverage, and record keep, narrow,
   redesign, instrument, or defer decisions for representative controls.

The first GPUaaS pilot is complete. Commit-to-task coverage remained below the
configured interpretation threshold in 14-, 30-, and 90-day windows, so every
cohort correctly remained `insufficient_coverage`. The pilot kept the
coverage-first suppression and existing safety controls, narrowed the validated
claim to data-quality readback, and identified commit association, structured
outcomes, friction, preflight/security metadata, and evidence-trigger semantics
as the next instrumentation boundaries. See
[GPUaaS Control-Effectiveness Pilot](../assessment/gpuaas-control-effectiveness-pilot-2026-08-02.md).

The pilot must run before any control is relaxed based on these metrics.
