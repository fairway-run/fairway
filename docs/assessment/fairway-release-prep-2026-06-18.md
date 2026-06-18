# Fairway Release Preparation Assessment - 2026-06-18

Task: FW-218
Prepared by: Fairway implementation track

## Candidate

- Candidate version: v0.1.6
- Source branch: main
- Source SHA at assessment start: 56eb933
- Publish task: FW-220
- Dashboard lifecycle/version-readback prerequisite: FW-219

This assessment prepares the release packet after coordination cleanup. It does
not tag, publish, restart dashboards, or change public exposure.

## Included Work

- AI Cloud-aligned shared read-only dashboard hostname guidance and compatibility
  plan for the older consumer-specific hostname.
- Review-wait read model, dashboard projection, watcher wake guidance, and
  blocked-task-safe wake text.
- Bounded active evidence capture and repeated live-window/completion-handback
  coordination surfaces.
- Live-operation control-room model for approval-gated execution handoffs.
- Coordination-intelligence docs covering track memory packets, parked waits,
  bounded wakes, retry packets, advisory recommendation guards, dashboard
  projections, risk-scaled review profiles, delivery/process-overhead reporting,
  and automation candidate detection.
- Memory-only completion reconciliation and detailed design backlog backfill,
  including follow-up Fairway tasks FW-221 through FW-224.

## Distribution Posture

- `.goreleaser.yaml` continues to build Fairway CLI archives for Darwin and
  Linux on amd64 and arm64.
- Homebrew cask publishing remains through `fairway-run/homebrew-tap`.
- Shared dashboard hostnames are deployment-owned and are not embedded in the
  binary, archive names, cask metadata, or generated release assets.
- No GoReleaser or Homebrew change is required for the AI Cloud hostname
  guidance unless a future package embeds a public dashboard URL.

## Required Publish Verification

FW-220 should run and record:

- `git status --short --branch` from the reviewed release commit.
- `go test ./...`
- `go vet ./...`
- `goreleaser check`
- `go run ./cmd/fairway config validate`
- `go run ./cmd/fairway workflow check --mode deploy --require-clean --require-pushed`
- `go run ./cmd/fairway reconcile active --dry-run`
- binary build and smoke commands for `version`, `ready`, `reconcile active
  --dry-run`, and dashboard status/version readback.
- GitHub release, checksums/assets, Homebrew cask version, tap commit, and
  `brew fetch --cask --force fairway-run/tap/fairway` evidence after publish.

## Known Limits Before Publish

- FW-219 must still add dashboard lifecycle restart and version-readback
  guidance before shared dashboards are restarted with a release binary.
- FW-220 must perform the actual tag, artifact build/smoke, documentation
  release publication or staging, dashboard restart/readback when authorized,
  and release verification.
- Core42 DNS/Cloudflare changes for `fairway.aicloud.core42.dev` or
  `aicloud-fairway.core42.dev` remain deployment-owned and outside this release
  preparation slice.
- FW-221 through FW-224 are follow-up product backlog items, not blockers for
  this release unless Architecture Control promotes one before publish.
