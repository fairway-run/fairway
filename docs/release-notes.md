# Release Notes

Fairway `v0.1.1` is the first public release with signed/notarized macOS
artifacts and a Homebrew cask. The initial `v0.1.0` release artifact was yanked
because its CLI version metadata still reported the development version.

Product boundary reminder for current releases: Fairway provides engineering
control and accountability for agent-driven delivery. It is not an autonomous
workflow engine, CI runner, issue tracker replacement, LLM provider
abstraction, credential store, or provider-cost gate. Release and adapter work
must preserve the rules in [Product boundaries](design/product-boundaries.md).

## v0.3.0

`v0.3.0` makes Fairway a durable engineering control and evidence plane across
replaceable agent harnesses. An execution system can now contribute versioned
harness runs, task-scoped observations, and evaluator results without becoming
the authority for Fairway task state, review, promotion, or release.

The new harness record contract uses source-qualified identities, canonical
payload replay checks, and atomic ingestion. The CLI can print the supported
contract, ingest bounded JSON batches, inspect runs and individual records, and
produce a task-scoped report. Task detail exposes the same cited record summary.
Records retain safe hypotheses, expected observations, observed measurements,
evaluator outcomes, artifact references, and digests; they reject raw prompts,
reasoning, transcripts, tool bodies, generated-content dumps, credentials, and
secret-like material.

Outcome-efficiency ratios appear only when a verified outcome and complete
denominator exist. Exact comparable cost is not currently retained, so Fairway
reports cost as unavailable instead of estimating it. Repeated actions,
stagnation, failed expectations, and evaluator regressions can produce cited
trajectory findings, but `reframe_hypothesis`, `change_execution_profile`, and
`request_input` remain advisory questions. They do not redirect a provider,
cancel execution, approve work, or change task state.

A bounded GPUaaS pilot validates contract ingestion, idempotent replay,
readback, evaluator linkage, outcome-efficiency reporting, and trajectory
signals against realistic engineering scenarios. The pilot demonstrates a
consumer path and records false-positive limits; it does not establish causal
control effectiveness, model superiority, or autonomous supervision.

### Upgrade

1. Back up the project Fairway database and managed contract files.
2. Install `v0.3.0`.
3. Run `fairway config validate` and `fairway reconcile active --dry-run`.
4. Inspect the contract with `fairway contract harness-record --format json`.
5. Ingest new prospective records only from a reviewed harness adapter, then
   inspect them with `fairway harness runs --task <task-id>` and
   `fairway harness report --task <task-id>`.
6. Do not infer or backfill historical hypotheses, observations, evaluators,
   denominators, or outcomes when no authoritative source exists.

### Known Limits

- Fairway does not execute, supervise, redirect, stop, or retry harness runs.
- Trajectory findings are cited advisory signals and can have false positives;
  they do not replace engineering judgment or independent review.
- Exact comparable provider cost is unavailable, and unavailable denominators
  remain unavailable rather than becoming zero.
- The GPUaaS assessment is a bounded consumer pilot, not a benchmark or proof of
  causal delivery improvement.
- Seaway remains an optional design-level integration boundary; this release
  does not include a Seaway adapter.
- This release does not claim sovereign deployment readiness, regulatory
  certification, independent security assessment, or a complete AI Quality
  System.

## v0.2.7

`v0.2.7` makes Fairway's working model and current product boundary visible in
the product itself. Teams collaborate while intent, constraints, ownership, or
proof remain uncertain. They delegate one bounded slice when its result can be
checked independently, challenge the claim with evidence and review, and cross
merge, release, deploy, or live boundaries only through the authority that owns
them. These are working modes, not new Fairway task states or another SDLC
phase.

The dashboard now opens with a read-only Product Overview that connects the
accountable engineering record to current project evidence, execution-surface
selection, system boundaries, and the adoption path. The Quality Workspace
compares cited lifecycle stages across tasks and links into the underlying
Quality Record without producing a score or approval. A GPUaaS operator
walkthrough exercised Overview, Wall, Board, Diagnostics, Quality, Reports,
filtered exports, and task detail against a real 1,920-task store and found no
blocking gap.

