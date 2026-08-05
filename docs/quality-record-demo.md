# Quality Record Demo

This ten-minute demonstration shows Fairway's implemented quality-record and
measurement boundary without presenting the product as an autonomous reviewer,
quality score, or complete AI Quality System.

## 1. Start With One Task

Choose a task that has intent, evidence, review, and a delivered commit:

```bash
fairway task-detail <task-id>
fairway quality-record <task-id>
```

Point out that the projection spans intent, decisions, production context,
evidence, verification, judgment, promotion, outcomes, and lessons. Every fact
cites a Fairway record or names an external authority.

## 2. Show Honest States

Use a record containing at least one absent or unresolved stage:

```bash
fairway quality-record <task-id> --format json | jq '.summary, .sections'
```

Explain the five states:

- `present`: a cited record exists;
- `missing`: an expected Fairway fact was not recorded;
- `unavailable`: the fact was not measurable or retained;
- `conflicting`: cited facts disagree and require interpretation;
- `externally_owned`: another system or accountable person owns the authority.

Fairway does not generate a narrative to make an incomplete record look
complete.

## 3. Connect Work To Delivery

Show the append-only commit associations:

```bash
fairway task-detail <task-id>
git show --stat <commit-sha>
```

Normal `work start` and `work close` retain baseline, work, and completion
commits. Git remains the source authority; Fairway records the relationship.

## 4. Separate One Record From Population Claims

Run the advisory population report:

```bash
fairway control report --since 720h --format text
```

Lead with commit coverage and missing instrumentation. Do not interpret outcome
deltas when coverage, samples, or structured outcomes are insufficient. A
passing task record is not proof that a control caused a better outcome.

## 5. Close On Authority

Open the read-only task dashboard and identify the same Quality Record stages.
Then state the boundary plainly:

> Coding agents implement. Reviewers judge. Git and CI/CD promote and verify.
> Operators act on live systems. Fairway connects the cited record and reports
> what is missing; it does not silently acquire those authorities.

## Evidence To Cite

- [GPUaaS Quality Record Pilot](assessment/gpuaas-quality-record-pilot-2026-08-05.md)
- [Control Effectiveness](design/control-effectiveness.md)
- [Quality Engineering for AI-Assisted Software Delivery](design/ai-quality-engineering.md)
- [Product Boundaries](design/product-boundaries.md)
