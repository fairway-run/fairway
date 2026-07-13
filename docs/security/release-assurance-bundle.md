# Release Assurance Bundle

FW-347 packages machine-verifiable evidence for one Fairway release candidate.
It is an input to release review and sovereign offline distribution, not a
certification, compliance result, dependency trust decision, deployment
approval, or release authorization.

Every bundle contains candidate archives, generated checksums, detached
Ed25519 signatures, SPDX SBOM, OpenVEX, dependency/license inventories and any
reviewed license disposition,
source/build provenance, build recipe, test summary, and vulnerability
disposition. The signed manifest binds exact release version, source revision,
builder identity, policy version, file digests, signing key ID, and measured
SLSA properties. Signing material is environment-only.

`fairway release assurance verify` requires a separately pinned public key and
exact expected version, source, builder, and policy. It fails on missing or
unknown files, path/symlink issues, hash/size/checksum/signature mismatch,
incomplete evidence classes, or a generated SLSA level claim.

An embedded public key is identity metadata, not trust. A valid bundle does not
prove source quality, dependency trust, absence of vulnerabilities,
hermeticity, reproducibility, or external certification. The no-findings path
produces an empty OpenVEX statement list only after govulncheck succeeds; it
does not turn scanner silence into `not_affected` claims.

License classifier exceptions are fail-closed. A committed override must pin
module version, upstream origin and commit, SPDX license identifier, license
path, and license-file SHA-256. The release script verifies all fields against
the downloaded module before replacing an `Unknown` inventory row; any other
unknown license stops the candidate release.

## Publication ordering

The tag workflow runs GoReleaser with `--skip=publish`. This produces the final
signed and notarized candidate archives without creating a GitHub release or
mutating the Homebrew tap. Fairway then generates and pinned-verifies the
assurance package over those exact archives. Only after that verification may
the workflow create a GitHub draft from the verified archive copies.

The workflow does not publish the draft or update Homebrew. Those are separate
reviewed release actions and must use the archive digests in the verified
bundle. A bundle-generation or verification failure therefore leaves no draft,
tap commit, public release, or deployment side effect.
