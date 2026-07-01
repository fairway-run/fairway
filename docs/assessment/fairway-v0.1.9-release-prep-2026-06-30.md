# Fairway v0.1.9 Release Preparation

Date: 2026-06-30

Task: FW-246

## Candidate

- Version candidate: `v0.1.9`
- Source SHA at preparation before FW-246 docs commit: `e722fda`
- Final release source SHA: refresh after FW-246 is reviewed, committed, and
  pushed in FW-247.
- Previous released tag: `v0.1.8`
- Release scope: Fairway changes after `v0.1.8` through FW-235/FW-245 and
  supporting provenance/release-readiness work.

## Included Work

- FW-231 through FW-234: supply-chain provenance model, provenance reports and
  prompt packets, release attestation links, and evidence retention posture.
- FW-236: safe evidence artifact viewer and redaction gate.
- FW-237: executable environment rehearsal packet templates.
- FW-238: reusable task recipe extraction/rendering/listing and dashboard recipe
  library.
- FW-239 through FW-245: reversible-risk profiles, grouped review mode,
  prototype-first profile, UX media evidence model, delivery/process-overhead
  metrics, owner rough-edge queue, and small-team autonomy operating model.
- FW-235: cross-project activity rollup reports for registered Fairway project
  DBs.

## Validation Packet

Validation already recorded on implementation tasks:

- Focused tests for review-policy profiles, grouped-review boundary handling,
  prototype-first behavior, UX media evidence, delivery report defect-source
  semantics, rough-edge expiry validation, safe artifact redaction, environment
  rehearsal packets, recipe privacy/schema checks, and cross-project reports.
- `go test ./...`: pass on the implementation slices.
- `go vet ./...`: pass on the implementation slices.
- `git diff --check`: pass on the implementation slices.
- `fairway config validate`: pass on the implementation slices.
- `fairway workflow check`: pass on the implementation slices, with expected
  dirty/unpushed warnings before each reviewed commit.
- `fairway reconcile active --dry-run`: clean before this release-prep task.

FW-246 release-prep validation:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `git diff --check`: pass.
- `go run ./cmd/fairway config validate`: pass.
- `go run ./cmd/fairway workflow check`: pass with expected dirty/unpushed
  warnings before this release-prep commit.
- `go run ./cmd/fairway reconcile active --dry-run`: no active reconciliation
  findings.
- `goreleaser check`: pass.
- `go run ./cmd/fairway workflow check --mode deploy --require-clean
  --require-pushed`: blocked as expected before FW-246 commit/push, reporting
  uncommitted release-prep docs and 11 unpushed commits.

Required before FW-247 tag/publish:

- `go test ./...`
- `go vet ./...`
- `git diff --check`
- `go run ./cmd/fairway config validate`
- `go run ./cmd/fairway workflow check --mode deploy --require-clean --require-pushed`
- `goreleaser check`
- build and smoke `fairway version`, `fairway ready`, `fairway reconcile active --dry-run`
- `fairway release verify` after publish with GitHub release, asset, Homebrew,
  tap, and `brew fetch` evidence.

## Dashboard And Docs Plan

- Publish documentation from the reviewed release notes, highlights, changelog,
  design docs, and assessment content.
- Restart shared read-only and local full-access AI Cloud/GPUaaS Fairway
  dashboards only in FW-248 after FW-247 publishes the released binary.
- Record dashboard status/version readback and public Cloudflare Access boundary
  probe evidence under FW-248.
- Do not restart dashboards from this release-prep task.

## Known Limits

- Trusted proxy identity verification is still a model/design surface only; no
  runtime Cloudflare Access JWT/header verifier is implemented in this release.
- The safe evidence viewer is for local operator inspection. It is not a public
  publishing sanitizer and does not make unreviewed artifacts safe to post.
- Recipes and rehearsal packets render bounded context only. They do not
  execute commands, send providers, approve reviews, mutate tasks, merge,
  deploy, release, or authorize live operations.
- Cross-project reports read registered DBs and surface unavailable stores; they
  do not migrate, repair, or mutate attached projects.

## Publish Blockers

- FW-246 release-prep review must approve governance, ops, and security.
- FW-246 release-prep commit must land before FW-247.
- Main must be pushed to the configured release remotes before tagging.
- Release secrets and signing/notarization posture must be available to the
  release workflow.
- No dashboard restart, public release declaration, Homebrew update, or tag is
  authorized by FW-246 alone.
