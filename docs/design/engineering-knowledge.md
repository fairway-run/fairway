# Engineering Knowledge

## Purpose

Fairway engineering knowledge is a project-owned, agent-maintained synthesis of
software architecture, product domains, operational lessons, decisions, and
open questions. It lets human and agent contributors build understanding over
time instead of re-deriving it from raw repositories, task histories, and
runtime artifacts for every question.

The model follows the LLM-wiki pattern of immutable or authoritative sources,
a maintained Markdown synthesis layer, and a project schema that governs
ingest, query, and lint. Fairway supplies the reusable lifecycle, provenance,
and validation framework. Each consumer repository owns its knowledge content.

## Product Boundary

Fairway is not a generic wiki host, transcript warehouse, vector database, or
source-of-truth replacement. Its contribution is the engineering control
contract around project knowledge:

- source registration and provenance;
- bounded ingest and page maintenance;
- deterministic structure and safety lint;
- stale, contradiction, orphan, and promotion findings;
- task-aware context selection;
- explicit movement between working memory, derived knowledge, and canonical
  documentation.

Knowledge pages are derived and non-canonical unless a project explicitly
promotes their content through its normal documentation review process.

## Ownership Split

| Owner | Responsibility |
|---|---|
| Fairway product | Schema, commands, validation, provenance, packets, lifecycle, metrics, and reusable templates |
| Consumer project | Source selection, knowledge pages, domain taxonomy, promotion targets, access policy, and review expectations |
| Git and canonical docs | Authoritative implementation and approved engineering truth |
| Fairway runtime store | Task, decision, evidence, review, and execution facts referenced by knowledge pages |

Project knowledge lives in the consumer repository so it follows the code,
remains portable without a Fairway service, and can be reviewed with normal Git
history.

## Default Project Layout

```text
doc/agent-wiki/
├── README.md
├── index.md
├── current-state.md
├── architecture/
├── product-domains/
├── environments/
├── decisions/
├── operations/
├── incidents-and-lessons/
├── open-questions.md
└── log.md
```

Projects may rename the root, but `index.md`, `open-questions.md`, and `log.md`
retain their semantic roles:

- `index.md` is content-oriented navigation and page metadata;
- `open-questions.md` contains unresolved contradictions and knowledge gaps;
- `log.md` is the bounded chronological ingest/query/lint record.

Working memory is not stored in this directory. It may be an ingest source or a
promotion candidate after verification.

## Page Contract

Every maintained page includes bounded frontmatter:

```yaml
---
knowledge_version: 1
title: Node trust model
status: verified
owner: platform-security
last_verified: 2026-07-22
source_sha: b3b346cd3499bc2ef69dbff28d28890228e11d73
sources:
  - path: doc/architecture/node-trust.md
  - fairway_decision: 123
  - fairway_evidence: 456
supersedes: []
---
```

Allowed initial status values are:

| Status | Meaning |
|---|---|
| `draft` | Derived content not yet checked against all cited sources |
| `verified` | Checked against the named sources and source revision |
| `stale` | Source changes or age require re-verification |
| `conflicted` | Cited authorities disagree and the contradiction is unresolved |
| `superseded` | Replaced by a linked page or canonical document |

`verified` means source-grounded within the declared scope. It does not mean
approved architecture, accepted risk, compliance, or release readiness.

## Source Classes

The project manifest registers source classes and their authority:

```toml
[knowledge]
root = "doc/agent-wiki"

[[knowledge.sources]]
name = "contracts"
path = "doc/api"
authority = "canonical"

[[knowledge.sources]]
name = "architecture"
path = "doc/architecture"
authority = "canonical"

[[knowledge.sources]]
name = "working-memory"
path = "tmp-ux/memory/active"
authority = "advisory"
tracked = false
```

Fairway records source references and digests, not unrestricted copies of
source content in its database.

## Operations

### Ingest

