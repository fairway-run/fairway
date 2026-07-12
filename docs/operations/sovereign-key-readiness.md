# Sovereign Cryptography Readiness Runbook

Use this runbook to prepare the local evidence consumed by
`fairway readiness crypto`. It does not create keys, inspect private material,
change encryption, contact a certification service, or authorize deployment.

## Preparation

1. Create a restricted local artifact directory under the project root.
2. For each required boundary, identify owner, custodian, key reference,
   algorithm/protection mechanism, module name/version, and assurance posture.
3. Place redacted approval, custody, rotation, and recovery proof in that
   directory. Do not copy private keys, credentials, tokens, recovery secrets,
   or plaintext restricted data.
4. For a claimed externally validated module, include a local copy/reference
   record for the exact validation certificate and validated configuration.
5. Configure one `[[sovereign_crypto_boundaries]]` row per required boundary.

Example row; repeat it for `in_transit`, `at_rest`, `backup`,
`evidence_export`, and `signing`:

```toml
[[sovereign_crypto_boundaries]]
name = "backup"
owner = "customer"
custodian = "customer-security"
key_reference = "pkcs11:backup-key-2026-01"
algorithm = "AES-256-GCM"
module_name = "customer-approved-module"
module_version = "1.0"
module_assurance = "customer_approved"
approval_evidence = ".fairway/artifacts/crypto/backup-approval.json"
custody_evidence = ".fairway/artifacts/crypto/backup-custody.json"
rotation_evidence = ".fairway/artifacts/crypto/backup-rotation.json"
recovery_evidence = ".fairway/artifacts/crypto/backup-recovery.json"
```

For `module_assurance = "fips_140_3_validated"`, also set:

```toml
validation_certificate = ".fairway/artifacts/crypto/module-certificate.json"
validated_configuration = ".fairway/artifacts/crypto/validated-configuration.json"
```

## Verification

```bash
fairway config validate
fairway readiness crypto
fairway --json readiness crypto > .fairway/artifacts/crypto/readiness.json
fairway doctor --format json > .fairway/artifacts/crypto/doctor.json
```

Stop if readiness is false. Resolve each named missing field or local proof; do
not replace absent evidence with a URL, unverified claim, or generated rationale.
Review the report's `product_claim` and `prohibited_claims` before using it in an
assurance package.

## Rotation And Recovery Rehearsal

For each key purpose, record:

- old and new metadata-only key IDs and module/configuration versions;
- planned overlap and revocation time;
- successful verification/decryption before and after rotation;
- failure when the revoked key is used;
- restore behavior when the required backup key is present and absent;
- owner decision and rollback/stop condition;
- cleanup proof for temporary files and restored copies.

Do not rehearse against production or restricted customer data unless a separate
approved operation authorizes it. A successful Fairway readiness report is
evidence completeness, not module validation, certification, compliance, or
customer authorization.
