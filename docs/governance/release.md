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
- Checksums: `fairway_<version>_checksums.txt`, signed when a key is available.

## CI release flow

1. Tag pushed → GitHub Actions runs `goreleaser release`.
2. Artifacts attached to the GitHub Release.
3. Homebrew tap updated (when configured; not before v0.1).
4. No automatic publishing to package registries until v1.0.

## Pre-1.0 distribution

- `go install github.com/subashram/fairway/cmd/fairway@latest`.
- Direct downloads from GitHub Releases.

## Post-1.0 distribution

- Homebrew tap: `brew install subashram/tap/fairway`.
- Possibly Scoop, Nix, AUR — community-contributed.

## Yanking a release

A release with a critical bug is yanked by:

1. Deleting the tag on the remote (`git push --delete origin vX.Y.Z`).
2. Editing the GitHub Release to "Draft" with a note explaining.
3. Cutting `vX.Y.Z+1` with the fix.

Never re-use a version number.
