# Fairway v0.1.10 Release Preparation

Date: 2026-07-06

Task: FW-276

## Candidate

- Version candidate: `v0.1.10`
- Source SHA at initial preparation before FW-276 docs commit: `55cc67e`.
- Shared-team repeat-pilot implementation SHA: `500af2b`.
- Final release source SHA: record after this FW-285 refresh is reviewed,
  committed, and pushed, before creating the release tag.
- Previous released tag: `v0.1.9`
- Release scope: Fairway changes after `v0.1.9` through FW-275, including
  dashboard performance, shared-team model/implementation pilots, disposable
  Postgres rehearsal proof, lab packaging, lane lifecycle, agent-output
  contracts, and pilot closeout.

## Included Work

- FW-250 through FW-254: dashboard route timing, board fast path, batched
  review/evidence projections, short snapshot cache with singleflight, and
  lazy-loaded heavy diagnostics.
- FW-255 through FW-259: shared-team store/API/concurrency/deployment
  operating models, including the decision that a server-backed store is an
  operating model and not merely a Postgres backend.
- FW-269 through FW-272: loopback-only read-only server/API skeleton,
  identity/authz guard, append-only evidence/checkpoint write pilot, and
  guarded status/review write pilot.
- FW-273 and FW-277: disposable Postgres rehearsal packet and disposable
  apply/import/readback proof with DSN environment handling and
  Fairway-prefixed schema guardrails.
- FW-274: local Mac mini GitLab lab deployment runbook for small-team
  read-only Fairway operation.
- FW-275: small-team shared pilot assessment and rough-edge closeout, with a
  repeat-pilot recommendation before promotion.
- FW-278 through FW-280: Fairway doctor diagnostics, lane runtime lifecycle
  commands, and agent-optimized output contracts.
- FW-281 and FW-282: typed delivery-resource read models and CI write-mode
  fixture isolation.
- FW-283 and FW-284: managed small-team server lifecycle plus a versioned
  clean-state operator/CI rehearsal that promotes bounded read-only operation
  while preserving write, network, identity, and storage limits.

## Validation Packet

Validation already recorded on implementation tasks:

- Focused tests for dashboard performance read models, snapshot cache,
  diagnostics lazy loading, server identity/authz, write-pilot idempotency,
  guarded status/review writes, Postgres rehearsal helpers, lane lifecycle, and
  agent-output contracts.
- `go test ./...`: pass on the implementation slices.
- `go vet ./...`: pass on the implementation slices.
- `git diff --check`: pass on the implementation slices.
- `fairway config validate`: pass on the implementation slices.
- `fairway workflow check`: pass on the implementation slices, with expected
  dirty/unpushed warnings before reviewed commits.
- `fairway reconcile active --dry-run`: clean before each closeout, except for
  expected status-decision prompts while tasks were still active.

FW-276 release-prep validation:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `git diff --check`: pass.
- `go run ./cmd/fairway config validate`: pass.
- `goreleaser check`: pass.
- `go run ./cmd/fairway workflow check`: expected to warn before FW-276 commit
  because the release-prep docs are dirty and the branch has unpushed commits.

Required before tag/publish:

- `go test ./...`
- `go vet ./...`
- `git diff --check`
- `go run ./cmd/fairway config validate`
- `go run ./cmd/fairway workflow check --mode deploy --require-clean --require-pushed`
- `goreleaser check`
- local build and smoke: `fairway version`, `fairway ready`,
  `fairway reconcile active --dry-run`
- `fairway release verify` after publish with GitHub release, asset, Homebrew,
  tap, and `brew fetch` evidence.

## Deployment And Dashboard Plan

- FW-276 authorizes release preparation only.
- A separate publish task must authorize tag, push, GitHub release, Homebrew
  update, and release verification.
- A separate dashboard restart/readback task must restart shared dashboards
  with the released binary and record status/version/binary readback.
- No public exposure, trusted-proxy deployment, server write deployment, or
  runtime store switch is authorized by this release-prep packet.

## Known Limits

- Shared-team write APIs remain pilot-only and loopback-only.
- Trusted proxy identity and non-loopback exposure require a separate reviewed
  deployment/security task.
- Postgres proof is disposable compatibility/import/readback only; no runtime
  store adapter or migration cutover is implemented.
- Dashboard diagnostics remain intentionally explicit and can still be heavy on
  large stores.
- Bounded read-only small-team operation is supported only with a loopback
  Fairway origin on an operator-controlled host. Shared writes, non-loopback
  origins, trusted-proxy verification, and Postgres runtime storage remain
  preview or unsupported.

## Publish Blockers

- FW-285 refreshed release-prep review must approve governance, ops, and
  security.
- The FW-285 refresh commit must land before any publish task.
- Main must be pushed to configured release remotes before tagging.
- Release secrets and signing/notarization posture must be available to the
  release workflow.
- No dashboard restart, public release declaration, Homebrew update, or tag is
  authorized by FW-276 alone.
