# Sovereign deployment threat model

## Status and scope

This document defines the `v1` threat and responsibility baseline for Fairway's
Sovereign Deployment Ready reference work. It is an engineering security input,
not a certification, legal conclusion, system authorization, Security Target,
or claim that the current Fairway release implements every control below.

The baseline covers three bounded deployment shapes:

| Profile | Boundary | Expected connectivity |
|---|---|---|
| `sovereign-offline` | One customer-controlled host or isolated local network; local SQLite remains the execution store. | No outbound DNS, HTTP, telemetry, update, remote provider, remote identity, or remote asset dependency during install and operation. Explicit loopback and local sockets only. |
| `sovereign-connected` | Customer-controlled network with explicit internal services and allowlisted update/evidence transfer paths. | Deny by default. Every identity, notifier, provider, registry, package, and update dependency is named and customer approved. |
| `restricted-shared` | Shared-team Fairway service and store inside a customer-controlled restricted-data boundary. | Authenticated, encrypted internal paths only. Shared writes require verified identity, scoped authorization, audit, conflict control, and separately reviewed deployment support. |

The evaluated scope must name the Fairway version, source revision, binary and
package digests, profile ID/version/digest, configuration identity, store mode,
host or orchestration baseline, enabled adapters, trust roots, data boundary,
and evidence-package digest. A result for one scope does not transfer to another
version, configuration, deployment, customer, or jurisdiction.

## Assets

- Fairway binary, release bundle, checksums, signatures, SBOM, VEX, and build
  provenance;
- project configuration, assurance profiles, policy versions, and deployment
  baseline;
- Fairway task, decision, evidence-reference, review, checkpoint, wait, session,
  notification, audit, and state-history records;
- assurance package manifests, normalized evidence indexes, readiness reports,
  exception links, and external-assessment references;
- customer identity, authorization, network, key, backup, restore, retention,
  and trusted-time configuration;
- offline trust roots, update media, rollback targets, security advisories, and
  disposal records.

Fairway does not make raw prompts, private transcripts, provider tool bodies,
credentials, secrets, or arbitrary artifact contents part of the assurance
record. External source-control, CI, package, identity, signing, backup, and
deployment systems remain authoritative for facts they create.

## Trust boundaries

| Boundary | Trusted only for | Must not imply |
|---|---|---|
| Human operator | Explicit local or external action under a named authorization | That Fairway authorized deploy, release, credential use, or live mutation |
| Reviewer | Attributable verdict in a configured review domain | Product certification, customer risk acceptance, or self-review |
| Dashboard | Read-oriented display of Fairway state | Write, send, approval, merge, deploy, release, or live authority |
| Provider or utility adapter | Bounded lifecycle, notification, or evidence metadata | Provenance, approval, unrestricted command execution, or secret custody |
| Identity proxy or API token | Identity only after configured cryptographic or origin proof | Implicit role, project, review, or consequential-action authority |
| Source/build/release systems | Source revision, build, artifact, and release facts they produce | Trustworthiness beyond verified provenance and the evaluated builder boundary |
| Customer platform | Network, host, identity, encryption, key custody, retention, and backup controls | Fairway product implementation or external certification |
| External assessor or certification body | Findings or certificates for its named scheme and exact evaluated scope | Broader versions, configurations, customers, jurisdictions, or unevaluated controls |

## Assumptions

- The customer can enforce the selected host, network, identity, key, storage,
  backup, time, and physical-security boundary.
- Offline installation begins with documented trust roots delivered separately
  from the bundle being verified.
- Fairway can detect and report evidence gaps but cannot determine legal
  applicability or accept customer risk.
- A compromised operating system, hypervisor, administrator, build platform, or
  signing root can invalidate evidence unless an independent control detects it.
- Availability and denial-of-service protection depend partly on the customer
  platform and are not established by local evidence integrity alone.

## Threats and required controls

| Threat | Attack or failure | Required baseline controls | Primary responsibility | Residual decision owner |
|---|---|---|---|---|
| `T01 malicious-artifact` | A binary, plugin, profile, update, evidence package, or removable-media bundle is substituted or contains malicious content. | `SDR-PROVENANCE`, `SDR-ADAPTER-CONTAINMENT`, `SDR-VULNERABILITY-AND-SUPPORT`, `SDR-EVIDENCE-AND-CLAIMS`; digest/signature verification before use; no automatic execution from evidence. | Product and customer | Customer security owner |
| `T02 build-compromise` | Build infrastructure emits a backdoored artifact or forged/incomplete provenance. | `SDR-PROVENANCE`; trusted builder policy; source-to-subject verification; independent release review; reproducibility claims only when demonstrated. | Product and external assessor | Product security owner |
| `T03 operator-impersonation` | An attacker acts as an operator through a stolen token, unverified proxy header, stale session, or identity fallback. | `SDR-IDENTITY-AND-AUDIT`, `SDR-CRYPTOGRAPHY-AND-KEYS`; verified identity; scoped roles; expiry/revocation; attribution. | Shared | Customer identity owner |
| `T04 reviewer-impersonation` | A writer spoofs reviewer identity, self-reviews, replays a verdict, or applies a verdict outside its domain or commit. | `SDR-IDENTITY-AND-AUDIT`, `SDR-EVIDENCE-AND-CLAIMS`; authenticated reviewer binding; review scope; anti-replay/idempotency; reviewed source identity. | Product and customer | Review-policy owner |
| `T05 insider-misuse` | An authorized user alters state, exports sensitive metadata, weakens policy, or performs a consequential action beyond need. | `SDR-IDENTITY-AND-AUDIT`, `SDR-DATA-MINIMIZATION`; least privilege; separation of duties; explicit promotion boundaries; retention and review. | Shared | Customer risk owner |
| `T06 evidence-tamper` | Records or packages are deleted, inserted, reordered, rewritten, backdated, or mixed across projects/scopes. | `SDR-IDENTITY-AND-AUDIT`, `SDR-EVIDENCE-AND-CLAIMS`; append-only facts where possible; audit binding; fixed manifests; source-project and task checks; trusted time boundary. | Product and customer | Evidence owner |
| `T07 rollback` | An old binary, policy, DB, trust root, or evidence snapshot is restored and presented as current. | `SDR-PROVENANCE`, `SDR-IDENTITY-AND-AUDIT`, `SDR-RECOVERY`, `SDR-VULNERABILITY-AND-SUPPORT`; version/readback; externally anchored audit where required; approved rollback target; post-restore verification. | Shared | Customer operations owner |
| `T08 backup-loss` | Backups are absent, unreadable, exposed, use lost keys, or cannot restore the expected state. | `SDR-CRYPTOGRAPHY-AND-KEYS`, `SDR-RECOVERY`; encrypted/versioned backups; restore drills; key recovery; retention; disposal proof. | Customer | Customer operations owner |
| `T09 adapter-escape` | A provider, notifier, utility, tracker, browser asset, or command adapter bypasses the network/data boundary or gains hidden authority. | `SDR-NETWORK-ISOLATION`, `SDR-ADAPTER-CONTAINMENT`, `SDR-DATA-MINIMIZATION`; disabled-by-default remote adapters; allowlists; capability preflight; metadata-only records; no dashboard send authority. | Product and customer | Customer platform owner |
| `T10 data-boundary-failure` | Secrets, private paths, prompts, transcripts, raw tool bodies, evidence contents, or restricted metadata leave the approved boundary. | `SDR-NETWORK-ISOLATION`, `SDR-IDENTITY-AND-AUDIT`, `SDR-DATA-MINIMIZATION`, `SDR-EVIDENCE-AND-CLAIMS`; data inventory; redaction; local roots; export allowlist; egress deny; negative tests. | Shared | Customer data owner |
| `T11 key-compromise` | A signing, encryption, token, or backup key is exposed, substituted, unavailable, or not customer controlled. | `SDR-CRYPTOGRAPHY-AND-KEYS`; external secret store; key identity and purpose; rotation/revocation/recovery; pinned verification roots. | Customer and product | Customer key owner |
| `T12 network-escape` | DNS, redirects, proxy environment variables, remote assets, update checks, or adapters create unexpected outbound traffic. | `SDR-NETWORK-ISOLATION`, `SDR-ADAPTER-CONTAINMENT`; deny-by-default network policy and egress-denied rehearsal. | Shared | Customer network owner |

## Control baseline

| Control | Objective | Responsibility | Minimum evidence before a bounded readiness claim | Delivery task |
|---|---|---|---|---|
| `SDR-BOUNDARY` | Bind every result to exact product, deployment, policy, customer boundary, profile, and evidence versions. | Shared | Configuration identity, profile digest, source/release identity, scope statement, customer applicability decision. | FW-341, FW-349, FW-354 |
| `SDR-NETWORK-ISOLATION` | Fail closed on undeclared DNS, HTTP, telemetry, update, identity, asset, notifier, and provider dependencies. | Shared | Capability inventory, egress-deny tests, redirect/proxy/DNS negative tests, disconnected rehearsal. | FW-342 |
| `SDR-PROVENANCE` | Verify source, builder, artifact, release, SBOM/VEX, signature, and provenance subjects offline. | Product | Signed release bundle, checksums, provenance, builder policy, SBOM, VEX, verification log. | FW-343, FW-347 |
| `SDR-IDENTITY-AND-AUDIT` | Bind shared actions and review judgments to verified actors and detect audit tampering without storing private content. | Shared | Identity proof, role policy, negative auth tests, self-review/replay tests, tamper verification, retention, revocation/expiry evidence. | FW-344, FW-346 |
| `SDR-CRYPTOGRAPHY-AND-KEYS` | Name every encryption/signing boundary, module, key owner, trust root, and recovery path. | Shared | Crypto inventory, module/version/config reference, customer key custody, rotation/recovery rehearsal, explicit FIPS non-claim unless externally supported. | FW-345 |
| `SDR-RECOVERY` | Restore the exact supported state and safely roll back software, policy, data, and trust roots. | Customer | Backup manifest, restore/readback proof, rollback target, compatibility result, key-loss procedure, cleanup proof. | FW-348 |
| `SDR-ADAPTER-CONTAINMENT` | Keep providers, notifiers, trackers, utilities, and remote assets disabled or explicitly bounded. | Product and customer | Adapter inventory, allowlist/disable proof, network and authority negative tests, notification evidence. | FW-342, FW-348 |
| `SDR-DATA-MINIMIZATION` | Keep restricted content and identifiers inside the approved boundary and out of routine Fairway records/exports. | Shared | Data inventory, redaction tests, artifact-root policy, export review, egress-deny evidence, retention/disposal policy. | FW-346, FW-349 |
| `SDR-VULNERABILITY-AND-SUPPORT` | Deliver advisories, affected versions, mitigations, fixes, VEX, and offline updates through a supported lifecycle. | Product | Vulnerability policy, advisory fixtures, LTS/EOL policy, synthetic offline patch rehearsal. | FW-353 |
| `SDR-OFFLINE-REHEARSAL` | Prove bounded install, operation, verification, recovery, rollback, and cleanup with outbound network denied. | Shared | Non-authoring disconnected rehearsal packet, exact timing/readback, failure and cleanup evidence. | FW-350 |
| `SDR-INDEPENDENT-ASSESSMENT` | Obtain a qualified security assessment for the exact scope. | External assessor and customer | Assessment report reference, findings/retest status, assessor identity, exact criteria, and explicit limitations. | FW-352 |
| `SDR-EVIDENCE-AND-CLAIMS` | Produce deterministic packages and reject claims beyond recorded evidence and external authority. | Product | Signed assurance package, offline verification, responsibility/gap matrix, claim manifest, public wording review. | FW-349, FW-354, FW-355 through FW-361 |

No control is satisfied merely because it appears in this table. Readiness is
computed from the selected profile and current evidence; customer and external
assessment responsibilities remain gaps until their named owners provide proof.

## Framework and evaluation-input crosswalk

This crosswalk is navigation for assessors. It is incomplete and does not claim
equivalence, control inheritance, conformity, an Evaluation Assurance Level, or
that one framework satisfies another.

| Baseline area | NIST SSDF 1.1 | NIST SP 800-171 Rev. 3 families | EU CRA 2024/2847 technical-documentation input | Product-evaluation input |
|---|---|---|---|---|
| Scope, policy, roles | PO.1, PO.2 | Planning; Access Control; Identification and Authentication | Product description, intended purpose, versions, cybersecurity risk assessment | Target of Evaluation boundary; assumptions; organizational security policies; security objectives |
| Secure environment and adapters | PO.5, PS.1 | Configuration Management; System and Communications Protection; System and Services Acquisition | Architecture/design and solutions used for applicable essential requirements | Operational environment; attack surface; functional requirements and configuration assumptions |
| Source, build, release integrity | PS.2, PS.3 | Supply Chain Risk Management; System and Services Acquisition; System and Information Integrity | Design/development information, third-party components, tests, software bill of materials when applicable | Development, delivery, configuration-management, and life-cycle evidence |
| Vulnerability and support | RV.1, RV.2, RV.3 | Risk Assessment; Incident Response; System and Information Integrity | Vulnerability-handling processes, support period, test reports | Flaw-remediation and vulnerability-analysis evidence |
| Audit, evidence, and assessment | PO.4, PW.7, PW.8 | Audit and Accountability; Security Assessment and Monitoring | Risk assessment, test reports, applied specifications, technical documentation updates | Security problem definition, Security Target inputs, assurance evidence, evaluator findings |
| Recovery and continuity | PO.5 | Media Protection; System and Information Integrity | Product maintenance/support evidence where applicable | Operational guidance, secure state, recovery assumptions, residual risks |

Authoritative references:

- [NIST SP 800-218, SSDF 1.1 final](https://csrc.nist.gov/pubs/sp/800/218/final)
- [NIST SP 800-171 Rev. 3 final](https://csrc.nist.gov/pubs/sp/800/171/r3/final)
- [NIST SP 800-53 Rev. 5](https://csrc.nist.gov/pubs/sp/800/53/r5/upd1/final)
- [Regulation (EU) 2024/2847](https://eur-lex.europa.eu/eli/reg/2024/2847/oj)
- [Common Criteria CC:2022 Release 1 publications](https://www.commoncriteriaportal.org/cc/index.cfm)

Common Criteria terms are used only to structure future evaluation inputs: an
exact Target of Evaluation, security problem definition, assumptions, threats,
organizational security policies, objectives, functional/assurance requirement
selection, and evidence. This baseline is not a Protection Profile, Security
Target, CEM evaluation, EAL result, or certificate.

## Validation and change control

The threat model and baseline are versioned together. A new threat, deployment
mode, authority path, data class, adapter type, store, identity source, crypto
module, public claim, or external framework version requires a profile diff,
threat review, control/evidence update, and independent approval. Historical
packages retain their original profile and evidence digests.
