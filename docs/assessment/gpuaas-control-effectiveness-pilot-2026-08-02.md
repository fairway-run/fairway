# GPUaaS Control-Effectiveness Pilot

Date: 2026-08-02

Fairway task: `FW-387`

Fairway source: `5b5d8ddd741ba745af30ddd252100dea425bce22`

GPUaaS source: `2bdba92f3cf1b870cd34ac47dd89eaa21e71e00e`

## Scope

This pilot ran the implemented `fairway.control-effectiveness.v1` advisory
model against the GPUaaS platform-foundation Fairway store and Git history. It
asked whether the current records support interpretation of representative
review, evidence, preflight, security, and process controls.

The pilot is an observational data-quality assessment. It does not establish
causal control impact, change policy, or authorize removal of a control.

## Method

The same committed Fairway binary produced JSON reports for contemporaneous
14-, 30-, and 90-day windows:

```text
fairway --config .fairway/platform-foundation-config.toml \
  control report --since <336h|720h|2160h> --format json
```

The project had no control-effectiveness override. The run therefore used the
versioned product defaults:

- minimum sample size: 5;
- minimum coverage ratio: 0.80;
- material outcome delta: 0.10;
- high-friction p90: 900 seconds;
- configuration revision: `unversioned`;
- 90-day configuration digest:
  `sha256:4cfee86de7bd5ffe9493e2ac60e422dac4f114d3fc2208f1953229dcdf842234`.

Representative 14-day outcome horizons were inspected inside deterministic
small, medium, large, and unknown diff-size bands. The pilot did not combine
those bands into one effectiveness ranking.

## Coverage Result

Coverage is the first result and the interpretation gate.

| Observation window | Eligible commits | Covered commits | Commit coverage | Eligible files | Covered files | File coverage |
|---|---:|---:|---:|---:|---:|---:|
| 14 days | 225 | 48 | 21.3% | 3,154 | 2,963 | 93.9% |
| 30 days | 454 | 157 | 34.6% | 7,348 | 7,282 | 99.1% |
| 90 days | 2,425 | 434 | 17.9% | 22,492 | 22,365 | 99.4% |

Commit-to-task coverage is below the configured 80% threshold in every
window. The high changed-file coverage does not compensate for this gap. Broad
task source/target path ownership can cover files even when individual commits
are not durably associated with the task that produced them.

All 195 14-day, 363 30-day, and 555 90-day cohorts were therefore classified
`insufficient_coverage`. This suppression is correct product behavior.

## Current Maintenance Facts

The 30-day report contains:

- 257 promoted tasks;
- 12 unique configured control IDs across 363 risk/size/horizon cohorts;
- 1,569 observed control facts;
- 186 unknown control-state facts;
- no explicit attributable bypass facts;
- no attributable friction facts;
- no structured incident, rollback, reopen, corrective, or superseding-task
  outcome links;
- Git-derived `post_promotion_touch` as the only available outcome.

The Git-touch proxy is too common to stand alone as a defect signal. Among the
367 applicable, mature, outcome-known facts where the control was observed at
the 14-day horizon, 335 contain `post_promotion_touch`. This is still a
task/control-fact count: a task may appear once per applicable control, and a
same-file touch may represent planned follow-up, adjacent feature work, or
refactoring. It must not be called escaped rework.

## Representative Controls

The table below describes execution signal at the 30-day window and 14-day
outcome horizon. It does not compare outcome rates because the coverage and
bypass denominators are insufficient.

| Control | Applicable | Observed | Unknown | Triggered | Passed | Friction samples |
|---|---:|---:|---:|---:|---:|---:|
| `review:architecture` | 115 | 108 | 7 | 12 | 96 | 0 |
| `review:backend` | 107 | 93 | 14 | 15 | 78 | 0 |
| `review:ops` | 105 | 91 | 14 | 16 | 75 | 0 |
| `review:product-quality` | 47 | 42 | 5 | 6 | 36 | 0 |
| `review:security` | 103 | 95 | 8 | 14 | 81 | 0 |
| `gate:platform-foundation:boundary-guard-report` | 6 | 5 | 1 | 0 | 5 | 0 |
| `gate:platform-foundation:ownership-map-evidence` | 10 | 8 | 2 | 0 | 8 | 0 |

