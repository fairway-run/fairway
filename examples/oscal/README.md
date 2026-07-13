# Fairway OSCAL assessor input

Fairway assurance package v2 exports
`oscal-component-definition.json` from the same deterministic profile,
readiness, product version, review date, and evidence references used by the
human control views.

Use a NIST-oriented starter when NIST control identifiers are required:

```bash
fairway assurance package export \
  --profile examples/assurance-profiles/fairway-sovereign-nist-800-53-r5-assessor-input-v1-starter.yaml \
  --product-version <exact-version> \
  --scope release \
  --scope-id <assessment-id> \
  --task <task-id> \
  --at <RFC3339-review-clock> \
  --out <new-directory>
```

The export targets OSCAL component-definition model 1.1.3 and uses the custom
property namespace `https://docs.fairway.run/ns/assurance`. Validate the file
against the official NIST OSCAL schema and the recipient's selected toolchain
before assessor import. Fairway package verification proves deterministic
agreement with Fairway state; it does not perform certification, determine
control applicability, or produce an assessment result.

