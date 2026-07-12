# Issue Tracker Integrations

Fairway should integrate with existing planning systems without making them the
source of truth for agent execution. Plane, Jira, and Linear are first-class
targets: Plane because it is open source, self-hostable, and useful as an
external team/product collaboration surface; Jira because many teams already use
it for epics, stories, bugs, and release tracking; Linear because its issues,
projects, cycles, labels, and lightweight status model are a strong fit for fast
agent-driven execution. The same boundary should also fit GitHub Issues and
similar tools.

## Principle

External issue trackers own product planning, stakeholder visibility, external
team collaboration, and roadmap discussion. Fairway owns local agent
coordination: task state, owner, evidence, handoffs, reviews, sessions,
checkpoints, and merge-readiness facts.

Integration must be explicit and operator-driven by default. No background sync
should overwrite DB-owned execution state.

## Why Not Just Jira Or Linear?

Jira and Linear are excellent planning and tracking systems, but they do not
model the operational details of multi-agent coding well enough to be fairway's
execution store.

Fairway tracks facts that issue trackers usually flatten into comments, labels,
or status fields:

- which role/lane owns the task right now,
- which worktree and branch are active,
- which agent session is alive or stale,
- what evidence command was run and whether it passed, failed, was skipped, or
  was blocked,
- who handed off to whom and why,
- which review route applies and whether the task is merge-ready,
- which checkpoint or watcher is stale,
- what the coordinator should do next.

Trying to encode those as Jira/Linear custom fields would make the integration
fragile and vendor-specific. Fairway keeps those execution facts local and
structured, then exports concise summaries back to the planning tool when that
helps humans.

## Storage And Tracking Model

```text
Plane / Jira / Linear / GitHub Issues
  product planning, stakeholder visibility, external team collaboration,
  external roadmap, epics, cycles

Fairway
  local execution truth, agent coordination, evidence, handoffs, reviews

Git
  code truth, branches, commits, PRs

CI
  verification truth, builds, tests, deploy checks
```

External trackers may seed and observe fairway; they must not silently override
fairway's execution facts.

| System | Owns | Fairway interaction |
|---|---|---|
| Plane / Jira / Linear | Product intent, roadmap grouping, issue discussion, external team collaboration, stakeholder status. | Import task definitions, store external links, export summaries. |
| Fairway DB | Local task state, lane ownership, sessions, evidence, handoffs, reviews, checkpoints. | Source of truth for coordination and merge-readiness. |
| Git | File changes, branches, commits, PR refs. | Referenced by fairway tasks, evidence, reviews, and merge checks. |
| CI | Automated verification results and artifacts. | Recorded as fairway evidence; not executed by fairway core. |

## Sync Direction

Integration should follow four explicit moves:

1. Import external issues into fairway task definitions.
2. Link fairway task IDs to external issue keys or URLs.
3. Execute and track agent work in fairway.
4. Export concise status, blocker, evidence, or completion summaries back to
   the external tracker when requested.

`fairway tracker reconcile --dry-run` should detect drift and propose actions,
for example:

- external issue closed while the fairway task is still `in_progress`,
- fairway task done while the external issue is still open,
- external priority changed,
- linked external issue was archived or deleted,
- new external blocker appeared.

Reconcile should show proposed changes first. It should not mutate local
execution state or remote tracker state without an explicit operator action.

## Provider-Neutral Adapter Contract

Tracker adapters implement a planning mirror contract. The contract is shared
by Plane, Jira, Linear, and future providers; provider-specific packages may
translate fields, but Fairway core only deals in provider-neutral operations and
links.

Required operations:

| Operation | Purpose | Mutation boundary |
|---|---|---|
| `configure` | Validate provider URL/workspace/project/team settings and credential source. | No Fairway state or remote writes in dry-run. |
| `dry_run_import` | Preview external issues as Fairway task definitions. | No Fairway task writes until an explicit apply path exists. |
| `link` | Store a Fairway task ID to external issue reference. | Mutates only `tracker_links`. |
| `export_status` | Render a completion/blocker/status summary for a linked external issue. | Remote comment/status only when an adapter apply command exists; never mutates Fairway execution state. |
| `export_comment` | Render an evidence/review/handoff summary for a linked external issue. | Remote comment only when explicitly applied. |
| `reconcile` | Report drift between linked Fairway tasks and external tracker state. | Advisory by default; no local or remote mutation. |
| `resolve` | Normalize an external id or URL into a provider reference. | No mutation. |

The initial Go contract lives in `internal/tracker`. It defines:

- provider registry for `plane`, `jira`, and `linear`,
- provider-neutral task, issue, reference, mapping, and reconcile result types,
- default field mappings that distinguish planning mirror fields from Fairway
  execution truth,
- dry-run reconcile actions that report drift checks without mutating anything.

Provider-specific adapters should live under `internal/tracker/<provider>` only
after the generic contract is sufficient.

## Commands