Fairway remains the harness-neutral cross-run coordination record. Coding
agents, subagents, provider threads, and utilities remain replaceable execution
attachments. Seaway is defined as an optional, independently usable per-run
admission, execution, policy, event, and result layer. The design specifies
correlation, retry, approval-response idempotency, replay scope, degradation,
and authority separation, but intentionally schedules no adapter until Seaway
publishes a versioned contract and fixtures.

### Upgrade

1. Back up the project Fairway database and managed contract files.
2. Install `v0.2.7`.
3. Run `fairway config validate`, `fairway agent-contract status`, and
   `fairway reconcile active --dry-run`.
4. Open `fairway dashboard` and inspect Overview and Quality against the
   project's own records.
5. Continue using existing task, session, evidence, review, and promotion
   commands. No new collaboration/delegation state or Seaway dependency is
   introduced.

### Known Limits

- Fairway does not infer that a task is safe to delegate; specifiability,
  verification cost, risk, and authority still require engineering judgment.
- Seaway integration is a reviewed contract boundary, not a released adapter or
  dependency.
- Missing, unavailable, conflicting, and externally owned Quality Record facts
  remain visible and are not converted into scores or generated conclusions.
- This release does not claim sovereign deployment readiness, regulatory or
  export classification, certification, or independent security assessment.

## v0.2.6

`v0.2.6` makes Fairway's quality record inspectable and its governance
measurable without turning either into autonomous authority.

The new `fairway quality-record <task-id>` command and task dashboard project
nine cited lifecycle stages: intent, decisions, production context, evidence,
verification, judgment, promotion, outcomes, and lessons. Each stage reports
`present`, `missing`, `unavailable`, `conflicting`, or `externally_owned`.
Fairway does not fill gaps with generated prose, produce a quality score, or
acquire review, merge, deploy, release, or risk-acceptance authority.

Normal work lifecycle commands now retain append-only task-to-commit
associations. Structured outcome links distinguish incidents, rollbacks,
reopens, corrective work, and superseding tasks from same-file touch proxies.
Attributable friction records distinguish measured, open, unavailable, and
missing control cost.

Advisory control-effectiveness reports combine those records with Git facts,
risk and diff-size cohorts, coverage gates, and read-only drill-down. The first
GPUaaS control pilot correctly suppressed every ranking under low commit
coverage. A second 288-task pilot validated deterministic Quality Record
reconstruction and exposed one real blocked-then-passed verification conflict,
while retaining the consumer's absent historical outcome/friction facts as
unavailable. These pilots validate data-quality diagnosis, not causal control
effectiveness or a complete AI Quality System.

Public docs and the website now present the implemented capability as
engineering quality records, continuity, and control. The canonical category
claim remains engineering control and accountability for agent-driven
delivery.

### Upgrade

1. Back up the project Fairway database and managed contract files.
2. Install `v0.2.6`.
3. Run `fairway config validate`, `fairway agent-contract status`, and
   `fairway reconcile active --dry-run`.
4. Inspect a completed task with `fairway quality-record <task-id>`.
5. Use normal `fairway work start` and `fairway work close` for new work so
   commit association is captured prospectively. Do not backfill historical
   outcomes or friction unless a durable authoritative source exists.

### Known Limits

- Historical GPUaaS commit associations, structured outcomes, and attributable
  friction are intentionally not inferred or backfilled.
- Verifier qualification, reviewer authentication/competence, comparison-class
  process capability, and a validated closed improvement loop remain incomplete
  or externally owned.
- Observational control reports do not establish causality and cannot relax a
  mandatory safety invariant.
- Fairway does not claim regulatory certification or a complete AI Quality
  System.

## v0.2.5

`v0.2.5` changes Fairway releases from tag-first builds to qualified candidate
promotion. A manual production rehearsal builds, tests, signs, notarizes,
smokes, and packages an exact pushed `main` commit before the final remote tag
exists.

The public product entry points now present more of Fairway's current product
model: execution control, engineering continuity, operating knowledge, and
assurance. Implemented memory, knowledge, rule-pack, and assurance capabilities
are distinguished from planned execution profiles instead of remaining
underrepresented in the earlier public positioning.

