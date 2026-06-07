# Release Notes

Fairway `v0.1.1` is the first public release with signed/notarized macOS
artifacts and a Homebrew cask. The initial `v0.1.0` release artifact was yanked
because its CLI version metadata still reported the development version.

## v0.1.3

### What Changed

- Provider usage accounting now records normalized usage events with provider,
  session, task, role, phase, model, token counts, source, and confidence.
  Unknown usage remains unknown rather than being reported as zero.
- Provider-supported OpenTelemetry ingestion is available through
  `examples/session-adapters/provider-otel-ingest.sh`. The bridge maps
  structural OTLP log, metric, and trace attributes into Fairway usage records
  without requiring prompt, tool-body, raw API body, auth-token, transcript, or
  generated-content capture.
- Codex usage can be attributed through Codex-shaped OTel events,
  `codex exec --json` / NDJSON `turn.completed.usage`, or caller-supplied
  start/end token snapshots.
- Claude Code usage can be attributed through OTel token/cost metrics and API
  request token attributes while keeping raw prompt/tool/body telemetry disabled
  for usage accounting.
- Work batches are now a first-class coordination model. A batch can group
  multiple granular Fairway tasks under one branch, worktree, validation
  command set, CI/deploy run, and shared evidence mapping.
- `fairway batch create|add|remove|evidence|link|show|list` supports shared
  validation planning and maps batch evidence back to member tasks by default.
- Dashboard reports and task detail now expose batch context so operators can
  distinguish granular task progress from validation batches.
- The board task table now supports URL-backed sort state, debounced search
  URL state, clearable filter chips, and a project filter for shareable operator
  views.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- adapter syntax checks pass for `provider-event.sh`,
  `provider-otel-ingest.sh`, and `codex-usage-adapter.sh`.
- Docusaurus portal builds from `website/`.
- `goreleaser check` passes.
- GitHub Actions CI and Docs Portal workflows pass on the pushed commit.
- Release verification confirms GitHub release state, asset URLs, Homebrew cask
  version, tap commit, and `brew fetch`.

## v0.1.2

### What Changed

- Active-work reconciliation now reports
  `monitor_session_without_backing_proof` when a CI/deploy/UAT/provider monitor
  session is active but has no backing automation id, PID/tmux pane, external
  run plus polling command, or fresh bounded manual checkpoint.
- Dashboard wall, board diagnostics, and task detail surfaces now show monitor
  proof warnings so stale monitor bookkeeping is not mistaken for live work.
- Session metadata now records provider-neutral monitor proof fields for
  automation-backed, process-backed, external-run-backed, and manually bounded
  monitor sessions.
- Watcher and agent docs now require backing monitor proof before leaving
  monitor tasks `in_progress`.
- Release attempts can now use `fairway packet release-run` and
  `fairway release verify` to track version/tag/source SHA, release notes,
  changelog state, CI/docs/signing/notary evidence, GitHub release state, asset
  URL checks, Homebrew cask version, tap commit, and brew fetch verification.
- The release guard explicitly fails the v0.1.2 failure mode where the Homebrew
  cask points to the new version while the GitHub release is still a draft.

## v0.1.1

### What Is Included

- Local-first Fairway CLI with SQLite-backed tasks, state transitions, evidence,
  handoffs, reviews, sessions, checkpoints, watchers, packets, and worktrees.
- Config-driven lanes, roles, branch naming, review routes, workstream profiles,
  state-machine controls, and gate definitions.
- Unified dashboard system:
  - `/` wall view for lane-level coordination
  - `/board` operator board with filtering, sorting, pagination, export, and
    diagnostics tab
  - `/tasks/<id>` task detail view
- Provider-session coordination with event checkpoints for external Codex,
  Claude, tmux, or shell attachments.
- Active-work reconciliation and workflow guard:
  - stale `in_progress` detection
  - session/task mismatch reporting
  - dirty docs/code detection
  - unpushed commit detection
  - deploy/UAT hygiene mode
- Adoption artifact generation for workstream readiness, evidence gaps, review
  routing, regression-pack validation, and gate status.
- Release packaging for signed/notarized macOS artifacts and a Homebrew cask
  publishing path.
- Public docs portal build and Cloudflare Pages deployment for
  [fairway.run](https://fairway.run).

### Operating Guidance

- Use `fairway workflow check` before handoff, review, deploy, or UAT work.
- Use `fairway workflow check --mode deploy --require-clean --require-pushed`
  before asking another track to deploy from a branch.
- Record evidence and make an explicit status decision in the same work burst.
- Keep parent/backlog tasks out of `in_progress` unless someone is actively
  producing a rollup artifact.
- Commit at meaningful review boundaries instead of accumulating a full day of
  unrelated changes.

### Known Limits

- Provider-specific session launchers are still maturing. Fairway records and
  reconciles provider events, but most agents are still started manually.
- The dashboard supports the current wall, board, diagnostics, and task-detail
  flows, but richer in-place mutations and saved views are future work.
- Jira and Linear integrations remain documented adapters, not full production
  API integrations.
- Postgres support is a compatibility/adoption target, not the default runtime
  store.
- The docs portal is public and product-focused; archived adoption notes remain
  available for provenance but should not be treated as the current user path.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- `goreleaser check` passes.
- `go run ./cmd/fairway workflow check --mode deploy --require-clean --require-pushed`
  reports no blocking findings.
- Docusaurus portal builds with `npm run build` from `website/`.
- GitHub Actions CI and Docs Portal workflows pass on the pushed commit.
- macOS signing and notarization credentials are available only through local or
  CI secret stores.
- Homebrew tap repository has an initialized `main` branch.
- Required release secrets are configured on `fairway-run/fairway`:
  `HOMEBREW_TAP_GITHUB_TOKEN`, `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`,
  `MACOS_CODESIGN_IDENTITY`, `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`, and
  `MACOS_NOTARY_ISSUER_ID`.
- Local signing/notarization smoke has passed with ignored certificate artifacts.
- Homebrew cask update is verified after the tagged release publishes.