Review controls produced changes-requested signals in every represented size
band. Architecture review triggered on 4 of 61 observed large tasks, 3 of 16
medium tasks, and 5 of 31 small tasks. Backend review triggered on 6 of 45
large, 3 of 19 medium, and 5 of 27 small tasks. Security review triggered on 6
of 53 large, 2 of 15 medium, and 5 of 26 small tasks. These are execution-yield
facts, not proof of incremental effectiveness.

The two evidence gates have observable pass coverage but no retained trigger
semantics or attributable friction. No stable preflight-family control appears
in the current report. `review:security` is represented as `quality_gate`, not
as a configured mandatory `security_invariant`. Those are instrumentation and
configuration gaps; they are not evidence that the controls are ineffective.

## Decisions

| Decision | Boundary | Rationale |
|---|---|---|
| **Keep** | Coverage-first suppression | It prevented low-linkage records from producing persuasive but invalid rankings. |
| **Keep** | Architecture, backend, ops, product-quality, and security review controls | They have observable execution and trigger yield. This pilot cannot estimate incremental effect or justify removal. |
| **Keep** | Mandatory safety invariants | Sparse incidents and absent bypasses can never authorize removing security, credential, release, deploy, or live-operation safeguards. |
| **Narrow** | Current product claim | Control analytics is implemented and validated for coverage/data-quality readback. GPUaaS has not yet validated causal or incremental control effectiveness. |
| **Redesign** | GPUaaS commit association | Make task identity durable in normal commit/promotion flow so commit coverage does not depend mainly on message convention or broad path ownership. |
| **Instrument** | Structured outcomes | Link incidents, rollbacks, reopens, corrective work, and superseding tasks before relying on same-file touches. |
| **Instrument** | Control friction | Record attributable control start/resolution or review-wait facts; unavailable must remain distinct from zero. |
| **Instrument** | Preflight and security semantics | Add stable control IDs/families and reviewed mandatory-invariant metadata rather than inferring them from names. |
| **Instrument** | Evidence-gate trigger semantics | Distinguish a gate that found a defect from one that merely recorded a passing artifact. |
| **Defer** | Outcome-delta rankings and control-removal proposals | Resume only after coverage and outcome-integrity gates are met; do not manufacture bypass cohorts for measurement. |

## Next Measurement Gate

The next GPUaaS effectiveness interpretation should require all of the
following:

1. commit-to-task and changed-file coverage at or above 80% for two
   consecutive 30-day windows;
2. per-cohort control-state coverage at or above 80% for any cohort being
   interpreted;
3. retained structured outcomes for incidents, rollbacks, reopens, and
   corrective/superseding work where those events occur;
4. attributable friction samples for controls whose cost is being discussed;
5. stable preflight and security-family metadata, including reviewed mandatory
   invariants;
6. natural, explicitly authorized bypass/defer facts for non-mandatory controls
   when they occur, without weakening a control to create an experiment;
7. nonzero, naturally occurring observed and bypassed cohorts, each meeting the
   configured minimum sample, before any observed-versus-bypassed outcome delta
   is interpreted; otherwise the comparison remains `insufficient_sample`;
8. contemporaneous size/risk cohorts that preserve those separate observed and
   bypassed denominators.

Until then, Fairway should continue reporting coverage, missing instrumentation,
execution yield, and uncertainty without ranking controls.

## Evidence

The project-local raw outputs are retained outside version control at:

```text
GPUasService/tmp-ux/fairway-control-effectiveness-pilot-2026-08-02/
```

Their SHA-256 digests, derivation notes, classification counts, and the bounded
representative-control extracts used by this assessment are committed in
[`evidence/fw-387/gpuaas-control-pilot-summary.json`](evidence/fw-387/gpuaas-control-pilot-summary.json).
