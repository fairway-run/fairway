# Rule Packs

Rule packs are reusable operating knowledge for Fairway-tracked work. They let
a project turn repeated architecture, security, implementation, review, CI/CD,
UAT, and operations practices into portable rules and templates.

Fairway stays product-neutral. Rule packs supply project or domain knowledge;
Fairway loads them, matches them to tasks, recommends evidence, records proof,
and shows gaps.

## Positioning

```text
Agent-driven delivery = engineering work performed with coding agents
Fairway = engineering control and accountability layer
Workstream profiles = project operating shape
Rule packs = reusable operating knowledge
Tasks = concrete work
Evidence = proof that rules/gates were handled
```

This is broader than a secure-coding ruleset. Project CodeGuard is a useful
security input, but Fairway needs a wider model that also covers
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

Project-specific rules live with the consuming product, not in Fairway core and
not in the generic platform pack:

```text
fairway-rules-service-platform/
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

Rule packets are rendered with `fairway packet rules <task-id>`. The packet is
read-only and includes selected rules, non-applicable rationale, required
evidence, recommended commands, review domains, and residual-risk/stop-condition
fields. Recording the rendered packet as evidence is an explicit operator or
agent action; packet rendering does not approve reviews or close tasks.

## Source Resolution

The first Fairway implementation should be local-first:

```toml
[[rule_sources]]
name = "fairway-platform"
source = "path:../fairway-rules-platform"
mode = "advisory"

[[rule_sources]]
name = "service-platform"
source = "path:../fairway-rules-service-platform"
mode = "disabled" # enable only after local path and vocabulary validation

[[rule_sources]]
name = "codeguard"
source = "path:../fairway-rules-codeguard"
mode = "disabled" # represented for future adoption; not fetched
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

Missing or unreadable local sources are treated according to mode. In
`advisory` mode, Fairway emits an error-severity load finding that includes the
source name, resolved path, and mode, then continues loading other valid rule
sources. In `blocking` mode, the same missing or unreadable source fails closed
and stops the command. This lets projects keep optional advisory packs in local
configs without losing all rule visibility, while preserving blocking sources as
real policy authority.

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
service-platform.runtime
service-platform.operations
service-platform.platform
service-platform.security
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
  "service-platform.runtime"
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

Path patterns use slash-separated globs. Supported syntax:

- `*` and character classes inside a single path segment, using Go
  `path.Match` semantics;
- `**` as a complete path segment, matching zero or more path segments;
- mid-pattern globstar such as `packages/**/gen/**`;
- suffix globstar such as `doc/api/**`.

Unsupported syntax is reported during rule-pack validation. In particular,
`**` must be a complete path segment; patterns such as `packages/**gen/**` are
invalid because authors could otherwise assume a dead pattern is active.

Matched rules should be ordered deterministically:

1. higher `risk_floor` first: `critical`, `high`, `medium`, `low`;
2. then by rule ID.

`risk_floor` means the rule applies only to tasks whose risk is at or above the
floor. It does not raise the task's risk level.

Rule match output uses three status values:

- `selected`: the rule applies to the task and its evidence/review expectations
  should be considered for packets, reports, and readiness checks;
- `non_applicable`: the rule was loaded but did not apply to the task after
  group, path, tag, kind, profile, risk, or review-domain matching;
- `disabled`: the rule source is configured as disabled and is not evaluated.

Earlier FW-146 planning text used `considered` as a possible match status. The
implemented vocabulary does not include that fourth state; this FW-161
reconciliation records the drift without changing historical review outcomes.

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

## CI Validation

Reusable rule-pack repositories should run validation before a pack is treated
as reusable:

```bash
fairway rules validate .
```

Project repositories with local packs should validate both the Fairway config
and each local pack:

```bash
fairway config validate
fairway rules validate rules
```

Example GitHub Actions snippets live in
`examples/rule-pack-ci/github-actions-platform.yml` for a standalone
`fairway-rules-platform` style repository and
`examples/rule-pack-ci/github-actions-project.yml` for project-local packs. The
helper script `scripts/validate-rule-packs.sh <dir>...` is intentionally
local-only; it does not fetch remote rule sources.

Validation warnings are expected when a reusable platform pack is run inside a
project whose review-domain vocabulary differs from the pack's vocabulary. For
example, a platform pack may mention `security` while a project config only
defines `appsec`. Treat that as a project adoption/configuration finding unless
the source is configured as blocking and the project has agreed to enforce the
pack as-is.

## Project Adoption Checklist

Do not treat an example `[[rule_sources]]` block as live project adoption. A
project has adopted a rule source only when the active `.fairway/config.toml` or
another reviewed project config points at a local source and the rollout state
is recorded.

Before enabling a project rule source:

1. Confirm the local source path exists in the checkout or adjacent workspace.
   Enabled `path:` and `file:` sources are local only; Fairway does not fetch
   `github:` or other remote sources.
2. Choose the mode deliberately:
   `advisory` for first rollout and learning, `blocking` only after the project
   agrees the evidence requirements are enforceable, `disabled` for represented
   but inactive future sources.
3. Compare the pack's review domains with the project vocabulary in config.
   Record expected vocabulary warnings as adoption findings instead of treating
   them as review approvals or failures.
4. Run `fairway config validate` and `fairway rules validate <pack-dir>`.
5. Add CI validation using the snippets under `examples/rule-pack-ci/` or an
   equivalent local command.
6. Run the first rollout in advisory mode, inspect `fairway rules match
   <task-id>`, `fairway packet rules <task-id>`, and `fairway merge-ready
   <task-id>` warnings, then decide whether to promote specific sources or
   groups to blocking.
7. Record the adoption decision as Fairway evidence or a governance checkpoint,
   including source path, mode, known warnings, and the command output used.

Consumer projects should reference this checklist rather than assuming
remote fetch support or copying example paths that may not exist locally.

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

A service platform should then add its own domain rules for runtime lifecycle,
gateway behavior, billing attribution, identity hierarchy, infrastructure, and
resource isolation.

## Public Model

The public story should be:

```text
Fairway coordinates the work.
Rule packs encode reusable operating knowledge.
Profiles bind rule packs to a project or track.
Evidence proves that the selected rules were handled.
```

This lets Fairway support service platforms, security review, docs, release
engineering, and future projects without hardcoding one project's operating
model into the core product.

Large structure-preserving code migrations may additionally use the optional
[Migration Execution Profile](migration-execution-profile.md). That profile
adds a bounded rule-pack completeness bakeoff and verifier qualification model.
It does not change the rule-pack ownership model, permit automatic rule
amendment, or make migration controls the default for ordinary engineering
work.
