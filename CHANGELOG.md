# Changelog

All notable changes to Fairway are documented here.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and uses semantic versioning.

## Unreleased

### Added

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
- Release workflow uses GoReleaser OSS for CLI binary signing/notarization
  instead of requiring GoReleaser Pro.
