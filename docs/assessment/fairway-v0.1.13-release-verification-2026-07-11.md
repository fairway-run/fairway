# Fairway v0.1.13 release verification

Date: 2026-07-11  
Task: FW-335  
Result: pass

## Release Identity

- Version and tag: `v0.1.13`
- Source SHA: `3bcc70d5ab9aca3f01780ca089bd6d537eeb8b40`
- Previous release and rollback reference: `v0.1.12`
- Release workflow: `29177531717`
- Public release: <https://github.com/fairway-run/fairway/releases/tag/v0.1.13>
- Homebrew tap commit: `654e1fd66fa21653da1c678fe0640c6b79862306`

The annotated tag was created once at the approved SHA and pushed to the
release remote. No existing tag was moved.

## Workflow And Assets

The GitHub Release workflow completed successfully. It ran tests and vet,
loaded the reviewed release highlights, imported the Developer ID certificate,
validated the GoReleaser configuration, signed and notarized both Darwin
binaries, built four platform archives, generated checksums, created the
Homebrew cask, and uploaded the release assets.

Public HTTP `200` readback passed for:

- `fairway_0.1.13_checksums.txt`
- `fairway_0.1.13_darwin_amd64.tar.gz`
- `fairway_0.1.13_darwin_arm64.tar.gz`
- `fairway_0.1.13_linux_amd64.tar.gz`
- `fairway_0.1.13_linux_arm64.tar.gz`

`sha256sum -c` passed for all four archives. The extracted Darwin arm64 binary
reported `0.1.13`, and strict `codesign` verification passed. The release
workflow's notarization hooks completed successfully for both Darwin builds.
`spctl --type execute` reports that the valid command-line binary is not an app;
that result is not used as a notarization failure because the artifact is a CLI
binary rather than an application bundle.

## Homebrew

The public cask reports:

- version `0.1.13`;
- description `Engineering control and accountability for agent-driven delivery`;
- homepage `https://fairway.run`;
- arm64 download URL under the public `v0.1.13` release.

`brew fetch --cask --force fairway-run/tap/fairway` passed.

## Fairway Verification

`fairway release verify` passed with:

- public release state;
- exact source SHA and tag;
- CI, docs, signing, and notarization status `pass`;
- all five public asset checks `200`;
- Homebrew version `0.1.13`;
- tap commit and brew fetch status.

The sanitized provenance bundle is
`docs/assessment/fairway-v0.1.13-provenance.json`. It excludes raw prompts,
transcripts, tool bodies, generated-content dumps, credentials, and the local
checkout path.

## Deferred Work

- Shared dashboard restart/version readback is intentionally not part of
  FW-335 and requires a separate reviewed lifecycle task.
- FW-336 owns the post-release provider-replacement value demonstration.
- Portal dependency refresh and generic provenance-path sanitization remain
  bounded maintenance work; neither changes the published release authority.

## Verdict

Fairway `v0.1.13` is public and verified. The source tag, release assets,
checksums, signing/notarization workflow, Homebrew cask, public documentation,
and rollback reference are consistent with the reviewed release packet.
