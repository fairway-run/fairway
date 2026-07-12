# Assurance profiles

## Purpose

Fairway assurance profiles describe how recorded engineering facts can support
an assessment. They make evidence collection repeatable without turning Fairway
into a certification authority, auditor, policy approver, or workflow mutation
surface.

The first schema is `fairway.assurance-profile.v1`. A profile is a local YAML or
JSON file validated with:

```bash
fairway assurance profile validate examples/assurance-profiles/example-evidence-support.yaml
fairway --json assurance profile validate examples/assurance-profiles/example-evidence-support.yaml
fairway assurance profiles list --dir examples/assurance-profiles
fairway assurance evidence map --profile examples/assurance-profiles/example-evidence-support.yaml --task FW-355 --at 2026-07-12T12:00:00Z
fairway assurance readiness --profile examples/assurance-profiles/example-evidence-support.yaml --scope task_set --scope-id assurance-core --task FW-355 --at 2026-07-12T12:00:00Z
```

The example profile is deliberately not a standards mapping. Fairway also
ships [versioned starter profiles](../assurance/starter-profiles.md) for a
small, explicitly incomplete selection of objectives from named sources. The
[authoring guide](../assurance/authoring.md) and
[compatibility policy](../assurance/compatibility.md) define how custom
profiles are reviewed and versioned.

## Contract

A profile declares:

- profile and framework identity, exact version, and HTTPS source;
- applicability and supported project, task-set, or release scopes;
- control objectives and assessment objectives;
- product, customer, shared, or external-assessor responsibility;
- accepted evidence classes, minimum counts, results, and freshness;
- whether an independent external assessment is required;
- prohibited claims and prohibited workflow actions.

The schema contains no command, script, hook, expression, query, or provider
prompt field. Unknown fields fail validation. Profile files must be regular
local `.yaml`, `.yml`, or `.json` files no larger than 1 MiB; symlinks and remote
URLs are rejected. Framework sources are HTTPS references without embedded
credentials, query strings, or fragments.

## Fixed vocabulary

Scope types are `project`, `task_set`, and `release`.

Responsibilities are:

| Value | Meaning |
|---|---|
| `product` | The evaluated product or producing project owns the evidence. |
| `customer` | The adopting organization must provide and assess the evidence. |
| `shared` | Product and customer evidence are both required. |
| `external_assessor` | Only an independent external assessment can satisfy the objective. |

Evidence classes are bounded to existing engineering fact families: task,
decision, evidence, review, CI, release, provenance, rehearsal, exception,
external assessment, configuration, backup/restore, vulnerability, identity,
and audit. A later read model maps those classes to Fairway records; the profile
does not contain evidence or mutate the Fairway database.

## Fail-closed validation

Validation rejects:

- unknown schema versions or unknown fields;
- duplicate control IDs or duplicate evidence classes;
- unsupported scope, responsibility, evidence, and result values;
- invalid or non-positive freshness durations;
- secret-like values and shell/executable markers;
- missing `certified`, `compliant`, or `authorized` claim prohibitions;
- missing evidence-only authority or any required prohibited action.

The required authority boundary prohibits certification, compliance declaration,
risk acceptance, approval, workflow mutation, merge, deploy, release, credential
use, public-exposure changes, and live operations.

`assurance profile diff` compares two valid profile files and classifies stable
changed paths as metadata-only, additive, or breaking. It treats same-version
content changes, framework changes, control removals or edits, narrowed scopes,
and reduced claim or action guards as breaking. The report is review input
only; it does not approve an update or infer framework equivalence.

## Status and authority

FW-355 implements profile parsing and validation only. It does not evaluate a
control, infer compliance, generate an assessor package, or write findings.
Those read-only surfaces are split across FW-356 through FW-359.

FW-356 adds the first deterministic read model with `assurance evidence map`.
It projects existing task, evidence, review, and task-decision records into
normalized fact references and evaluates only whether current, in-scope facts
match each control's explicit evidence requirement. It does not persist mapped
facts or control results.
The optional `--at` argument fixes the evaluation clock. With the same profile,
Fairway facts, and clock, JSON output is byte-stable; the selected clock is
reported as `evaluated_at`.

The projection excludes command text, notes, review reasons, decision
rationale, artifact paths, and artifact contents. Each fact retains a stable
Fairway reference, class, result, timestamp, actor or producer when recorded,
project/task scope, applicability, freshness, state, and confidence boundary.
Fact freshness is labeled `requirement_relative`; each control requirement
evaluates its own `maximum_age` against `evaluated_at`, so one class can be
stale for a strict control and current for a longer retention window.
Stale, conflicting, superseded, unreviewed, out-of-scope, and external
assessment facts remain visible and cannot satisfy a requirement. External
assessment records are labeled as assertions requiring assessor validation.
Controls marked `external_assessment_required` remain unsupported in this
read model even when all product-side facts match; Fairway does not convert an
external assertion into an assessor conclusion.

## Readiness and gaps

FW-357 builds a scope-level report over normalized maps. Requirements are
evaluated across the selected project, task set, or release scope, rather than
requiring every individual task to contain the minimum evidence count. The
project scope always selects the complete configured project. Task-set and
release scopes require an explicit scope identifier and task list. The
stable status vocabulary is:

- `satisfied_by_recorded_evidence`
- `partial`
- `missing`
- `stale`
- `conflicting`
- `customer_responsibility`
- `external_assessment_required`
- `exception_recorded`
- `not_applicable_with_rationale`

The report never emits `compliant` or `certified` as a control status. A gap
names the control and evidence class, responsible party, next evidence action,
recorded source references, freshness rule, and assessor boundary. Gap output
is read-only and does not automatically create tasks, accept exceptions, or
change control state.

FW-358 consumes the same normalized maps and readiness report to create the
bounded, optionally signed bundle described in
[assurance packages](assurance-packages.md).

Generated findings and packages remain assessment inputs. A certification or
authorization may be recorded later only as explicit external evidence from the
named authority for the exact scope, product version, and configuration.
