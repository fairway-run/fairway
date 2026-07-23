# Release Notes

Fairway `v0.1.1` is the first public release with signed/notarized macOS
artifacts and a Homebrew cask. The initial `v0.1.0` release artifact was yanked
because its CLI version metadata still reported the development version.

Product boundary reminder for current releases: Fairway provides engineering
control and accountability for agent-driven delivery. It is not an autonomous
workflow engine, CI runner, issue tracker replacement, LLM provider
abstraction, credential store, or provider-cost gate. Release and adapter work
must preserve the rules in [Product boundaries](design/product-boundaries.md).

## v0.1.13

### What Changed

- The default dashboard wall no longer waits for coordinator, reconciliation,
  closeout, and audit projections. Those read-only diagnostics remain available
  through bounded progressive panels without rendering skipped work as a clean
  zero.
- SSE event delivery now reads incrementally from a durable cursor and performs
  bounded review-wait sweeps. Large stores no longer require a full event and
  review hydration every second for each idle dashboard client.
- Reports, task detail, coordinator plans, and audit checks use batch readers
  for transitions, evidence, reviews, handoffs, and notifications. Unknown or
  static routes return bounded responses instead of building the wall.
- The public product story now leads with accountable intent, material
  decisions, evidence, independent judgment, and explicit promotion. The
  quickstart proves one complete local work record without requiring users to
  learn the advanced lane, watcher, shared-team, or release model first.
- Documentation is organized by reader journey and claim status. Implemented,
  validated-practice, experimental, planned, and non-goal statements are
  separated, and an internal AI Cloud case study reports both observed benefit
  and process cost without claiming causation or external adoption.
- Standalone Fairway no longer contains hidden GPUaaS project, role, route, or
  example defaults. Current AI Cloud and GPUaaS references remain only where
  explicitly labeled as case study, assessment, release history, archive, or
  compatibility material.
- The docs backlog audit now emits `consumer_lessons`. The previous
  `gpuaas_lessons` JSON field remains populated for one compatibility window.
- Cold-start packets now separate memory disposition from current task status,
  label checkpoint chronology, deduplicate guidance, and explain cited-source
  freshness relative to the current repository revision.
- Engineering-knowledge lint detects canonical source/frontmatter authority
  conflicts. Projects can opt into warning-blocking CI with
  `knowledge lint --fail-on-warning`; warnings remain advisory by default.

### Known Limits

- Shared-team write APIs, trusted-proxy runtime verification, non-loopback
  Fairway origins, and Postgres runtime storage remain preview or unsupported.
  This release does not promote them to supported production operation.
- The AI Cloud timing comparison is observational, uses small non-equivalent
  cohorts, and is not a general productivity or customer-adoption claim.
- The static portal dependency tree currently reports four moderate
  development-chain advisories. The configured high-severity gate passes; a
  separate dependency-maintenance slice owns upgrades.
- The old `gpuaas_lessons` JSON compatibility key is temporary and should be
  removed only through a separately documented compatibility decision.

### Release Checklist

- `go test ./...` and `go test -tags=integration ./...` pass.
- Focused race tests for store, coordinator, dashboard, audit, and CLI pass.
- `go vet ./...`, `git diff --check`, `go mod tidy`, and backlog YAML parsing
  pass without source drift.
- `fairway config validate`, workflow guard, and active reconciliation pass.
- `goreleaser check` and a release-linked `0.1.13` version/ready/reconcile build
  pass.
- The clean first-value rehearsal and public desktop/mobile portal review pass.
- A separate reviewed task owns the tag, GitHub release, signing/notarization,
  Homebrew update, release verification, and dashboard restart decision.

## v0.1.12

### What Changed

- The common work path now atomically starts task/session/checkpoint state and
  provides compact `work status`, guarded `work verify`, and composed
  `work close` surfaces. These commands use existing Fairway facts and gates;
  they do not create reviews or grant merge, deploy, release, credential,
  public-exposure, or live-operation authority.
- First-class task decisions and track-memory lifecycle records make material
  choices, scope additions, quality assessment, supersession, promotion, and
  replacement-agent continuation durable without treating raw provider
  transcripts as authority.
- Progressive common-path guidance, failure-routing accuracy, reviewer-route
  preflight, lifecycle-aware wait hygiene, managed binary cache commands, and
  consumer capability readiness reduce repeated coordination and environment
  debugging while keeping consequential controls explicit.
