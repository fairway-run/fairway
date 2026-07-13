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

## Offline distribution composition

FW-343 composes two verified release-assurance packages into a separately
signed offline distribution: the intended current release and a pinned
rollback release. The outer manifest requires all supported archives and
standalone verifier binaries, local docs/configuration/deployment baselines,
fixed lifecycle scripts, and an exact signed inventory. Both nested packages
must verify against the same separately pinned key before export and again
during offline verification.

This composition does not weaken either nested package or create a new trust
root. The verifier itself must be obtained or digest-approved through a trusted
software-intake channel; a binary cannot establish its own trust by verifying
the bundle that contains it. Operational guidance is in
[Sovereign Offline Distribution Bundle](../operations/sovereign-offline-bundle.md).

The sovereign rehearsal media builder composes this package twice, once for an
exact current source and once for a pinned rollback source. Its trust bootstrap
records immutable package, builder, and reviewed license-policy digests only.
The explicit current license policy is copied into both nested evidence sets;
its module version, origin, commit, path, and digest checks still run against
each source dependency graph. A digest-pinned current release tool exports and
verifies both packages when the rollback source predates the assurance command;
the rollback archive and collected dependency evidence still come from the
exact rollback source. Review verdicts remain durable Fairway facts and are not
copied into or used to rewrite signed media.

The composed sovereign output is not retained incrementally. It is assembled
and verified in a private sibling staging directory. Retained logging uses a
synchronous file descriptor that is restored and closed before final scanning.
The promoter snapshots every staged path, mode, size, and byte before and after
the secret scan; any delayed append or inventory change fails closed instead of
being promoted. The quiescent tree is then atomically renamed, with no later
retained write. Any failed phase removes staging, including its build log and
partial signed artifacts, before retaining the single bounded
`diagnostics/failure.json`. The failure packet is operational status only and
cannot be interpreted as verified media, trust bootstrap, certification, or
release authority.
