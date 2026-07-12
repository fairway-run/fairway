# Fairway Sovereign Deployment Ready

## Meaning

Sovereign Deployment Ready is a future, version-scoped Fairway product label
for a release and configuration that has completed the named Fairway sovereign
profile's engineering controls, disconnected rehearsal, evidence packaging,
independent review, and known-gap disclosure.

It is not a certification mark, legal or regulatory conclusion, Authority to
Operate, CUI authorization, FIPS 140-3 validation, Common Criteria certificate,
EU CRA conformity statement, export classification, national-cloud approval,
or promise that one package fits every sovereign or restricted environment.

No released Fairway version carries this label until FW-354 completes the
machine-checkable claim manifest and a separate reviewed release task publishes
the exact version. The current
`fairway-sovereign-deployment-ready-v1-starter@v1` assurance profile is an
evidence-organizing draft, not a readiness declaration.

## Reference modes

- `sovereign-offline`: disconnected single-host or isolated-local-network
  operation with offline install, verification, update, rollback, backup, and
  documentation. Configure `[runtime] profile = "sovereign-offline"`; config
  loading then rejects active remote listeners, identity, providers, adapters,
  notifiers, rule sources, proxies, and tracker dependencies. Doctor and
  capability readiness expose the redacted dependency inventory. This product
  guard complements, but does not replace, customer-enforced host/network
  egress denial used by the disconnected rehearsal.
- `sovereign-connected`: customer-controlled connectivity with explicit
  dependencies, allowlists, identity, audit, keys, and data paths.
- `restricted-shared`: shared-team read/write service inside a restricted-data
  boundary, only after the shared runtime, identity, concurrency, storage, and
  deployment controls are separately implemented and reviewed.

Support for one mode does not imply support for the others.

## Claim taxonomy

Every product, package, dashboard, documentation, procurement, or public claim
must use one of these classifications and cite its exact scope.

| Classification | Required basis | May say | Must not imply |
|---|---|---|---|
| `implemented` | Current source and repository tests for the exact commit/version | The named capability exists in that source and has the cited tests. | It worked in the customer environment or satisfies an external control. |
| `validated` | Bounded execution or rehearsal with durable evidence for the exact version/configuration/profile | The named behavior was exercised in the stated environment on the stated date. | Universal suitability, production authorization, or independent assessment. |
| `independently_assessed` | Non-authoring qualified party, exact criteria/scope, findings, limitations, and report reference | The named party assessed the stated scope and recorded the stated outcome. | Accredited certification or broader versions/configurations. |
| `externally_certified` | Named certification body/authority, scheme, certificate, exact product and configuration, validity period, and limitations | Only the wording and scope supported by the certificate. | Fairway self-certification, jurisdiction-wide acceptance, or controls outside the certificate. |
| `customer_responsibility` | Named customer owner and required evidence/action | The customer must provide or decide the identified boundary. | Product implementation or satisfaction by Fairway evidence. |
| `out_of_scope` | Reviewed rationale, exact profile/version, and consequence | The named control or environment was not evaluated. | Not applicable to every customer or no residual risk. |

`validated` here is a Fairway engineering-evidence state, not a reference to a
validated cryptographic module. Cryptographic-module claims must name the exact
module, version, configuration, certificate, operating environment, and owner.

## Required claim identity

Every Sovereign Deployment Ready claim must name:

- claim ID and exact human wording;
- classification from the table above;
- Fairway product version and source commit;
- binary, release-bundle, profile, and evidence-package digests;
- sovereign mode and deployment/configuration identity;
- evaluated task/release scope and data boundary;
- assessment date, review date, expiry or revalidation trigger;
- product, customer, reviewer, and external-assessor responsibilities;
- current gaps, exceptions, residual risks, and unsupported features;
- external authority, scheme, certificate, validity, and limitations when the
  classification is `externally_certified`.

A claim is invalid when any required identity is absent, the referenced
package fails offline verification, the profile/version differs, required
evidence is stale or conflicting, a customer/external responsibility is hidden,
or wording exceeds its classification.

## Bounded wording

Acceptable engineering wording before external certification:

> Fairway `<version>` in configuration `<configuration>` was validated against
> profile `<profile>@<version>` using assurance package `<digest>` on `<date>`.
> The package reports recorded evidence, customer responsibilities, external
> assessment requirements, exceptions, and gaps. This is not certification or
> authorization.

Externally certified wording is allowed only when copied from or reviewed
against the named authority's certificate and exact evaluated scope.

Prohibited unsupported wording includes:

- “sovereign compliant” without an exact sovereign profile and evidence version;
- “certified,” “authorized,” or “approved” based only on Fairway records;
- “FIPS validated” without the exact externally validated module/configuration;
- “CUI authorized” or “NIST 800-171 compliant” without customer applicability,
  system boundary, assessment method, and authorized outcome;
- “EU CRA compliant,” “Common Criteria certified,” an EAL, or a national-cloud
  approval without the corresponding qualified or accredited process;
- a statement that one customer, version, configuration, or rehearsal applies
  to another.

## Readiness package

The eventual release packet must include:

1. exact product/release/configuration/profile inventory;
2. architecture, data-flow, trust-boundary, and
   [threat-model](sovereign-threat-model.md) documents;
3. control/responsibility matrix with product, customer, shared, and external
   assessment rows;
4. signed offline installation/update/rollback bundle and verifier;
5. source/build/release provenance, checksums, SBOM, VEX, licenses, and
   vulnerability disposition;
6. identity/authorization, audit, cryptography/key, network, data, backup,
   restore, retention, and disposal evidence;
7. disconnected non-authoring rehearsal and independent security-assessment
   references;
8. known gaps, exceptions, residual risks, unsupported modes, LTS/EOL, and
   offline advisory/update procedures;
9. signed Fairway assurance package and offline verification report;
10. machine-checkable claim manifest and reviewed public wording.

The package is designed to reduce assessor collection and traceability work.
It does not replace assessor sampling, testing, legal applicability decisions,
customer control evidence, risk acceptance, or an accredited certification
process.

## Responsibility boundary

Fairway owns product implementation, deterministic evidence organization,
package generation, verification tooling, release provenance, and truthful
product wording. The customer owns deployment applicability, restricted-data
classification, host/network/identity/key/backup controls, operational risk,
legal and procurement requirements, and authorization decisions. A qualified
assessor or certification body owns independent findings and certification
outcomes.

Shared API identity and command controls are specified in
[Sovereign Identity And Command Authorization](sovereign-identity-authorization.md).
The signed profile supplies cryptographic actor, scope, expiry, revocation,
separation-of-duty, and dual-control evidence; it does not make Fairway an
identity provider or certification authority.

The task sequence FW-342 through FW-354 implements, rehearses, assesses, and
packages this baseline. Until those tasks and their independent gates complete,
the documents and starter profile are design inputs only.
