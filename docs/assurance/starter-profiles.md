# Starter assurance profiles

Fairway ships versioned starter profiles that organize existing engineering
facts for assessment preparation. They are deliberately small mappings, not
complete control catalogs, legal interpretations, certification schemes, or
claims that a product satisfies the named framework.

The directory contains five named starter mappings plus the project-neutral
`example-evidence-support` schema example retained at its established path for
compatibility, so `profiles list` reports six entries.

```bash
fairway assurance profiles list --dir examples/assurance-profiles
fairway assurance profile validate examples/assurance-profiles/nist-ssdf-1.1-starter.yaml
```

## Catalog

| Profile | Pinned source | Bounded coverage | Explicit limit |
|---|---|---|---|
| `nist-ssdf-1.1-starter` | [NIST SP 800-218, SSDF 1.1 final (February 2022)](https://csrc.nist.gov/pubs/sp/800/218/final) | Four selected practices covering requirements, accountability, provenance, and vulnerability evidence. | NIST's SSDF 1.2 is still a draft and is not silently included. This is not the full SSDF catalog. |
| `nist-sp-800-171-rev3-starter` | [NIST SP 800-171 Rev. 3 final (May 2024)](https://csrc.nist.gov/pubs/sp/800/171/r3/final) | Three selected requirements plus an explicit customer CUI-scope decision. | The customer or contracting authority determines CUI applicability, system boundary, organization-defined parameters, and assessment rigor. Use the [SP 800-171A Rev. 3](https://csrc.nist.gov/pubs/sp/800/171/a/r3/final) procedures with a qualified assessor. |
| `eu-cra-2024-2847-technical-documentation-starter` | [Regulation (EU) 2024/2847](https://eur-lex.europa.eu/eli/reg/2024/2847/oj) | Selected manufacturer, vulnerability-handling, and technical-documentation evidence topics. | A qualified party determines legal scope, role, classification, applicable dates, conformity route, and required documentation. This is not legal advice. |
| `slsa-1.2-supply-chain-starter` | [SLSA specification 1.2](https://slsa.dev/spec/v1.2/) | Selected source and Build L1-L3 evidence concepts. | The profile does not assign, attest, or certify a SLSA level. Hardened-builder assessment remains external. |
| `fairway-sovereign-deployment-ready-v1-starter` | Fairway reference model `v1-starter` | Deployment boundary, provenance, disconnected rehearsal, customer identity/audit, recovery, and external assessment. | This is a draft reference profile. It does not establish a sovereign, restricted-technology, jurisdictional, certification, or authorization outcome. |

The profiles separate four responsibility classes: `product`, `customer`,
`shared`, and `external_assessor`. Readiness output preserves customer and
external-assessment rows as gaps. It never upgrades those rows from product
evidence alone.

## Using a starter

1. Pin the profile file, ID, version, framework version, and file digest in the
   assessment plan.
2. Review applicability and remove no customer or assessor boundary merely to
   make a report green.
3. Create a new profile version for any mapping change and inspect it with
   `assurance profile diff`.
4. Run readiness against an explicit project, task set, or release scope.
5. Export and optionally sign an assessor package.
6. Verify the package offline and hand it to the authorized assessor or
   customer decision owner.

See [authoring custom profiles](authoring.md) and the
[compatibility policy](compatibility.md).

The [2026-07-12 package pilot](../assessment/fairway-assurance-package-pilot-2026-07-12.md)
records measured Fairway sovereign and bounded AI Cloud results, including
machine time, gaps, privacy checks, manual assessor work, and promote/repeat
recommendations without an external assurance claim.
