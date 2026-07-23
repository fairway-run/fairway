# Release Candidate Promotion

## Decision

Fairway builds, signs, notarizes, tests, and packages a release candidate before
the final remote tag exists. The final tag promotes those exact immutable
candidate assets; it does not rebuild them.

This separates two concerns:

1. **Candidate qualification** proves that an exact pushed `main` commit can
   produce the complete release packet under the production signing,
   notarization, SBOM, license, vulnerability, and assurance controls.
2. **Release promotion** binds an annotated semantic-version tag to one
   successful rehearsal run and creates a draft release from that run's exact
   assets.

## Candidate identity

The manual `Release Rehearsal` workflow is dispatched with:

- semantic version in `vX.Y.Z` form;
- exact lowercase 40-character source SHA;
- source SHA equal to the selected `main` workflow-dispatch head;
- no existing remote tag for the requested version.

The workflow checks out that SHA, creates a local-only annotated tag so
GoReleaser produces final names and embedded version metadata, then runs the
same signing, notarization, test, assurance, and packaging path used for a
release.

## Promotion packet

The uploaded artifact contains exactly:

- four platform archives;
- one GoReleaser checksum file;
- one signed release-assurance archive;
- one portable assurance checksum;
- `rehearsal.json`.

`rehearsal.json` uses schema `fairway.release-rehearsal.v1` and binds the exact
version, source SHA, workflow builder identity, policy version, creation time,
asset names, byte sizes, and SHA-256 digests. Creation and verification reject
missing files, extra files, symlinks, malformed identity, checksum drift, and
asset mutation.

The workflow also extracts the macOS arm64 archive, checks its embedded Fairway
version, and runs strict native `codesign` verification before uploading the
packet.

Signing, notarization, and assurance private-key secrets are scoped only to the
steps that require them. Tests, checkout, setup, smoke, packet assembly, and
artifact upload do not receive the raw secrets. The imported signing keychain
is locked and its temporary import files are removed immediately after the
GoReleaser build.

## Promotion binding

The release owner creates one annotated tag whose message includes:

```text
fairway-rehearsal-run: <github-actions-run-id>
```

The tag-triggered workflow:

1. requires an annotated tag and exactly one rehearsal run id;
2. resolves the tag's commit;
3. requires the referenced run to be a successful `workflow_dispatch` of
   `.github/workflows/release-rehearsal.yml` on `main` at that exact commit;
4. downloads exactly one unexpired artifact named for the version and SHA;
5. verifies the rehearsal manifest and safely extracts the assurance archive
   with bounded path, type, symlink, duplicate, and size checks;
6. verifies signed assurance against expected
   version, source, builder, policy, and pinned public key;
7. uploads a verified promotion input for a separate write-authorized job;
8. creates a draft GitHub release from the packet's exact files.

Candidate verification runs with read-only repository permissions and checkout
does not persist credentials. The separate promotion job receives
`contents: write` only after verification and does not check out or execute
candidate code. The workflow has no signing or notarization secrets and does
not run GoReleaser. This is the enforcement point for "build once, promote
exactly."

## Failure and retry behavior

- A failed rehearsal has no tag, release, Homebrew, or deployment side effect.
- A corrected candidate uses a new rehearsal run. The final annotation names
  only the approved run.
- A failed promotion may be rerun against the same tag and immutable packet
  when no release asset changed.
- If candidate bytes or source change, run a new rehearsal. Never modify a
  packet or reuse a published version.
- Rehearsal artifacts expire after 30 days. Promotion after expiry requires a
  new rehearsal of the same source and version.

Publication of the draft and Homebrew update remain explicit reviewed actions
after promotion verification.
