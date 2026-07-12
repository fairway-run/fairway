# Sovereign Audit Export Runbook

Use this runbook to create, externally retain, and verify a customer-signed
Fairway audit checkpoint. Run it only in an approved local environment. The
commands do not send data to a WORM/SIEM or modify workflow state.

## Prepare

1. Record a redacted trusted-time artifact under the project root. Do not place
   credentials, tokens, private keys, restricted payloads, or raw logs in it.
2. Load the customer Ed25519 private key into an environment variable through
   the approved local secret mechanism. Do not pass it on argv.
3. Load the pinned public key into a different environment variable.
4. Choose stable policy, retention, legal-hold, external-target, and exact
   Fairway/source version identifiers.
5. Ensure the export parent exists and the output directory does not.

## Genesis

```bash
fairway audit export \
  --out .fairway/artifacts/audit/genesis \
  --policy customer-audit-v1 \
  --source-version fairway-vX.Y.Z-source-<commit> \
  --trusted-time-source customer-ptp \
  --trusted-time-evidence .fairway/artifacts/audit/time-proof.json \
  --retention-policy customer-retention-v1 \
  --legal-hold none \
  --external-target worm:customer/audit \
  --signing-key-env FAIRWAY_AUDIT_SIGNING_KEY \
  --genesis

fairway audit verify \
  --dir .fairway/artifacts/audit/genesis \
  --trusted-public-key-env FAIRWAY_AUDIT_PUBLIC_KEY
```

Copy the complete directory to the approved independently controlled target.
Verify the retained copy, record its manifest digest and target receipt as
separate evidence, then protect the local copy according to policy.

## Subsequent Checkpoint And Key Rotation

```bash
fairway audit export \
  --out .fairway/artifacts/audit/current \
  --policy customer-audit-v1 \
  --source-version fairway-vX.Y.Z-source-<commit> \
  --trusted-time-source customer-ptp \
  --trusted-time-evidence .fairway/artifacts/audit/time-proof.json \
  --retention-policy customer-retention-v1 \
  --legal-hold active \
  --external-target siem:customer/audit \
  --signing-key-env FAIRWAY_AUDIT_NEW_SIGNING_KEY \
  --previous .fairway/artifacts/audit/genesis \
  --previous-trusted-public-key-env FAIRWAY_AUDIT_OLD_PUBLIC_KEY

fairway audit verify \
  --dir .fairway/artifacts/audit/current \
  --trusted-public-key-env FAIRWAY_AUDIT_NEW_PUBLIC_KEY \
  --previous .fairway/artifacts/audit/genesis \
  --previous-trusted-public-key-env FAIRWAY_AUDIT_OLD_PUBLIC_KEY
```

Do not delete the previous export or old verification key until retention and
legal-hold policy permits it. A non-genesis export without the previous package
fails continuity verification.

## Backup, Restore, And Failure

Rehearse from disposable copies:

1. back up the Fairway DB, configuration, pinned public keys, and externally
   retained audit packages through their separately approved encrypted paths;
2. restore them to a clean local environment;
3. verify every retained checkpoint in order;
4. export the restored current history against the latest retained checkpoint;
5. prove that a DB restored behind that checkpoint is rejected;
6. prove that record/file/signature substitution and a wrong pinned key fail;
7. record cleanup and disposal evidence.

On any verification or continuity failure, stop promotion, preserve the DB and
packages read-only, record the exact bounded error, notify the evidence owner,
and follow the customer incident/legal-hold procedure. Do not regenerate a
genesis package to hide a missing or divergent checkpoint.
