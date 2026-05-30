# GPUaaS / ARC Adoption Track

This track captures GPUaaS and ARC adoption needs as concrete instances of
Fairway's generic [workstream profiles](workstream-profiles.md), without turning
Fairway into a CI/CD system, scanner, deployment tool, docs portal, or agent
runner.

Fairway's job is the coordination control plane around those tools:

- work state,
- agent/session visibility,
- evidence,
- handoffs,
- review routing,
- release readiness,
- risk,
- adoption proof.

Other tools still execute builds, tests, scans, deployments, and documentation
hosting.

## Adoption Goal

GPUaaS should switch from its Ruby/shell queue model to Fairway only after a
parity artifact proves that Fairway can represent current queue, review,
regression, session, worktree, and readiness behavior.

ARC adoption should follow the same model: first prove the coordination and
evidence flow, then add project-specific gates as configuration and packets.

For the GPUaaS platform-foundation work, Fairway should first act as the
orchestrator of architecture-owned execution:

- package, route, schema, event, worker, and frontend ownership maps,
- report-only boundary guards,
- evidence/status vertical slices,
- V3 page-contract handoffs,
- review routing across backend, frontend, ops, architecture, and governance,
- task evidence and residual-risk records.

Fairway should not become the production-readiness authority until the
release-readiness, environment/ring, risk, and GitLab-evidence capabilities in
this track are implemented and validated.

## Priority Order

1. Configurable workstream profiles.
2. Adoption artifact command.
3. Configurable packet templates.
4. Structured guard evidence.
5. Task ownership metadata.
6. Multi-reviewer merge readiness.
7. Release readiness/risk profile.
8. GitLab evidence bridge.
9. Environment and ring awareness.
10. Risk register reporting.
11. SDK/developer readiness gates.
12. Documentation export for portal systems such as Docusaurus.

## 1. Adoption Artifact Command

`fairway adoption artifact` produces a generic adoption-readiness report after
importing a queue. It should include:

- task import summary from the current DB,
- ready set and ready-by-role counts,
- review-route samples,
- regression-pack catalog validation,
- merge/evidence gap counts and a bounded sample,
- which gates are advisory vs. blocking,
- worktree health,
- session health,
- coordinator issues and recommendations.

This is the proof artifact for adoption readiness. It is not a replacement for
the GPUaaS scripts until discrepancies are reviewed and closed.

`fairway parity artifact` remains as a compatibility spelling for GPUaaS-style
script-to-Fairway comparisons.

## 2. Platform-Foundation Orchestration Profile

GPUaaS platform-foundation work needs Fairway to coordinate architecture-driven
code and deployment restructuring without inviting broad move-only refactors.

The initial profile should support an epic like:

```text
PLATFORM-FOUNDATION-001
  Establish platform foundation code and deployment boundaries
```

Required task families:

- ownership maps:
  - package ownership,
  - route ownership,
  - schema/table ownership,
  - event subject ownership,
  - worker/binary ownership,
  - frontend surface ownership;
- report-only boundary guards:
  - import-boundary report,
  - route-placement report,
  - schema-owner report,
  - frontend module-boundary report;
- first vertical slice:
  - platform evidence facade,
  - platform status/ops facade,
  - route modules,
  - V3 page contracts,
  - release evidence inputs;
- follow-on foundations:
  - IAM facade,
  - platform registry skeleton,
  - product package facades,
  - guard graduation.

Fairway should provide configuration examples and packet templates for this
work. The goal is to make the orchestrator ask for maps, guards, facades,
contracts, and evidence before asking agents to move files.

### Needed Fairway Enhancements

These implemented commands make Fairway useful for the first
platform-foundation track:

- `packet architecture-map` (implemented)
  - scope, current owner, target owner, migration risk, source docs, acceptance;
- `packet boundary-guard` (implemented)
  - guard intent, report-only findings, false positives, graduation criteria;
- `packet vertical-slice` (implemented)
  - target seam, old path, new path, adapter, proof commands, rollback plan;
- route/review examples for `doc/architecture/platform-foundation/**`;

The generic follow-up should be configurable profiles, declarative packet
templates, ownership metadata, structured guard evidence, and dashboard grouping
by task kind. See [workstream-profiles.md](workstream-profiles.md).

## 3. Release Readiness Gates

Release readiness should be first-class Fairway state, separate from CI/CD
execution:

- security review complete,
- UAT automation evidence attached,
- regression packs passed or deferred with risk,
- release owner approval,
- residual risk recorded,
- environment/ring approval recorded.

Fairway should summarize whether evidence satisfies these gates. It should not
deploy or run pipelines itself.

## 4. Security / Release / UAT Packets

Packets should extend the existing context, bugfix, and watcher model:

- `packet security-review`,
- `packet release-risk`,
- `packet uat-readiness`,
- `packet production-readiness`.

These packets should share one vocabulary:

- proof commands,
- findings,
- mitigations,
- owner,
- residual risk,
- signoff,
- expiry or revisit date.

## 5. GitLab Evidence Bridge

GitLab remains the execution system. Fairway should understand GitLab evidence:

- pipeline URL,
- job status,
- artifact links,
- security scan links,
- release tag,
- deploy environment.

Fairway should answer whether the GitLab evidence satisfies task or release
gates. It should not replace GitLab pipelines.

## 6. Environment / Ring Awareness

Fairway should model approval targets such as:

- `dev`,
- `uat`,
- `staging`,
- `prod`,
- `ring-0`,
- `ring-1`,
- `ring-2`.

The model should state which tasks/features/releases are approved for a target.
Deployment remains outside Fairway.

## 7. Risk Register

Add a lightweight risk register tied to tasks and releases:

- risk,
- owner,
- severity,
- mitigation,
- residual risk,
- accepted by,
- expiry/revisit date.

This gives security and architecture a durable place to track accepted risk
without encoding it as scattered comments or tracker labels.

## 8. SDK / Developer Enablement Gates

GPUaaS SDK exposure is product surface area, not just docs. Fairway should
support readiness gates for:

- SDK docs complete,
- examples validated,
- API contract stable,
- internal developer quickstart tested,
- external/public readiness decision recorded.

## 9. Documentation Export

Fairway may export structured docs/evidence for a portal such as Docusaurus:

- active roadmap,
- completed release evidence,
- architecture decisions,
- open risks,
- developer onboarding tasks.

The portal hosts and renders; Fairway exports the authoritative coordination
state.

## Boundary

Fairway coordinates work, evidence, risk, and readiness. It does not execute
builds, tests, scans, deploys, or docs hosting.
