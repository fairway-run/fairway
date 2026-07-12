# Fairway v0.1.13 independent release-readiness review

Date: 2026-07-11  
Task: FW-333  
Reviewed source: `196cf2ef86e02ae1a03a4cf751790098d625edcd`  
Baseline tag: `v0.1.12`  
Verdict: **hold for bounded release preparation**

## Scope

This review covers the complete `v0.1.12..196cf2e` delta: dashboard and SSE
performance work, batched store/coordinator/audit projections, documentation
information architecture, first-value onboarding, standalone-product cleanup,
ecosystem boundaries, the AI Cloud case study, and the public portal rebuild.

It is independent of the implementation-task closeouts. Passing child reviews
are evidence inputs; they are not substituted for review of the integrated
release candidate.

## Findings

### Release blocker: no v0.1.13 release packet

`CHANGELOG.md` still says `No unreleased changes yet`, `docs/release-notes.md`
ends at `v0.1.12`, and `docs/release-highlights.md` describes the prior release
train. The post-tag runtime, performance, compatibility, narrative, and portal
changes therefore have no reviewed candidate scope, upgrade statement, known
limits, or release checklist.

Create a `v0.1.13` release-preparation slice tied to the exact final source SHA.
It must distinguish runtime changes from documentation changes and retain the
preview/unsupported boundaries for shared writes, trusted-proxy verification,
non-loopback origin exposure, and Postgres runtime storage.

### Release blocker: stale package and release positioning

The introductory product-boundary paragraph in `docs/release-notes.md` and the
Homebrew cask description in `.goreleaser.yaml` still describe Fairway as a
`coordination control plane`. Publishing them would contradict the canonical
product and portal narrative.

Replace release-facing metadata with the reviewed category: engineering control
and accountability for agent-driven delivery. Verify the GitHub repository and
release descriptions use the same bounded language without making compliance,
market-adoption, or autonomous-authority claims.

## Validation

The following passed against the reviewed source plus this assessment task's
uncommitted backlog registration:

- `go test ./...`
- `go test -tags=integration ./...`
- `go vet ./...`
- race tests for `internal/store`, `internal/coordinator`,
  `internal/dashboard`, `internal/audit`, and `cmd/fairway`
- `git diff --check`
- backlog YAML parse
- `goreleaser check`
- `fairway config validate`
- `fairway workflow check` (only the expected uncommitted review metadata)
- `fairway reconcile active --dry-run`
- `go mod tidy` with no module-file drift
- release-linked `0.1.13` binary build, version, ready, and reconcile smoke
- disposable clean-repository first-value path through init, bootstrap commit,
  doctor, work start, decision, evidence, work close, and task-detail readback
- Docusaurus production build
- `npm audit --omit=dev --audit-level=high`
- public HTTP 200 probes for homepage, quickstart, product, ecosystem,
  integrations, AI Cloud case study, and product boundaries
- live desktop `1440x900` and mobile `390x844` browser review with no horizontal
  overflow, failed images, blank/loading state, or clipped primary actions
- public security-header readback for CSP, permissions policy, referrer policy,
  MIME sniffing protection, and frame denial

## Compatibility Review

- The standalone core no longer selects adoption samples from a magic GPUaaS
  project name or role.
- `consumer_lessons` replaces the product-specific audit field while the old
  `gpuaas_lessons` JSON key remains populated for one documented compatibility
  window.
- Remaining AI Cloud references in current public material are explicitly an
  internal-use case study, not customer evidence or a general product default.
- Remaining GPUaaS examples are labeled compatibility fixtures or historical
  evidence. They do not define the default configuration.
- No database migration was added after `v0.1.12`; rollback to the prior binary
  remains structurally available after normal backup/readback.

## Residual Risk

- The portal dependency audit reports four moderate development-chain
  advisories in `http-proxy-middleware`, `joi`, nested `js-yaml`, and
  `webpack-dev-server`. The high-severity production gate passes and the portal
  is emitted as static content. Track dependency refresh separately; do not
  silently promote these advisories as fixed.
- The AI Cloud case-study timing comparison is observational and explicitly
  does not establish causation or external adoption.
- Public portal publication currently precedes the binary release. Until
  `v0.1.13` is published, release notes must make the distinction between
  current documentation and the latest tagged binary clear.
- An external narrative review may improve wording but is advisory unless it
  identifies a factual, privacy, security, or authority-boundary defect.

## Version And Rollback Recommendation

Use `v0.1.13`. The delta is backward-compatible feature, performance, and
documentation work within the existing `0.1.x` line. Do not move `v0.1.12`.

Before tagging:

1. prepare and independently review the v0.1.13 changelog, notes, highlights,
   package metadata, exact SHA, known limits, and verification packet;
2. rerun clean/pushed deploy-mode workflow checks on that reviewed SHA;
3. build with release linker flags and verify `version=0.1.13`;
4. tag and publish under a separate authorized task;
5. verify release assets, signing/notarization, Homebrew cask, `brew fetch`,
   public docs/source readback, and rollback reference to `v0.1.12`;
6. restart shared dashboards only under a separate reviewed lifecycle task if
   they should consume the new performance fixes.

## Verdict

**Hold the tag, not the implementation.** The integrated source is technically
release-ready based on current automated, race, rehearsal, and browser proof.
Release-facing scope and metadata are not ready. Complete the two bounded
release-preparation blockers above, then proceed to an independently authorized
`v0.1.13` publish task.