- The measured common-path pilot found materially shorter validation-to-close
  and active-to-done time with complete session/checkpoint/evidence coverage,
  but did not produce enough labeled precision data to justify a blocking
  reversible-work intent-to-diff gate. `work verify` therefore reports
  deterministic declared, accepted-decision, and unexplained path classes as
  advisory evidence; existing consequential gates remain blocking.
- `fairway explain code` produces deterministic `fairway.explain-code.v1`
  JSON or Markdown from committed Git metadata and cited Fairway task,
  contract, decision, evidence, and review facts. It reports conflicts and
  missing provenance instead of inventing historical rationale.
- An optional loopback-only `local_ollama` adapter can render a validated
  `fairway.explain-narrative.v1` advisory narrative. Statements must be labeled
  `recorded`, `inferred`, or `unknown`, and recorded/inferred statements require
  packet citations. Generated text is displayed only and is never accepted or
  persisted as provenance.

### Known Limits

- LLM narrative output is not deterministic execution or historical truth.
  The release supports only an explicitly configured loopback `local_ollama`
  narrative endpoint; credentialed remote explanation providers are not
  implemented.
- `explain code` resolves committed source. Symbol resolution currently covers
  Go functions, methods, types, constants, and variables; source bodies are not
  emitted.
- Reversible intent-to-diff findings remain advisory until a later measured
  pilot demonstrates sufficient precision and safety value. Security, live,
  deploy, release, credential, public-exposure, migration, irreversible, and
  other configured consequential boundaries remain blocking.
- Managed binary cache commands install only an explicit local executable.
  They do not download releases, update consumer configs, or restart running
  processes. Consumer readiness reports do not install, migrate, or upgrade.
- This release does not promote shared-team write pilots, trusted-proxy
  identity, non-loopback server exposure, a Postgres runtime switch, dashboard
  mutation, provider-send authority, or autonomous approval.

### Release Checklist

- All included tasks are done with required reviews and recorded validation.
- `go test ./...` and `go test -tags=integration ./...` pass.
- `go vet ./...` and `git diff --check` pass.
- `fairway config validate` and `goreleaser check` pass.
- `fairway workflow check --mode deploy --require-clean --require-pushed`
  passes on the exact reviewed source before tagging.
- A local release-ldflags build reports `0.1.12` and passes ready/reconcile
  smoke checks.
- Separate reviewed tasks own tag/publish/Homebrew/docs verification and the
  two shared dashboard restarts/version readback.

## v0.1.11

### What Changed

- Dashboard lifecycle status now reports binary and version from a versioned
  managed-process identity record and verifies that record against the live
  process command before reporting `running`.
- Legacy integer-only pid files report `unknown` instead of substituting the
  querying CLI's version and binary. Start, stop, and restart fail closed for
  legacy, mismatched-process, or mismatched-listen records until the operator
  verifies and replaces the process.

### Upgrade Note

Restart managed dashboards once with `v0.1.11` to replace legacy pid files
with `fairway.dashboard-lifecycle.v1` JSON records. Verify the process path with
`ps` or the operating system process inspector during that first restart.

## v0.1.10

### What Changed

- Dashboard performance is materially improved for larger Fairway stores. Route
  timing logs expose slow projections, `/board` now has a fast default path,
  review/evidence gate projections use batch reads, repeated GETs use a short
  snapshot cache with singleflight, and heavy diagnostics are lazy-loaded from a
  read-only panel endpoint.
- Shared-team Fairway moved from design into bounded pilot surfaces. The
  release includes a loopback-only read-only server/API skeleton, API-token
  identity and command authorization guards, append-only evidence/checkpoint
  write-pilot endpoints, and guarded status/review write-pilot endpoints with
  idempotency, audit rows, expected-state checks, and reviewer identity
  accountability.
- Postgres and team-store work remains rehearsal-grade, not a runtime switch.
  The release adds a disposable rehearsal packet plus optional disposable
  Postgres apply/import/readback proof using `psql`, DSN environment variables,
  Fairway-prefixed schemas, and read-model equivalence checks.
- Small-team operation is better packaged: Mac mini GitLab lab deployment
  guidance, Fairway doctor diagnostics, lane runtime lifecycle commands, and
  agent-optimized output contracts make local/shared operation easier to start,
  inspect, hand off, and automate.
- Bounded read-only small-team operation is now supported on an
  operator-controlled host. A manual pilot plus a clean-state CI rehearsal
  prove config/doctor, backup/restore, managed lifecycle, status/task/report/
  wait readback, write-disabled posture, timing, and cleanup. The same
  versioned harness is available to operators and CI.

