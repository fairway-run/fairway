# Release

## Versioning

Semver. Pre-1.0 means breaking changes are allowed in minor versions; document them in the changelog.

- `v0.x.y` — pre-stable. CLI surface and config schema may change in `x`.
- `v1.0.0` — schema and CLI surface frozen. Migrations forward-compatible.

## Tags

- Tag format: `vX.Y.Z`. No `v0.0.1-rc1` shenanigans during week 1.
- Tag from `main` only.
- Annotated tags: `git tag -a v0.1.1 -m "v0.1.1"`.

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
- The macOS CLI binaries are signed with native `codesign` and submitted with
  native `notarytool` from the macOS release runner before Homebrew publishing.
  GoReleaser OSS still drives the release, archives, checksums, GitHub release,
  and cask update. GoReleaser Pro is not required unless Fairway later ships
  macOS app bundles, DMGs, or PKGs.

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

Before the first release, initialize the tap repository with a `main` branch so
GoReleaser can push the first generated cask:

```bash
tmpdir=$(mktemp -d /tmp/fairway-homebrew-tap.XXXXXX)
cd "$tmpdir"
git init
git branch -M main
mkdir -p Casks
touch Casks/.gitkeep
cat > README.md <<'EOF'
# Homebrew Tap for Fairway

    brew tap fairway-run/tap
    brew install --cask fairway
EOF
git add README.md Casks/.gitkeep
git commit -m "Initialize Homebrew tap"
git remote add origin git@github.com:fairway-run/homebrew-tap.git
git push -u origin main
```

## Required release secrets

Set these GitHub Actions secrets on `fairway-run/fairway` before cutting a
release tag:

| Secret | Purpose |
|---|---|
| `HOMEBREW_TAP_GITHUB_TOKEN` | Fine-scoped token with write access to `fairway-run/homebrew-tap`. |
| `MACOS_SIGN_P12` | Base64-encoded Developer ID Application `.p12` certificate. |
| `MACOS_SIGN_PASSWORD` | Password for the `.p12` certificate. |
| `MACOS_CODESIGN_IDENTITY` | Developer ID Application identity fingerprint or exact name used by native `codesign`. |
| `MACOS_NOTARY_KEY` | Base64-encoded App Store Connect `.p8` API key. |
| `MACOS_NOTARY_KEY_ID` | App Store Connect API key id. |
| `MACOS_NOTARY_ISSUER_ID` | App Store Connect issuer id. |

For local/manual signing experiments, `.apple-app-specific.env` may hold local
Apple credential material. It is ignored by git and must never be committed,
printed in logs, pasted into task evidence, or shared with GPUaaS/Core42 domain
secrets. CI should use the App Store Connect API key secrets above.

## macOS signing and notarization baseline

Fairway uses Developer ID signing and native Apple notarization for macOS CLI
artifacts. The current release baseline is:

- Certificate type: `Developer ID Application`.
- Certificate chain: `Developer ID Application -> Developer ID Certification
  Authority -> Apple Root CA`.
- Team identifier: Apple Developer team id for the Fairway release account.
- Hardened runtime: enabled by the release hook with `codesign --options
  runtime`.
- Notarization auth: App Store Connect API key, not Apple ID password auth.

GoReleaser Pro is only needed if Fairway later ships native app bundles,
macOS DMGs, or macOS PKGs. The first release distributes a CLI binary, so the
OSS release path plus native macOS build hooks is sufficient.

For the first public release, use the working Developer ID certificate from
the previous Sub-CA. The G2 certificate is available, but local validation showed
that `codesign` will not build the G2 chain until the Developer ID G2
intermediate is trusted from the System chain. Previous Sub-CA certificates
created after February 1, 2022 expire on February 1, 2027; switch to G2 well
before that date and require a passing local/CI `codesign --verify --strict`
check before publishing.

Local release material lives under ignored paths only:

| Local path | Purpose |
|---|---|
| `.release-certs/developerID_application_identities.p12` | Local Developer ID signing bundle. |
| `.release-certs/developerID_application_identities.p12.base64` | Value source for `MACOS_SIGN_P12`. |
| `.release-certs/macos-sign-p12-password.local` | Value source for `MACOS_SIGN_PASSWORD`. |
| `.release-certs/AuthKey_<KEY_ID>.p8` | App Store Connect notary API key. |
| `.release-certs/AuthKey_<KEY_ID>.p8.base64` | Value source for `MACOS_NOTARY_KEY`. |
| `.apple-app-specific.env` | Local-only Apple release environment values. |

Back up these files outside git, for example under the operator's private
iCloud project folder. Do not place certificate passwords, private keys, API
keys, issuer ids, or key ids in public docs or task evidence. Do not use
`dist/certs/` for durable release credentials because GoReleaser cleans
`dist/`.

