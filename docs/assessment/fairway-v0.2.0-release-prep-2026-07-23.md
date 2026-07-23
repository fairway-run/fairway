# Fairway v0.2.0 Release Preparation

Date: 2026-07-23
Task: `FW-383`
Previous release: `v0.1.13`
Preparation baseline: `39f4c5784b6d50305e798421223d672555f697b0`

## Candidate Scope

`v0.2.0` consolidates Fairway's memory, project-owned engineering knowledge,
versioned agent contracts, assurance evidence tooling, and reviewed migration
operating model. It is a pre-1.0 product-model release rather than a
compatibility-preserving patch.

The candidate remains local-first and provider-neutral. It does not promote
shared-team writes, hosted retrieval, provider-private memory, Postgres runtime
storage, autonomous approval, migration execution, certification, legal or
export classification, or independent security-assessment authority.

## Included Work

- database-backed track memory lifecycle and bounded cold-start packets;
- legacy `tmp-ux` migration and terminal-memory retirement;
- deterministic engineering-knowledge lint, ingest, snapshot binding, query,
  freshness, promotion, and custody controls;
- measured GPUaaS adoption across three architecture workstreams;
- versioned generated agent contracts with explicit upgrade and adoption;
- migration execution profile, rule-pack completeness, and verifier
  qualification design;
- assurance profiles and evidence mapping;
- signed/offline release and sovereign rehearsal packages;
- restricted security advisory and customer-held-key rehearsal tooling;
- deployment-baseline and immutable trust-bootstrap packaging.

## Compatibility And Upgrade

Fairway is pre-1.0 and has one controlled adopter. `v0.2.0` intentionally
optimizes for the corrected product model rather than preserving accidental
behavior.

Durable state remains protected:

- database migrations are forward-applied through the normal store open path;
- agent-contract upgrades are explicit and losslessly preserve the old
  contract for manual extraction of project-owned policy;
- newer contract schema or revision state cannot be silently downgraded;
- legacy memory is migration input, never a second canonical authority;
- rollback reference is the released `v0.1.13` binary plus a pre-upgrade
  database and control-file backup.

The rollback order is: stop active project processes, restore the pre-upgrade
database and control files, reinstall `v0.1.13`, then validate config, task
readback, and active reconciliation. Running the older binary directly against
newer migrated state is not the rollback model.

## External Boundaries

The following existing tasks do not block this general product release:

- `FW-351`, qualified jurisdiction and export-control applicability;
- `FW-352`, independent sovereign security assessment;
- `FW-354`, sovereign deployment-ready release packet.

They remain blocked because engineering and AI reviewers cannot supply
qualified legal, export-control, certification, or independent-assessment
evidence. This release therefore makes no sovereign certification,
market-access, ECCN/EAR/ENC, procurement, or independent-assessment claim.

The historical GPUaaS dashboard walkthrough tasks `FW-135` and `FWRD-154` are
deferred to the immediate `v0.2.0` GPUaaS adoption run, where the named
operator can evaluate the released binary against the current project.

## Validation Plan

Before tagging:

- run unit and integration tests;
- run focused race suites;
- run `go vet`, `go mod tidy`, and `git diff --check`;
- parse and validate the Fairway backlog;
- run release, offline-distribution, restricted-advisory, and sovereign
  rehearsal gates that do not require external claims;
- run `goreleaser check`;
- build the documentation portal and run its configured dependency threshold;
- verify the full retained npm tree after upgrading and pinning Wrangler and
  applying the patched `sharp` override reports no known vulnerabilities;
- require the pushed preparation commit to complete the real Cloudflare Pages
  deployment and public route readback before tagging;
- build a release-linked `0.2.0` binary and exercise config validation,
  agent-contract status, ready, and active reconciliation;
- obtain independent integrated architecture, governance, operations, and
  security review of the exact release candidate.

After the candidate is clean, pushed, and reviewed:

1. create one annotated `v0.2.0` tag at the approved source;
2. push the tag to `fairway-run/fairway`;
3. require the tag workflow to produce signed/notarized candidate archives and
   a verified release-assurance package;
4. inspect and publish the draft;
5. update and verify the Homebrew cask;
6. run `fairway release verify` against public assets and the exact source;
7. install `v0.2.0` and run the GPUaaS adoption/walkthrough.

## Current Verdict

The following preparation gates pass on the current dirty release boundary:

- `go test ./...`;
- `go test -tags=integration ./...`;
- focused race suites for CLI, store, coordinator, dashboard, audit, knowledge,
  and memory migration;
- `go vet ./...`;
- `go mod tidy` with no module-file drift;
- `git diff --check`;
- `fairway config validate`;
- `goreleaser check`;
- small-team read-only lifecycle rehearsal;
- Docusaurus production build;
- `npm audit --audit-level=high`, with zero reported vulnerabilities after
  upgrading and pinning Wrangler `4.113.0` and overriding its vulnerable exact
  transitive `sharp@0.34.5` pin with `sharp@0.35.3`;
- `npm audit --omit=dev --audit-level=high`, with zero reported production
  dependency vulnerabilities;
- release-linked `0.2.0` init, version, config, agent-contract, ready, and
  reconciliation smoke in a disposable repository.
- disposable `v0.1.13` project upgrade to `v0.2.0`, followed by restoration of
  the backed-up database and agent contract and clean `v0.1.13` task and
  reconciliation readback.

Release preparation is technically green but not yet authorized for tagging.
Independent integrated review, an exact release-prep commit, clean pushed CI,
the real Wrangler `4.113.0` Pages deployment and public docs readback, and the
separately observed tag workflow remain required.
