# Rule Packs

Rule packs are reusable operating knowledge for Fairway-tracked work. They let
a project turn repeated architecture, security, implementation, review, CI/CD,
UAT, and operations practices into portable rules and templates.

Fairway stays product-neutral. Rule packs supply project or domain knowledge;
Fairway loads them, matches them to tasks, recommends evidence, records proof,
and shows gaps.

## Positioning

```text
Fairway = coordination control plane
Workstream profiles = project operating shape
Rule packs = reusable operating knowledge
Tasks = concrete work
Evidence = proof that rules/gates were handled
```

This is broader than a secure-coding ruleset. Project CodeGuard is a useful
security input, but GPUaaS/Fairway needs a wider model that also covers
contract-first development, platform boundaries, CI/CD, UAT, deploy evidence,
ops verification, provider sessions, worktrees, reviews, and release closeout.

## Ownership

Rule-pack ownership follows the scope of the knowledge:

```text
fairway-rules-platform
  Reusable operating rules that apply across many engineering projects.

<project-org>/fairway-rules-<project>
  Project or product-specific rules owned by that project.

external converted packs
  External guidance converted into Fairway-native rule-pack format.
```

Everything reusable across projects belongs in `fairway-rules-platform`.
Anything that depends on one project's domain, runtime, product contracts,
environment topology, or UAT model belongs in that project's own rule-pack
repository.

Fairway core does not own rule content. It owns loading, validation, matching,
evidence checks, dashboard/report surfacing, and closeout behavior.

## Core Invariants Win

Rule packs cannot override Fairway core safety invariants. Examples:

- task state transitions and explicit status decisions;
- no-self-review and review identity separation;
- configured review-domain requirements;
- push-intent and closeout guards;
- provider-session lifecycle reconciliation;
- destructive cleanup requiring explicit command and evidence.

Disabling a rule source only disables that source's recommendations and
pack-level checks. It does not disable Fairway core behavior.

## Pack Layout

A reusable platform pack should be portable across repositories:

```text
fairway-rules-platform/
  README.md
  schemas/
    rule.schema.yaml
  rules/
    core/
      contract-first.md
      evidence-before-done.md
      error-envelope-correlation-id.md
    delivery/
      deploy-run-required.md
      frontend-e2e-required.md
      deterministic-utility-first.md
    fairway/
      provider-session-checkpoints.md
      worktree-merge-model.md
      no-self-review.md
    security/
      no-query-string-tokens.md
      authz-negative-tests.md
      supply-chain-evidence.md
  templates/
    task-classification.md
    review-packet.md
    deploy-run.md
  profiles/
    platform-foundation.yaml
  examples/
    fairway-rule-source.toml
```

GPUaaS-specific rules live in the GPUaaS/product org, not in Fairway core and
not in the generic platform pack:

```text
fairway-rules-gpuaas/
  schemas/
    rule.schema.yaml
  rules/
    runtime/
      node-agent-task-lifecycle.md
      app-runtime-launch-contract.md
      terminal-gateway-token-session.md
    operations/
      provider-runtime-validation.md
    platform/
      billing-attribution.md
    security/
      tenant-resource-isolation.md
```

## External Guidance

External sources such as Project CodeGuard should not be parsed directly as
Fairway rule packs unless they already conform to Fairway's native schema. The
expected path is conversion:

```text
project-codeguard guidance
  -> converted/vendored fairway-rules-codeguard pack
  -> loaded by Fairway as a normal rule source
```

The converted pack owns mapping decisions such as rule IDs, evidence names,
review domains, applicability, and status. This keeps Fairway's loader simple
and avoids pretending incompatible upstream formats are native Fairway rules.

## Rule Schema

The canonical schema for a pack lives inside that pack:

```text
schemas/rule.schema.yaml
```

Fairway should validate a pack against its schema instead of maintaining a
divergent copy of the same structure in Fairway core. The common metadata
shape is:

```yaml
id: platform.contract-first
title: Contract changes precede implementation
status: draft
applies_when:
  source_paths:
    - doc/api/**
    - cmd/api/**
    - packages/web/src/lib/gen/**
  tags:
    - surface:api
    - surface:frontend
  task_kinds:
    - task
    - architecture-map
  profiles:
    - platform-foundation
risk_floor: medium
required_evidence:
  - contract-updated
  - generated-artifacts-clean
  - focused-tests
recommended_commands:
  - make codegen
  - CODEGEN_ENFORCE_CLEAN=1 bash scripts/ci/sdk_codegen_smoke.sh
review_domains:
  - backend
  - frontend
  - architecture
stop_conditions:
  - contract behavior is ambiguous
  - generated artifacts drift unexpectedly
related_rules:
  - platform.evidence-before-done
```

Rules do not carry per-rule version numbers. The rule-pack repository or source
pin is versioned as a unit. Individual rules keep `status` so deprecated or
draft rules can remain visible without being treated as active policy.

The Markdown rule body explains intent, examples, anti-patterns, required
evidence, and review notes. The structured header drives matching, dashboard
display, packet templates, and closeout checks.

## Source Resolution

The first Fairway implementation should be local-first:

```toml
[[rule_sources]]
name = "fairway-platform"
source = "path:../fairway-rules-platform"
mode = "advisory"

[[rule_sources]]
name = "gpuaas"
source = "path:../fairway-rules-gpuaas"
mode = "blocking"

[[rule_sources]]
name = "codeguard"
source = "path:../fairway-rules-codeguard"
mode = "advisory"
```

Initial loader support is limited to local `path:` or `file:` sources. Network
fetching is intentionally out of scope for the first slice.

Remote source declarations may be documented for future use, but blocking
remote sources must eventually be pinned by immutable commit SHA and content
checksum. Mutable branch or tag references are not acceptable as blocking rule
authority.

Modes:

- `advisory`: recommend rules and evidence, but do not block closeout.
- `blocking`: missing required evidence blocks configured readiness checks.
- `disabled`: source remains configured but is not evaluated.

## Group Resolution

Rule groups are derived from the configured source name plus the `rules/`
subdirectory:

```text
<source-name>.<rules-subdirectory>
```

Examples:

```text
fairway-platform.core
fairway-platform.delivery
fairway-platform.security
fairway-platform.fairway
gpuaas.runtime
gpuaas.operations
gpuaas.platform
gpuaas.security
```

Fairway validation should list available groups and warn on profile references
that do not resolve.

## Relationship To Profiles

Workstream profiles define the shape of a track. Rule packs define reusable
operating knowledge.

The project Fairway config is authoritative for binding rules to profiles:

```toml
[[workstream_profiles]]
name = "platform-foundation"
rule_groups = [
  "fairway-platform.core",
  "fairway-platform.delivery",
  "fairway-platform.fairway",
  "gpuaas.runtime"
]
```

Pack-local `profiles/*.yaml` files are starter templates and documentation.
They are not authoritative for a running project unless imported into project
config.

`applies_when.profiles` on an individual rule is a narrowing filter only. It
does not bind a rule source to a project profile by itself.

## Matching Semantics

For one rule, populated applicability axes are combined with AND:

```text
source/target path match
AND tag match
AND task kind match
AND profile match
AND risk_floor match
```

Within a single axis, multiple values are OR unless the field explicitly says
otherwise. Across rules, matches are independent; every matched rule applies.

Matched rules should be ordered deterministically:

1. higher `risk_floor` first: `critical`, `high`, `medium`, `low`;
2. then by rule ID.

`risk_floor` means the rule applies only to tasks whose risk is at or above the
floor. It does not raise the task's risk level.

Non-applicability rationale is first-class. When a rule appears close but is
not applicable, the packet or review should be able to record why.

## Evidence Types

Evidence type strings are join keys. Drift here is expensive because a manual
operator may understand that two names mean the same thing while a loader will
not.

Fairway should expose rule evidence types with a command such as:

```bash
fairway rules evidence-types
```

The command should list evidence types referenced by loaded rule packs, profile
gates, and recorded evidence. In blocking mode, a rule that requires an
evidence type with no known gate, command, or recorded evidence pattern should
produce a warning or blocker according to configuration.

## Review Domains And Tags

Review domains are project-defined strings validated against project config.
Fairway should warn when a loaded pack references a review domain that the
project does not know.

Tags are opaque strings. Prefixes such as `surface:`, `gate:`, `work-type:`,
`program:`, and `environment:` are useful taxonomy conventions, but Fairway core
should not hardcode their meaning unless a project profile or adapter defines
that mapping.

## Fairway Behavior

Initial Fairway support should be advisory and evidence-oriented:

- load configured local rule-pack sources;
- validate rule metadata;
- list available rule groups and evidence types;
- match rules to a task using source paths, target paths, tags, kind, profile,
  risk level, and review domains;
- show applicable rules on task detail and review packets;
- recommend required evidence and commands;
- record selected rules as evidence;
- report missing required rule evidence in `merge-ready` or `workflow check`
  when configured as blocking.

`merge-ready` and `workflow check --mode close --task-id <id>` evaluate selected
rules against the task's recorded evidence artifact types. Missing evidence from
`blocking` rule sources is a blocker. Missing evidence from `advisory` rule
sources is a warning. Disabled and non-applicable rules do not affect readiness.

Fairway should not treat a rule match as automatic approval. A rule match only
states what evidence and review are expected.

## Reference Rule Groups

The first reusable platform rule pack should include a small set of high-signal
rules:

- contract-first;
- generated-code/codegen clean;
- frontend e2e for user-visible changes;
- structured error envelopes and correlation IDs;
- auth material never in query strings;
- authorization negative tests for tenant/project/resource boundaries;
- audit required for privileged mutations;
- outbox required for domain events;
- API-first ops verification;
- deploy-run required for meaningful CI/deploy/UAT attempts;
- deterministic utility-first for repetitive CI/CD/UAT work;
- Fairway provider session checkpoints;
- worktree merge model and push intent;
- review-domain independence and no self-approval.

GPUaaS should then add domain-specific rules for node-agent, provider runtime,
app runtime, terminal gateway, billing attribution, IAM hierarchy, MAAS/LXD,
and GPU allocation isolation.

## Public Model

The public story should be:

```text
Fairway coordinates the work.
Rule packs encode reusable operating knowledge.
Profiles bind rule packs to a project or track.
Evidence proves that the selected rules were handled.
```

This lets Fairway support GPUaaS, security review, docs, release engineering,
and future projects without hardcoding one project's operating model into the
core product.
