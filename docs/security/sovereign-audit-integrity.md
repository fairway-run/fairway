# Sovereign Audit Integrity

FW-346 adds a signed, tamper-evident export over Fairway's existing
`audit_events` source of truth. It does not add a second audit database, make
the SQLite file append-only, transmit records to an external service, operate a
WORM/SIEM, certify a deployment, or authorize an action.

## Bound Record

The exporter reads one project in database `id` order. Each canonical record
binds:

- source record ID and sequence;
- project, actor, action, and optional task;
- the SHA-256 digest of detail, never raw detail content;
- record creation time;
- the previous record hash and current record hash.

The signed manifest binds the record file digest/size, first and last IDs,
record count, chain head, trusted-time evidence reference/digest, retention
policy, legal-hold status, external target reference, source version, signing
key identity, and continuity checkpoint.

Policy, source version, and trusted-time metadata belong to the signed export
manifest rather than the immutable record hash. This lets a later export change
those versioned controls while still proving that its audit-fact prefix matches
the previous retained checkpoint. The previous manifest digest preserves the
earlier metadata exactly.

This detects changes to an exported record, deletion, insertion, reordering,
scope substitution, file substitution, and manifest substitution. A hash chain
by itself cannot detect database rollback. Fairway therefore requires either an
explicit genesis checkpoint or continuity to a previously signed, pinned, and
externally retained export. A current database behind or divergent from that
checkpoint fails export.

## Signing And Rotation

The export signing key is customer-controlled Ed25519 material supplied through
an environment variable. It is never accepted on argv, written to the package,
or included in errors. Verification requires a separately supplied pinned
public key. For rotation:

1. verify the previous export with the old pinned public key;
2. export the current history with the new private key and `--previous`;
3. verify the new export with the new public key and the previous export with
   the old public key;
4. retain the old public key and checkpoint for the applicable audit period;
5. record rotation approval and revocation separately.

The embedded public key supports key identity calculation; it is not trusted
until it matches the operator's pinned key.

## Time And Rollback Boundary

`generated_at` uses the local clock, while `trusted_time_source` and the digest
of a local trusted-time evidence artifact are signed into the manifest and each
record. Fairway does not validate an NTP/PTP/RFC 3161 service. The customer or
assessor verifies that the evidence came from an approved time boundary.

Rollback detection is relative to the previous retained checkpoint. Deleting
every external checkpoint removes that anchor. Operators must therefore copy
each accepted export to an independently controlled immutable or append-only
target before treating continuity as established.

## Privacy And Authority

The package excludes raw detail, prompts, transcripts, tool bodies,
credentials, secrets, and artifact contents. Actor/action/task metadata and
detail hashes can still be sensitive and remain subject to customer access,
retention, legal-hold, and disposal policy.

Successful verification means the pinned signature, package files, ordered
chain, trusted-time evidence binding, and supplied previous checkpoint agree.
It does not establish compliance, certification, completeness of every
customer log source, accuracy of human-entered metadata, authorization, risk
acceptance, or proof that a WORM/SIEM retained the package.
