# Fairway v0.2.6 Release Readiness

Date: 2026-08-05  
Fairway task: `FW-397`  
Candidate version: `v0.2.6`

## Scope

This release candidate groups one coherent product increment:

- advisory control-effectiveness analytics and coverage-first suppression;
- durable task-to-commit association;
- structured operational outcomes and attributable control friction;
- the cited, read-only Quality Record CLI/dashboard projection;
- the second GPUaaS measurement pilot;
- measured Quality Record positioning and a reproducible demonstration.

The exact candidate source SHA is the pushed release-preparation commit that
contains this assessment. The production rehearsal and annotated tag must bind
that SHA; this document does not substitute for the immutable promotion packet.

## Qualification Already Completed

- focused package tests for provenance, store, control analytics, Quality
  Record, CLI, and dashboard behavior;
- repeated full `make check` gates through the implementation sequence;
- Docusaurus production build and desktop/mobile visual readback;
- green CI and Docs Portal workflows for the implementation and GPUaaS pilot
  commits preceding release preparation;
- a 288-task GPUaaS Quality Record population run with deterministic output
  after removing the generation timestamp;
- a contemporaneous 30-day GPUaaS control report retaining low-coverage
  suppression and absent historical instrumentation.

## Release Gates Still Required

1. CI and Docs Portal pass on the exact release-preparation commit.
2. The production release rehearsal for `v0.2.6` passes on that exact SHA,
   including tests, GoReleaser, signing, notarization, candidate smoke, and
   signed assurance verification.
3. Governance and architecture approve the release content and authority
   wording on the exact candidate SHA.
4. The annotated `v0.2.6` tag binds the successful rehearsal run id.
5. Tag promotion verifies and stages the immutable candidate without rebuilding.
6. Published release assets and Homebrew installation are read back before the
   release task is closed.

## Claim Boundary

`v0.2.6` may claim a supported cited Quality Record and advisory control
measurement with bounded GPUaaS data-quality validation. It may not claim
causal control effectiveness, comprehensive software quality measurement,
qualified arbitrary verifiers or reviewers, continuous autonomous improvement,
regulatory certification, or a complete AI Quality System.

## Rollback

Do not move or recreate an immutable tag. Before consumer upgrade, retain the
matching project database and managed-contract backup. If candidate rehearsal
or promotion fails, leave the version untagged or the draft unpublished, fix
the owning defect on `main`, and run a new rehearsal against the new exact SHA.
