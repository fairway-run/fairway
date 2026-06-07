# Fairway Review Debt Sweep

Date: 2026-06-07
Task: FW-137
Owner: governance

## Scope

This sweep captures historical tasks that are already `done` but still fail
current `coordinator plan` review-domain checks. The goal is to make the debt
explicit, not to fabricate retrospective approval.

Source command:

```bash
go run ./cmd/fairway coordinator plan --recommendation-limit 100
```

Plan summary at sweep start:

```text
top=review-gated ready=2 active=1 waiting=5 blocked=4 stale=0 complete=69 review_gated=30 approval_gated=0 utility_gated=0 batch_recommended=0
```

## Backfill Rule

Historical review debt can be cleared only when one of these is true:

- the required reviewer/domain performs and records a real review,
- an existing review artifact is attached and the reviewer records an approval
  referencing that artifact,
- governance records an explicit waiver with scope, reason, and residual risk.

Absent one of those, the task remains review-gated. The sweep must not convert
old chat summaries, commit presence, or local validation output into approval
without a named reviewer/domain decision.

## Review-Gated Inventory

| Task | Role | Missing domains | Resolution |
|---|---|---|---|
| FWRD-101 | backend | arch | Pending architecture review or waiver. |
| FWRD-104 | backend | arch, governance | Pending architecture/governance review or waiver. |
| FWRD-130 | backend | arch, governance | Pending architecture/governance review or waiver. |
| FWRD-131 | backend | arch | Pending architecture review or waiver. |
| FW-116 | backend | arch, governance, ops | Pending architecture/governance/ops review or waiver. |
| FW-118 | backend | arch, governance, ops | Pending architecture/governance/ops review or waiver. |
| FW-128 | backend | arch, backend, governance, ops | Pending architecture/backend/governance/ops review or waiver. |
| FW-108 | backend | arch, governance | Pending architecture/governance review or waiver. |
| FW-109 | backend | arch, governance | Pending architecture/governance review or waiver. |
| FW-111 | backend | arch, governance | Pending architecture/governance review or waiver. |
| FW-112 | backend | governance | Pending governance review or waiver. |
| FW-113 | backend | arch, governance | Pending architecture/governance review or waiver. |
| FW-172 | governance | governance | Pending governance review by non-authoring reviewer or waiver. |
| FW-170 | ops | ops | Pending ops review by non-authoring reviewer or waiver. |
| FW-171 | ops | ops | Pending ops review by non-authoring reviewer or waiver. |
| FW-117 | ops | arch, governance, ops | Pending architecture/governance/ops review or waiver. |
| FW-114 | ops | arch, governance, ops | Pending architecture/governance/ops review or waiver. |
| FWRD-102 | ui | arch | Pending architecture review or waiver. |
| FWRD-103 | ui | arch | Pending architecture review or waiver. |
| FWRD-110 | ui | arch | Pending architecture review or waiver. |
| FWRD-120 | ui | arch | Pending architecture review or waiver. |
| FWRD-121 | ui | arch | Pending architecture review or waiver. |
| FWRD-123 | ui | arch | Pending architecture review or waiver. |
| FWRD-124 | ui | arch | Pending architecture review or waiver. |
| FWRD-132 | ui | arch | Pending architecture review or waiver. |
| FWRD-140 | ui | arch | Pending architecture review or waiver. |
| FWRD-170 | ui | arch | Pending architecture review or waiver. |
| FW-107 | ui | arch, governance | Pending architecture/governance review or waiver. |
| FW-115 | ui | arch, governance, ui | Pending architecture/governance/ui review or waiver. |
| FW-119 | ui | arch, governance, ui | Pending architecture/governance/ui review or waiver. |

## Recommended Sweep Order

1. Batch the dashboard-retirement FWRD tasks by architecture reviewer because
   most missing domains are architecture-only.
2. Batch backend/session/provider lifecycle tasks (`FW-108`, `FW-109`,
   `FW-111`, `FW-116`, `FW-118`, `FW-128`) with backend plus governance/ops
   only where required.
3. Batch CI/release/utility tasks (`FW-114`, `FW-117`, `FW-170`, `FW-171`) with
   ops first, then architecture/governance if still required.
4. Leave self-review-shaped items (`FW-170`, `FW-171`, `FW-172`) pending until
   a separate reviewer or explicit waiver is available.

## Current Decision

No historical approvals were backfilled by this sweep. All 30 review-gated
items remain pending until the required reviewer records approval, changes, or
waiver. This is intentional: it keeps coordinator-plan noise visible without
weakening the review-domain model.

FW-137 closes when this inventory, rule, and evidence are recorded. A later
review-debt execution task can process the inventory by domain batch.
