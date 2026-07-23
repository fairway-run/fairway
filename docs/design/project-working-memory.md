# Project Working Memory

## Purpose

Fairway project working memory is the short-lived, provider-neutral context
needed to resume an active engineering workstream without reconstructing it
from chat history. It formalizes the local `tmp-ux` convention already used by
consumer projects and connects that convention to Fairway's existing
database-backed task, session, checkpoint, decision, evidence, review, and
track-memory records.

Working memory is not canonical project documentation, a transcript archive,
or a second task tracker. It is a compact projection of current execution
state. Fairway remains authoritative for workflow facts and Git remains
authoritative for source state.

## Product Boundary

| Surface | Owns |
|---|---|
| Fairway task and track memory | Durable task state, source-fact references, ownership, review date, and disposition history |
| Project `tmp-ux` memory | Local active-work summary and provider-independent resume context |
| Context packet | Rendered point-in-time execution input for a provider |
| Agent knowledge | Maintained cross-task synthesis derived from cited sources |
| Canonical project documentation | Approved product, architecture, contract, and operational truth |

No memory record grants review, merge, push, deploy, release, credential,
public-exposure, or live-operation authority.

## Project Layout

The default local layout is:

```text
tmp-ux/
├── memory/
│   ├── index.yaml
│   ├── active/
│   │   └── <track-id>.md
│   └── archive/
├── handoffs/
├── presentations/
└── experiments/
```

Projects may configure a different root, but the semantic roles remain the
same. Existing flat `tmp-ux/*memory*.md` files are migration inputs and remain
readable until explicitly archived or promoted.

The local memory directory is untracked by default. A project that chooses to
version active memory must make that policy explicit and must still preserve
the authority hierarchy below.

## Active Memory Contract

Each active file uses bounded YAML frontmatter:

```yaml
---
memory_version: 1
project: gpuaas
track: node-recovery
status: active
updated_at: 2026-07-22T20:15:00Z
source_sha: b3b346cd3499bc2ef69dbff28d28890228e11d73
fairway_tasks:
  - NRM-901
active_session: codex-node-recovery-r1
authority: working-memory
contains_secrets: false
---
```

Required sections are:

```markdown
# Working Memory: <track>

## Objective
## Current Verified State
## Decisions
## Active Work
## Blockers
## Next Actions
## Evidence
## Promotion Candidates
## Recent Changes
```

`Current Verified State` must distinguish verified facts, hypotheses, and
superseded facts. Runtime claims name their environment, observation time, and
source SHA when applicable. Evidence is linked rather than copied.

## Index Contract

`tmp-ux/memory/index.yaml` is the machine-discoverable entry point. It contains
only bounded metadata:

```yaml
version: 1
active:
  - track: node-recovery
    path: memory/active/node-recovery.md
    fairway_track: node-recovery
    updated_at: 2026-07-22T20:15:00Z
```

The index does not copy memory content. A new provider reads the index, selects
only the relevant track, and then asks Fairway for current task and track facts.

## Authority And Precedence

When records disagree, use this order:

1. Code, contracts, schemas, and external systems for facts they execute.
2. Canonical project architecture and operations documentation.
3. Fairway task, decision, evidence, review, session, and checkpoint records.
4. Maintained agent knowledge.
5. Project working memory.
6. Provider conversation history.

A memory conflict with a higher authority is a stale-memory finding, not a
reason to reinterpret the higher authority.

## Lifecycle

```text
initialize -> refresh -> handoff/resume -> compact -> promote -> archive
```

- **Initialize:** create the indexed file from the active Fairway track and
  current Git state.
- **Refresh:** rewrite current state at meaningful transitions rather than
  appending narration.
- **Handoff/resume:** render a bounded packet combining the local memory with
  current Fairway facts.
- **Compact:** retain current truth and a short recent-change window; move old
  chronology to archive or durable Fairway history.
- **Promote:** move stable cross-task knowledge into agent knowledge or
  canonical documentation with source references.
- **Archive:** mark the workstream inactive, preserve disposition metadata, and
  remove it from normal context selection.

Suggested defaults are 50 KiB or 500 lines per active file, a maximum of 20
recent-change entries, and a 24-hour stale threshold for actively executed
tracks. Projects can configure tighter limits.

## Fairway Integration

Fairway already provides `memory show|update|append|packet|stale|reconcile|
disposition|history`. The project-memory increment should add a filesystem
contract around those durable records rather than create a second memory
database.

Proposed command direction:

```bash
fairway memory init --track <track-id>
fairway memory status [--track <track-id>]
fairway memory lint [--track <track-id>]
fairway memory compact --track <track-id>
fairway memory promote --track <track-id> --target <path>
fairway memory archive --track <track-id> --reason <text>
fairway memory cold-start --track <track-id> --for <provider>
```

Commands default to preview when they would rewrite, move, promote, or archive
files. They do not infer task completion or approval from memory text.

## Deterministic Lint

The first release uses deterministic checks only:

- required frontmatter and sections;
- unique active track mapping;
- referenced paths and Fairway tasks exist;
- source SHA is syntactically valid and its relationship to the current worktree
  is reported;
- active memory is within configured size and freshness limits;
- terminal tasks are not presented as active work;
- promotion targets are inside allowed project paths;
- obvious secrets, credentials, private keys, raw authorization values, and
  provider-private payloads are rejected;
- archived memory is excluded from default cold-start packets.

LLM suggestions may help compact or classify content, but deterministic
validation remains authoritative for the contract.

## Cold-Start Acceptance Test

A provider receives only the repository and Fairway access. Without prior chat
history it must be able to:

1. discover the active memory index;
2. identify the relevant track;
3. state the current objective and exact next action;
4. name the current source SHA and environment for runtime claims;
5. distinguish verified facts from hypotheses;
6. locate supporting Fairway evidence and reviews;
7. avoid repeating a recorded failed approach;
8. identify which facts require refresh.

The pilot records resume time, missing-context questions, stale findings,
incorrect authority choices, file-maintenance time, and repeated investigation.
The feature is successful only if it lowers reconstruction cost without adding
equivalent ceremony.

## Security And Privacy

- Never store credentials, tokens, cookies, private keys, raw authorization
  headers, private transcripts, or unrestricted tool output.
- Prefer identifiers, digests, safe paths, and Fairway evidence references.
- Keep local untracked memory out of release and artifact bundles unless an
  explicit reviewed export includes it.
- Scan before packet rendering, promotion, archival export, and diagnostics.
- Do not send memory to a provider that is not authorized for the referenced
  project or evidence.

## Adoption Sequence

1. Add the schema, index, lint, and cold-start contract without migrating files.
2. Convert one active GPUaaS track as a bounded pilot.
3. Compare cold-start behavior with the existing flat memory file.
4. Add compaction and promotion only after the pilot establishes useful rules.
5. Migrate remaining active files gradually; archive rather than bulk-delete
   historical files.

