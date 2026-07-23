# GPUaaS Integrated Memory and Knowledge Pilot

Date: 2026-07-22  
Fairway source: `8d0fe68`  
Consumer: `/Users/subash/dev/GPUasService`  
GPUaaS pilot commits: `5796695f5`, `6976aadd0`  
Track: `HARNESS-SPIFFE-NODE-TERMINAL-SECURITY-MATRIX-001`

## Scope

The pilot combined Fairway track memory with a project-owned engineering
knowledge base. GPUaaS enabled `[knowledge]` at `doc/agent-wiki`, used
preview-first ingest for two architecture sources, curated five reachable
pages, and composed those pages with current Fairway task, dependency,
checkpoint, blocker, and Git state.

The sources have different authority:

- `SPIFFE_Node_Agent_Terminal_Identity_ADR_v1.md` is a canonical proposed
  decision with a disposable-kind claim boundary;
- `Node_Operations_and_Agent_Lifecycle_v1.md` is an operational target model
  that declares `source_of_truth: false` and
  `implementation_state: not-assessed`.

Track memory cites GPUaaS checkpoints `3781` and `4341`. Live checkpoint facts
remain operational evidence and are not promoted into canonical architecture.

## Method

Two isolated providers received only a retained cold-start JSON file. They were
prohibited from reading repository files, legacy `tmp-ux`, Git history, prior
evaluations, or external sources. The fixed rubric asked each provider to:

1. state the objective, authority decisions, blockers, and next action;
2. answer the cross-task node/terminal identity question;
3. separate canonical claims, operational design, verified synthesis, and live
   task facts;
4. identify stale, contradictory, or unsupported claims;
5. count clarification questions, repeated investigations, and authority
   selection errors.

The packets and bounded provider-evaluation summaries are retained in
`docs/assessment/evidence/fw-376/`. The summaries preserve the evaluation
contract, decisive excerpts, findings, and measured counts; they are not full
provider transcripts.

## Results

| Measure | Initial | Corrected |
|---|---:|---:|
| Knowledge lint pages | 5 | 5 |
| Knowledge lint findings | 0 | 0 |
| Cold-start wall time | 0.17 seconds | 0.14 seconds |
| Cold-start JSON size | 10,038 bytes | 8,344 bytes |
| Knowledge payload | 4,016 bytes | 3,985 bytes |
| Clarifications required | 0 | 0 |
| Repeated investigations required | 0 | 0 |
| Authority-selection errors | 1 | 0 |

The first provider resumed the workstream without clarification, but an
independent review found that the source manifest classified the entire
architecture root as canonical. That made a non-authoritative,
implementation-not-assessed lifecycle model appear canonical and allowed
target-state requirements to read as current implementation. This was a real
synthesis and authority error that deterministic lint did not catch.

GPUaaS commit `6976aadd0` corrected the model:

- `architecture-decision` is canonical;
- `architecture-model` is operational;
- lifecycle and consolidated open-question pages are draft/unverified;
- current-state claims are limited to the canonical ADR's explicit boundary;
- unsupported shadow-integration and implemented-lifecycle claims were
  removed from knowledge pages.

The corrected clean packet reported commit `6976aadd0`, `dirty=false`, two
verified canonical-derived pages, two draft operational pages, and the correct
per-source authority. A second isolated provider selected no incorrect
authority, required no clarification, and needed no repeated investigation to
start the dependency-safe matrix work. It also correctly refused to infer that
the packet proves transport directionality, audit implementation, replay
resistance, or production readiness.

## Residual Findings

The pilot exposed presentation and maintenance refinements:

- checkpoint history needs timestamps or an explicit historical label because
  completed tasks currently show both old `active` and later `done` excerpts;
- memory disposition and current task status need distinct labels;
- equivalent blockers and generated inspect-status next actions should be
  deduplicated;
- source SHA and current Git SHA need a clearer freshness explanation;
- source frontmatter should eventually inform or validate configured authority
  instead of relying entirely on manual source classes.

The corrected packet intentionally cannot answer implementation-level details
of one-way audited transport from these sources alone. That is correct
fail-closed behavior: the operational lifecycle design is not evidence that
those controls are implemented.

The temporary provider packets were created with mode `0600` after the review
identified local multi-user exposure risk, copied into this internal repository
as retained review evidence, and removed from `/tmp` at closeout. A bounded
secret scan found no credentials, tokens, private keys, payment data, or raw
attestation material.

## Decision

Keep the integrated model with refinements. Track memory answers what is active
and what happens next; engineering knowledge answers which source supports a
bounded technical claim. The corrected packet improved resume quality without
exceeding its configured context budget and, importantly, failed closed when
asked for implementation proof absent from its sources.

Do not use the knowledge layer as a substitute for canonical contracts,
current task evidence, or live validation. Promotion and lint remain necessary
but insufficient gates; independent semantic review is required when a
synthesis influences an architecture, security, operations, or release
decision.
