# Reports

Fairway reports are retrospective dashboard pages. Wall and Board answer live
coordination questions; the [Quality workspace](quality-workspace.md) shows
which lifecycle evidence exists across tasks; Reports answer what changed, what evidence was produced,
and what follow-up work remains.

## Primary Users

| User | Question |
|---|---|
| Coordinator | What did every lane complete today, and what is the next safe queue? |
| Operator | Which CI, deploy, UAT, and monitor runs passed or need follow-up? |
| Reviewer | Which completed tasks still need required review domains or evidence? |
| Project lead | What took time, what created new work, and what is ready to summarize? |

## Default Daily Report

`/reports` opens to the local current day. The top of the page shows:

- date selector with today, yesterday, last seven days, and custom range,
- filters for role, profile, kind, domain, risk, and tags,
- completed task count split into implementation outcomes and monitor/deploy-run
  closures,
- moving task count,
- CI/deploy/UAT result summary,
- reviews recorded and missing required review domains,
- follow-up tasks created by taxonomy.

This section must be readable without opening a table. A coordinator should be
able to answer "what happened today?" from the first viewport.

## Report Sections

### Outcome Summary

Show counts and trend chips for:

- tasks completed,
- tasks moved,
- tasks created,
- blocked tasks opened or resolved,
- reviews recorded,
- evidence rows recorded,
- CI/deploy/UAT runs passed, failed, retried, or still running.

Separate real delivery outcomes from bookkeeping:

- implementation, docs, ops, security, and UI tasks are delivery outcomes,
- CI monitor tasks, deploy-run monitor tasks, watcher sessions, and heartbeat
  closures are operational bookkeeping unless they produced a direct artifact or
  follow-up.

### Lane Outcomes

Group completed and moved tasks by role, then domain/kind. Each group shows:

- count completed,
- representative task links,
- important blocked or follow-up tasks,
- latest evidence artifact link when present.

Reports also carry task tags into JSON/CSV exports and support `tag` query
filters for cross-cutting programs such as production readiness, security
review, documentation portal work, UAT hardening, or environment-specific work.

Long groups are paginated or expandable. The default should show the most recent
or highest-risk tasks first, not a complete wall of rows.

### CI, Deploy, And UAT Timeline

Render a chronological timeline for deploy-run and monitor-related activity:

- pipeline or deploy identifier,
- source SHA or branch when recorded,
- target environment,
- result,
- elapsed wait window when available,
- generated `CI-FIX`, `CD-FIX`, `UAT-BUG`, `OPS-FIX`, `HARNESS-FIX`, or
  `DOC-FIX` follow-up tasks.

This section is the main tool for learning why CI and deploy consumed time.

For environment rehearsal packets, delivery reports group failed, blocked, or
partial evidence by `packet=<id>` and `check=<id>` values from evidence notes.
This lets repeated route-readback, worker-access, smoke, rollback, or
evidence-contract failures show up as deterministic automation candidates
without adding a second rehearsal store.

### Reviews And Evidence

Show:

- reviews recorded during the period,
- tasks marked done but still missing required review domains,
- newly satisfied readiness gates,
- failed, partial, or warning evidence rows that need follow-up.

This should use the same review-domain and gate logic as the board and
`merge-ready` checks.

Report construction batches task history, evidence, handoff, review, and
notification reads across the visible task set. It must not call `TaskDetail`
once per task. Slow-request timing records `reports.facts.batch` with the task
count, bounded query count, and `task_detail_calls=0`. Rough-edge projection
reuses the same batch evidence boundary rather than hydrating each task.

Evidence artifact links in task detail and report drill-downs must go through
the safe artifact viewer rather than raw downloads. The viewer is enabled only
for recorded evidence paths inside configured `[fairway].local_artifact_paths`.
It renders Markdown, JSON, text, and HTML as escaped/redacted HTML, rejects path
traversal and symlink escapes, redacts secret-looking values and internal
URLs, and labels the evidence boundary as local-only, internal-only, or
publishable. The dashboard remains read-only; viewing an artifact does not
acknowledge, approve, create evidence, mutate task state, merge, deploy, or
publish artifact contents.

