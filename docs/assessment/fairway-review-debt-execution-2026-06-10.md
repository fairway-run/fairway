# Fairway Review Debt Execution Sweep

Date: 2026-06-10
Task: FW-151
Owner: governance

## Scope

This pass executes the follow-up from `FW-137` without weakening the review
model. The goal is to reduce false coordinator-plan noise and leave only real
historical review debt.

Source command:

```bash
go run ./cmd/fairway coordinator plan --recommendation-limit 80
```

## Policy

The sweep does not fabricate approvals. Historical review debt can be cleared
only by:

- a required reviewer recording a real review,
- a reviewer approving an existing artifact-backed review,
- governance recording an explicit waiver with scope, reason, and residual risk.

Validation logs, commit history, old chat summaries, or stale checkpoints are
not approval evidence by themselves.

## Finding

Before this pass, coordinator plan reported 38 review-debt actions. Eight were
false positives caused by stale terminal `review` checkpoints on tasks whose
required review domains were already approved.

Examples observed during the sweep:

- `FW-150` had approved `arch`, `governance`, `backend`, `ops`, and `security`
  reviews, but an older `review` checkpoint still appeared as review debt.
- `FW-123` had approved `arch`, `governance`, and `backend` reviews, but an
  old implementation-ready checkpoint still appeared as review debt.

The issue was in coordinator planning semantics, not in task review data.

## Change

Coordinator planning no longer treats a terminal task's open `review`
checkpoint as review debt by itself. Review debt is now sourced from the
current task definition's required review domains and recorded approved
reviews.

This preserves the intended model:

- terminal task with missing required review domains: review debt;
- terminal task with all required review domains approved: no review debt,
  even if an old `review` checkpoint exists;
- non-terminal task in explicit review state: active review gate.

## Current Result

After the fix, coordinator plan reports 30 historical review-debt items. These
match the original `FW-137` inventory class and remain intentionally pending.

Remaining review debt:

| Task | Role | Missing domains |
|---|---|---|
| FW-107 | ui | arch, governance |
| FW-108 | backend | arch, governance |
| FW-109 | backend | arch, governance |
| FW-111 | backend | arch, governance |
| FW-112 | backend | governance |
| FW-113 | backend | arch, governance |
| FW-114 | ops | arch, governance, ops |
| FW-115 | ui | arch, governance, ui |
| FW-116 | backend | arch, governance, ops |
| FW-117 | ops | arch, governance, ops |
| FW-118 | backend | arch, governance, ops |
| FW-119 | ui | arch, governance, ui |
| FW-128 | backend | arch, backend, governance, ops |
| FW-170 | ops | ops |
| FW-171 | ops | ops |
| FW-172 | governance | governance |
| FWRD-101 | backend | arch |
| FWRD-102 | ui | arch |
| FWRD-103 | ui | arch |
| FWRD-104 | backend | arch, governance |
| FWRD-110 | ui | arch |
| FWRD-120 | ui | arch |
| FWRD-121 | ui | arch |
| FWRD-123 | ui | arch |
| FWRD-124 | ui | arch |
| FWRD-130 | backend | arch, governance |
| FWRD-131 | backend | arch |
| FWRD-132 | ui | arch |
| FWRD-140 | ui | arch |
| FWRD-170 | ui | arch |

## Validation

Commands:

```bash
GOCACHE=/private/tmp/fairway-gocache go test ./...
GOCACHE=/private/tmp/fairway-gocache go vet ./...
git diff --check
go run ./cmd/fairway config validate
go run ./cmd/fairway reconcile active --dry-run
go run ./cmd/fairway coordinator plan --recommendation-limit 80
```

Result:

- tests passed,
- vet passed,
- diff check passed,
- config validation passed,
- active reconciliation was clean,
- review debt dropped from 38 to 30 without recording synthetic approvals.

## Next Action

Leave the remaining 30 items as historical review debt unless a reviewer wants
to perform real artifact-backed reviews or governance chooses explicit scoped
waivers. They should not block current GPUaaS-driven Fairway product work.
