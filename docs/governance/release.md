# Release

## Versioning

Semver. Pre-1.0 means breaking changes are allowed in minor versions; document them in the changelog.

- `v0.x.y` — pre-stable. CLI surface and config schema may change in `x`.
- `v1.0.0` — schema and CLI surface frozen. Migrations forward-compatible.

## Tags

- Tag format: `vX.Y.Z`. No `v0.0.1-rc1` shenanigans during week 1.
- Tag from `main` only.
- Annotated tags: `git tag -a v0.1.0 -m "v0.1.0"`.

## Changelog

- `CHANGELOG.md` at repo root, kept in [Keep a Changelog](https://keepachangelog.com) format.
- One section per released version, plus an `Unreleased` section at the top.
- Each PR touches the `Unreleased` section.

## goreleaser

- Config at `.goreleaser.yaml`.
- CI runs `goreleaser check` so release config drift is caught before tags.
- Targets: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`.
- Archives: `fairway_<version>_<os>_<arch>.tar.gz`.
- Checksums: `fairway_<version>_checksums.txt`.
- GitHub Releases are published from `fairway-run/fairway`.
- The macOS artifacts are signed and notarized before Homebrew publishing when
  the Apple release secrets are configured.

## CI release flow

1. Tag pushed → GitHub Actions runs `goreleaser release`.
2. Artifacts attached to the GitHub Release.
3. Homebrew cask updated in `fairway-run/homebrew-tap`.
4. No automatic publishing to other package registries until v1.0.

## Release repositories

- Source/release repo: `fairway-run/fairway`.
- Homebrew tap repo: `fairway-run/homebrew-tap`.
- Tap command: `brew tap fairway-run/tap`.
- Install command: `brew install --cask fairway`.

The tap repository is separate from the source repository. The release workflow
must use a dedicated tap token; the default `GITHUB_TOKEN` is not enough for
cross-repository tap updates.

## Required release secrets

Set these GitHub Actions secrets on `fairway-run/fairway` before cutting a
release tag:

| Secret | Purpose |
|---|---|
| `GORELEASER_KEY` | Enables GoReleaser Pro features used for macOS notarization. |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Fine-scoped token with write access to `fairway-run/homebrew-tap`. |
| `MACOS_SIGN_P12` | Base64-encoded Developer ID Application `.p12` certificate. |
| `MACOS_SIGN_PASSWORD` | Password for the `.p12` certificate. |
| `MACOS_NOTARY_KEY` | Base64-encoded App Store Connect `.p8` API key. |
| `MACOS_NOTARY_KEY_ID` | App Store Connect API key id. |
| `MACOS_NOTARY_ISSUER_ID` | App Store Connect issuer id. |

For local/manual signing experiments, `.apple-app-specific.env` may hold local
Apple credential material. It is ignored by git and must never be committed,
printed in logs, pasted into task evidence, or shared with GPUaaS/Core42 domain
secrets. CI should use the App Store Connect API key secrets above.

## Docs portal deployment

The public Fairway docs portal should use separate Cloudflare credentials for
`fairway.run`. Do not reuse Core42/Core42.dev Cloudflare API tokens.

Expected secret split:

| Secret | Purpose |
|---|---|
| `FAIRWAY_CLOUDFLARE_API_TOKEN` | Cloudflare Pages/DNS token scoped only to the Fairway zone/project. |
| `FAIRWAY_CLOUDFLARE_ACCOUNT_ID` | Cloudflare account for Fairway Pages deploys. |
| `FAIRWAY_CLOUDFLARE_ZONE_ID` | Zone id for `fairway.run`. |
| `FAIRWAY_PAGES_PROJECT` | Cloudflare Pages project name. |

Cloudflare setup belongs to its own tracked task because it includes domain,
token, static-site, and public-content boundary decisions.

## Pre-1.0 distribution

- Local checkout install: `make install` (defaults to `~/.local/bin/fairway`).
- `go install github.com/fairway-run/fairway/cmd/fairway@latest`.
- Direct downloads from GitHub Releases.

## Post-1.0 distribution

- Homebrew cask: `brew tap fairway-run/tap && brew install --cask fairway`.
- Possibly Scoop, Nix, AUR — community-contributed.

## Yanking a release

A release with a critical bug is yanked by:

1. Deleting the tag on the remote (`git push --delete origin vX.Y.Z`).
2. Editing the GitHub Release to "Draft" with a note explaining.
3. Cutting `vX.Y.Z+1` with the fix.

Never re-use a version number.
