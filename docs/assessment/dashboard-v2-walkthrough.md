# Dashboard V2 Operator Walkthrough

Date: 2026-06-07
Owner: governance
Task: FWRD-154

## Scope

This walkthrough checks whether the current Fairway dashboard is usable enough
for a GPUaaS-style operator before the remaining dashboard polish tasks are
prioritized.

Surfaces covered:

- `/` wall view
- `/board` operator board
- `/board?tab=diagnostics`
- `/reports`
- `/tasks/<id>` task detail by navigation from table/lane links

## Current Status

Local dogfood walkthrough: complete against the Fairway repository's real
`.fairway` state.

Named GPUaaS operator walkthrough: pending. Do not treat this as final external
operator signoff until a GPUaaS operator runs the checklist below and adds their
notes.

Go/no-go decision: provisional no-go for flipping any remaining compatibility
defaults solely on this document. The dashboard is usable enough for daily
Fairway dogfooding, but GPUaaS operator signoff is still required before using
this as the default cutover evidence.

Decision owner: governance.

## Environment

Repository: `/Users/subash/dev/fairway`

Command:

```bash
fairway dashboard --listen 127.0.0.1:7886 --no-open
```

Verification commands:

```bash
go run ./cmd/fairway --json reconcile active --dry-run
go run ./cmd/fairway --json audit work-coverage --dry-run --since-duration 24h
```

Result:

- Active reconciliation was clean.
- Work-coverage audit surfaced historical advisory findings:
  - uncovered changed files,
  - done tasks missing review-domain records.

Those findings are useful operator input and were visible from the wall
diagnostics banner and board diagnostics tab. They are not dashboard rendering
blockers.

## Walkthrough Checklist

### Wall

Checks:

- Role lanes render for `backend`, `ui`, `arch`, `ops`, and `governance`.
- Active provider session is visible.
- Coverage diagnostics banner appears when audit findings exist.
- Lane panels expose `Open full details for <role>` links to `/board?role=<role>`.

Observed:

- Active session showed the governance walkthrough session for `FWRD-154`.
- Coverage banner showed high-risk audit findings with an Open diagnostics link.
- Lane detail links were present for the first roles checked.

Assessment: usable.

### Board

Checked URL:

```text
/board?status=todo&columns=title,id,role,status,risk_level&sort=title
```

Checks:

- Filter chips are visible and clearable.
- Selected columns render in the requested order.
- Sort state is preserved in the URL.
- CSV and JSON export links preserve filters, sort, and columns.

Observed:

- Filtered count showed `10 / 48`.
- Headers rendered as Title, ID, Role, Status, Risk.
- CSV and JSON export links included `columns`, `sort`, and `status`.

Assessment: usable.

### Diagnostics

Checked URL:

```text
/board?tab=diagnostics
```

Panels present:

- Active Reconciliation
- Coverage Diagnostics
- Sessions
- Worktrees
- Watchers
- Checkpoints

Assessment: usable. The diagnostics tab is the right place to send operators
when wall banners indicate coverage or active-work issues.

### Reports

Checked URL:

```text
/reports
```

Observed:

- Daily Report rendered.
- Markdown, JSON, and CSV exports were present.

Assessment: usable for daily retrospective review.

## Feedback Captured

No blocking dashboard gaps were found during the local dogfood pass.

Useful follow-ups remain, but they are not blockers for using the dashboard in
Fairway dogfooding:

- `FWRD-111` wall handoff arc animation
- `FWRD-113` wall activity ticker
- `FWRD-114` heartbeat pulse
- `FWRD-126` board keyboard navigation
- `FWRD-129` board virtualization
- `FWRD-141` multi-project board support
- `FWRD-142` multi-project wall support
- `FWRD-151` performance budget verification
- `FWRD-160` remove old v1 templates/surface flag

Potential GPUaaS operator questions to validate:

- Is the wall diagnostics banner too noisy when historical review-domain gaps
  exist?
- Is `/board?tab=diagnostics` enough for CI/deploy-loop triage?
- Are column chooser and export controls discoverable without explanation?
- Does the board table stay responsive with the live GPUaaS queue size?

## Blocking Gaps

None identified from the local dogfood walkthrough.

External operator signoff remains open. If the GPUaaS operator identifies
blocking gaps, file them as follow-up Fairway tasks before any dashboard default
flip.

## Operator Signoff

GPUaaS operator:

Date:

Decision:

Notes:
