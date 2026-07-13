# Fairway sovereign security target input

## Status and authority

This document is a versioned input for a future product evaluation. It is not a
Common Criteria Security Target, Protection Profile, EUCC application,
conformity assessment, Evaluation Assurance Level claim, certificate, or
authorization. A qualified sponsor and evaluation facility must select the
scheme, exact Target of Evaluation, Security Functional Requirements,
Security Assurance Requirements, evaluation method, and claim wording.

## Product identification

- product: Fairway standalone Go binary and its separately delivered static
  dashboard assets, documentation, assurance profiles, and offline verifiers;
- version: supplied by each generated assurance package as
  `product_version`, with source, binary, and package identities supplied by
  provenance evidence;
- reference modes: `sovereign-offline`, `sovereign-connected`, and
  `restricted-shared`, evaluated separately;
- reference profile: a named immutable assurance profile and its digest;
- review date: supplied by the package `review_date` and bounded to that
  package's evidence clock.

## Candidate Target of Evaluation boundary

The candidate software boundary contains Fairway command parsing, local or
reviewed shared-server authorization guards, SQLite-backed engineering facts,
deterministic read models, evidence normalization, signed package generation,
offline verification, dashboard read-only projections, and the fixed adapter
interfaces enabled by the selected configuration.

The candidate boundary excludes the host operating system, hypervisor,
container runtime, database service in future shared mode, reverse proxy,
identity provider, DNS, network enforcement, customer key store, backup media,
external retention system, source-control host, CI runner, provider service,
notifier transport, and assessor tooling. Those are operational-environment
dependencies with customer or external responsibility unless an evaluation
scope explicitly includes them.

No shared-write, trusted-proxy, public-listener, provider-send, notifier,
Postgres, or remote-adapter capability is in the evaluated boundary merely
because a design or preview exists.

## Assets

- accountable task, decision, review, evidence, checkpoint, session,
  notification, handoff, wait, audit, and release metadata;
- policy, profile, configuration, source, builder, binary, package, and trust
  root identities;
- local data-store integrity and recoverability;
- customer-restricted metadata and evidence references;
- command authorization and separation-of-duty decisions;
- signed package manifests and verification results.

Artifact bodies, prompts, transcripts, raw tool bodies, credentials, private
legal advice, and provider-private data are not routine Fairway assets because
they are excluded from the normal record and package models.

## Security problem input

The threat inventory and responsibility split are canonical in
[Sovereign Threat Model](sovereign-threat-model.md). The security problem input
includes malicious artifacts, build compromise, actor and reviewer
impersonation, insider misuse, evidence tamper, rollback, backup loss, adapter
escape, data-boundary failure, key compromise, and network escape.

Operational-environment assumptions include:

- the customer enforces the declared network boundary and protects the host;
- trusted identity, time, keys, backup media, and retention targets are
  available and administered by named owners;
- the operator verifies package and binary identity before use;
- consequential approval, deployment, release, public exposure, credential,
  and live-operation decisions remain outside dashboard and generated-output
  authority;
- external assessors and certification bodies remain independent of Fairway's
  evidence-generation path.

## Organizational security policy input

The candidate policy input requires least privilege, verified actor identity,
no self-review, metadata-only routine records, customer-controlled secrets and
trust roots, deny-by-default disconnected operation, deterministic package
verification, bounded retention, explicit rollback, independent review, and
truthful scope-limited claims.

## Security objectives input

Fairway should:

1. authenticate or positively bind actors before command-scoped authority;
2. enforce explicit roles and separation of duty for consequential facts;
3. preserve deterministic engineering records and detect package or audit
   tampering within the stated checkpoint boundary;
4. fail closed on undeclared remote dependencies in sovereign-offline mode;
5. minimize stored and exported content to bounded metadata references;
6. verify exact source, builder, binary, profile, package, and trust identities;
7. expose customer, shared, and external-assessment responsibilities as gaps;
8. support bounded backup, restore, upgrade, rollback, and key-recovery proof;
9. reject generated certification, compliance, authorization, approval, and
   risk-acceptance claims;
10. preserve customer and assessor authority over deployment and evaluation.

The operational environment should protect hosts, networks, identities, keys,
time, backup/retention systems, physical media, and restricted data, and should
perform the independent assessment and authorization activities selected for
the target jurisdiction.

## Requirements and assurance input

No Common Criteria SFR or SAR set is selected here. The NIST and Fairway
assurance profiles provide traceability inputs, not an equivalence mapping.
Potential evaluation work must derive requirements from the selected scheme
and resolve dependencies, operations, assignments, refinements, rationale,
guidance, test depth, vulnerability analysis, configuration management,
delivery, and life-cycle evidence with the evaluator.

The generated OSCAL component definition is likewise an implementation-layer
input. It does not become a Security Target, System Security Plan, assessment
result, EU CRA technical file, or certificate without the corresponding
qualified process.

## Known incomplete inputs

- qualified jurisdiction and export-control review is owned by FW-351;
- disconnected non-authoring rehearsal is owned by FW-350;
- independent architecture, code, penetration, authorization-abuse,
  supply-chain, offline-escape, audit-tamper, and baseline assessment is owned
  by FW-352;
- restricted advisory and LTS operation is owned by FW-353;
- exact released claim identity and wording are owned by FW-354 and a separate
  reviewed publish task.

