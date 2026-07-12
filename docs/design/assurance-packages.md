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

- `manifest.json`: profile/scope identity and SHA-256 digests for package files;
- `signature.json`: optional Ed25519 signature over the manifest digest;
- `profile.json`: the exact canonical validated profile used for the export;
- `readiness.json`: the complete bounded scope-level readiness/gap report;
- `scope.json`: profile, framework, scope, tasks, clock, and authority boundary;
- `controls.json`, `controls.md`, and `controls.csv`: equivalent control views;
- `evidence-index.json`: normalized metadata references only;
- `decisions.json`, `reviews.json`, `provenances.json`, and `exceptions.json`:
  grouped Fairway references;
- `responsibilities.json`: customer/shared/external-assessment controls;
- `gaps.json`: bounded readiness gaps and next evidence actions;
- `oscal-control-map.json`: an explicit bridge boundary, not an OSCAL document;
- `VERIFY.md`: offline verification instructions.

In the FW-358 exporter slice, `VERIFY.md` gives manual digest, scope, profile,
key-fingerprint, and Ed25519 verification steps and does not name a Fairway
verifier that has not shipped. FW-359 owns the offline verifier command and
will replace that temporary manual boundary once implemented.

No command text, notes, review reasons, decision rationale, artifact paths,
artifact bodies, prompts, transcripts, raw tool bodies, credentials, or secrets
are exported. The OSCAL control map must be transformed and validated by an
authoritative OSCAL toolchain before assessor use.

## Determinism and claims

The same profile, Fairway state, selected scope, and `--at` clock produce the
same bytes. `created_at` is explicit versioned package metadata and therefore
changes only when the caller changes `--at`.

Export fails closed for invalid profile/package schemas, unsafe output paths,
existing output directories, malformed signing keys, and profile text that
asserts certified, compliant, authorized, or FIPS-validated status. Static
package language names the evidence-only authority boundary. An external
assessor remains responsible for any certification conclusion.
