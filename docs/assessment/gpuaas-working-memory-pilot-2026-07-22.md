# GPUaaS Working Memory Pilot

Date: 2026-07-22  
Fairway source: `aceff59`  
Consumer: `/Users/subash/dev/GPUasService`  
Track: `UAT-NODE-RECOVERY-MESH-DISPOSABLE-001`

## Scope

The pilot migrated the completed GPUaaS node-runtime trust working memory from
`tmp-ux/gpuaas-node-runtime-trust-memory-2026-07-13.md` into Fairway track
memory. The legacy Markdown was treated as an input only. Fairway stored the
bounded extracted fields and source-fact IDs, not the raw file contents.

The migrated memory was backed by GPUaaS checkpoint `3934`. The provider packet
also read current task, session, checkpoint, dependency, and Git state from the
GPUaaS Fairway project.

## Results

| Measure | Result |
|---|---:|
| Import preview before apply | yes |
| Explicit apply required | yes |
| Cold-start wall time | 0.10 seconds |
| Cold-start JSON size | 4,123 bytes |
| Clarifications required | 0 |
| Secret-pattern findings in retained import/packet output | 0 |
| Stale legacy facts found | 1 |
| Repeated investigation required | 0 |
| Incorrect authority choices | 0 |
| Operator maintenance time | approximately 5 minutes |
| Exact extracted facts represented before retirement | 2 of 2 |

The stale fact was the legacy file's old active-scope statement. It was retained
as the extracted legacy fact for exact coverage while the current objective,
decision, blocker posture, and next action were separately curated from the
closed Fairway task and committed UAT evidence.

The cold-start packet identified:

- the completed node-recovery objective and source checkpoint;
- the verified final UAT commit and decision;
- no active node-recovery product blocker;
- the separate environment-gated high-assurance SPIRE work;
- current related SPIFFE tasks reached through dependency closure;
- repository branch, commit, and clean working-tree posture.

## Retirement

Before retirement, `fairway memory coverage` reported the legacy file as
`covered` with `represented_facts=2` and `extracted_facts=2`. The completed track
memory was then archived. `fairway memory retire-file` returned `eligible=true`
for SHA-256
`919bb00da242dca86aded6de89695578b25f738635be189a47394f98540ac9d9`.
The ignored legacy file was removed after that readback.

The remaining `tmp-ux` inventory is intentionally not declared migrated by this
pilot. The final inventory contains 17 memory-named files: safe files remain
uncovered and four unsafe or oversized files remain explicitly rejected. This
prevents a false repository-wide completion claim.

## Decision

Keep the Fairway track-memory model. It materially improves bounded resume and
authority clarity at low maintenance cost. Continue migration one active track
at a time; do not bulk-import rejected or obsolete files, and do not treat
legacy `tmp-ux` content as an authority after a track is migrated.
