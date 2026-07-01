# Shared-Team Fairway Operating Model

Fairway is local-first. A single binary plus SQLite remains the default for one
developer, one coordination host, local demos, airgap single-host work, and
temporary drill rooms. Shared-team Fairway is justified only when multiple
operators, reviewers, provider lanes, dashboards, or utility monitors need to
coordinate the same Fairway project from different machines.

Shared-team mode is not just a Postgres backend. The store is one component of
a larger operating model that includes authority boundaries, identity, audit,
tenancy, read/write APIs, provider attachment, offline fallback, exports, and
deployment operations.

## Decision Rule

Stay local-first when:

- one person or one workstation owns all writes;
- a shared read-only dashboard is enough for observers;
- multi-project visibility can be handled by the local dashboard registry;
- the work is an isolated demo, drill, or airgap rehearsal with simple file
  backup/restore;
- the cost of running a shared service is higher than the coordination risk.

Use a shared team control plane only when:

- more than one trusted writer must update the same project state;
- review, provider, coordinator, and operator lanes run on different hosts;
- dashboard observers need consistent cross-machine state without copying DB
  files;
- audit and backup requirements exceed a local SQLite file;
- stale handoff, wait, or session state must be visible to the team even when a
  single laptop is offline.

Postgres or another server-backed store may support this mode, but it does not
create the mode by itself. The product must also define identity, API,
conflict, and operations boundaries.

## Authority Boundaries

Shared-team mode keeps the same principle as local Fairway: Fairway records and
surfaces coordination facts; it does not perform or approve the underlying
engineering work.

| Surface | May do | Must not do |
|---|---|---|
| Dashboard | Read task state, waits, sessions, evidence references, reports, diagnostics, identity confidence, and audit summaries. | Send provider prompts, approve reviews, mutate tasks, merge, push, deploy, run live operations, or hold provider credentials. |
| CLI | Perform explicit trusted-operator mutations such as status changes, evidence, reviews, handoffs, waits, sessions, imports, and release checks. | Hide authority behind background sync, silently claim work, or bypass review/profile gates. |
| Coordinator/watch | Compute deterministic next actions, stale waits, missed handoffs, and bounded wake attempts through configured adapters. | Approve, merge, deploy, execute live actions, or override stop conditions. |
| Provider adapters | Attach sessions, record lifecycle, usage metadata, evidence, and delivered notifications. | Own task truth, store raw prompts/transcripts/secrets by default, or decide approval/merge/deploy outcomes. |
| Reviewers | Record review verdicts for named domains. | Self-review their own lane or waive unrelated domains without a configured policy. |
| Operators | Execute live/deploy/credential actions only outside Fairway after explicit authorization. | Treat dashboard visibility or coordinator recommendation as execution authorization. |
| Automation utilities | Record deterministic evidence and monitor handbacks. | Become a CI runner, workflow engine, deploy runner, or cleanup engine. |

Any future shared write dashboard must be a separate reviewed product boundary.
The current shared dashboard remains read-only.

## Roles And Identity

Shared-team Fairway needs identity for accountability, not for hidden authority.
Every write should be attributable to a human, provider session, utility
adapter, or service account with a scoped purpose.

Minimum identity model:

- writer identity: actor ID, role/lane, provider or auth source, and session ID;
- viewer identity: optional read-only identity confidence from a trusted proxy;
- service identity: named utility adapter or coordinator with configured scope;
- review identity: domain reviewer, verdict, reviewed commit or artifact, and
  route reason;
- task ownership: owner/lane remains a Fairway field, independent of auth user.

Identity headers from Cloudflare Access, OIDC proxies, or similar systems are
trusted only when the origin is reachable exclusively through that proxy and the
configured verifier passes. Until then, proxy identity is advisory display
metadata; it does not authorize writes.

Future server APIs must require explicit authentication for write endpoints and
record enough actor metadata for audit. Credentials must come from environment,
OS credential stores, or deployment secret managers, not committed config.

## Tenancy And Project Scope

Fairway's schema already scopes rows by `project_id`. Shared-team mode keeps
that invariant:

- task IDs are unique within a project, not globally;
- all reads and writes carry one effective project scope;
- cross-project dashboards and reports are read models over registered projects;
- a writer cannot mutate another project by changing a request parameter after
  authentication;
- exports and backups must name the project IDs they include.

For small teams, one Fairway server may host multiple projects only if project
scoping, identity, backup, and audit are explicit. Otherwise, deploy one
Fairway control plane per project boundary.

## Read/Write API Model

The CLI remains the reference mutation surface. A future server API should be a
remote transport for the same command semantics, not a second workflow.

API principles:

- read models may be cached briefly, but the DB remains authoritative;
- writes are command-shaped and auditable;
- each write returns the resulting state or a clear conflict/guard failure;
- server APIs do not accept arbitrary prompt bodies as durable authority;
- command parity with local CLI is required before release;
- dashboard and provider APIs are separated so read-only dashboard exposure
  cannot inherit provider-send or task-mutation authority.

FW-257 owns the detailed server API and identity boundary. FW-256 only defines
the operating expectation.

## Multi-Writer Concurrency

Shared-team mode must expect simultaneous writers. The operating model is
fail-closed and explicit:

- task status transitions validate current state inside one transaction;
- conflicting writes return a conflict with the current task state and next
  safe command;
- append-only evidence, checkpoints, reviews, notifications, and sessions keep
  stable IDs and actor metadata;
- denormalized fields are updated in the same transaction as their audit rows;
- clients refresh from Fairway state after a conflict instead of replaying stale
  chat or local memory.

FW-258 owns the detailed concurrency, sync, conflict, offline replay, and import
model.

## Offline And Local Fallback

Local-first remains important even when a team store exists. A provider lane may
lose network access, work in an airgap, or operate from a laptop during a drill.

Allowed fallback patterns:

- local read-only export packets for review or handoff context;
- local SQLite scratch work for a bounded task, promoted later through an
  explicit import/reconciliation packet;
- evidence artifacts stored locally with references recorded when the team
  store is reachable again;
- read-only dashboard snapshots for observation.

Forbidden fallback patterns:

- silently merging divergent local and team-store writes;
- treating offline provider chat as authoritative state;
- bypassing review or live-operation gates because the team store is offline;
- backdating approvals or status changes without clear actor/time evidence.

When offline work is promoted back to a shared store, Fairway should require a
visible reconciliation packet that names changed tasks, evidence, conflicts,
and any manual decisions.

## Audit And Evidence

Shared-team mode increases the audit requirement. Every state-changing command
should preserve:

- actor identity and role/lane;
- provider/session ID when applicable;
- command source and timestamp;
- task/project scope;
- before/after state for mutable rows;
- evidence or checkpoint reference for non-trivial status changes;
- review and approval domain when applicable.

Fairway still stores references and metadata, not raw secrets, raw prompt
bodies, private transcripts, raw tool bodies, provider auth state, or generated
content dumps by default. Safe evidence viewing and provenance exports remain
bounded by their own redaction and retention rules.

## Dashboard And Control Room

The shared dashboard remains an observation surface:

- wall and board state;
- waits, handbacks, sessions, stale state, and diagnostics;
- read-only task detail;
- delivery, review, usage, rough-edge, recipe, and cross-project reports;
- identity confidence and proxy verification status when available.

It does not send provider prompts or mutate task state. Provider wakes and
external notifications belong to coordinator/provider adapter surfaces with
delivery evidence. Live operations remain operator-executed after explicit
authorization.

A tmux or zellij control room may show shared Fairway state, server logs,
dashboard status, provider panes, and release/deploy monitors. The layout is an
operator convenience, not the source of truth.

## Operating Lifecycle

A shared-team Fairway deployment should have named lifecycle states:

1. `local-only`: default single-binary SQLite use.
2. `shared-read-only`: dashboard exposed through a trusted proxy, with all
   writes still local CLI.
3. `shared-write-pilot`: server API/team store enabled for a small project with
   explicit rollback and audit review.
4. `shared-write-supported`: command parity, backup/restore, identity,
   conflict handling, and operational monitoring are release-gated.

Promotion between states requires evidence. A product team should not jump from
local-only to shared-write-supported because a dashboard is slow or a laptop was
restarted.

## Relationship To Follow-Up Designs

FW-256 defines the operating model. Follow-up tasks own the deeper designs:

- FW-257: shared-team server API, authentication, trusted identity, and
  read/write endpoint boundary;
- FW-258: multi-writer concurrency, sync, conflict handling, offline/local
  fallback, and import/export reconciliation;
- FW-259: deployment topology, operational runbook, backup/restore, monitoring,
  release gates, and dashboard/service lifecycle.

No public exposure, server runtime, storage migration, live operation, deploy
authority, dashboard write authority, or provider-send authority is authorized
by FW-256.

## Acceptance Checklist

- The default recommendation remains local-first SQLite.
- Shared-team mode is justified by multi-writer/team coordination needs, not
  dashboard performance alone.
- Dashboard, CLI, coordinator, provider, reviewer, operator, and automation
  authorities are separated.
- Identity and audit are accountability mechanisms, not hidden approval grants.
- Project scoping and tenancy remain explicit through `project_id`.
- Offline/local fallback is allowed only through explicit promotion and
  reconciliation packets.
- Follow-up tasks are bounded so later server/API/store/runtime work receives
  separate review.
