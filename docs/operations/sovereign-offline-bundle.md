# Sovereign Offline Distribution Bundle

FW-343 defines a signed, removable-media-style Fairway distribution for
disconnected installation, update, and rollback. It combines two independently
verified release-assurance packages with local operator material and a small
standalone verifier. It does not certify a deployment or authorize installation.

## Contents

`fairway release offline export` creates one
`fairway.offline-distribution-manifest.v1` directory containing:

- current and rollback release-assurance directories, each with Fairway
  archives for Darwin/Linux on amd64/arm64, checksums, detached signatures,
  SBOM, VEX, dependencies, licenses/disposition, source/build provenance,
  build recipe, passing test summary, and vulnerability disposition;
- standalone `fairway-offline-verify` binaries for the same four platforms;
- local documentation, a generic Fairway configuration example, and all
  versioned sovereign deployment baselines;
- fixed `verify.sh`, `install.sh`, and `rollback.sh` operator scripts; and
- an exact file inventory signed with Ed25519.

The manifest binds current and rollback versions, source revisions, builder
identities, policy versions, signing-key ID, hashes, sizes, executable modes,
required asset classes, non-claims, and authority boundary. Unknown files,
missing platforms, unsafe paths, symlinks, duplicate JSON keys, digest or mode
changes, signature substitution, and identity mismatches fail verification.

## Trust bootstrap

The public key must be pinned through a customer-controlled channel separate
from the bundle. The included verifier is signed inventory content, but running
an untrusted verifier cannot bootstrap trust in itself. Before first use, obtain
the verifier binary or its approved digest through the organization's trusted
software-intake process. Thereafter use the pinned verifier and public key to
verify the complete media copy before every install or rollback.

Signing keys remain environment-only. Do not place private keys, credentials,
tokens, internal URLs, prompts, transcripts, or raw tool output in the bundle.

## Build

For a complete non-publishing rehearsal from exact current and rollback Git
identities, use the one-command builder:

```bash
scripts/release/build_sovereign_rehearsal_media.sh \
  --current-version 0.1.14-rehearsal.<short-sha> \
  --rollback-ref v0.1.13 --rollback-version 0.1.13 \
  --output-root /absolute/retained/fairway-sovereign-rehearsal \
  --builder-id local:reviewed-rehearsal-builder \
  --policy-version sovereign-rehearsal-v1 \
  --created-at 2026-07-12T12:00:00Z
```

The command builds both sources in detached temporary worktrees, creates four
normalized archives for each version, runs both release-assurance builders and
verifiers, creates and verifies the outer offline media, and writes a separate
`trust-bootstrap/` packet. The trust packet contains immutable source, version,
builder, archive, manifest, public-key, verifier, build-helper, and reviewed
license-policy digests. The current source's reviewed license override policy
is copied into each nested assurance package and applied to both current and
rollback dependency inventories; every override remains pinned to module
version, origin, origin commit, license path, and license digest. This makes the
policy input explicit without assuming that an older rollback tree contains the
current policy file. One current-source Fairway release tool performs export
and verification for both nested packages, because a rollback source may
predate the assurance command; its binary digest is in the trust packet, while
SBOM, dependency, license, and vulnerability collection still runs inside each
exact source tree. The trust packet deliberately contains no pending/approved
review state; record the latest independent acceptance as Fairway evidence or a
reviewed handoff without rewriting the packet.

The builder creates one ephemeral release-signing root, stores its private file
mode `0600` only under a temporary directory, removes it before writing trust
readback, unsets key environments, and secret-scans retained output. Its Go
archive helper normalizes timestamps, owners, modes, entry ordering, and gzip
metadata and rejects symlinks, non-regular files, AppleDouble files, and
`.DS_Store`, avoiding macOS extended-attribute portability noise. The retained
scan rejects private/secret/token filenames, PEM private-key headers, bearer
credential values, and long secret-like key assignments across text and binary
bytes without treating bounded verifier error strings as credentials. All
candidate output is built in a hidden sibling staging directory. Retained log
output is synchronous and is closed before the final scan. The promoter hashes
the complete staged inventory and bytes before and after scanning, rejects any
change, and atomically renames only that quiescent tree to the requested output
path. No retained file is written after the final scan.