The resulting immutable packet binds version, source SHA, workflow identity,
release policy, seven release assets, sizes, and SHA-256 digests. The final
annotated tag names one successful rehearsal run. Its workflow downloads and
verifies that exact packet and signed assurance before creating a draft; it
does not rebuild and receives no signing, notarization, or Homebrew
credentials.

This prevents a final tag from being the first realistic release execution and
keeps failed candidate attempts free of tag, release, tap, or deployment side
effects.

The rehearsal also exposed and fixed an evidence-ordering race: when adjacent
records shared the same timestamp, pass/fail supersession could be read in an
undefined order. Batched evidence reads now use insertion identity as the
deterministic tie-breaker.

## v0.2.4

`v0.2.4` is the publishable follow-up to the immutable unpublished `v0.2.3`
candidate. The `v0.2.3` workflow passed, created a gated draft, and produced
verified signed archives and an assurance bundle. Pre-publication asset
inspection then caught that the assurance checksum file named the runner's
absolute temporary path instead of the downloadable asset basename.

Checksum generation now uses the release helper to write a bounded SHA-256
record containing only the assurance asset basename. Regression coverage proves
that nested build paths are not retained and existing checksum outputs are not
overwritten. Product behavior and the cumulative `v0.2.0` feature scope are
unchanged. Install `v0.2.4`.

## v0.2.3 (unpublished candidate)

`v0.2.3` is an immutable unpublished candidate. It fixed pinned-builder
provenance capture and created a gated draft, but pre-publication inspection
rejected its non-portable assurance checksum.

The tag workflow now pins GoReleaser `v2.17.0`, locates exactly one executable
in the action tool cache, validates its custody, and passes that exact tool into
build-provenance capture. Product behavior and the cumulative `v0.2.0` feature
scope are unchanged; the correction is carried forward into `v0.2.4`.

## v0.2.2 (unpublished candidate)

`v0.2.2` is an immutable unpublished candidate. It fixed the release-test
synchronization defects exposed by `v0.2.0` and `v0.2.1`, but its release
workflow failed closed during build-provenance capture before draft creation.

The `v0.2.0` workflow built signed and notarized archives but failed closed
before draft creation because the release-assurance trust configuration was not
configured. After that trust root was provisioned, the retry exposed a
scheduler-sensitive dashboard SSE test on the macOS release runner. `v0.2.1`
fixed that assertion but exposed another fixed-sleep race in the same SSE
stream helper before release artifacts were built.

This release uses bounded behavioral synchronization for SSE stream startup,
incremental event hydration, post-hydration completion, idle polling, and
review-wait sweeps. Product behavior and the `v0.2.0` feature scope are
otherwise unchanged. The `v0.2.0` and `v0.2.1` tags remain immutable and
unpublished. Install `v0.2.4` and follow the cumulative upgrade procedure
below.

## v0.2.1 (unpublished candidate)

`v0.2.1` replaced one fixed idle-poll sleep with a bounded condition wait, but
the macOS release runner exposed a second scheduler-sensitive SSE helper before
artifact publication.

## v0.2.0 (unpublished candidate)

### What Changed

- Track memory is now a first-class Fairway record with lifecycle, disposition,
  cold-start, provider-replacement, and closeout behavior. Completed or
  superseded work is archived instead of leaking into current next-action
  guidance.
- Engineering knowledge is project-owned and source-grounded. Deterministic
  lint, ingest, snapshot binding, freshness checks, query packets, and
  promotion surfaces keep derived synthesis separate from canonical source
  authority.
- GPUaaS adoption expanded the model across repair/recovery, failure and
  upgrade domains, and logical workload identity. The bounded pilot showed
  fast cold starts and correct source ranking without requiring embeddings,
  hosted retrieval, or provider-private memory.
- Generated `.fairway/AGENTS.md` files now carry an independent schema,
  revision, generating-binary identity, and managed-content hash.
  `agent-contract status|plan|apply` supports explicit upgrades, preserves
  the complete legacy contract in `AGENTS.local.md` for manual migration,
  detects local edits, and prevents older binaries from silently downgrading
  newer contracts.
