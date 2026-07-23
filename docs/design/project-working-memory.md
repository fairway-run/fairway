# Project Working Memory

## Purpose

Fairway already owns durable project working memory. The `track_memory` and
`track_memory_lifecycle` records in the Fairway store preserve curated,
provider-neutral context across tasks, sessions, provider replacement, and
context loss.

The remaining product work is to finish migration from legacy project-local
`tmp-ux/*memory*.md` files, make cold-start behavior complete, and prevent a
second memory authority from returning.

Working memory is not canonical documentation, a transcript archive, or a
second task tracker. It is a compact projection of current execution state
backed by Fairway source facts.

## Existing Fairway Capability

Fairway currently provides:

```bash
fairway memory show [--track <track-id>]
fairway memory update --track <track-id> ...
fairway memory append --track <track-id> ...
fairway memory packet --track <track-id> [--for <provider>]
fairway memory stale [--older-than <duration>]
fairway memory reconcile [--older-than <duration>]
fairway memory disposition --track <track-id> ...
fairway memory history [--track <track-id>]
```

The store already validates active-memory ownership, review dates, and source
checkpoint, evidence, or review references. It records explicit `active`,
`promote`, `archived`, and `superseded` lifecycle dispositions and includes
memory in backup, export, restore, dashboard, wait, and reconciliation
surfaces.

This design does not replace those capabilities.

## Authority

| Surface | Owns |
|---|---|
| Fairway task and track memory | Durable task state, curated working memory, source-fact references, ownership, review date, and disposition history |
| Context packet | Rendered point-in-time provider input from current Fairway facts |
| Legacy `tmp-ux` memory | Migration input only; local and non-authoritative |
| Engineering knowledge | Maintained cross-task synthesis derived from cited sources |
| Canonical project documentation | Approved product, architecture, contract, and operational truth |

When records disagree, use this order:

1. Code, contracts, schemas, and external systems for facts they execute.
2. Canonical project architecture and operations documentation.
3. Fairway task, decision, evidence, review, checkpoint, and track-memory
   records.
4. Maintained engineering knowledge.
5. Legacy local memory files.
6. Provider conversation history.

No memory record grants review, merge, push, deploy, release, credential,
public-exposure, or live-operation authority.

## Canonical Track-Memory Contract

The existing Fairway record is the canonical working-memory schema:

```text
track_id
title
purpose
operating_mode
active_scope
current_objective
decisions[]
blockers[]
open_questions[]
next_actions[]
source_checkpoint_ids[]
source_evidence_ids[]
source_review_ids[]
owner
review_by
disposition
promotion_target
canonical_commit
superseded_by_track_id
updated_at
```

Active memory requires an owner, review date, and at least one existing Fairway
source fact. Repeated progress narration, raw logs, prompts, and transcripts do
not belong in track memory.

## Legacy `tmp-ux` Posture

Projects historically used local untracked files such as:

```text
tmp-ux/gpuaas-program-execution-memory-2026-06-16.md
tmp-ux/gpuaas-node-runtime-trust-memory-2026-07-13.md
```

These files demonstrated the need for durable memory but are no longer the
target architecture. They are:

- local migration sources;
- non-authoritative;
- not required for a new provider to resume work;
- not silently copied into the Fairway database;
- archived only after coverage and source-fact checks pass.

`tmp-ux` remains available for presentations, experiments, temporary drafts,
and bounded scratch work. Files used as active durable memory are deprecated.

## Migration Workflow

The migration is explicit and preview-first:

```text
inventory legacy files
  -> classify active, historical, duplicate, or canonical candidate
  -> extract a bounded proposed Fairway memory record
  -> validate cited Fairway source facts
  -> operator applies the record
  -> compare cold-start packets
  -> promote durable knowledge or docs candidates
  -> archive the legacy file
```

Proposed command direction:

```bash
fairway memory import --file <tmp-ux-file> --track <track-id>
fairway memory coverage [--root tmp-ux]
fairway memory cold-start --track <track-id> --for <provider>
fairway memory retire-file --file <path> --track <track-id> --reason <text>
```

`import` defaults to a structured preview. Apply mode writes only bounded
fields accepted by the existing track-memory store. It never treats every
heading, historical event, or generated sentence as durable memory.

`coverage` reports which legacy memory files are represented by active,
promoted, archived, or superseded Fairway records and which still require a
disposition. It does not infer that a task is done.

`retire-file` records the migration evidence and proposed file action. It does
not delete a file by default.

## Cold-Start Completion

`fairway memory packet` is already the provider-independent resume surface. The
remaining cold-start increment should ensure the packet includes or links:

- current task and dependency state;
- active session and latest checkpoint;
- accepted decisions and unresolved questions;
- bounded evidence and review references;
- current branch, commit, and worktree posture when available;
- exact next action and stop condition;
- stale-memory and source-fact warnings.

The packet must select one track explicitly. It does not load every memory
record or legacy file into provider context.

When domain context is useful, this command composes with Engineering Knowledge
through one bounded cold-start response. Track memory is rendered first and may
name relevant knowledge topics; only those indexed pages are added within a
separate knowledge budget. Duplicate citations are collapsed. Knowledge is
optional and cannot displace the current objective, blocker, stop condition, or
next action. The detailed composition contract is defined in
`docs/design/engineering-knowledge.md`.

## Deterministic Migration Checks

The first increment uses deterministic checks only:

- the target track exists or is explicitly named for creation;
- active memory has owner, review date, and valid source facts;
- task, checkpoint, evidence, and review references exist;
- proposed text is bounded and secret-scanned;
- raw logs, authorization material, private keys, and transcript bodies are
  rejected;
- a legacy file cannot be marked covered by an unrelated track;
- terminal tasks are not projected as active work;
- promoted memory requires a canonical target and commit;
- archived or superseded memory is excluded from default cold-start packets.

Model-assisted extraction may propose field values, but deterministic
validation and explicit apply remain authoritative.

## Cold-Start Acceptance Test

A provider receives only the repository and Fairway access. Without prior chat
history or `tmp-ux` files it must be able to:

1. identify the relevant Fairway track;
2. state the current objective and exact next action;
3. distinguish verified facts from unresolved questions;
4. locate supporting task, decision, evidence, checkpoint, and review facts;
5. identify the current source and environment posture when relevant;
6. avoid repeating a recorded failed approach;
7. identify stale memory requiring refresh.

The GPUaaS pilot records resume time, clarification count, stale findings,
repeated investigation, maintenance time, and incorrect authority choices. The
migration is successful only if Fairway memory replaces reliance on private
file conventions without adding equivalent ceremony.

## Security And Privacy

- Never import credentials, tokens, cookies, private keys, raw authorization
  headers, private transcripts, or unrestricted tool output.
- Prefer identifiers, digests, safe paths, and Fairway source references.
- Keep `tmp-ux` out of release and evidence bundles unless an explicit reviewed
  export includes a bounded artifact.
- Scan before import preview, apply, packet rendering, and migration evidence.
- Do not send memory to a provider unauthorized for the referenced project or
  evidence.

## Adoption Sequence

1. Add legacy-memory inventory, import preview, and coverage reporting around
   the existing Fairway memory store.
2. Complete the bounded cold-start packet using existing Fairway facts.
3. Migrate one active GPUaaS track and compare against the legacy file.
4. Promote stable cross-task content into engineering knowledge or canonical
   documentation.
5. Archive legacy memory gradually after coverage proof; do not bulk-delete
   historical files.
