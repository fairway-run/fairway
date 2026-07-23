# GPUaaS Memory and Knowledge Adoption

Date: 2026-07-23

Fairway task: `FW-377`

GPUaaS source: `ec2b93f67`

## Scope

This increment closed the refinements found by the first integrated GPUaaS
pilot and expanded the model from one SPIFFE/node-agent lane to three current
scale-operations workstreams:

1. `ARCH-REPAIR-QUARANTINE-RECOVERY-001`;
2. `ARCH-FAILURE-UPGRADE-DOMAINS-001`;
3. `ARCH-LOGICAL-WORKLOAD-IDENTITY-RESOLUTION-001`.

Each workstream now has curated Fairway track memory and a bounded,
project-owned engineering-knowledge page. The pages remain draft because their
source documents declare `source_of_truth: false` and
`implementation_state: not-assessed`.

## Product Changes

Cold-start memory packets now report memory disposition separately from current
task status. Checkpoint excerpts include timestamps and the explicit
`historical=true` label under a `newest_first_historical` chronology contract.
Blockers and next actions are deduplicated, and Fairway no longer generates
synthetic `inspect <task> status=<status>` actions.

Knowledge query packets now report the current repository revision and a
freshness label per selected page:

- `current_repository_revision`;
- `current_content_at_recorded_revision`;
- `stale`;
- `unverifiable`.

Knowledge lint now validates canonical source classes against source-document
frontmatter. `source_of_truth: false` is a hard authority conflict.
`implementation_state: not-assessed` produces a warning for canonical
sources. `knowledge lint --fail-on-warning` lets a project intentionally make
review dates and other warnings CI-blocking without changing the default
advisory policy.

## GPUaaS Adoption

GPUaaS commit `ec2b93f67` added three knowledge pages and a narrow
`operations-model` source class rooted only at `doc/operations`. Lint initially
caught an attempted citation of operations documents through the
architecture-only class. The manifest was corrected rather than broadening the
architecture root.

Each track memory cites a current Fairway checkpoint, has an accountable owner
and review date, and carries one concrete next action. No task was claimed or
advanced by the adoption work; all three task statuses remain `todo`.

## Measured Cold Starts

Three separate invocations of the compiled Fairway binary generated packets
from the clean GPUaaS revision. A fixed JSON rubric evaluated only each retained
packet; it did not read the GPUaaS repository, Git history, conversation
history, or legacy `tmp-ux` memory.

| Track | Wall time | Packet bytes | Knowledge bytes / budget | First page |
|---|---:|---:|---:|---|
| Repair lifecycle | 0.50 s | 14,274 | 7,836 / 8,192 | `repair-quarantine-recovery.md` |
| Failure domains | 0.17 s | 12,122 | 7,841 / 8,192 | `failure-upgrade-domains.md` |
| Workload identity | 0.18 s | 12,162 | 7,817 / 8,192 | `logical-workload-identity-resolution.md` |

All three packets:

- reported `memory_disposition=active` and `track_task_status=todo`;
- labeled checkpoint order and historical excerpts explicitly;
- contained no duplicate blockers or next actions;
- contained no generated inspect-status action;
- ranked the intended workstream page first;
- stayed within the separate knowledge budget;
- reported clean GPUaaS revision `ec2b93f67`;
- explained that cited source bytes remain current at their recorded revision
  even though the repository advanced;
- marked no selected source stale.

## Validation

Fairway:

```text
go test ./...
go vet ./...
git diff --check
```

GPUaaS:

```text
fairway knowledge lint
git diff --check
```

GPUaaS knowledge lint reported 8 pages and 0 findings before commit.

## Decision

Keep deterministic, project-owned retrieval as the default. The three expanded
workstreams remain small enough for lexical selection and explicit source
classes. Embeddings, hosted retrieval, and provider-private memory are not
needed for this phase.

The packets improve execution continuity and authority selection. They do not
prove that the target architectures are implemented, reviewed, or ready for
production. Those claims remain with the owning GPUaaS tasks, canonical
contracts, reviews, and live evidence.

Evidence is retained under `docs/assessment/evidence/fw-377/`.