Set release secrets without echoing values:

```bash
gh secret set MACOS_SIGN_P12 \
  --repo fairway-run/fairway \
  < .release-certs/developerID_application_identities.p12.base64

gh secret set MACOS_SIGN_PASSWORD \
  --repo fairway-run/fairway \
  < .release-certs/macos-sign-p12-password.local

gh secret set MACOS_CODESIGN_IDENTITY \
  --repo fairway-run/fairway \
  --body "$MACOS_CODESIGN_IDENTITY"

gh secret set MACOS_NOTARY_KEY \
  --repo fairway-run/fairway \
  < .release-certs/AuthKey_<KEY_ID>.p8.base64

gh secret set MACOS_NOTARY_KEY_ID \
  --repo fairway-run/fairway \
  --body "$MACOS_NOTARY_KEY_ID"

gh secret set MACOS_NOTARY_ISSUER_ID \
  --repo fairway-run/fairway \
  --body "$MACOS_NOTARY_ISSUER_ID"

gh secret set HOMEBREW_TAP_GITHUB_TOKEN \
  --repo fairway-run/fairway
```

Local signing smoke:

```bash
tmpdir=$(mktemp -d /tmp/fairway-sign-test.XXXXXX)
go build -o "$tmpdir/fairway" ./cmd/fairway
codesign --force --timestamp --options runtime \
  --sign "<Developer ID Application identity>" \
  "$tmpdir/fairway"
codesign --verify --strict --verbose=4 "$tmpdir/fairway"
codesign -dv --verbose=4 "$tmpdir/fairway"
```

For archive notarization smoke tests, sign the binary, zip the artifact, then
submit the zip to `notarytool` with the App Store Connect API key:

```bash
xcrun notarytool submit fairway.zip \
  --key .release-certs/AuthKey_<KEY_ID>.p8 \
  --key-id "$MACOS_NOTARY_KEY_ID" \
  --issuer "$MACOS_NOTARY_ISSUER_ID" \
  --wait
```

`stapler` cannot staple a zip archive. For the current tar/zip CLI
distribution, an accepted notarization result is the expected release signal.
Use a `.pkg` or `.dmg` if Fairway later needs stapled offline verification.

## First Homebrew publish

Run these checks before tagging:

```bash
git status --short
go test ./...
go vet ./...
goreleaser check
(cd website && npm run build)
go run ./cmd/fairway workflow check \
  --mode deploy \
  --require-clean \
  --require-pushed
```

Create one release-run task and packet for each meaningful release attempt:

```bash
fairway packet release-run FW-REL-012 \
  --version v0.1.2 \
  --tag v0.1.2 \
  --source-sha "$(git rev-parse HEAD)" \
  --release-notes docs/release-notes.md \
  --changelog-state "CHANGELOG.md has v0.1.2 section" \
  --ci-status pass \
  --docs-status pass \
  --signing-status pass \
  --notary-status pass \
  --release-url "https://github.com/fairway-run/fairway/releases/tag/v0.1.2" \
  --homebrew-tap-commit <tap-commit-sha> \
  --verification-command "go test ./..." \
  --verification-command "go vet ./..." \
  --verification-command "goreleaser check" \
  --verification-command "brew fetch --cask --force fairway-run/tap/fairway"
```

Cut the release tag from a clean, pushed `main`:

```bash
git fetch --all --tags
git status --short --branch
git tag -a v0.1.1 -m "v0.1.1"
git push fairway-run v0.1.1
```

Watch the release workflow:

```bash
gh run list --repo fairway-run/fairway --workflow release.yml --limit 5
gh run watch --repo fairway-run/fairway
```

The GitHub Release is intentionally created as a draft. Review artifacts,
checksums, signing/notarization logs, and the generated Homebrew cask before
publishing the draft.

Do not treat the release as Homebrew-ready while the GitHub Release is still a
draft. GoReleaser can update `fairway-run/homebrew-tap` before the draft is
published, but cask URLs point at `releases/download/vX.Y.Z/...` and return 404
until the release is public. The release-run checklist should classify this as
failed verification:

```text
Homebrew cask version == tag, but GitHub release is draft or asset URL is 404.
Action: publish the reviewed release draft, then verify asset URLs and brew fetch.
```

Verify Homebrew after the cask update lands:

```bash
brew untap fairway-run/tap || true
brew tap fairway-run/tap
brew install --cask fairway
fairway help
brew uninstall --cask fairway
```

For a lower-impact verification that does not install Fairway, update the tap
cache and fetch the cask:

```bash
tapdir=$(brew --repository fairway-run/tap)
git -C "$tapdir" fetch origin main --prune
git -C "$tapdir" reset --hard origin/main
brew info --cask fairway-run/tap/fairway
brew fetch --cask --force fairway-run/tap/fairway
```

The expected result is:

- GitHub release is not draft.
- Release assets include checksums and all target archives.
- Asset URLs return 200 after redirects.
- Homebrew cask reports the new version.
- `brew fetch --cask --force fairway-run/tap/fairway` succeeds.

Record the observed release state with the release verification guard:

```bash
fairway release verify \
  --version v0.1.2 \
  --tag v0.1.2 \
  --source-sha "$(git rev-parse HEAD)" \
  --release-notes docs/release-notes.md \
  --changelog CHANGELOG.md \
  --ci-status pass \
  --docs-status pass \
  --signing-status pass \
  --notary-status pass \
  --release-state public \
  --release-url "https://github.com/fairway-run/fairway/releases/tag/v0.1.2" \
  --asset "https://github.com/fairway-run/fairway/releases/download/v0.1.2/fairway_v0.1.2_checksums.txt=200" \
  --homebrew-version 0.1.2 \
  --homebrew-tap-commit <tap-commit-sha> \
  --brew-fetch-status pass \
  --verification-command "brew fetch --cask --force fairway-run/tap/fairway"
```

Git tags include the `v` prefix (`v0.1.2`), while Homebrew cask versions
normally omit it (`0.1.2`). `fairway release verify` normalizes that prefix for
the Homebrew comparison but still fails real semantic mismatches.

The v0.1.2 lesson is explicit: if `--release-state draft` while
`--homebrew-version` matches the release tag after prefix normalization,
`fairway release verify` fails.
Publish the reviewed GitHub release draft first, then verify asset URLs and
`brew fetch` before treating the Homebrew cask as usable.

If the cask publish fails, fix the tap or release config and rerun the release
workflow only when the generated cask will point to the same immutable tag and
checksums. If the released artifact itself is wrong, yank and cut a new version;
never reuse a version number.

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

Minimum Cloudflare token permissions:

| Scope | Permission | Why |
|---|---|---|
| Account / selected Fairway account | Cloudflare Pages Edit or Pages Write | Create/update Pages projects and publish deployments. |
| Account / selected Fairway account | Account Settings Read | Resolve account metadata during Pages deploy/setup tooling. |
| Zone / `fairway.run` only | DNS Edit or DNS Write | Create/update `fairway.run`, `www.fairway.run`, and `docs.fairway.run` DNS records. |
| Zone / `fairway.run` only | Zone Read | Resolve the zone and validate custom-domain routing. |

If Cloudflare Pages is connected directly to GitHub and Cloudflare owns deploys,
the CI token may not need DNS write after initial custom-domain setup. Keep DNS
write on a setup/admin token when possible, and use a narrower Pages-only deploy
token for routine GitHub Actions deploys.

Local Fairway Cloudflare credentials may be stored in
`.env.cloudflare.fairway-run`. The file is ignored by git and must not be
printed, committed, or mixed with Core42/Core42.dev credentials.

## Docs portal edge security

Do not assume Cloudflare Pages alone is the complete security posture. The docs
portal setup should explicitly verify the following before the site is treated as
production-ready:

- Cloudflare bot protection is enabled for `fairway.run` using the available
  plan capability, such as Bot Fight Mode, Super Bot Fight Mode, or an
  equivalent managed bot rule.
- Security headers are shipped from the Pages project, normally through a
  `_headers` file in the static output path:
  - `X-Frame-Options: DENY`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Permissions-Policy` with unused browser features disabled
  - `Content-Security-Policy` appropriate for the generated Docusaurus assets
- The Pages preview host is not indexed if preview deployments are public. Use
  `X-Robots-Tag: noindex` for `*.pages.dev` preview hosts where appropriate.
- Custom-domain routing is verified for `fairway.run`, `www.fairway.run`, and
  `docs.fairway.run`.
- Any WAF/bot challenge rule is checked against normal docs traffic, GitHub
  release downloads, Homebrew install paths, and legitimate search crawlers.

Cloudflare setup belongs to its own tracked task because it includes domain,
token, static-site, and public-content boundary decisions.

## Pre-1.0 distribution

- Homebrew cask for tagged releases:
  `brew tap fairway-run/tap && brew install --cask fairway`.
- Direct downloads from GitHub Releases.
- Source install before the first tag:
  `go install github.com/fairway-run/fairway/cmd/fairway@latest`.
- Local checkout install: `make install` (defaults to `~/.local/bin/fairway`).

## Post-1.0 distribution

- Homebrew cask remains the primary macOS path.
- Possibly Scoop, Nix, AUR — community-contributed.

## Yanking a release

A release with a critical bug is yanked by:

1. Deleting the tag on the remote (`git push --delete origin vX.Y.Z`).
2. Editing the GitHub Release to "Draft" with a note explaining.
3. Cutting `vX.Y.Z+1` with the fix.

Never re-use a version number.
