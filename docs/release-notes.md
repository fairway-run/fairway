# Release Notes

Fairway `v0.1.1` is the first public release with signed/notarized macOS
artifacts and a Homebrew cask. The initial `v0.1.0` release artifact was yanked
because its CLI version metadata still reported the development version.

Product boundary reminder for current releases: Fairway supports governed
agentic engineering as a coordination control plane. It is not an autonomous
workflow engine, CI runner, issue tracker replacement, LLM provider
abstraction, credential store, or provider-cost gate. Release and adapter work
must preserve the rules in [Product boundaries](design/product-boundaries.md).

## v0.1.5

### What Changed

- Public positioning now leads with Governed Agentic Engineering as the
  operating model and describes Fairway as the coordination control plane for
  that model.
- Rule packs can now be configured as local sources, validated, matched against
  task metadata, surfaced on task detail and reports, enforced by
  `merge-ready` / `workflow check --mode close`, and rendered through
  `fairway packet rules <task-id>`.
- Blocking rule sources can require matching evidence artifact types before
  merge readiness passes. Advisory rule gaps are warnings. Disabled and
  non-applicable rules remain visible without affecting readiness.
- Rule packets are read-only review/handoff artifacts. Rendering a packet does
  not approve reviews, close tasks, or mutate state; agents must explicitly
  record the packet as evidence when used.
- Rule-pack validation CI examples document `fairway rules validate` for both
  reusable platform packs and project-local packs before a pack is treated as
  reusable.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- `git diff --check` passes.
- `fairway config validate` passes.
- `fairway reconcile active --dry-run` reports no active reconciliation
  findings.
- Rule-pack docs and command examples remain aligned with
  `docs/design/rule-packs.md` and `docs/design/cli.md`.

## Unreleased

### Dashboard Sharing

- Shared read-only dashboard guidance now uses AI Cloud-aligned hostname
  planning for Core42 deployments, with a documented compatibility window for
  older consumer-specific routes and an explicit note that the hostname update
  does not require GoReleaser or Homebrew changes unless a future package embeds
  a public dashboard URL.

### Release Process

- GitHub Releases for v0.1.6 and later use
  `docs/release-highlights.md` for a short, reader-facing `## Highlights`
  section before the generated changelog detail.
- Release owners update the highlights from the current release notes,
  changelog, and release-run assessment, then get governance wording approval
  and ops workflow approval before tagging.

## v0.1.4

### What Changed

- Public documentation now has a clearer adoption path across README,
  Docusaurus navigation, product boundaries, backlog source authority, agent
  guide, dashboard docs, and release notes.
- Remote push intent is enforced through `fairway record push-intent` and
  closeout/workflow guard findings, keeping provider/thread branches local
  scratch branches unless a promotion intent is recorded.
- Historical review debt is captured as an explicit assessment artifact instead
  of being silently backfilled.
- Dashboard performance blockers are reconciled against later FWRD-161/FWRD-162
  evidence, with FWRD-129 and FWRD-151 preserved as historical/deferred blocked
  tasks.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- Docusaurus portal builds from `website/`.
- `goreleaser check` passes.
- `fairway release verify` remains blocked until the v0.1.4 tag, GitHub
  release assets, Homebrew tap commit, public release state, and `brew fetch`
  evidence exist.

## v0.1.3

### What Changed

- Public documentation now presents the stable adoption path first: quickstart,
  product boundaries, backlog source authority, agent guide, dashboard,
  workstream profiles, and release notes. Historical GPUaaS/dashboard redesign
  material remains archived or assessment-scoped rather than the default user
  path.
- Remote push intent is now an explicit workflow guard. `fairway record
  push-intent` records why a branch is pushed remotely, supports
  `main-validation`, `integration`, `review`, `release`, `backup`, and
  `exception` intents, and requires a reason for `exception`.
- Lane closeout reports remote branches without valid push-intent evidence as
  closeout debt, preserving the model where worker/provider branches are local
  scratch by default and orchestrator or reviewer/merge lanes push integrated
  validation units.
- Historical review debt and dashboard performance blockers are documented as
  explicit assessment artifacts rather than hidden coordinator-plan noise.

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
- The board now includes a URL-backed column chooser with toggleable optional
  columns and up/down ordering controls.
- The board selection bar now opens CSRF-backed bulk action dialogs for claim,
  handoff, non-terminal status changes, and evidence recording. Each bulk
  mutation records per-task audit events.
- Board exports now run server-side for CSV and JSON using the current filters,
  sort order, and visible columns while exporting all filtered rows.
- Wall lanes now expand inline to show queue, current work, pending reviews,
  latest events, and a role-filtered board link.
- Wall and board accessibility now have broader focus-visible coverage, initial
  theme-toggle labeling, and table-header `aria-sort` semantics.

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
