# Changelog

All notable changes to Fairway are documented here.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and uses semantic versioning.

## Unreleased

No unreleased changes yet.

## v0.1.13

### Added

- A reader-oriented public documentation portal with a five-minute first-value
  path, evidence-backed capability labels, an ecosystem responsibility map,
  integration guidance, and an explicitly internal AI Cloud case study.
- Repeatable dashboard contention and real-data assessment packets for
  separating product projection defects from deployment and dataset hygiene.

### Changed

- Fairway is now consistently described as engineering control and
  accountability for agent-driven delivery. Coordination, lanes, sessions,
  and worktrees remain capabilities rather than the product category.
- Core defaults, audit language, examples, help, and current documentation no
  longer assume one GPUaaS consumer. Historical and compatibility material is
  labeled rather than silently rewritten.
- Dashboard wall, diagnostics, reports, task detail, coordinator, audit, and
  store projections use bounded or batched read models so larger Fairway data
  sets do not make routine views wait on every heavy diagnostic.
- Dashboard SSE polling uses incremental cursors and bounded review-wait sweeps
  rather than hydrating the full event and review surface every second.

### Fixed

- Unknown and static dashboard routes no longer fall through to expensive wall
  projections.
- The docs backlog audit reports generic consumer lessons while retaining the
  previous JSON key for one documented compatibility window.

## v0.1.12

### Added

- Atomic common-path work start/status plus guarded verification and closeout
  commands over existing task, session, checkpoint, evidence, review, git, and
  reconciliation primitives.
- Structured task decision records with independent quality assessment,
  supersession, accepted scope additions, and context-packet projection.
- Track-memory lifecycle history, promotion/disposition controls, and
  progressive common-path dashboard guidance.
- Review routing preflight, lifecycle-aware failure routing and wait hygiene,
  managed local binary cache lifecycle, and consumer capability/minimum-version
  readiness reporting.
- Deterministic grounded code explanation packets for file, line, Go symbol,
  commit, and task entry points, with cited contracts, decisions, evidence, and
  reviews.
- Optional loopback-only local Ollama narrative rendering with strict
  recorded/inferred/unknown labels, citation validation, privacy rejection,
  and no Fairway state writeback.

### Changed

- Reversible intent-to-diff classification remains advisory after the measured
  common-path pilot did not establish sufficient precision for blocking
  promotion. Existing consequential boundaries remain blocking.
- Common-path recommendations and failure routing distinguish lifecycle,
  review, notification, and capability state more precisely without turning
  advisory findings into workflow authority.

### Fixed

- Latest review verdict reads now use durable append order so tied timestamps
  or wall-clock movement cannot invert review-completion state.

## v0.1.11

### Fixed

- Dashboard lifecycle status no longer reports the querying CLI's version and
  binary as the identity of an older running dashboard process.
- Managed dashboards now use versioned JSON lifecycle records and fail closed
  for legacy, mismatched-process, or mismatched-listen records before start,
  stop, or restart signals a process.

## v0.1.10

### Added

- Dashboard route/projection timing instrumentation, board fast path, batched
  review/evidence projections, short snapshot cache with singleflight, and
  lazy-loaded diagnostics panels for larger Fairway stores.
- Shared-team read-only server/API skeleton, identity/authz guard, append-only
  evidence/checkpoint write pilot, and guarded status/review write pilot with
  idempotency, audit, expected-state, and reviewer-accountability controls.
- Disposable Postgres rehearsal packets and optional disposable
  apply/import/readback proof using environment-sourced DSNs and
  Fairway-prefixed schemas.
- Mac mini GitLab lab deployment runbook, Fairway doctor diagnostics, lane
  runtime lifecycle commands, agent-output contracts, and small-team shared
  pilot assessment.
- Managed read-only server lifecycle commands and a clean-state operator/CI
  rehearsal covering config diagnostics, backup/restore, API readback, timing,
  write-disabled assertions, and cleanup.

### Changed

- Bounded read-only small-team operation is supported on an operator-controlled
  host with a loopback Fairway origin. Shared writes, non-loopback origins,
  trusted proxy verification, dashboard writes, provider-send, deploy,
  live-operation authority, and a production Postgres runtime switch remain
  preview or unsupported.
- Dashboard performance work keeps heavyweight diagnostics available while
  making the default board path suitable for routine operator use.

## v0.1.9

### Added

- Supply-chain provenance reports, prompt packets, hash manifests, and release
  attestation links that avoid embedding raw prompts, transcripts, tool bodies,
  generated content, auth tokens, or provider-private data.
- Safe read-only dashboard evidence artifact viewer for configured local roots,
  with traversal/symlink/remote/directory rejection, escaped rendering, and
  credential/internal URL redaction before display truncation.
