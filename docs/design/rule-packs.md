# Rule Packs

Rule packs are reusable operating knowledge for Fairway-tracked work. They let a
project turn repeated architecture, security, implementation, review, CI/CD,
UAT, and operations practices into portable rules and templates.

Fairway should stay product-neutral. A rule pack supplies project or domain
knowledge; Fairway loads it, recommends applicable rules, records evidence, and
shows gaps.

## Positioning

```text
Fairway = coordination control plane
Workstream profiles = project operating shape
Rule packs = reusable operating knowledge
Tasks = concrete work
Evidence = proof that rules/gates were handled
```

This is broader than a secure-coding ruleset. Project CodeGuard is a useful
security rule-pack source. GPUaaS/Fairway needs a wider model that also covers
contract-first development, platform boundaries, CI/CD, UAT, deploy evidence,
ops verification, provider sessions, worktrees, reviews, and release closeout.

## Rule Pack Shape

Rule-pack ownership should follow the scope of the knowledge:

```text
fairway-rules-platform
  Reusable operating rules that apply across many engineering projects.

<project-org>/fairway-rules-<project>
  Project or product-specific rules owned by that project.

external security packs
  External guidance sources, for example Project CodeGuard.
```

Everything reusable across projects belongs in `fairway-rules-platform`.
Anything that depends on one project's domain, runtime, product contracts,
environment topology, or UAT model belongs in that project's own rule-pack
repository.

Fairway core should not own either class of rule content. It should own the
loader, matching, evidence, dashboard, and closeout behavior.

A reusable platform pack should be portable across repositories:

```text
fairway-rules-platform/
  README.md
  rules/
    contract-first.md
    evidence-before-done.md
    deploy-run-required.md
    frontend-e2e-required.md
    no-query-string-tokens.md
    api-first-ops-verification.md
    provider-session-checkpoints.md
    worktree-merge-model.md
  templates/
    task.md
    review-packet.md
    deploy-run.md
    ci-fix.md
    uat-finding.md
    security-rule-selection.md
  profiles/
    platform-foundation.yaml
    production-readiness.yaml
    security-review.yaml
  examples/
    gpuaas.yaml
```

GPUaaS-specific rules should live in the GPUaaS/product org, not in Fairway
core and not in the generic platform pack:

```text
<gpuaas-org>/fairway-rules-gpuaas/
  rules/
    node-agent-task-lifecycle.md
    app-runtime-launch-contract.md
    gpu-allocation-isolation.md
    terminal-gateway-token-rules.md
    platform-billing-attribution.md
    maas-lxd-kind-runtime.md
```

CodeGuard can be consumed as a security-focused rule source:

```text
project-codeguard/
  rules/
    authentication
    authorization
    input-validation
    cryptography
    supply-chain
    cloud-platform
    data-protection
    mcp-security
```

## Rule Schema

A rule should be structured enough for Fairway to reason about it:

```yaml
id: platform.contract-first
title: Contract changes precede implementation
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
risk_floor: medium
required_evidence:
  - contract-updated
  - codegen-enforced-clean
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
```

The Markdown rule body explains intent, examples, anti-patterns, and review
notes. The structured header drives matching, dashboard display, packet
templates, and closeout checks.

## Fairway Behavior

Initial Fairway support should be advisory and evidence-oriented:

- load configured rule-pack sources;
- validate rule metadata;
- match rules to a task using source paths, target paths, tags, kind, profile,
  risk level, and review domains;
- show applicable rules on task detail and review packets;
- recommend required evidence and commands;
- record selected rules as evidence;
- report missing required rule evidence in `merge-ready` or `workflow check`
  when configured as blocking.

Fairway should not treat a rule match as automatic approval. A rule match only
states what evidence and review are expected.

## Configuration

Rule sources should be explicit and versionable:

```toml
[[rule_sources]]
name = "fairway-platform"
source = "github:fairway-run/fairway-rules-platform"
version = "v0.1.0"
mode = "advisory"

[[rule_sources]]
name = "gpuaas"
source = "github:<gpuaas-org>/fairway-rules-gpuaas"
version = "v0.1.0"
mode = "blocking"

[[rule_sources]]
name = "codeguard"
source = "github:cosai-oasis/project-codeguard"
version = "v1.3.1"
mode = "advisory"
```

Modes:

- `advisory`: recommend rules and evidence, but do not block closeout.
- `blocking`: missing required evidence blocks configured readiness checks.
- `disabled`: source remains configured but is not evaluated.

## Relationship To Profiles

Workstream profiles define the shape of a track. Rule packs define reusable
operating knowledge.

Profiles may reference rule groups:

```toml
[[workstream_profiles]]
name = "platform-foundation"
rule_groups = [
  "platform.contracts",
  "platform.release",
  "security.authz",
  "fairway.provider-sessions"
]
```

This keeps Fairway generic while allowing a project to say: for this profile,
these operating rules matter.

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

The ownership story should be equally explicit:

```text
Fairway owns coordination mechanics.
fairway-rules-platform owns reusable cross-project operating rules.
Each project owns its project-specific rule pack.
External sources such as CodeGuard provide imported guidance.
```