### Known Limits

- Shared-team server write surfaces are still pilot-only and loopback-only.
  They do not authorize public exposure, dashboard-originated mutation,
  provider-send, merge, deploy, release, or live-operation authority.
- Trusted proxy and non-loopback deployments require separate reviewed
  deployment/identity work. Cloudflare, Pomerium, VPN, or mTLS deployment
  posture is not enabled by this release-prep task.
- Postgres rehearsal proves disposable compatibility/import/readback only. It
  does not implement the production runtime adapter, switch Fairway's active
  store, prove full command parity, or claim migration/cutover readiness.
- Dashboard diagnostics can still be expensive on large stores; the default
  board path is fast, while full diagnostics remain intentionally explicit.
- Supported small-team operation remains read-only with a loopback Fairway
  origin. Remote viewers require a separately operated identity-aware proxy,
  SSH tunnel, or VPN boundary; this release does not promote a non-loopback
  origin or Fairway-verified trusted-proxy identity.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- `git diff --check` passes.
- `fairway config validate` passes.
- `fairway workflow check --mode deploy --require-clean --require-pushed`
  passes before tagging.
- `goreleaser check` passes.
- `fairway release verify` confirms the public GitHub release, asset URLs,
  Homebrew cask version, tap commit, and `brew fetch` result.
- Dashboard restart/version readback is handled by a separate tracked task if
  this candidate is published.

## v0.1.9

### What Changed

- Supply-chain provenance is now a Fairway-owned release primitive. Tasks can
  export provenance reports and prompt packets, build content-free SHA-256
  manifests over selected artifacts, and link release attestations without
  storing raw prompts, transcripts, tool bodies, generated content, auth tokens,
  or provider-private data.
- The dashboard can render configured local evidence artifacts through a safe
  read-only viewer. The viewer is limited to task-recorded evidence under
  configured local roots, rejects traversal/symlink/remote/directory paths,
  escapes rendered content, redacts common credential/internal URL classes
  before display truncation, and is defense-in-depth rather than a publishing
  sanitizer.
- Review policy profiles now support reversible-risk defaults, grouped-review
  inheritance, and prototype-first workflows. Reversible work can move quickly
  with evidence while live, deploy, release, irreversible, credential,
  security, production, and public-exposure boundaries still require explicit
  review.
- UX media evidence, delivery/process-overhead metrics, owner rough-edge queues,
  and the small-team autonomy operating model make product feedback, screenshots
  or UAT proof, review usefulness, loop signals, and found-while-using gaps
  visible without turning every small reversible slice into release approval.
- Environment deploy preflight packets and reusable task recipes turn repeated
  handoff/checklist work into bounded packets. They can render readiness,
  evidence, forbidden-action, closeout, and source-fact context, but they do not
  create tasks, approve work, wake providers, merge, deploy, release, mutate
  dashboards, or authorize live operations.
- Multi-project `/reports` now aggregates registered Fairway project DBs into a
  read-only Cross-Project Activity rollup. Rows and exports include project
  labels so duplicate task ids across registered DBs remain distinct, filters
  include project/status/evidence type, and unavailable project DBs degrade into
  visible unavailable rows instead of hiding the rest of the report.

### Known Limits

- Trusted proxy identity verification remains model-only until a later
  high-risk dashboard-security implementation task adds runtime verifier
  middleware/config.
- The safe evidence viewer is a local operator aid, not a public artifact
  publishing sanitizer. Public release/docs content must still be reviewed and
  redacted at the source.
- Recipes and rehearsal packets are packet/rendering surfaces only. They do not
  execute commands, send provider messages, or mutate project state.
- Multi-project reports read available registered DBs but do not migrate,
  repair, or mutate unavailable project stores.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- `git diff --check` passes.
- `fairway config validate` passes.
- `fairway workflow check --mode deploy --require-clean --require-pushed`
  passes before tagging.
- `goreleaser check` passes.
- `fairway release verify` confirms the public GitHub release, asset URLs,
  Homebrew cask version, tap commit, and `brew fetch` result.
- AI Cloud/GPUaaS read-only and local full-access dashboards are restarted with
  the released `v0.1.9` binary and dashboard status/version readback is
  recorded under FW-248.

## v0.1.8

### What Changed

- Multi-project dashboard registration now supports multiple Fairway configs
  under the same repository root when their DB/config identity differs. This
  lets one repo publish separate lanes such as platform and docs work without
  one registration replacing the other, while legacy path-only registry rows
  can upgrade safely.