- Reversible-risk, grouped-review, and prototype-first review policy profiles,
  plus UX media evidence summaries, process-overhead reporting, owner
  rough-edge queue, and small-team autonomy operating documentation.
- Environment deploy preflight packet rendering, reusable task recipe
  extraction/rendering, and cross-project `/reports` activity rollups for
  registered Fairway project DBs.

### Changed

- Multi-project reports now label duplicate task IDs by project, expose
  project/status/evidence-type filters, and degrade unavailable project DBs into
  visible report rows instead of failing available project visibility.
- Dashboard/report additions remain read-only and do not add provider-send,
  workflow mutation, approval, merge, deploy, release, or live-operation
  authority.

## v0.1.8

### Added

- `fairway notify send` for explicitly configured `log` and `webhook`
  external notifier adapters. Send destinations and bearer tokens are resolved
  from environment variables at send time, rate limits are attempt-based, and
  notification evidence records send attempts plus delivered/failed outcomes.
- Environment deploy preflight packet guidance for reusable demo, staging,
  airgap, and production-like deploy rehearsal handoffs using existing packet
  templates, evidence, checkpoints, handoffs, completion handbacks, and
  read-only dashboard/report projections.

### Changed

- Project registry identity now includes repository path, DB path, and config
  path so same-repo multi-config Fairway dashboards can show separate project
  lanes without one registration replacing another.
- Dashboard and operator docs clarify same-repo multi-config labels and
  environment readiness projection while preserving the read-only dashboard
  trust boundary.

## v0.1.7

### Added

- Durable generic wait commands: `fairway wait add` records parked work,
  repeated handoffs, live-window waits, and non-review waits through existing
  checkpoint-backed wait projection, and `fairway wait ack` records explicit
  acknowledgement without deleting history.
- Advisory provider adapter configuration with read-only listing and validation
  surfaces, keeping provider output non-authoritative and outside approval,
  merge, deploy, wake, or provider-private data authority.
- Dry-run external notifier configuration and `fairway notify dry-run` support
  for fixed-template notification previews without Slack/email/Teams hard
  dependencies or dashboard send authority.
- Trusted proxy identity verification design for future Cloudflare Access or
  identity-aware proxy verification, documented as a model only with runtime
  verifier middleware split to a later security task.

### Changed

- Docs clarify that external notifier intent records store template/mode
  metadata only, not arbitrary wake prompts, transcripts, raw tool bodies,
  generated content, auth tokens, or provider-private data.

## v0.1.5

### Added

- Configurable local rule-pack sources, rule validation, evidence-type
  discovery, and task rule matching.
- Dashboard task-detail and report surfaces for selected rule matches, missing
  blocking/advisory evidence, and non-applicable rule rationale.
- `merge-ready` and `workflow check --mode close` rule evidence checks, where
  blocking rule sources fail readiness and advisory sources warn.
- `fairway packet rules <task-id>` for read-only selected/non-applicable rule
  review packets with required evidence, recommended commands, review domains,
  and residual-risk fields.

### Changed

- Public positioning now leads with Governed Agentic Engineering as the
  operating model and describes Fairway as the coordination control plane for
  that model.

## v0.1.4

### Added

- Remote push intent recording with `fairway record push-intent`, including
  closeout/workflow guard findings for remote branches without recorded intent.
- Review-debt and dashboard-performance reconciliation assessment artifacts.
- Public docs navigation updates for product boundaries, backlog sources,
  dashboard, agent guide, and release notes.

## v0.1.3

### Added

- Dashboard v2 is now the unified dashboard system: `/` serves the wall,
  `/board` serves the operator board with URL-backed sorting, search, filters,
  columns, bulk actions, and CSV/JSON export, `/board?tab=diagnostics` serves
  operational diagnostics, `/reports` serves retrospectives, and `/tasks/<id>`
  remains task detail.
- Dashboard board saved views now support personal views in
  `~/.fairway/views.json`, read-only team views from `.fairway/views.json`,
  "Save current view", and Cmd/Ctrl+1..9 shortcuts for the first nine personal
  views.
- Dashboard board keyboard navigation now supports row cursor movement,
  task-detail opening, search/menu focus, row selection, status/handoff dialogs,
  theme toggle, wall navigation, help, and Escape close behavior.
- Dashboard multi-project mode now mounts `/board` with the same operator
  toolbar and table, including project filter chips, Project column display,
  saved-view query state, and CSV/JSON exports that respect the project filter.
- Dashboard multi-project mode now mounts `/` as a grouped wall with
  collapsible project headers, project-prefixed activity, and per-project
  readiness rollups while keeping `/projects` as the compact registry summary.