The viewer must redact the full artifact before applying any display truncation
so an oversized secret cannot leak a prefix at the cutoff. Supported redaction
classes include bearer tokens, authorization/cookie/set-cookie headers,
token/secret/password/API key fields, common OAuth/client credential fields
such as `access_token`, `refresh_token`, `id_token`, `client_secret`, and
`ssh_private_key`, and internal URLs for localhost, RFC1918,
Tailscale/CGNAT `100.64.0.0/10`, `.local`, and `.internal` hosts. The redactor
is intentionally conservative and does not make local-only evidence
publishable; publish decisions still require explicit review and evidence
classification.

### Supply-Chain Provenance

Reports may include a provenance section for task, day, release, or commit
scopes. Provenance is a summary over existing Fairway metadata: task state,
checkpoints, sessions, handoffs, evidence references, review verdicts,
provider usage counts, batches, commits, and release verification. It is not a
raw transcript, prompt, tool-body, generated-content, credential, or artifact
content export.

The CLI exposes the same read model with:

```bash
fairway provenance report --task <task-id> --format markdown
fairway provenance report --since 168h --format json
fairway provenance prompt-packet --task <task-id>
```

The section should answer why work happened, who or what executed it, which
evidence existed, which gates were satisfied, which commit carried it, and
whether a release bundle or attestation reference exists. See
[supply-chain-provenance.md](supply-chain-provenance.md).

### Rule Pack Signals

Show rule-pack applicability without turning the report into a rule dump:

- selected rule matches across the report scope,
- tasks that selected at least one rule but are missing required evidence,
- blocking rule gaps,
- advisory rule gaps,
- non-applicable rule checks.

The summary should use the same configured rule sources, profile rule groups,
task metadata, and evidence artifact types as task detail and merge-readiness
logic. Blocking and advisory gaps must be visually distinct so coordinators can
separate stop-the-line work from improvement guidance.

### Drill-Down Table

The table is last, not first. It supports:

- search,
- role/status/profile/kind/domain/risk filters,
- include/exclude monitor and deploy-run bookkeeping,
- sortable columns,
- pagination,
- export for the selected filtered scope.

## Export

Reports support:

- Markdown for handoff and daily summaries,
- JSON for automation and audit,
- CSV for spreadsheet analysis.

Exports must use the same filters and date range visible in the browser.
The first implementation uses `format=md`, `format=json`, or `format=csv` on
`/reports`; all other query parameters are preserved so exports match the
browser scope exactly.

In multi-project dashboard mode, `/reports` uses the local project registry as
the aggregation boundary. The report shows a Cross-Project Activity section
with project, task, session, evidence, review, status movement, and activity
counts for the selected window. Drill-down rows and CSV/JSON/Markdown exports
include a project label so duplicate task ids from different Fairway DBs remain
distinguishable. Filters include project, profile, role, tag, date range,
status, and evidence type. The report is read-only; mutations still happen by
running Fairway commands in the source project worktree.

## Design Requirements

Reports should use the same dashboard shell and visual language as Wall, Board,
Diagnostics, and Task Detail:

- clear navigation between Wall, Board, Reports, Diagnostics, and task detail,
- compact summary cards with stable dimensions,
- no nested cards,
- bounded tables with pagination,
- status chips and small labels instead of paragraphs of explanation,
- no raw SQLite-shaped dumps as the default presentation.

## Implementation Notes

Use `task_state_history`, task definitions, evidence, reviews, checkpoints, and
session tables as read models. The report should query by persisted timestamps
and project timezone rules consistently with the dashboard's "done today"
metric.

Supply-chain provenance reports use the same read-model principle. The first
implementation exports deterministic JSON/Markdown from existing records
instead of adding a second provenance store.

The dashboard treats monitor/watch/deploy-run shaped tasks as bookkeeping for
summary purposes. They remain visible in the CI/deploy/UAT timeline and can be
included in the drill-down table with `include_bookkeeping=1`.

Tests should include:

- local-day boundary behavior,
- monitor/deploy-run exclusion and inclusion,
- role/domain grouping,
- follow-up taxonomy grouping,
- export matching the filtered browser scope.