On failure, staging, logs, partial media, trust material, manifests, archives,
and readback are removed before a new output containing exactly
`diagnostics/failure.json` is written. That bounded file contains only schema,
phase, exit code, private-material disposition, and the authority boundary. The
builder never publishes, tags, installs, deploys, changes public exposure, or
grants credential/live authority.

Build both release-assurance directories first. The current directory must
match checked-out `HEAD`; both must contain the four supported archives and
verify under the same pinned public key.

```bash
export FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY='<base64-ed25519-private-key>'
export FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY='<base64-ed25519-public-key>'

scripts/release/build_offline_distribution_bundle.sh \
  artifacts/current-release-assurance \
  artifacts/rollback-release-assurance \
  artifacts/offline
```

The builder cross-compiles four standalone verifiers, adds curated local
material, exports and pinned-verifies the bundle, then produces a tar archive
and SHA-256 sidecar. It does not fetch release archives or publish anything.

The lower-level command accepts repeatable typed assets:

```bash
fairway release offline export \
  --out <new-directory> \
  --current-assurance-dir <directory> \
  --rollback-assurance-dir <directory> \
  --trusted-public-key-env FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY \
  --signing-key-env FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY \
  --current-version <version> --current-source-sha <sha> \
  --current-builder-id <id> --current-policy-version <id> \
  --rollback-version <version> --rollback-source-sha <sha> \
  --rollback-builder-id <id> --rollback-policy-version <id> \
  --asset documentation:<name>=<path> \
  --asset configuration:<name>=<path> \
  --asset deployment_baseline:<name>=<path> \
  --asset verifier:<name>=<path>
```

## Verify and operate

Copy the complete directory to approved media. In the disconnected boundary:

```bash
export FAIRWAY_OFFLINE_TRUSTED_PUBLIC_KEY='<separately-pinned-public-key>'
./scripts/verify.sh
./scripts/install.sh /absolute/customer-owned-prefix
./scripts/rollback.sh /absolute/customer-owned-prefix
```

The lifecycle scripts require an absolute non-root prefix, verify before every
mutation, select an exact host archive, reject archives containing anything
other than one `fairway` binary, install through a temporary file and atomic
rename, preserve the prior binary under `backups/`, and print exact version
readback. They do not start services, alter configuration or data, contact a
network, approve a deployment, or delete customer backups.

The installed binary can run the loopback dashboard and read-only shared server
without a `git` executable. Worktree and lane-closeout projections are marked
`deferred` in that environment; they are never presented as clean or merged.
Coordinator diagnostics do not recommend ready-task dispatch while repository
cleanliness is unknown.
Commands that actually inspect or mutate Git state, including `worktree
status`, still fail with an actionable missing-Git error. Install Git only when
the disconnected operating procedure explicitly requires those repository
workflows; dashboard/server startup does not fetch or install it.

Use the reusable compatibility rehearsal after bundle creation:

```bash
scripts/ci/offline_distribution_rehearsal.sh \
  <bundle-directory> <artifact-directory> <current-version> <rollback-version>
```

The rehearsal copies the bundle into a read-only removable-media-style
directory; verifies it; installs the rollback baseline; creates a temporary
Fairway DB; records a task; backs up; upgrades; validates config and state;
backs up again; rolls back; reopens the same state; records binary, config,
data, and backup digests; and proves temporary-path cleanup. All paths are
temporary except the caller-owned artifact directory.

## Known limits and authority

- Verification proves package integrity and exact declared identity, not that
  the source is defect-free or suitable for a customer's system boundary.
- Archive signatures and evidence do not prove dependency trust, hermeticity,
  reproducibility, backup recoverability beyond the cited rehearsal, or
  external certification.
- Customer operators own media custody, verifier bootstrap trust, configuration
  approval, host/network controls, backup retention, applicability, and risk
  decisions.
- Fairway does not install automatically, start services, accept risk, certify,
  approve, deploy, expose, or perform live operations through this bundle.

Record the exact bundle digest, pinned key ID, current/rollback identities,
rehearsal artifact directory, deviations, and customer decision as evidence.