- Dashboard wall now consumes typed SSE events for handoff arcs, live
  verb-first activity ticker entries, relative timestamps, and heartbeat pulse
  states on working task pills.
- Active-work reconciliation now detects monitor sessions without backing proof,
  so CI/deploy/UAT/provider monitors cannot leave fake active work behind when
  no automation, process, external poller, or bounded manual checkpoint exists.
- Active-work reconciliation and dashboard diagnostics now report
  `monitor_completion_resume_needed` when monitors finish cleanly but ready work
  remains and no active session/watcher has resumed the coordinator loop.
- The dashboard now includes `/reports`, a daily retrospective view with
  delivery-vs-bookkeeping summaries, lane outcomes, CI/deploy/UAT timeline,
  follow-up taxonomy, review/evidence summaries, bounded drill-down rows, and
  Markdown/JSON/CSV exports for the selected filters.
- Plane local evaluation docs and fixtures now define the repeatable workspace
  setup, seed issues, field mapping questions, and planning-only boundary for
  future tracker adapter work.
- Provider-neutral tracker contract support now covers Plane, Jira, and Linear
  registry entries, dry-run configure/import/export/resolve/reconcile command
  surfaces, and Plane/Jira/Linear link persistence without allowing tracker
  state to mutate Fairway execution state.
- Plane tracker adapter spike commands now render dry-run Plane issue payloads,
  fixture import previews, and execution-summary comments from Fairway state
  while explicitly rejecting apply/write paths.
- Provider usage accounting records normalized provider/session/task usage with
  source and confidence, including input, cached input, output, reasoning, and
  total token fields when an adapter can provide them.
- Provider-neutral OpenTelemetry usage ingestion maps OTLP logs, metrics, and
  traces into Fairway usage records without requiring prompt, tool-body, raw API
  body, auth-token, transcript, or generated-content capture.
- Codex usage attribution supports Codex-shaped OTel events,
  `codex exec --json` / NDJSON `turn.completed.usage`, and caller-supplied
  start/end token snapshots.
- Claude Code usage attribution supports OTel token/cost metrics and API
  request token attributes while keeping content telemetry disabled for usage
  accounting.
- Work batches now model shared implementation and validation units across
  multiple granular tasks, with CLI support for batch creation, membership,
  evidence mapping, CI/deploy-run links, dashboard context, and audit findings
  for over-split validation work.
- Release-run packets and `fairway release verify` now coordinate release
  attempts, including release notes, changelog state, CI/docs/signing/notary
  evidence, GitHub release state, asset URL checks, Homebrew cask version, and
  brew fetch verification.
- Local-first task queue, SQLite store, migrations, role lanes, worktrees,
  sessions, evidence, handoffs, reviews, checkpoints, packets, watchers,
  regression packs, tracker links, dashboard, TUI, and release packaging.
- Workstream profiles with profile gates, profile-aware task metadata,
  readiness reports, dashboard grouping/filtering, configurable packet
  templates, structured guard evidence, and review-domain merge readiness.
- Workflow checks that combine git cleanliness, unpushed commit detection,
  deploy-run guidance, and active-work reconciliation into one operator guard.
- Draft release notes for the first `v0.1.0` release candidate and a public
  archive index for historical decision/adoption notes.
- Homebrew release runbook covering tap initialization, signed macOS artifacts,
  notarization credentials, first-tag workflow, and post-publish verification.

### Changed

- GPUaaS remains the first adoption example, while Fairway core stays generic
  around profiles, evidence, reviews, readiness, and risk.
- Dashboard v2 has replaced the legacy mixed dashboard. There is no dashboard
  version selector; `/` is the wall, `/board` is the operator board,
  `/board?tab=diagnostics` is diagnostics, `/reports` is retrospectives, and
  `/tasks/<id>` is task detail. Historical configs that still contain
  `[dashboard] surface` continue to load because unknown TOML keys are ignored,
  but the key is not part of the active config contract. See
  [docs/design/dashboard.md](docs/design/dashboard.md).
- Saved links that used `/?status=<state>` should migrate to
  `/board?status=<state>`. Other board state is query-string based, so saved
  `/board` links for filters, sort, columns, page, and project remain
  shareable.
- Docusaurus navigation now prioritizes current product docs, release notes,
  and governance; historical GPUaaS adoption and dashboard redesign material
  moved to archive/provenance.
- GoReleaser cask metadata now uses the repository's Apache-2.0 license.
- Release workflow uses GoReleaser OSS with native macOS `codesign` and
  `notarytool` hooks for CLI binary signing/notarization instead of requiring
  GoReleaser Pro.
