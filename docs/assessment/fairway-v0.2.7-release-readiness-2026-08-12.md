# Fairway v0.2.7 Release Readiness

Date: 2026-08-12
Fairway task: `FW-405`
Candidate version: `v0.2.7`

## Scope

This candidate groups one coherent product increment:

- the read-only Product Overview and Quality Workspace;
- the collaborative problem-solving and governed-delegation model;
- refreshed public category copy and system maps;
- the Fairway-Seaway product boundary and optional integration contract; and
- the current GPUaaS operator walkthrough against the real consumer store.

The exact candidate source SHA is the reviewed and pushed release-preparation
commit containing this assessment. Production rehearsal and the annotated tag
must bind that SHA.

## Qualification Completed Before Release Preparation

- focused and full Go tests plus `go vet` for the dashboard and product-model
  changes;
- Docusaurus production builds and desktop/mobile visual inspection;
- read-only browser walkthrough of Overview, Wall, Board, Diagnostics, Quality,
  Reports, exports, and task detail against 1,920 GPUaaS tasks;
- independent architecture review of the Fairway-Seaway boundary,
  collaborative-delegation model, rendered Overview, and GPUaaS requirement
  disposition; and
- named GPUaaS operator approval with no blocking dashboard gap identified.

The documentation toolchain was updated to Docusaurus 3.10.2 and current React
patch releases. Direct overrides remove every npm advisory with an available
upstream fixed version. The remaining audit chain is the Docusaurus build-time
`image-size` dependency: upstream 2.0.2 has no fixed release, the advisory is a
denial-of-service loop for ICNS/JXL/HEIF parsing, and this repository contains
none of those asset formats. The published site is static and does not execute
the parser at runtime. Re-evaluate when Docusaurus or `image-size` publishes a
fix; do not ingest untrusted documentation assets in the meantime.

## Exact-Candidate Gates

1. `make check`, GoReleaser configuration validation, Docusaurus production
   build, config validation, docs/backlog audit, and workflow check pass on the
   exact clean candidate.
2. Architecture, governance, operations, and security approve scope and claim
   wording on that commit.
3. `main` is pushed and CI plus Docs Portal pass for the exact SHA.
4. The production `v0.2.7` rehearsal passes tests, signing, notarization,
   candidate smoke, signed assurance, and immutable packet verification.
5. The annotated tag binds the successful rehearsal run id and promotion
   creates a draft from the verified packet without rebuilding.
6. Draft assets and checksums are inspected before publication; public asset
   URLs and Homebrew fetch are verified before closeout.

## Claim Boundary

`v0.2.7` may claim a supported cross-run coordination record, cited Quality
Records, the read-only Overview and Quality Workspace, and a documented working
model for collaboration and bounded delegation. It may describe Seaway only as
an optional independently usable product with a design-level integration
contract.

It may not claim automatic delegatability, autonomous approval, a released
Seaway adapter, sovereign deployment readiness, legal or export classification,
regulatory certification, independent security assessment, or a complete AI
Quality System.

## Rollback

Do not move or recreate an immutable tag. Retain the matching project database
and managed-contract backup before consumer upgrade. If rehearsal or promotion
fails, leave the version untagged or the draft unpublished, fix the owning
defect on `main`, and rehearse the new exact SHA.
