# Changelog

All notable changes to Fairway are documented here.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and uses semantic versioning.

## Unreleased

### Added

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
- Docusaurus navigation now prioritizes current product docs, release notes,
  and governance; historical GPUaaS adoption and dashboard redesign material
  moved to archive/provenance.
- GoReleaser cask metadata now uses the repository's Apache-2.0 license.
- Release workflow uses GoReleaser OSS with native macOS `codesign` and
  `notarytool` hooks for CLI binary signing/notarization instead of requiring
  GoReleaser Pro.