```bash
fairway tracker configure plane --url <url> --workspace <slug> --project <id-or-slug>
fairway tracker import plane --query <filter> [--parent <task-id>] [--dry-run]
fairway tracker configure jira --url <url> --project <key>
fairway tracker import jira --query <jql> [--parent <task-id>] [--dry-run]
fairway tracker configure linear --workspace <name> --team <key>
fairway tracker import linear --filter <text> [--parent <task-id>] [--dry-run]
fairway tracker link <task-id> --provider <plane|jira|linear> --external-id <id> [--url <url>]
fairway tracker export-status <task-id> [--provider <plane|jira|linear>] [--external-id <id>] [--dry-run]
fairway tracker resolve --provider <plane|jira|linear> [--external-id <id>] [--url <url>]
fairway tracker reconcile [--dry-run]
fairway tracker plane export --task-id <task-id>
fairway tracker plane import --fixture examples/tracker-adapters/plane/evaluation-workspace.yaml
fairway tracker plane comment --task-id <task-id> --external-id <plane-issue-id>
```

The initial adapter can live behind a generic tracker interface:

| Fairway concept | Tracker mapping |
|---|---|
| `task_definitions.id` | Local ID; external key stored as metadata/link. |
| `title` | Issue summary/title. |
| `notes` | Issue description plus selected custom fields. |
| `kind` | Issue type, normalized by config. |
| `parent_id` | Epic/story/subtask relationship when supported. |
| `priority` | Configured priority mapping. |
| `dependencies` | Issue links such as blocks/depends-on. |
| `acceptance_checks` | Acceptance criteria field, checklist, or labels. |

Mutable fairway state should not be mirrored blindly. A status export should
write a concise comment or transition only when explicitly requested.

## Plane Adapter

Plane support should start before Jira/Linear adapter implementation because it
can be self-hosted locally and used as a realistic open-source planning surface
for product, external teams, and stakeholders.

The first Plane task is not adapter code. It is a local setup and evaluation:

- run Plane locally with documented Docker or compose steps,
- create a Fairway evaluation workspace/project,
- model representative consumer/Fairway epics, tasks, follow-ups, cycles/modules,
  labels, and comments,
- evaluate which Plane concepts map cleanly to Fairway tasks, parents,
  profiles, roles, review domains, evidence links, and follow-up taxonomy,
- record which Plane concepts should remain planning-only and never mutate
  Fairway execution state.

The repeatable local setup and seed fixture live in
[plane-local-evaluation.md](../operations/plane-local-evaluation.md) and
`examples/tracker-adapters/plane`.
That evaluation should be completed before FW-121 defines the provider-neutral
adapter contract.

After that evaluation, Plane adapter support should start with:

- one-way export from Fairway tasks to Plane issues,
- link storage between Fairway task IDs and Plane issue identifiers/URLs,
- optional import of Plane issues explicitly selected for execution,
- optional status/comment export for completion, blockers, evidence, review
  outcomes, and daily report links,
- config-driven mapping for workspace, project, issue states, labels, modules,
  cycles, priorities, and custom fields where available,
- dry-run output before any remote write.

Plane credentials should come from the environment or OS credential store, not
from `.fairway/config.toml`.

The first Plane adapter spike is dry-run only:

```bash
export PLANE_BASE_URL=http://localhost:8088
export PLANE_WORKSPACE=fairway-eval
export PLANE_PROJECT=FWPLANE
# optional for future apply support; not printed or committed
export PLANE_API_TOKEN=...

fairway tracker plane export --task-id FW-122
fairway tracker plane import --fixture examples/tracker-adapters/plane/evaluation-workspace.yaml
fairway tracker plane comment --task-id FW-122 --external-id FWPLANE-122
```

These commands render Plane issue/comment/import payloads from local Fairway
state and fixture data. They do not call Plane, do not create/update Plane
issues, do not import Fairway tasks, and do not mutate Fairway execution state.
Passing `--apply` currently fails with an explicit unsupported error; a future
adapter may add apply operations only with dry-run parity tests and credential
handling outside committed config.

## Jira Adapter

Jira support should start with:

- one-way import from JQL into fairway task definitions,
- link storage between fairway task IDs and Jira issue keys,
- optional status/comment export for task completion, blockers, and evidence,
- config-driven mapping for project, issue types, priorities, labels, and custom
  fields,
- dry-run output before any remote write.

Jira credentials should come from the environment or OS credential store, not
from `.fairway/config.toml`.

## Linear Adapter

Linear support should start alongside Jira, not as a distant follow-up:

- one-way import from Linear team/project/cycle filters into fairway task
  definitions,
- link storage between fairway task IDs and Linear issue identifiers/URLs,
- mapping Linear projects or initiatives to fairway epics/stories,
- mapping Linear cycles to optional fairway import labels or metadata, not to
  required execution windows,
- optional status/comment export for task completion, blockers, evidence, and
  review outcomes,
- config-driven mapping for teams, states, priorities, labels, projects, and
  custom fields,
- dry-run output before any remote write.

Linear is especially useful for tracking discovered follow-up work. `fairway
spawn --sibling` or `fairway spawn --child` should be able to link a new local
task to a new or existing Linear issue when the operator asks for it, while
keeping the fairway DB authoritative for who is working, what evidence exists,
and whether the task is merge-ready.

Linear credentials should come from the environment or OS credential store, not
from `.fairway/config.toml`.

## Non-Goals

- Fairway does not become a replacement Plane UI.
- Fairway does not become a replacement Jira UI.
- Fairway does not become a replacement Linear UI.
- Fairway does not continuously sync status in the background.
- Fairway does not require Plane, Jira, Linear, or any tracker for core
  workflow.
- Fairway does not treat tracker workflow states as the local state machine
  unless the operator intentionally maps them.
