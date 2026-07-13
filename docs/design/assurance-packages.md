# Assurance packages

## Purpose

Fairway assurance packages organize recorded engineering facts for an assessor.
They reduce evidence-preparation effort without claiming that Fairway performs
an audit, certifies a product, declares compliance, accepts risk, or authorizes
an operation.

Export a package with a fixed evaluation/creation clock:

```bash
fairway assurance package export \
  --profile examples/assurance-profiles/example-evidence-support.yaml \
  --product-version 0.1.13 \
  --scope task_set \
  --scope-id assurance-core \
  --task FW-355 \
  --task FW-356 \
  --at 2026-07-12T12:00:00Z \
  --out /tmp/fairway-assurance-package
```

To sign the package, provide a base64 Ed25519 seed or private key through an
environment variable and name only the variable on the command line:

```bash
fairway assurance package export ... \
  --signing-key-env FAIRWAY_ASSURANCE_SIGNING_KEY
```

The key value is never placed in argv, written to the package, or included in
an error. Signing proves possession of the selected key over the package
manifest; the manifest records the public-key SHA-256 identity for trust
pinning during offline verification. Signing does not prove control sufficiency
or certification.

## Fixed contents

The output directory must not already exist. Fairway writes files with bounded
names and does not copy artifact content:

- `manifest.json`: product/profile/scope identity, review date, and SHA-256 digests for package files;
- `signature.json`: optional Ed25519 signature over the manifest digest;
- `profile.json`: the exact canonical validated profile used for the export;
- `readiness.json`: the complete bounded scope-level readiness/gap report;
- `scope.json`: product, profile, framework, producing project, scope, tasks, clock, review date, and authority boundary;
- `controls.json`, `controls.md`, and `controls.csv`: equivalent control views;
- `evidence-index.json`: normalized metadata references only;
- `decisions.json`, `reviews.json`, `provenances.json`, and `exceptions.json`:
  grouped Fairway references;
- `responsibilities.json`: customer/shared/external-assessment controls;
- `gaps.json`: bounded readiness gaps and next evidence actions;
- `oscal-control-map.json`: an explicit bridge boundary, not an OSCAL document;
- `oscal-component-definition.json`: a deterministic OSCAL 1.1.3
  implementation-layer component definition carrying control status,
  responsibility, profile/product/review identity, assessment objectives, and
  evidence-index links;
- `VERIFY.md`: offline verification instructions.

`VERIFY.md` names the offline verifier delivered by FW-359:

```bash
fairway assurance package verify --dir /tmp/fairway-assurance-package
fairway assurance package verify --dir /tmp/fairway-assurance-package \
  --trusted-public-key-env FAIRWAY_ASSURANCE_PUBLIC_KEY
```

The trust pin is a base64 Ed25519 public key read from the named environment
variable. It is never accepted directly in argv. A signed package without the
expected key is reported as `verified_unpinned` and fails overall verification;
the self-contained public key proves signature consistency, not signer trust.

Verification distinguishes package integrity, recorded-evidence sufficiency,
signature trust, and external certification. A cryptographically intact but
unpinned or unexpected key does not become trusted. Missing, stale,
conflicting, customer-owned, exception, or external-assessment-required proof
fails overall verification without being mislabeled as file tampering.

No command text, notes, review reasons, decision rationale, artifact paths,
artifact bodies, prompts, transcripts, raw tool bodies, credentials, or secrets
are exported. The OSCAL component definition must still be validated and, when
needed, transformed by the authoritative OSCAL version and toolchain selected
for the assessment. It is not an SSP, assessment plan, assessment result,
POA&M, Security Target, or certificate.

## Determinism and claims

The same profile, Fairway state, selected scope, and `--at` clock produce the
same bytes. `created_at` is explicit versioned package metadata and therefore
changes only when the caller changes `--at`.

Package v2 requires an identifier-safe exact product/source version. The
review date is the UTC date of the caller-supplied creation/evaluation clock.
Newer binaries continue to verify fixed-contract v1 packages; v1 packages do
not acquire v2 product metadata or OSCAL content retroactively.

All selected task maps and normalized facts must carry the same non-empty
Fairway project identity. Export and verification reject cross-project mixing;
multi-project evidence must be packaged as separate assessment scopes.

Export fails closed for invalid profile/package schemas, unsafe output paths,
existing output directories, malformed signing keys, and profile text that
asserts certified, compliant, authorized, or FIPS-validated status. Static
package language names the evidence-only authority boundary. An external
assessor remains responsible for any certification conclusion.

Run `fairway assurance claims validate --path <document>...` over release,
public, and procurement-facing Markdown. It is a fail-closed wording guard for
common unsupported framework and regulatory assertions, not legal or assessor
review.

The guard accepts only direct negation of each claim-state occurrence or an
explicit statement that the claim wording is prohibited. Headings and labels
such as `draft`, `example`, `input only`, conditional wording, or a general
external-review caveat do not exempt a positive assertion. Each clause is
evaluated independently so a positive claim cannot borrow a negation from an
earlier clause. The guard is defense in depth; it does not replace legal or
qualified assessor review.
