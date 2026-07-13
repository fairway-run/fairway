# Restricted Advisory and LTS Patch Channel

FW-353 provides a disconnected security-notification package for Fairway
operators who cannot allow installations to contact a public service. The
channel reuses customer-controlled removable-media and software-intake
boundaries. It does not add telemetry, update checks, dashboard send controls,
or autonomous patch deployment.

## Package Contract

`fairway security advisory export` writes a new directory with an exact signed
inventory:

| Path | Purpose |
|---|---|
| `advisory.json` | Strict `fairway.security-advisory.v1` machine record |
| `advisory.md` | Deterministic human view regenerated during verification |
| `patch/patch-bundle.bin` | Opaque offline patch artifact bound by digest and identifier |
| `manifest.json` | Advisory, patch, rollback, file, and signing-key identity |
| `signature.json` | Ed25519 signature over the manifest SHA-256 digest |

The machine record requires severity, affected and fixed versions,
mitigations, VEX updates with justification, support track, patch bundle id,
rollback bundle id, and publication time. Input is strict JSON: unknown or
duplicate keys fail. Secret-like content, unsafe identifiers, symlinks,
unknown files, digest changes, generated-view changes, identity substitution,
and unpinned keys fail closed.

For a real patch, `patch-bundle.bin` should be the already verified signed
offline distribution produced by the release assurance flow. This advisory
wrapper does not replace `fairway release offline verify`; run both verifiers.

## Export

Author a privacy-reviewed JSON record. Keep signing material in an environment
variable supplied by the release secret boundary:

```bash
export FAIRWAY_ADVISORY_SIGNING_KEY='<base64 Ed25519 seed/private/PKCS8 DER>'

fairway security advisory export \
  --advisory advisory.json \
  --patch-bundle fairway-offline-v0.1.14.tar.gz \
  --out fairway-sa-2026-001 \
  --signing-key-env FAIRWAY_ADVISORY_SIGNING_KEY
```

Export does not fetch, publish, notify, install, or deploy. Transfer the package
and an independently approved public-key fingerprint through the customer
software-intake boundary.

## Verify and Acknowledge

At the disconnected destination, pin the public key separately and require the
expected identities:

```bash
export FAIRWAY_ADVISORY_PUBLIC_KEY='<base64 Ed25519 public/PKIX DER>'

fairway security advisory verify \
  --dir fairway-sa-2026-001 \
  --expected-id FAIRWAY-SA-2026-001 \
  --expected-patch-bundle-id fairway-offline-v0.1.14 \
  --expected-rollback-bundle-id fairway-offline-v0.1.13 \
  --trusted-public-key-env FAIRWAY_ADVISORY_PUBLIC_KEY

fairway security advisory acknowledge \
  --dir fairway-sa-2026-001 \
  --expected-id FAIRWAY-SA-2026-001 \
  --expected-patch-bundle-id fairway-offline-v0.1.14 \
  --expected-rollback-bundle-id fairway-offline-v0.1.13 \
  --trusted-public-key-env FAIRWAY_ADVISORY_PUBLIC_KEY \
  --customer-ref restricted-site-001 \
  --status received \
  --at 2026-07-12T12:00:00Z \
  --out FAIRWAY-SA-2026-001-ack.json
```

Acknowledgement is a local receipt fact tied to the verified manifest, signing
key, patch digest, and exact rollback bundle. `deferred` and `rejected` remain
visible outcomes. The record is not an approval, patch-import command, maintenance decision, risk
acceptance, deployment action, or message backchannel. Customers decide whether
and how to return an acknowledgement through their approved channel.

## LTS, End of Support, and Emergency Response

[SECURITY.md](https://github.com/fairway-run/fairway/blob/main/SECURITY.md) is
canonical for support tracks and response targets. Each restricted advisory
must name one of `standard`, `lts`, or
`emergency` and exact affected/fixed versions. End-of-support notices use the
same signed package even when no patch is attached in a future schema; v1
always requires a non-empty patch artifact, so an EOS-only v1 package uses a
reviewed informational artifact and an explicit non-installable identifier.

An emergency patch does not bypass release assurance. It still needs signed
artifacts, independent review, rollback identity, and customer-controlled
import. Planned root rotation overlaps old and new roots. Suspected key
compromise uses an independent intake channel; a compromised root cannot
bootstrap trust in its own replacement.

## Synthetic Rehearsal

Run the bounded channel rehearsal:

```bash
scripts/ci/restricted_advisory_rehearsal.sh \
  .fairway/artifacts/fw-353-restricted-advisory-rehearsal
```

The harness builds or accepts a local Fairway binary, generates an ephemeral
Ed25519 key, exports a synthetic advisory and opaque synthetic patch, copies it
through a removable-media-style directory, verifies with a separately supplied
public key, records receipt, proves tamper rejection, and removes ephemeral key
material. It does not contact a notifier, install a binary, alter Fairway task
state, deploy, change public exposure, or claim a real vulnerability was fixed.

## Known Limits

- The package is a delivery and integrity envelope, not a vulnerability scanner
  or external certification result.
- VEX statements remain maintainer assertions requiring review and supporting
  evidence.
- Customer acknowledgement is not cryptographically signed in v1; customers
  should protect and return it using their local evidence-retention policy.
- The embedded public key is metadata only. Trust comes from the separately
  pinned key or fingerprint.
- Actual patch import must separately verify the nested offline distribution and
  follow the customer's change, backup, rollback, and deployment controls.