An ingest reads one bounded source set, proposes page changes, updates the
index, and records provenance. It never changes canonical source documents.
Agent-generated changes remain a normal Git diff subject to project review.

### Query

A query starts from `index.md`, selects relevant pages, and includes citations
to their source records. Useful answers can be proposed as new derived pages,
but they remain `draft` until checked against primary sources.

### Lint

Lint reports:

- invalid metadata or unsafe paths;
- missing or inaccessible cited sources;
- source revisions newer than page verification;
- orphan pages and broken links;
- duplicate page identities;
- conflicting claims marked by ingest or reviewers;
- unbounded pages or logs;
- unverified content cited as sole support for another verified page;
- sensitive content and secret-pattern findings;
- promotion targets that are missing or stale.

Deterministic findings are separated from model-suggested semantic findings.
The latter are advisory until a person or configured review accepts them.

### Promote

Stable knowledge may be promoted into canonical documentation. Promotion
requires a target path, reviewed Git commit, and links back to source facts.
Fairway marks the derived page promoted or superseded only after that commit is
recorded. Promotion never occurs silently.

## Proposed Command Direction

```bash
fairway knowledge init
fairway knowledge status
fairway knowledge ingest --source <name-or-path> [--apply]
fairway knowledge query --topic <text> --format packet
fairway knowledge lint
fairway knowledge promote <page> --target <canonical-path>
fairway knowledge archive <page> --reason <text>
```

`init`, `status`, and deterministic `lint` form the first implementation slice.
Ingest starts as preview plus reviewed file changes. Semantic retrieval, graph
projection, and automated maintenance are later capabilities, not MVP
dependencies.

## Relationship To Working Memory

```text
Fairway task facts and project sources
                 |
                 v
         project working memory
          (active and temporary)
                 |
        verified cross-task value
                 v
          engineering knowledge
          (derived and maintained)
                 |
        stable approved contract
                 v
       canonical project documentation
```

The two features remain separate:

- Working Memory optimizes immediate execution continuity.
- Engineering Knowledge optimizes accumulated project understanding.

Knowledge must not become the place where an active task is claimed, blocked,
approved, or completed.

## Cold-Start And Query Acceptance

A new contributor with repository and Fairway access should be able to:

1. find the knowledge index without prior conversation;
2. identify relevant pages for a named task or domain;
3. distinguish canonical sources, verified synthesis, drafts, and conflicts;
4. follow citations to source files, commits, decisions, and evidence;
5. avoid loading the complete project corpus into the provider context;
6. identify stale pages before relying on them;
7. propose, review, and record a correction without overwriting history.

The pilot measures answer grounding, context size, source lookup time,
contradictions found, stale claims, maintenance time, repeated investigation,
and incorrect authority choices.

## Security And Privacy

- Source roots and promotion targets are project-relative and path validated.
- Symlinks and paths escaping configured roots are rejected.
- Raw prompts, transcripts, tool bodies, credentials, auth material, private
  keys, and unrestricted runtime dumps are excluded.
- Pages cite safe evidence references rather than embedding sensitive content.
- Generated text is scanned before write, packet rendering, export, and
  promotion.
- Project access rules apply to knowledge retrieval; Fairway does not widen
  repository or evidence access.

## MVP And Deferred Scope

The MVP includes:

- project manifest and scaffold;
- page metadata and index contract;
- deterministic status and lint;
- provenance references;
- working-memory promotion proposal;
- bounded task/provider context packet;
- GPUaaS cold-start pilot.

Deferred until measured need:

- embeddings or a vector database;
- hosted knowledge service;
- cross-project semantic search;
- automatic writes triggered by every commit;
- graph database projection;
- autonomous conflict resolution or canonical-doc promotion.

At moderate scale, Markdown, Git, explicit indexes, and repository search are
the preferred implementation. More infrastructure requires evidence that the
simple model no longer meets retrieval or maintenance goals.

