# Dashboard Redesign Closeout

Date: 2026-06-07
Owner: governance
Queue: `docs/archive/dashboard-redesign-backlog.yaml`

## Decision

Retire `docs/archive/dashboard-redesign-backlog.yaml` as the active dashboard
implementation queue after this closeout document is reviewed.

The backlog has served its purpose as the dashboard v2 build plan. The remaining
open dashboard items are intentionally blocked external-signoff or historical
performance tasks, not ready implementation work. Future dashboard work should
enter Fairway as normal product backlog items rather than continuing to import
or operate from the archived redesign queue.

Do not update `.fairway/config.toml` away from the archived dashboard queue
until this closeout decision is reviewed. After review, set the active
`queue_source` to the next approved queue or to `inline` if the DB is the active
source of truth.

Review status:

- Architecture: approved on 2026-06-07.
- Governance: approved on 2026-06-07.

Active queue replacement:

- Promoted backlog: `docs/roadmap/fairway-product-backlog.yaml`.
- Local config target: `yaml:docs/roadmap/fairway-product-backlog.yaml`.
- Source material retained at `examples/fairway-adoption-improvements.yaml`.

## Shipped State

The dashboard is now the unified local operator surface:

- `/` renders the wall view.
- `/board` renders the operator board.
- `/board?tab=diagnostics` renders operational diagnostics.
- `/reports` renders retrospective reports.
- `/tasks/<id>` renders task detail.
- `/wall` redirects to `/`.

Shipped wall capabilities:

- role lanes with backlog, claimed, working, review, and done columns;
- lane expansion with queue, working detail, pending review, and latest events;
- active-session visibility distinct from task status;
- diagnostics banners for active-work and coverage/learning findings;
- typed SSE handling for handoff arcs, live activity ticker entries, relative
  timestamps, and heartbeat pulse states;
- multi-project wall grouping with collapsible project headers and project
  readiness rollups.

Shipped board capabilities:

- URL-backed search, filters, sort, pagination, and column state;
- filter chips for role, status, project, profile, kind, domain, risk, and
  review domain;
- column chooser and saved views;
- keyboard navigation for board operators;
- bulk coordination actions for supported non-terminal dashboard mutations;
- CSV/JSON export of the current filtered view;
- server-side board windows for large filtered task sets;
- multi-project board support with a Project column and project-aware exports.

Shipped diagnostics/report/detail capabilities:

- active reconciliation findings;
- work-coverage and CI/deploy learning findings;
- sessions, worktrees, watchers, and checkpoints;
- daily/date-range reports with Markdown, JSON, and CSV export;
- task detail with metadata, evidence, sessions, reviews, usage, batches, and
  task-scoped diagnostics.

## Validation

The final dashboard stack was reviewed and pushed after these checks passed:

```bash
GOCACHE=/private/tmp/fairway-gocache go test ./...
GOCACHE=/private/tmp/fairway-gocache go vet ./...
git diff --check
fairway reconcile active --dry-run
```

Final active queue state at closeout:

```text
total: 51
done: 48
blocked: 3
ready FWRD tasks: none
```

## Intentional Blocked Items

### `FWRD-129` Board Virtualization At 200-Row Threshold

Status: blocked.

Reason: the client-side virtualization slice did not satisfy the original
first-paint/sort budget because the server still emitted the full 1000-row
payload. Follow-up `FWRD-161` implemented server-side board windows and resolved
the operational board latency issue. `FWRD-129` remains blocked as historical
evidence that the original client-only acceptance was not met.

Reconciliation: FW-138 confirms this remains the intended state. Do not mark
`FWRD-129` done unless the original acceptance is rewritten or waived.

### `FWRD-151` Performance Budget Verification

Status: blocked.

Reason: the initial 1000-task performance verification failed. Later
server-side windowing resolved wall/board/sort latency, and `FWRD-162` accepted
the RSS exception at `<=52 MiB` for the documented local single-project
operator fixture. `FWRD-151` remains blocked because its original strict budget
was not met as written.

Reconciliation: FW-138 confirms this remains the intended state. The current
release posture depends on the reviewed FWRD-162 exception, not on treating the
original strict FWRD-151 budget as passed.

Primary evidence:

- `docs/assessment/dashboard-performance-budget.md`

### `FWRD-154` GPUaaS Operator Walkthrough And Feedback Capture

Status: blocked.

Reason: local Fairway dogfood walkthrough is documented, but named GPUaaS
operator signoff is still pending. Do not represent this as external operator
approval until the signoff section is completed or explicitly waived.

Primary evidence:

- `docs/assessment/dashboard-v2-walkthrough.md`

## Deferred Work

These are not blockers for retiring the redesign queue:

- browser Paint Timing API capture from a surface that exposes exact paint
  entries;
- multi-project performance measurement beyond the current local single-project
  fixture;
- future operator feedback from GPUaaS signoff;
- future dashboard enhancements that arise from normal product backlog intake.

## Retirement Procedure

After this document is reviewed:

1. Update `.fairway/config.toml` so `queue_source` no longer points at
   `docs/archive/dashboard-redesign-backlog.yaml`.
2. Preserve `docs/archive/dashboard-redesign-backlog.yaml` as historical design
   evidence, not an active queue.
3. Keep the three blocked tasks as explicit historical/product evidence unless
   a reviewer chooses to close or supersede them with a documented decision.
4. Start new Fairway dashboard or adoption work only from the next approved
   backlog source.
