# Fairway v0.3.0 Release Readiness

Date: 2026-08-22
Fairway task: `FW-414`
Candidate version: `v0.3.0`

## Scope

This candidate groups one coherent product increment:

- the versioned cross-harness run, observation, and evaluator-result contract;
- atomic, idempotent ingestion plus CLI and task-detail readback;
- verified outcome-efficiency and cited trajectory analysis;
- a bounded GPUaaS consumer pilot and provider-neutral adapter example; and
- public positioning of Fairway as the durable engineering control and evidence
  plane across replaceable harnesses.

The exact candidate source SHA will be the release-preparation commit containing
this assessment after commit-bound review and push. Production rehearsal and
the annotated tag must bind that SHA.

## Underlying Increment Qualification

- completed independent architecture, implementation, analysis,
  consumer-pilot, and public-positioning reviews for `FW-409` through `FW-413`;
- full Go tests, `go vet`, focused race tests, PostgreSQL compatibility, and
  Docusaurus production build on the integrated implementation;
- contract checks for atomic replay, conflicting-record rejection, caller-
  asserted namespace qualification, record linkage, and secret-like content
  rejection; and
- GPUaaS pilot readback showing verified outcome ratios only with complete
  denominators and cited advisory trajectory findings with explicit
  false-positive limits.

## Exact-Candidate Gates

1. Full Go, integration, formatting, lint, release-configuration, migration,
   and Docusaurus production checks pass on the clean candidate.
2. Architecture, governance, operations, and security approve the exact release
   metadata, upgrade guidance, supply-chain posture, and claim boundary.
3. `main` is pushed and CI plus Docs Portal pass for the exact SHA; the new
   public harness contract and GPUaaS pilot pages are reachable.
4. The production `v0.3.0` rehearsal passes tests, signing, notarization,
   candidate smoke, vulnerability/license/SBOM evidence, signed assurance, and
   immutable packet verification.
5. The annotated tag binds the successful rehearsal run id and promotion
   creates a draft from that verified packet without rebuilding.
6. Draft assets and checksums are inspected before publication; public asset
   URLs and Homebrew fetch are verified before closeout.

## Claim Boundary

`v0.3.0` may claim versioned cross-harness engineering records, safe atomic
ingestion, cited task-scoped readback, verified outcome-efficiency calculations
when denominators are complete, and advisory trajectory findings. It may cite
the GPUaaS assessment as a bounded consumer pilot.

It may not claim autonomous supervision, automatic redirection, causal control
effectiveness, model or harness ranking, exact comparable cost, inferred
history, a released Seaway adapter, sovereign deployment readiness, regulatory
certification, independent security assessment, or a complete AI Quality
System.

## Migration And Rollback

Migration `018_harness_records.sql` adds append-only harness record tables and
does not rewrite existing task, evidence, review, session, or quality-record
facts. Back up the project database before upgrade. Do not synthesize historical
harness records.

Consumer rollback uses the `v0.2.7` binary and the matching pre-upgrade database
and managed-contract backup. In-place binary downgrade against a database that
has applied migration 018 is not a supported rollback procedure. Stop Fairway,
preserve any post-upgrade database separately for investigation or export,
restore the pre-upgrade backup, and then start `v0.2.7`.

Do not move or recreate an immutable tag. If rehearsal or promotion fails,
leave the version untagged or the draft unpublished, fix the owning defect on
`main`, and rehearse the corrected exact SHA. If a published artifact is wrong,
yank it and cut a new version; never reuse `v0.3.0`.
