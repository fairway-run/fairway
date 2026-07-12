# Assurance profile compatibility

Assurance profiles are pinned assessment inputs. Fairway never silently updates
a consumer from one profile file or version to another.

## Version rules

- A profile ID identifies one mapping lineage and must not be repurposed.
- Published content under one profile version is immutable. Any content change
  requires a new version.
- Framework identity, source, or version changes are breaking.
- Removing a scope, guard, control, evidence requirement, responsibility, or
  external-assessment boundary is breaking.
- Changing a control objective, responsibility, assessment objective,
  freshness rule, accepted result, or evidence contract is breaking.
- Adding a control, supported scope, prohibited claim, or prohibited action is
  additive, but still requires a new profile version and review.
- Reusing a version for any content change is breaking.

`fairway assurance profile diff` emits `unchanged`, `metadata_only`,
`additive`, or `breaking`, plus stable changed paths. `compatible=true` means
the structural change is additive or metadata-only; it does not approve the
mapping, prove framework equivalence, or authorize automatic adoption.

```bash
fairway assurance profile diff \
  --from profiles/customer-v1.yaml \
  --to profiles/customer-v2.yaml \
  --format json
```

Consumers should pin the profile ID, profile version, framework version, and
file digest used for each readiness report or package. Historical packages
remain verifiable with their embedded profile. Unknown future schemas fail
closed and require explicit migration and compatibility review.