- Fairway core implements rule-pack loading, matching, evidence checks, and
  packet rendering while projects and domain packs own the rule definitions.
  Migration execution profiles, rule-pack completeness bakeoffs, and verifier
  qualification now have a reviewed design contract; they are not yet an
  implemented migration engine.
- Assurance profiles, evidence-gap reports, signed and offline bundles,
  restricted advisory packaging, customer-key rehearsal, and sovereign
  deployment baselines provide bounded evidence-acceleration capabilities.
  They do not grant certification, legal, export, procurement, market-access,
  or independent-assessment authority.
- Fairway's pre-1.0 policy now explicitly favors correcting the durable product
  model over retaining accidental behavior. Durable project data and policy
  still require safe forward migration and downgrade protection.
- The documentation deployment toolchain is pinned to Wrangler `4.113.0`.
  Current transitive security fixes require this upgrade from `4.98.0`; the
  release gate includes a real Cloudflare Pages deployment and public
  documentation readback before tagging.

### Upgrade

1. Back up the project Fairway database and project control files.
2. Install `v0.2.4`, the published release carrying this feature scope.
3. Run `fairway config validate` and `fairway agent-contract status`.
4. Review drift with `fairway agent-contract plan`.
5. For an unversioned generated contract, run
   `fairway agent-contract apply --adopt-legacy`. This losslessly copies the
   entire old contract to `.fairway/AGENTS.local.md`; remove obsolete
   Fairway-generated guidance and retain only project-owned instructions before
   committing it.
6. Run `fairway preflight`, `fairway knowledge lint`, and
   `fairway reconcile active --dry-run`.

Binary updates do not change project process when the embedded agent-contract
revision is unchanged.

Rollback requires the matching pre-upgrade backup:

1. stop active Fairway sessions and dashboard processes for the project;
2. restore the pre-upgrade `.fairway/state.db`;
3. restore the pre-upgrade `.fairway/AGENTS.md` and other changed control files;
4. reinstall `v0.1.13`;
5. run `fairway config validate`, task readback, and
   `fairway reconcile active --dry-run`.

Do not point an older binary at a database or managed contract already migrated
by a newer release and treat successful startup as rollback proof.

### Known Limits

- Migration execution profiles and verifier qualification are design-only in
  this release.
- Deterministic lexical retrieval remains the default. Embeddings and hosted
  retrieval are intentionally deferred until measured project evidence shows a
  concrete limitation.
- Shared-team write APIs, trusted-proxy production verification, non-loopback
  service exposure, Postgres runtime storage, autonomous approval, and
  provider-send authority remain preview or unsupported.
- Sovereign applicability, ECCN/EAR/ENC conclusions, jurisdictional claims,
  certification, and independent security assessment remain blocked on
  qualified external specialists. `v0.2.4` makes no such claim.

### Release Checklist

- Full unit, integration, race, vet, formatting, module-drift, and backlog
  validation pass on the exact candidate.
- Knowledge/memory tests, agent-contract lifecycle tests, release assurance,
  offline distribution, restricted advisory, and sovereign rehearsal gates
  pass.
- Docusaurus production build, full dependency audit, and production-dependency
  threshold pass.
- The pushed release-preparation commit completes a real Cloudflare Pages
  deployment with Wrangler `4.113.0`, followed by public route readback.
- The retained npm dependency tree reports no known vulnerabilities at release
  preparation time; the result is a dated scan, not a continuing guarantee.
- A release-linked `0.2.4` binary reports the expected version and passes
  config, contract-status, ready, and reconcile smoke.
- A disposable project initialized by `v0.1.13` survives `v0.2.4` upgrade and
  agent-contract adoption; restoring the pre-upgrade database and contract
  returns the project to clean `v0.1.13` task and reconciliation readback.
- The tag-triggered release workflow signs and notarizes Darwin binaries,
  builds four archives, verifies the signed assurance bundle, and creates a
  draft release.
- A separate publish step verifies public assets, checksums, Homebrew,
  provenance, and rollback to `v0.1.13`.

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
