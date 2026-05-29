# GPUaaS / ARC Adoption Track

This track captures GPUaaS and ARC adoption needs without turning Fairway into a
CI/CD system, scanner, deployment tool, docs portal, or agent runner.

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
shareable parity artifact proves that Fairway can represent current queue,
review, regression, session, worktree, and readiness behavior.

ARC adoption should follow the same model: first prove the coordination and
evidence flow, then add project-specific gates as configuration and packets.

## Priority Order

1. Parity artifact command.
2. Release readiness/risk data model.
3. Security, release, and UAT packets.
4. GitLab evidence bridge.
5. Environment and ring awareness.
6. Risk register reporting.
7. SDK/developer readiness gates.
8. Documentation export for portal systems such as Docusaurus.

## 1. Parity Artifact Command

`fairway parity artifact` produces a meeting/shareable report after importing a
copied GPUaaS queue. It should include:

- task import summary from the current DB,
- ready set and ready-by-role counts,
- review-route samples,
- regression-pack catalog validation,
- merge/evidence gap counts and a bounded sample,
- worktree health,
- session health,
- coordinator issues and recommendations.

This is the proof artifact for product, engineering, and security review. It is
not a replacement for the GPUaaS scripts until discrepancies are reviewed and
closed.

## 2. Release Readiness Gates

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

## 3. Security / Release / UAT Packets

Packets should extend the existing context, bugfix, and watcher model:

- `packet security-review`,
- `packet release-risk`,
- `packet uat-readiness`,
- `packet ciso-summary`.

These packets should share one vocabulary:

- proof commands,
- findings,
- mitigations,
- owner,
- residual risk,
- signoff,
- expiry or revisit date.

## 4. GitLab Evidence Bridge

GitLab remains the execution system. Fairway should understand GitLab evidence:

- pipeline URL,
- job status,
- artifact links,
- security scan links,
- release tag,
- deploy environment.

Fairway should answer whether the GitLab evidence satisfies task or release
gates. It should not replace GitLab pipelines.

## 5. Environment / Ring Awareness

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

## 6. Risk Register

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

## 7. SDK / Developer Enablement Gates

GPUaaS SDK exposure is product surface area, not just docs. Fairway should
support readiness gates for:

- SDK docs complete,
- examples validated,
- API contract stable,
- internal developer quickstart tested,
- external/public readiness decision recorded.

## 8. Documentation Export

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
