# Fairway sovereign data and privacy inventory

## Purpose

This inventory defines Fairway's product data classes for restricted and
disconnected deployments. It is an assessment input, not a customer data
classification, privacy impact assessment, retention authorization, or legal
conclusion. The adopting organization assigns sensitivity, residency,
retention, access, legal-hold, and disposal requirements for its deployment.

## Data classes

| Class | Examples | Routine storage | Assurance-package treatment | Primary owner |
|---|---|---|---|---|
| Project and task metadata | project, task ID, title, role, status, kind, tags, timestamps | Fairway DB | bounded identifiers and state facts | Shared |
| Decision metadata | typed decision, author, status, source references, timestamps | Fairway DB | normalized decision reference; rationale text excluded | Shared |
| Review metadata | domain, verdict, reviewer identity, commit, timestamps | Fairway DB | normalized current and superseded references; reason excluded | Shared |
| Evidence metadata | result, artifact type, timestamp, bounded reference | Fairway DB | normalized reference and result; artifact path and body excluded | Shared |
| Runtime coordination | session, checkpoint, wait, handoff, notification, acknowledgement | Fairway DB | included only when selected by a profile and normalized | Product and customer |
| Audit metadata | actor, action, object, result, sequence, time, previous hash | Fairway DB and signed exports | signed bounded audit records and checkpoint identity | Shared |
| Configuration and policy identity | config digest, profile ID/version, deployment mode, selected policy | Local config and package | exact identity and reviewed evidence reference | Shared |
| Supply-chain identity | source commit, builder, binary digest, SBOM/VEX/license/provenance references | Release artifacts and Fairway references | exact digests and normalized references | Product |
| Customer identity material | identity-provider subject, role, token/key/certificate metadata | Customer identity system; bounded subject may be recorded | proof reference only; no credential value | Customer |
| Credentials and private keys | API tokens, passwords, signing/encryption keys, session secrets | Customer secret store or process environment | prohibited | Customer |
| Artifact content | reports, screenshots, logs, videos, test output, legal material | Customer-approved local artifact roots | path and body prohibited; digest/reference only when supported | Customer |
| Provider content | prompts, transcripts, raw tool bodies, generated-content dumps, provider-private usage DBs | Outside routine Fairway storage | prohibited | Provider and customer |
| Private legal or assessor material | advice, exploit detail, draft findings, restricted certificates | Customer/assessor repository | bounded reference and disposition only | Customer or assessor |

## Data flows

1. CLI, reviewed server API, coordinator, or adapter receives a bounded command
   and verified actor context.
2. Fairway validates command scope, policy, idempotency, and privacy shape.
3. The store records structured metadata in the selected local or reviewed
   shared store.
4. Read models project the same records to CLI and read-only dashboard views.
5. Evidence mapping normalizes selected facts without loading artifact bodies.
6. Package export writes deterministic human and machine views, signs the fixed
   manifest when configured, and remains locally controlled.
7. Offline verification reads only the package and caller-supplied trust root.

Remote providers, notifiers, trackers, identity endpoints, assets, and update
sources are disabled in `sovereign-offline` mode. Customer-enforced DNS and
egress denial remains the authoritative network boundary.

## Required customer decisions

Before deployment, the customer records:

- project and task sensitivity and whether titles/tags may identify restricted
  programs;
- approved data paths, file ownership, permissions, encryption, backup,
  replication, residency, retention, legal hold, and disposal;
- identity-subject minimization and whether pseudonymous identifiers are
  required;
- which local artifacts may be referenced and who may open them;
- audit export destination, trusted time, key custody, WORM/SIEM behavior, and
  checkpoint retention;
- package/media classification, transfer, import, verification, storage, and
  destruction procedures;
- incident, breach, discovery, subject-rights, records-management, and legal
  review ownership where applicable.

## Product privacy controls

- strict schemas and bounded vocabularies reject arbitrary content in profiles,
  recipes, adapters, packages, and structured evidence paths;
- assurance packages omit commands, notes, reasons, rationale, artifact paths,
  artifact bodies, prompts, transcripts, raw tool bodies, credentials, and
  secrets;
- safe evidence viewing is local-root constrained, read-only, escaped,
  redacted before truncation, and documented as defense in depth rather than a
  publication sanitizer;
- metrics use bounded labels and avoid task content, credentials, and provider
  bodies;
- claim and package validation errors identify the field or line without
  echoing private input;
- dashboard projections do not gain send, approval, merge, deploy, release,
  credential, public-exposure, or live-operation authority.

## Residual risks

Metadata can still reveal project names, work timing, role assignments,
security posture, and operational structure. A digest can confirm guesses about
low-entropy content. Local administrators and backup operators can access the
store. Redaction cannot make arbitrary sensitive input safe for publication.
Customers must therefore minimize source data, restrict access, protect copies,
and independently validate exported material before transfer.

