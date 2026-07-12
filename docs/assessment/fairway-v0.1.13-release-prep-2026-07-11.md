# Fairway v0.1.13 release preparation

Date: 2026-07-11  
Task: FW-334  
Previous release: `v0.1.12`  
Release-prep baseline: `9e53ef68c345aef973a8c083b6d0a48bc59638f6`

## Candidate Scope

`v0.1.13` packages the post-`v0.1.12` dashboard performance train, generic
consumer boundary cleanup, first-value and documentation consolidation, public
portal narrative, ecosystem responsibility model, and AI Cloud case study.

The candidate does not promote shared-team write APIs, trusted-proxy runtime
verification, non-loopback origins, Postgres runtime storage, dashboard
mutation, provider-send authority, autonomous approval, merge, deploy, release,
credential, or live-operation authority.

## Included Work

- real-data dashboard assessment and repeatable contention benchmark;
- incremental SSE cursor and bounded review-wait polling;
- wall fast path and independently loaded diagnostics;
- batched report, coordinator, audit, transition, evidence, review, notification,
  and handoff projections;
- bounded task-detail and static/unknown-route behavior;
- evidence-backed product definition and claim inventory;
- reader-oriented documentation information architecture;
- clean-repository first-value rehearsal;
- progressive concept guide, integration and ecosystem responsibility pages;
- internal AI Cloud case study;
- standalone-product consumer-reference classification and compatibility window;
- responsive public portal rebuild and deployment.

## Validation Baseline

The independent FW-333 review passed full tests, integration tests, vet,
focused race tests, source/release guards, `go mod tidy`, release-linked version
smoke, clean first-value rehearsal, Docusaurus build, dependency threshold,
public route/security-header probes, and desktop/mobile browser review.

The final candidate must rerun the same checks after this release-prep commit
and after both configured remotes report the same clean source SHA.

## Compatibility And Upgrade

- No database migration was added after `v0.1.12`.
- `consumer_lessons` is the current docs-audit JSON field.
- `gpuaas_lessons` remains populated for one compatibility window.
- Existing GPUaaS-named example fixtures remain available but are explicitly
  compatibility examples; generic examples are the default documentation path.
- Rollback reference remains the released `v0.1.12` binary and normal SQLite
  backup/readback procedure.

## Known Limits

- Shared write, trusted proxy, non-loopback, and Postgres runtime capabilities
  remain preview or unsupported.
- The AI Cloud case-study measurements are observational and not a causal or
  external-adoption claim.
- Four moderate static-portal development-chain advisories remain visible; the
  high-severity gate passes and dependency refresh is separate maintenance.
- The public portal already reflects current source while the latest binary tag
  remains `v0.1.12`; publication of `v0.1.13` closes that version gap.

## Publication Boundary

FW-334 prepares and validates the candidate only. FW-335 separately owns the
annotated tag, release workflow, signing/notarization, GitHub assets, checksums,
Homebrew update, installed-version readback, release verification, and rollback
evidence. Dashboard restart remains a separate explicit lifecycle decision.
