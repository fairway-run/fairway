# GPUaaS Learned-Context Pilot

Date: 2026-08-31  
Fairway task: `FW-415`  
Consumer: `/Users/subash/dev/GPUasService`  
Fairway config: `.fairway/platform-foundation-config.toml`

## Decision

Keep the preview-first capture, optional hybrid retrieval, and separately
budgeted cold-start composition. Narrow default semantic behavior until the
consumer knowledge corpus is fresher and broader. The main observed retrieval
gap was not vector quality: the most relevant repair lesson was stale and
therefore intentionally excluded from the semantic index.

Do not enable automatic capture or semantic indexing by default from this
pilot. First refresh the GPUaaS knowledge pages and repeat the same query set
with more verified content.

## Boundary

The pilot used the repository's existing engineering-knowledge pages. It did
not ingest provider transcripts, change task state, verify stale pages, approve
architecture, or promote derived content. The local vector index was written
under ignored `.fairway/artifacts/` and is safe to delete.

The embedding adapter was an explicit local process using FastEmbed
`BAAI/bge-small-en-v1.5`. Fairway received only one bounded text request and one
numeric vector per invocation. The embedding model and runtime are not Fairway
dependencies.

## Corpus baseline

`knowledge status` reported:

- 8 maintained pages;
- 3 pages labelled verified and 5 labelled draft;
- 14 `source_revision_stale` findings;
- 2 eligible verified content pages in the disposable semantic index; and
- no change to Markdown or Fairway authority when the index was built.

The apparent difference between three verified pages and two indexed pages is
expected: navigation/special pages and pages with blocking findings are not
embedded.

## Retrieval observations

Three positive queries and one negative control compared deterministic lexical selection with hybrid
selection. Result counts stayed bounded and authority labels were unchanged.

| Query | Lexical | Hybrid | Packet bytes | Observation |
|---|---:|---:|---:|---|
| `node replacement hardware failure shared storage recovery` | 5 pages | 5 pages | 6,632 / 6,703 | Both returned the repair page plus current-state and identity context. Only the two eligible verified pages received semantic scores. |
| `host remediation after physical accelerator outage with durable dataset reattachment` | 4 pages | 4 pages | 4,193 / 4,264 | Both missed the repair-quarantine page. That page was excluded from the semantic index because it was stale/unverified, so hybrid retrieval correctly refused to use semantic similarity to bypass its authority state. |
| `tenant identity certificate trust terminal access` | 4 pages | 4 pages | 5,150 / 5,221 | Both returned the expected identity pages. Hybrid added ordering evidence without changing verification or freshness. |
| `invoice refund stripe ledger balance` | 0 pages | 0 pages | 260 / 305 | The calibrated hybrid query returned no unrelated node or identity page and reported that no page matched. |

Observed hybrid packet overhead was 71 bytes for each query. A missing-index
probe returned `retrieval_mode=lexical` with the explicit warning `semantic
index unavailable; lexical fallback used`.

The first negative-control run exposed two false-positive node pages at a
semantic-only admission threshold of `0.20`. Their cosine scores were `0.5061`
and `0.4755`. FW-415 therefore made the threshold explicit and raised the
default to `0.55`; the repeated negative control returned zero pages while the
positive vocabulary-mismatch query retained verified semantic matches at
`0.6249` and `0.6082`. This calibration is adapter/model-specific evidence,
not a universal similarity threshold.

## Cold-start observation

`memory cold-start --knowledge-auto` was run for
`ARCH-REPAIR-QUARANTINE-RECOVERY-001`.

- Base JSON packet: 4,300 bytes.
- Packet with optional knowledge: 14,340 bytes.
- Knowledge sub-packet: 7,698 accounted bytes under an 8,192-byte budget.
- Selected pages: 5.
- The execution-memory `.packet` projection was byte-for-byte identical before
  and after knowledge composition.
- Existing warnings that the track memory is stale and its task is terminal
  remained visible.

This proves that optional learned context did not displace or rewrite execution
state, actionability, blockers, or lifecycle warnings in this pilot.

## Maintenance and failure cost

- Disposable index size: 23,534 bytes.
- Rebuild preview after the model was cached: 0.89 seconds wall time.
- Index checksum:
  `9d1cabe5791913a9326c07c4666487bad86f337de2d255a7a595db475cd478ac`.
- Deleting or pointing at a missing index preserved lexical results.
- The local Ollama server reported that its current serving mode did not
  support embeddings, so the pilot used a separate replaceable local adapter.
  This confirms the product boundary: embeddings are optional adapter
  capability, not a required inference-engine assumption.

## Incorrect-authority audit

No query changed a page's `verified`, `stale`, `conflict`, source authority, or
freshness fields. Semantic similarity did not make stale repair material
verified or add it to the vector index. Imported bundle behavior is covered by
tests that rewrite all external pages as untrusted drafts with local ownership,
citations, verification, and promotion removed.

Incorrect authority choices observed after calibration: **0** in the four-query
and one cold-start sample. One pre-calibration relevance false positive was
detected and corrected; it never changed authority state. This is a bounded
result, not a general accuracy claim.

## Missed lessons and next qualification

The vocabulary-mismatch query exposed one missed repair lesson. The cause was a
stale/unverified corpus page, not insufficient semantic similarity. Before a
broader automation decision:

1. refresh or supersede the 14 stale source-revision findings;
2. verify the repair and node-lifecycle lessons against current canonical
   sources;
3. repeat the fixed query set and add negative queries that should retrieve no
   page; and
4. measure reviewer time for capture proposals rather than counting generated
   pages.

## Commands

```bash
fairway --config .fairway/platform-foundation-config.toml knowledge status
fairway --config .fairway/platform-foundation-config.toml knowledge index \
  --embed-command <local-adapter> \
  --embedding-model BAAI/bge-small-en-v1.5 \
  --output .fairway/artifacts/fw415-knowledge-index.json --apply
fairway --config .fairway/platform-foundation-config.toml knowledge query \
  --topic "<fixed-query>" --budget-bytes 8192
fairway --config .fairway/platform-foundation-config.toml knowledge query \
  --topic "<fixed-query>" --budget-bytes 8192 \
  --semantic-index .fairway/artifacts/fw415-knowledge-index.json \
  --embed-command <local-adapter> \
  --embedding-model BAAI/bge-small-en-v1.5
fairway --config .fairway/platform-foundation-config.toml memory cold-start \
  --track ARCH-REPAIR-QUARANTINE-RECOVERY-001 \
  --knowledge-auto --knowledge-budget-bytes 8192
```
