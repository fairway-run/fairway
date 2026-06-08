# Changelog

All notable changes to Fairway are documented here.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and uses semantic versioning.

## Unreleased

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
