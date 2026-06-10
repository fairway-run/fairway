# Fairway v0.1.5 Release Run

Date: 2026-06-10
Owner: ops
Version: v0.1.5
Tag: v0.1.5
Source SHA: cb93a3ba46531df87682c1aa3538e2f69f520bcc

## Scope

Release Fairway v0.1.5 after the governed agentic engineering positioning
refresh and rule-pack implementation/hardening work.

This release includes:

- public positioning around Governed Agentic Engineering;
- local rule-pack sources, validation, evidence-type discovery, and task rule
  matching;
- rule evidence checks in `merge-ready` and `workflow check --mode close`;
- task/report rule surfaces and `fairway packet rules <task-id>`;
- provider notification, adapter trust-boundary, and review-handback hardening
  completed before the release cut.

## Release Checks

| Check | Result | Evidence |
|---|---:|---|
| `go test ./...` | pass | Local prerelease and GitHub Release workflow |
| `go vet ./...` | pass | Local prerelease and GitHub Release workflow |
| `git diff --check` | pass | Local prerelease |
| `go run ./cmd/fairway config validate` | pass | Local prerelease |
| `goreleaser check` | pass | Local prerelease and GitHub Release workflow |
| `cd website && npm run build` | pass | Local prerelease and Docs Portal workflow |
| CI workflow for `cb93a3b` | pass | GitHub Actions run `27313889033` |
| Docs Portal workflow for `cb93a3b` | pass | GitHub Actions run `27313889043` |
| Release workflow for `v0.1.5` | pass | GitHub Actions run `27313998518` |
| GitHub release state | public | `https://github.com/fairway-run/fairway/releases/tag/v0.1.5` |
| Homebrew tap | pass | `fairway-run/homebrew-tap` commit `87e278cc3d89689e86cab5340bdf7daf2379d92f` |
| `brew fetch --cask --force fairway-run/tap/fairway` | pass | Homebrew reported cask `fairway (0.1.5)` |
| Checksum validation | pass | `shasum -a 256 -c fairway_0.1.5_checksums.txt --ignore-missing` |
| macOS codesign verification | pass | `codesign --verify --strict --verbose=2 ./fairway` |
| `fairway release verify` | pass | All release assets returned 200 |

## Published Assets

- `fairway_0.1.5_checksums.txt`
- `fairway_0.1.5_darwin_amd64.tar.gz`
- `fairway_0.1.5_darwin_arm64.tar.gz`
- `fairway_0.1.5_linux_amd64.tar.gz`
- `fairway_0.1.5_linux_arm64.tar.gz`

## Follow-Ups

- `FW-165`: normalize Homebrew version comparison in `fairway release verify`
  so `v0.1.5` and cask version `0.1.5` compare correctly.
- `FW-166`: update release workflow/action runtime posture for GitHub's
  Node.js 20 deprecation warnings.

## Decision

Release v0.1.5 is published and verified.

