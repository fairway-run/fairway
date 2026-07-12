# Authoring assurance profiles

Custom profiles map Fairway's bounded engineering-fact classes to selected
assessment objectives. A profile is declarative data. It cannot contain
commands, scripts, expressions, database queries, provider prompts, or workflow
actions.

## Start from the fixture

Copy `examples/assurance-profiles/fixtures/valid-custom-v1.yaml` into a
project-owned, reviewed profile directory. Give it an immutable profile ID and
version, then replace the example framework metadata with an exact official
HTTPS source and published version.

```bash
fairway assurance profile validate path/to/profile.yaml
fairway assurance profile diff --from path/to/previous.yaml --to path/to/profile.yaml
```

The invalid fixtures demonstrate fail-closed unknown-field and authority-guard
behavior. They are test inputs and must not be placed in the top-level starter
directory because `assurance profiles list` validates every profile there.

## Authoring rules

- Map only controls or assessment objectives the profile actually describes.
  State that a starter or subset is incomplete.
- Use the exact framework publication/version and an official source URL.
- Keep applicability explicit. A tag or task kind is selection, not proof that
  a framework applies.
- Assign `product`, `customer`, `shared`, or `external_assessor`
  responsibility. Do not convert customer or assessor obligations into product
  evidence merely to remove a gap.
- Require only Fairway's fixed metadata-only evidence classes and bounded
  results. Profiles cannot request artifact bodies, prompts, transcripts,
  credentials, or provider-private data.
- Select only positive results such as `pass`, `verified`, or `approve` when a
  requirement contributes to readiness. `partial`, `blocked`, `fail`, and
  `changes` facts remain visible to the assessor but must not roll up to
  `satisfied_by_recorded_evidence` in a starter mapping.
- Use freshness windows only where evidence must be current.
- Preserve every required prohibited claim and action.
- Treat exceptions as unresolved assessment inputs, not control satisfaction.

## Review packet

Include the authoritative source and version, included and omitted controls,
applicability, evidence rationale, responsibility boundaries, profile diff,
readiness fixtures, and confirmation that the profile adds no certification,
compliance, authorization, risk acceptance, approval, merge, deploy, release,
credential, public-exposure, or live-operation authority.

Use a qualified legal, security, compliance, or certification professional to
decide whether a mapping is suitable for a real assessment. Fairway provides a
deterministic evidence packet, not that professional judgment.