- `fairway notify send` adds explicitly configured real delivery adapters for
  external notifications. The first send-capable adapters are `log` and
  `webhook`; destinations and bearer tokens are resolved from environment
  variables at send time, notification evidence distinguishes send attempts,
  delivery, and failure, and the read-only dashboard still has no send
  authority.
- Environment deploy preflight packets document a reusable readiness and
  rehearsal model for demo, staging, airgap, and production-like handoffs.
  Operators can record route readback, worker access, smoke, rollback, blocker,
  next-owner, and next-action evidence before handoff without granting Fairway
  deploy, restart, public exposure, or live execution authority.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- `git diff --check` passes.
- `fairway config validate` passes.
- `fairway workflow check --mode deploy --require-clean --require-pushed`
  passes before tagging.
- `goreleaser check` passes.
- `fairway release verify` confirms the public GitHub release, asset URLs,
  Homebrew cask version, tap commit, and `brew fetch` result.
- AI Cloud/GPUaaS read-only and local full-access dashboards are restarted with
  the released `v0.1.8` binary and dashboard status/version readback is
  recorded.

## v0.1.7

### What Changed

- `fairway wait add` and `fairway wait ack` provide durable generic wait
  commands for parked work, repeated handoffs, live-window waits, and
  non-review waits. The implementation projects from existing checkpoint-backed
  wait state and does not add a parallel wait store.
- Advisory provider adapters can now be declared in config and inspected with
  Fairway CLI surfaces. Adapter output remains advisory only: it cannot approve,
  claim, merge, push, deploy, wake providers, mutate environments, or store
  prompts, transcripts, raw tool bodies, generated content, auth tokens, or
  provider-private data.
- External notifier configuration now has a dry-run/logging interface using
  fixed templates. This release does not add Slack, email, Teams, dashboard send
  authority, user subscriptions, or approval/merge/deploy authority.
- Trusted proxy identity verification is documented as a dashboard-security
  model for future Cloudflare Access or identity-aware proxy verification.
  Runtime verifier middleware/config is intentionally not implemented in this
  release and remains split to a later high-risk security task.

### Release Checklist

- `go test ./...` passes.
- `go vet ./...` passes.
- Docusaurus portal builds from `website/`.
- `goreleaser check` passes.
- `fairway config validate` passes.
- `fairway workflow check --mode deploy --require-clean --require-pushed`
  passes before tagging.
- `fairway release verify` confirms the public GitHub release, asset URLs,
  Homebrew cask version, tap commit, and `brew fetch` result.
- GPUaaS read-only and local full-access dashboards are restarted with the
  released `v0.1.7` binary and dashboard status/version readback is recorded.

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

### Coordination And Notification Control

- Review waits, completion handbacks, repeated live-window phases, and
  live-operation control-room handoffs are now treated as Fairway-owned
  coordination state instead of chat-only memory. Coordinator and dashboard
  surfaces can show the current wait, next actor, deadline, authorization
  state, stale age, and suggested command without giving the read-only dashboard
  send, approval, merge, deploy, or execution authority.
- Review-wait wake guidance is status-aware. A resolved review wait no longer
  implies task-level merge readiness when the task itself remains blocked,
  in-progress, or otherwise outside the merge-ready path.
- Bounded active evidence capture is documented and guarded so approved live
  operations can attach gate/runtime evidence while active without being
  mistaken for abandoned work, while stale, sessionless, or unbounded active
  work remains visible to reconciliation.

### Coordination Intelligence

- Coordination-intelligence docs and backlog now cover track memory packets,
  generic parked-track waits, bounded wake delivery, known-failure routing,
  retry packets, advisory recommendation guards, dashboard projections,
  risk-scaled review profiles, delivery/process-overhead reporting, repeated
  work automation candidates, and durable follow-up tasks for provider
  notification lifecycle, routability, retry policy, and backlog coverage.
- Process guidance now favors evidence and tests for small bounded Fairway
  slices, grouped review before release or authority-boundary changes, and
  measurable process rules that improve speed, quality, or safety.
- Memory-only completion and design-backlog cleanup has been reconciled into
  durable Fairway task records and assessment artifacts so release notes do not
  depend on provider chat history.

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
- Release preparation, dashboard lifecycle/version readback, and release
  publish are tracked as separate Fairway tasks. A release is not considered
  published until the tagged binary artifacts, documentation release content,
  dashboard restart/readback evidence, and Homebrew/GitHub verification are
  recorded under the publish task.

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
