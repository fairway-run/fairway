# Fairway assurance package pilot — 2026-07-12

## Decision

- Fairway Sovereign Deployment Ready starter: **repeat-with-fixes** after the
  product controls, disconnected rehearsal, customer evidence, and independent
  assessment exist.
- AI Cloud NIST SSDF starter: **promote** for bounded internal assessment
  preparation and evidence-gap discovery, not for an SSDF, legal, procurement,
  certification, compliance, or authorization claim.

Both signed packages were internally consistent and verified against a pinned
Ed25519 public key. Both correctly returned `control_sufficiency=insufficient`,
`external_certification=not_evaluated`, and `ok=false`.

## Scope

The pilot used the released profile contract and the current local Fairway
binary built from source after FW-360/FW-341. It did not import consumer source
files or artifact bodies.

| Package | Profile | Project and task-set scope |
|---|---|---|
| Fairway sovereign | `fairway-sovereign-deployment-ready-v1-starter@v1` | `fairway-assurance-core-20260712`: FW-341 and FW-355 through FW-360 |
| AI Cloud | `nist-ssdf-1.1-starter@v1` | `aicloud-node-security-20260712`: healthy node-identity renewal priority, strict node-agent task signing, and Schemathesis promotion evidence tasks |

The consumer config was read only. Package output was written under
`.fairway/artifacts/assurance-pilot-20260712/` in the Fairway repository, which
is an ignored local evidence root.

## Reproducibility identity

| Package | Profile SHA-256 | Manifest SHA-256 | Files | Signature |
|---|---|---|---:|---|
| Fairway sovereign | `85ae2eebdf483d4b875fd71211245d517cfc94f72ab78d4e638551d281ebb95c` | `928497a6beb9ecdac8fe447e7ad2e3d0b0fea219b986565472825621dd250825` | 17 | `verified_pinned` |
| AI Cloud | `542af5f16789138e91199fc12a44b9e996555d11e9cb8592a86fed9070342212` | `e87f7b999a20efac8cc7bfc0a6bd45b03a0cc57c66fe3df8bb71383b643c6c80` | 17 | `verified_pinned` |

Both manifests use evaluation time `2026-07-12T21:52:00Z`. The signing seed was
generated for this disposable pilot, passed through an environment variable,
and not written to argv, the packages, the assessment, or Fairway records.

## Timing

Machine timings were captured with `/usr/bin/time -p`:

| Operation | Fairway sovereign | AI Cloud |
|---|---:|---:|
| Export | 0.30 s | 0.02 s |
| Offline verify | 0.01 s | 0.01 s |

Selecting the two bounded task sets, running the four commands, and producing
the first machine-readable gap summary took about four operator minutes. That
does not include framework applicability, legal analysis, sampling, testing,
customer evidence collection, risk acceptance, or assessor judgment.

## Results

### Fairway sovereign starter

| Status | Controls |
|---|---:|
| `satisfied_by_recorded_evidence` | 0 |
| `partial` | 1 |
| `missing` | 3 |
| `customer_responsibility` | 1 |
| `external_assessment_required` | 1 |

The package contains 84 metadata-only facts across `task`, `review`, `ci`, and
`evidence` classes. It reports ten gap rows. The selected assurance-core tasks
provide partial evidence for the offline-rehearsal objective but do not provide
the required deployment configuration and decision, release provenance,
offline rehearsal, customer identity/audit, backup/restore, or independent
assessment proof. This is expected because FW-342 through FW-353 have not yet
implemented or assessed those controls.

### AI Cloud NIST SSDF starter

| Status | Controls |
|---|---:|
| `satisfied_by_recorded_evidence` | 1 (`PO.2`) |
| `partial` | 2 (`PO.1`, `RV.1`) |
| `missing` | 1 (`PS.3`) |

The package contains 13 metadata-only facts across `task`, `review`, `ci`, and
`evidence` classes and reports six gap rows. `PO.2` is supported only for the
starter's narrow objective: the selected completed tasks have accountable task
state and positive independent review facts. This does not assert that the
consumer implements the complete SSDF practice or full framework.

The remaining gaps are an accepted decision record for `PO.1`, explicit
release/provenance facts for `PS.3`, and vulnerability-class proof for `RV.1`.

## Stale, conflict, and false-satisfaction review

- Neither readiness report contains a stale or conflicting control status.
- The evidence indexes preserve 56 Fairway and two AI Cloud facts as
  `conflicting`, primarily because historical `changes` and later `approve`
  review rows share one evidence class. That is conservative, but it obscures
  resolved latest-domain review state. FW-362 tracks deterministic latest
  review resolution while retaining prior rows as superseded history.
- No control was found satisfied by `changes`, `partial`, `blocked`, or `fail`.
  FW-360 regressions prove those non-positive results cannot satisfy shipped
  starter requirements.
- The sampled `PO.2` support consists of `done` task facts and `approve` review
  facts. No failing or private artifact content was used.

## Privacy and trust checks

- Evidence indexes contain no `/Users/` paths, command text, kind-recovery
  artifact path, bearer value, password marker, prompt, transcript, raw tool
  body, source body, or artifact body.
- Package references are stable Fairway task/evidence/review identifiers.
- Integrity, control sufficiency, signature trust, and external certification
  remain separate verifier fields.
- The packages require a separately pinned public key before signature trust is
  `verified_pinned`.
- Verification is read only and wrote no finding back to either Fairway DB.

## Manual assessor work remaining

Fairway materially shortened deterministic collection, indexing, digesting,
signing, gap generation, and offline verification. It did not remove the need
to:

- decide legal/framework applicability and the exact system/product boundary;
- review whether each starter mapping is suitable and complete for that scope;
- inspect authoritative source artifacts referenced by metadata;
- collect customer identity, network, key, backup, retention, and operating
  evidence;
- sample and test control operation;
- assess vulnerability and release provenance quality;
- resolve exceptions and residual risks through authorized owners;
- perform independent or accredited assessment and issue any external outcome.

## Recommendation

Promote the assurance accelerator for internal readiness preparation because it
turns selected Fairway facts into a signed, offline-verifiable, assessor-shaped
packet in under one second of machine time and exposes missing proof without a
false positive. Repeat the sovereign reference package only after the remaining
controls are implemented and independently exercised. Complete FW-362 before
using review-history conflict counts as a quality metric.

This pilot is evidence about package utility. It is not evidence that Fairway or
AI Cloud is certified, compliant, authorized, FIPS validated, CUI authorized,
EU CRA conformant, Common Criteria evaluated, or approved for any jurisdiction.
