# Sovereign Identity And Command Authorization

FW-344 defines the customer-controlled identity boundary for Fairway shared API
use under a supported sovereign profile. It is an engineering control, not an
identity provider, certification, system authorization, or delegation of
deployment, release, credential, public-exposure, or live-operation authority.

## Supported Boundary

Set `[server].identity_mode = "sovereign_signed"`. An active shared server under
`[runtime].profile = "sovereign-offline"` rejects every other identity mode.
Each read and write requires a compact Ed25519-signed proof in
`Authorization: Bearer <proof>`. Fairway verifies it using a customer-controlled
public key and never receives or stores the private signing key.

The proof format is a deliberately narrow JWT-compatible three-segment envelope:

- header: exact `alg=EdDSA`, `typ=JWT`, and configured `kid`;
- claims: exact issuer, audience, subject, project, role, purpose, proof ID,
  issued-at, not-before, and expiry fields;
- signature: Ed25519 over the encoded header and claims.

Unknown or duplicate fields, algorithms, key IDs, malformed JSON, missing time claims,
future or expired sessions, sessions beyond the configured lifetime, project
confusion, and revoked proof/subject/key IDs fail closed. This is not general
JWT/JWK/OIDC support. A customer issuer must produce the exact
`fairway sovereign signed proof v1` contract described here and keep its signing
key outside Fairway.

## Configuration

```toml
[runtime]
profile = "sovereign-offline"

[server]
enabled = true
listen = "127.0.0.1:7880"
mode = "api-write-pilot"
read_only = false
write_enabled = true
identity_mode = "sovereign_signed"
allowed_roles = ["viewer", "operator", "reviewer:security", "authorizer"]
sovereign_public_key_env = "FAIRWAY_CUSTOMER_IDENTITY_PUBLIC_KEY"
sovereign_key_id = "customer-root-2026-01"
sovereign_issuer = "urn:customer:engineering-identity"
sovereign_audience = "fairway"
sovereign_revocation_file = "/var/lib/fairway/revocations.json"
sovereign_session_max_seconds = 900
sovereign_clock_skew_seconds = 30
sovereign_break_glass_max_seconds = 300
sovereign_dual_control_commands = ["set:status", "record:review"]
```

`FAIRWAY_CUSTOMER_IDENTITY_PUBLIC_KEY` contains the base64 encoding of exactly
one 32-byte Ed25519 public key. The revocation file must be an absolute,
bounded, private regular file with no group or other permissions. Symlinks are
rejected:

```json
{
  "schema": "fairway.sovereign-revocations.v1",
  "revoked_proof_ids": [],
  "revoked_subjects": [],
  "revoked_key_ids": []
}
```

The file is checked for every request so a customer can revoke an active proof,
subject, or key without trusting a proxy header or API-token fallback. Missing,
unreadable, malformed, or permissively accessible revocation state denies
access. Key rotation is an explicit configuration change in this first profile;
one key ID is active at a time.

## Session And Break-Glass Proofs

A normal primary proof uses `purpose=session`. It contains no command, task,
dual-control, idempotency, or reason authority fields. Its role must be listed in
`allowed_roles`, and command authorization remains explicit:

| Command | Primary role |
|---|---|
| `read:api` | `viewer`, `admin` |
| `record:evidence`, `record:checkpoint` | `operator`, `coordinator`, `admin`, `adapter:<name>` |
| `set:status` | `operator`, `coordinator`, `admin` plus dual control |
| `record:review` | matching `reviewer:<domain>` or `admin` plus dual control |

A `purpose=break_glass` proof must use role `admin`, include a bounded reason,
and expire within `sovereign_break_glass_max_seconds`. It does not bypass role,
project, revocation, idempotency, self-review, or dual-control checks. It grants
no deployment, release, merge, credential, public, or live authority.

## Dual Control

Consequential commands configured in `sovereign_dual_control_commands` require
`X-Fairway-Dual-Control: <proof>`. The second proof must:

- use `purpose=dual_control` and role `authorizer`;
- come from a subject distinct from the primary actor;
- name the exact command and task;
- bind the primary proof ID and exact `Idempotency-Key`;
- bind the SHA-256 digest of Fairway's canonical decoded command payload;
- fit inside the short break-glass lifetime limit;
- pass the same signature, scope, time, and revocation checks.

Exact request replay returns the existing result through Fairway's durable
idempotency ledger. Reusing the authorizer proof with another command, task,
payload, primary session, or idempotency key fails closed. Audit rows attribute the
primary actor and a stable redacted authorizer identity; they do not persist
proofs, subjects, keys, request bodies, prompts, transcripts, tool bodies, or
credentials.

Sovereign review writes cannot override the authenticated reviewer identity.
The review role must match the requested domain, and the existing owner/claimant
self-review guard still applies. A second authorizer is an execution control,
not a substitute for independent reviewer judgment.

## Machine-Readable Readback

The API index and `GET /api/v1/status` expose
`fairway.server-authorization-policy.v1` with:

- identity mode and exact project;
- whether identity is cryptographically verified and whether fallback exists;
- allowed roles and dual-control command list;
- session and break-glass maximum lifetimes;
- whether revocation state is mandatory.

This readback contains no identities, proof IDs, key values, revocation entries,
or credentials. It supports readiness and assessor evidence; it does not state
that an identity system, deployment, or organization is certified.

## Customer Responsibilities

The customer owns issuer hardening, signer access control, key generation and
custody, trustworthy time, subject lifecycle, role assignment, proof delivery,
revocation decisions, revocation-file integrity, emergency approval policy,
rotation and recovery, host and network controls, and independent assessment.
Fairway verifies the configured proof contract and records bounded audit facts.
Those facts are inputs to an assurance package, not self-certification.

For a disposable offline exercise of positive authorization, role rejection,
revocation, key loss/substitution, and recovery using ephemeral customer-style
roots, see [Sovereign Customer Key Rehearsal](../operations/sovereign-customer-key-rehearsal.md).
The rehearsal does not issue reusable credentials or define a production key
ceremony.
