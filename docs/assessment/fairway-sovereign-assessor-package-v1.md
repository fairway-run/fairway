# Fairway sovereign assessor package v1

## Purpose

This packet is the canonical index for preparing a Fairway product and
deployment assessment. It combines stable human references with a generated,
signed assurance package. It reduces collection and traceability work; it does
not select legal obligations, perform assessor sampling, accept risk, authorize
a system, or issue certification.

## Exact generated identity

Generate a package with the exact product/source version and fixed review
clock:

```bash
fairway assurance package export \
  --profile examples/assurance-profiles/fairway-sovereign-nist-800-53-r5-assessor-input-v1-starter.yaml \
  --product-version <version-or-source-identity> \
  --scope release \
  --scope-id <release-or-assessment-id> \
  --task <task-id> \
  --at <RFC3339-review-clock> \
  --signing-key-env FAIRWAY_ASSURANCE_SIGNING_KEY \
  --out <new-package-directory>
```

The v2 manifest and scope record `product_version` and `review_date`. Human
`controls.md`/`controls.csv` and `oscal-component-definition.json` carry the
same control status, responsibility, profile, product version, review date,
assessment objectives, and evidence references. Verification recomputes those
views from the signed package state.

## Canonical input inventory

| Required input | Canonical reference | Package or review use |
|---|---|---|
| Architecture and data flows | [Architecture](../architecture.md), [Sovereign Data Inventory](../security/sovereign-data-inventory.md) | Product/store/read-model boundaries and classified flow review |
| Threat model | [Sovereign Threat Model](../security/sovereign-threat-model.md) | Threats, controls, responsibilities, and residual owners |
| Security target draft | [Sovereign Security Target Input](../security/sovereign-security-target-draft.md) | Candidate TOE, environment, assets, assumptions, objectives, and incomplete evaluation inputs |
| Control responsibility matrix | this document, generated `controls.*`, `responsibilities.json`, and `gaps.json` | Product/customer/shared/external-assessor split |
| Secure development lifecycle | [Coding Standards](../governance/coding-standards.md), [Testing](../governance/testing.md), [Review Guards](../governance/review-guards.md), [Release Governance](../governance/release.md) | Source, tests, independent judgment, promotion, and release controls |
| Vulnerability process | repository security policy and [Release Assurance Bundle](../security/release-assurance-bundle.md) | Intake, severity, remediation, VEX, advisory, patch, and support evidence |
| Privacy and data inventory | [Sovereign Data Inventory](../security/sovereign-data-inventory.md) | Data classes, exclusions, flows, ownership, and residual privacy risk |
| Configuration baselines | [Sovereign Deployment Baselines](../operations/sovereign-deployment-baselines.md), [Sovereign Offline Bundle](../operations/sovereign-offline-bundle.md) | Versioned configuration, deviation, install, update, rollback, and recovery inputs |
| Test strategy | [Testing](../governance/testing.md), profile assessment objectives, and generated evidence index | Positive, negative, disconnected, tamper, recovery, and boundary test traceability |
| Known gaps | generated `gaps.json`, readiness report, this packet's limitations | Missing, stale, conflicting, customer, exception, and external-assessment actions |
| Evidence index | generated `evidence-index.json`, decisions/reviews/provenances/exceptions views | Metadata-only traceability without artifact bodies or private rationale |

## Bounded NIST control matrix

The NIST starter is a selected assessor-input subset, not a NIST baseline or
assessment result. The official control and assessment procedure publications
remain authoritative. Each generated row supplies an evidence-preparation
objective, not a verbatim NIST SP 800-53A determination.

Authoritative inputs:

- [NIST SP 800-53 Rev. 5 Update 1](https://csrc.nist.gov/pubs/sp/800/53/r5/upd1/final)
- [NIST SP 800-53A Rev. 5](https://csrc.nist.gov/pubs/sp/800/53/a/r5/final)
- [NIST OSCAL 1.1.3 component-definition model](https://pages.nist.gov/OSCAL-Reference/models/v1.1.3/component-definition/)
- [Regulation (EU) 2024/2847](https://eur-lex.europa.eu/eli/reg/2024/2847/oj)
- [Common Criteria CC:2022 Release 1](https://www.commoncriteriaportal.org/cc/index.cfm)

| Area | Selected controls | Fairway evidence focus | Responsibility boundary |
|---|---|---|---|
| Account and access | AC-2, AC-3 | verified actors, scoped roles, deny tests, separation of duty, audit | shared; customer identity remains customer evidence |
| Audit | AU-2, AU-9 | event inventory, tamper checks, signed export, retention, rollback continuity | shared; customer retention target and keys remain external |
| Configuration | CM-2, CM-3, CM-8 | baseline, deviations, reviewed changes, component inventory | shared |
| Recovery | CP-9, CP-10 | backup policy, restore, upgrade, rollback, key loss, disposal | customer/shared |
| Identity | IA-2, IA-5 | identity source, authentication, token/key/certificate lifecycle | customer/shared |
| Engineering assurance | RA-5, SA-11, SI-2 | vulnerability, tests, CI, review, remediation, advisory, patch | product with external validation where selected |
| Integrity and supply chain | SI-7, SR-4 | source, builder, binary, SBOM, VEX, signature, provenance | product |
| Boundary and cryptography | SC-7, SC-13 | network deny/allow, adapters, module/key/trust-root inventory and nonclaims | shared |

Generated statuses have deliberately narrow meanings:

- `satisfied_by_recorded_evidence` means only that the profile's explicit
  metadata requirements matched current recorded facts;
- `customer_responsibility` remains open until the customer supplies scoped
  evidence;
- `external_assessment_required` remains open until an authorized independent
  assessor supplies a reference;
- missing, partial, stale, conflicting, and exception states remain visible and
  cannot be promoted by wording.

## OSCAL boundary

Package v2 includes an OSCAL 1.1.3 component definition. It uses the selected
profile framework source, one software component, one control implementation,
and one implemented requirement per profile control. Fairway-specific
properties use `https://docs.fairway.run/ns/assurance` and carry status,
responsibility, profile/product/review identity, assessor boundary, and
assessment objectives. Links point to the package evidence index by bounded
reference text.

The component definition is an implementation-layer starting point for a
system owner or assessor. It is not an SSP, assessment plan, assessment result,
POA&M, Security Target, technical file, or certificate. The recipient validates
and transforms it with the OSCAL version and toolchain selected for the actual
assessment.

## EU and product-evaluation inputs

EU CRA preparation may reuse the product description, intended purpose,
architecture, risk/threat analysis, secure-development process, component
inventory, vulnerability handling, support period, test reports, SBOM/VEX,
release provenance, and technical-documentation change history. Applicability,
essential requirements, conformity-assessment route, declaration, marking,
reporting, and market obligations require qualified review under FW-351.

EUCC or Common Criteria preparation may reuse the candidate TOE boundary,
security problem, assumptions, organizational policies, objectives,
configuration management, delivery, guidance, testing, vulnerability, and
flaw-remediation inputs. A sponsor/evaluator must select the scheme, Security
Target, SFRs, SARs, EAL or package, CEM work units, evaluation facility, and
claim scope. Fairway does not infer one framework from another.

## Documentation claim guard

Run the read-only guard over public and procurement-facing Markdown before
review:

```bash
fairway assurance claims validate \
  --path docs/security/sovereign-deployment-ready.md \
  --path docs/assessment/fairway-sovereign-assessor-package-v1.md
```

The guard fails closed on positive ISO, SOC, CUI, FIPS, FedRAMP, EU CRA, EUCC,
Common Criteria, EAL, national-cloud, sovereign-cloud, or generic Fairway
certification/compliance/authorization assertions unless the line is explicitly
bounded as a nonclaim or external requirement. It is defense in depth and does
not replace legal, certification-body, assessor, or public-wording review.

## Current gaps and closeout

FW-350 must supply a non-authoring disconnected rehearsal. FW-351 must supply
qualified jurisdiction and export applicability. FW-352 must supply the
independent security assessment and retest disposition. FW-353 must supply the
restricted advisory/LTS channel. FW-354 may prepare a version-scoped readiness
claim only after those dependencies are complete; publish remains a separate
reviewed release action.
