# Sovereign Cryptography And Key Posture

FW-345 defines how Fairway records cryptographic boundaries and checks whether
the evidence needed for a bounded sovereign readiness claim is present. It does
not add encryption to SQLite, operate a KMS or HSM, generate customer keys,
validate a cryptographic module, certify a deployment, or authorize operation.

## Required Boundaries

Every sovereign profile records all five boundaries:

| Boundary | Protected path | Typical control owner |
|---|---|---|
| `in_transit` | CLI/API/dashboard/store/proxy traffic crossing the supported deployment boundary | Customer or shared |
| `at_rest` | Fairway DB, configuration, local evidence roots, logs, and service state | Customer or shared |
| `backup` | DB/config/evidence backup, transfer, retention, restore, and disposal | Customer |
| `evidence_export` | Assurance, provenance, audit, and assessor package export at rest and in transfer | Shared |
| `signing` | Release, assurance-package, audit-export, identity-proof, and offline-bundle signatures | Product, customer, or shared depending on key purpose |

Each `[[sovereign_crypto_boundaries]]` row names:

- accountable owner and key custodian;
- metadata-only key reference and exact algorithm/protection mechanism;
- exact module name, version, and assurance posture;
- local customer approval evidence;
- local custody, rotation, and loss/recovery evidence;
- when externally validated module posture is asserted, the exact validation
  certificate and validated-configuration evidence.

Key references are identifiers, never key material. Fairway rejects private-key,
credential, token, password, and secret-like values in these fields. Evidence
must be a non-symlink regular file under the project root. A URL, URN, path
traversal, missing path, or symlink does not satisfy readiness.

## Capability Check

```bash
fairway readiness crypto
fairway --json readiness crypto
```

The report schema is `fairway.sovereign-crypto-readiness.v1`. It lists each
boundary, owner, custodian, key/module/algorithm metadata, assurance posture,
resolved local evidence, and exact gaps. It exits non-zero until all five rows
and required proof are present. `fairway doctor` also surfaces each boundary;
missing proof blocks sovereign readiness when `runtime.profile` is
`sovereign-offline`.

The report is deterministic from configuration and local file existence. It
does not inspect private keys, decrypt data, invoke remote validators, or infer
that a configuration was applied correctly. Operators and assessors must verify
the referenced evidence and deployment state independently.

## FIPS 140-3 Boundary

`module_assurance = "fips_140_3_validated"` is accepted only when the boundary
also identifies a local validation certificate and the exact validated
configuration. That status means only:

> The named module/version/configuration has externally recorded validation
> evidence referenced by this readiness packet.

It does not mean:

- Fairway must not be represented as a FIPS 140-3 validated cryptographic module;
- an unlisted Go runtime, operating system, proxy, database, storage layer,
  signer, or HSM configuration inherits validation;
- this evidence grants the customer deployment no compliance conclusion, certification, or authorization;
- a certificate applies outside its named version, operating environment,
  algorithms, modes, key sizes, or security policy.

`customer_approved` can satisfy Fairway readiness when the customer's applicable
requirements permit it and approval/custody/rotation/recovery evidence is
present. The report remains `fips_posture=not_claimed`. `not_assessed` never
satisfies readiness.

Fairway uses standard platform cryptography already present in Go and customer
deployment controls. This task introduces no bespoke algorithm or crypto
primitive. Later release and assessor packages must carry exact module and
configuration references rather than converting this inventory into a Fairway
product claim.

## Responsibility And Lifecycle

The product owner documents which Fairway binaries and package formats use which
algorithms. The customer owns deployment encryption, key custody, platform/HSM
selection, access control, activation, rotation, revocation, escrow/recovery,
backup-key availability, disposal, and applicability decisions. Shared
boundaries must name both sides' responsibility.

Before promotion, rehearse:

1. normal key use without exporting private material;
2. rotation with old/new key identity and bounded overlap;
3. revoked or unavailable key failure;
4. backup restore with the required decryption key;
5. evidence/audit/package signature verification after rotation;
6. lost-key recovery and the case where recovery is intentionally impossible;
7. cleanup and proof that temporary plaintext/key material was not retained.

Record the packet as assessment input. A customer risk owner or qualified
external assessor decides whether the evidence meets the applicable policy or
certification scheme.
