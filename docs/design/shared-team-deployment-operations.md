# Shared-Team Deployment And Operations Model

FW-259 defines the deployment and operations model for future shared-team
Fairway. It is a design model only. It does not deploy a service, expose a
public endpoint, restart dashboards, cut a release, implement a server runtime,
enable dashboard writes, send provider prompts, migrate storage, or authorize
merge/deploy/live-operation behavior.

Shared-team Fairway is introduced only after the operating model, server/API
identity boundary, and concurrency model are reviewed:

- [shared-team-operating-model.md](shared-team-operating-model.md),
- [shared-team-server-api.md](shared-team-server-api.md),
- [shared-team-concurrency-and-sync.md](shared-team-concurrency-and-sync.md),
- [postgres-adapter.md](postgres-adapter.md).

## Deployment Topologies

| Topology | Intended use | Notes |
|---|---|---|
| Local single host | One operator or one team control-room host. | Default. SQLite and loopback dashboard. |
| Shared read-only dashboard | Team visibility with writes still local CLI. | Loopback Fairway origin plus trusted tunnel/proxy. |
| Single-host team server | Small team with one internal Fairway host and multiple remote writers. | Future pilot. Requires authn/authz, backup, and rollback. |
| Airgap/internal server | Private network or airgap environment with no external identity provider. | Use loopback/private TLS and the customer-controlled `sovereign_signed` identity profile; shared API tokens and unverified proxy headers do not satisfy sovereign identity readiness. Use encrypted local backups and explicit restore/key-recovery drills. |
| Managed shared control plane | Longer-lived team deployment with server-backed store, monitored service, and release gates. | Future supported mode after pilot evidence. |

No topology assumes Cloudflare. Cloudflare Access is one reference shared
read-only pattern. Pomerium, Tailscale, internal OIDC proxies, mTLS, VPN, or
private-network controls may be appropriate when their trust boundaries are
documented and verified.

## Non-Goals

- No SaaS-hosted Fairway product.
- No unauthenticated LAN write surface.
- No public write-capable dashboard.
- No dashboard-originated provider sends, approvals, merges, deploys, releases,
  or live operations.
- No automatic migration from local SQLite to a shared store.
- No hidden background sync between writable stores.

## Service Boundaries

A shared-team deployment has three separable services:

1. Fairway server/API process.
2. Fairway store, initially SQLite for local/single-host and future Postgres for
   shared write mode.
3. Optional identity-aware proxy/tunnel/load balancer.

Operators may run read-only dashboard sharing without a shared write API. They
may also run a private API pilot without public dashboard exposure. Do not tie
these switches together.

## Secrets And Identity Operations

Secrets must come from the operating environment, not committed config:

- API token material from environment variables, OS credential stores, or a
  deployment secret manager;
- Postgres DSN from an environment variable such as `FAIRWAY_POSTGRES_DSN`;
- TLS private keys from the platform secret store or reverse proxy;
- webhook/notifier tokens from environment variables;
- Cloudflare/Pomerium/Tailscale/OIDC secrets outside Fairway state.

Fairway may log token fingerprints, issuer, audience, mode, provider, and
verification result. It must not log raw JWTs, cookies, bearer tokens, service
tokens, client secrets, private keys, prompt bodies, private transcripts, or raw
tool bodies.

## TLS And Network Posture

Default local Fairway binds to `127.0.0.1`. Any non-loopback deployment must
have an explicit network posture:

- loopback plus tunnel/proxy for shared read-only dashboard;
- private network or VPN for internal team server;
- TLS/mTLS at the reverse proxy or server boundary;
- firewall rules limiting store and server access;
- origin isolation when identity headers are trusted;
- no direct public origin exposure unless a future reviewed deployment profile
  explicitly allows it.

For write-capable deployments, authentication must fail closed. Read-only
dashboard sharing may stay report-only for identity while mutation handlers
remain disabled.

## Backup And Restore

Every shared-team deployment needs a tested backup/restore posture before it is
trusted for team writes.

SQLite local/single-host:

- stop or quiesce writes before copying the DB when possible;
- copy the DB and WAL/shm files consistently or use Fairway backup/export
  commands when available;
- record provenance manifest for release or audit bundles;
- test restore into a temporary worktree/config.

Postgres/shared store:

- define backup schedule, retention, encryption, and restore owner;
- support point-in-time recovery for production-like deployments;
- record schema version and Fairway binary version with backups;
- run restore rehearsal before promotion to `shared-write-supported`;
- test read-model equivalence after restore.

Backups must not include provider credentials, raw prompt/transcript content,
or secrets outside the Fairway DB unless a separate compliance process owns
them.

Sovereign deployment readiness also requires one
`[[sovereign_crypto_boundaries]]` inventory row for in-transit, at-rest, backup,
evidence-export, and signing protection. `fairway readiness crypto` must identify
the owner, custodian, key/module/configuration, rotation/recovery proof, and any
externally validated module evidence. Fairway does not infer storage encryption
from file permissions and does not claim FIPS validation for itself.

## Upgrades And Rollback

Upgrade procedure for shared deployments:

1. read current dashboard/API status, binary path, version, config path, DB
   schema version, and backup status;
2. run release or pre-release validation from the candidate source/binary;
3. take a backup and record evidence;
4. stop or drain write traffic for schema-changing upgrades;
5. run migrations with a migration lock;
6. start the new binary with explicit pid/log/status paths;
7. run smoke checks: `version`, `config validate`, `ready`, `task-detail`,
   `reconcile active --dry-run`, dashboard/API health, and representative
   read models;
8. observe logs and dashboard timing for the configured window;
9. mark the upgrade done or roll back with evidence.

Rollback must preserve the pre-upgrade backup and identify whether writes
occurred after upgrade. If both old and new stores accepted writes, rollback is
an operator-reviewed reconciliation task, not an automatic command.

## Observability

Minimum signals:

- process health, pid/log path, binary path, version, and config path;
- DB connectivity and schema version;
- dashboard/API route timing and slow projection blocks;
- write success, forbidden, conflict, invalid, and failed counts;
- retryable transaction conflicts;
- notification/wake delivery attempts and failures;
- active session and stale checkpoint reconciliation findings;
- backup freshness and last restore rehearsal;
- disk usage for DB, WAL, logs, and evidence roots.

Log output must remain metadata-only. It should not include raw tokens, cookies,
JWTs, prompts, transcripts, raw tool bodies, or generated content dumps.

## Performance Budgets

Shared-team deployments should publish route and command budgets before support:

- `/board`: under 1s TTFB on the target project size, excluding explicit heavy
  diagnostic panels;
- `/`: under 2s TTFB for wall view;
- `/reports`: under 2-3s TTFB unless rendering export-scale history;
- status-changing commands: complete under 1s except when waiting on DB locks;
- reconcile/merge-ready: bounded by task/project size and reported with timing
  evidence when slow.

Performance fixes should prefer read-model batching, fast paths, snapshot
caching, lazy panels, and query/index improvements before changing deployment
topology. A server-backed store is not a dashboard-performance shortcut.

## Least Privilege

Principles:

- separate viewer, operator, reviewer, coordinator, adapter, and admin scopes;
- no shared admin token for providers;
- no provider credentials in Fairway config or DB;
- DB user has only the privileges Fairway needs;
- migration credentials may be separate from runtime credentials;
- dashboard read-only mode remains incapable of writes regardless of viewer
  identity;
- release/deploy/live-operation actions stay outside the server unless a
  separate reviewed boundary explicitly changes that.

## Disaster Recovery

Disaster recovery runbooks should cover:

- lost dashboard process;
- stale or unknown dashboard listener;
- corrupted local SQLite DB;
- failed Postgres primary or unavailable store;
- failed migration;
- identity proxy outage;
- backup restore to a temporary host;
- split-brain or divergent offline writes;
- lost provider adapter target.

Each runbook should name the command, owner, expected evidence, stop condition,
and rollback action. If recovery crosses production, credential, public
exposure, release, deploy, or live-operation boundaries, it requires explicit
authorization outside the dashboard.

## Release Gates For Shared-Team Runtime

Before any shared-team runtime is advertised as supported, require:

- reviewed FW-256/FW-257/FW-258/FW-259 design packet;
- command parity tests for local CLI and server API;
- authn/authz/security tests;
- migration and backup/restore rehearsal;
- conflict/idempotency/offline promotion tests;
- dashboard read-only and mutation-blocking tests;
- route timing budget evidence at representative data scale;
- operational docs for each supported topology;
- release notes that name known limits and unsupported modes;
- explicit review by arch, backend, security, governance, and ops.

## Dashboard Restart Guidance

Restarting existing dashboards with a new Fairway binary is an operator action,
not part of this design task. Use current lifecycle commands with explicit
pid/log files for multiple instances:

```bash
fairway dashboard restart --listen 127.0.0.1:7878 --read-only \
  --pid-file .fairway/fairway-dashboard-7878.pid \
  --log-file .fairway/fairway-dashboard-7878.log \
  --no-open
fairway dashboard status --listen 127.0.0.1:7878 \
  --pid-file .fairway/fairway-dashboard-7878.pid
```

Record version, binary path, local probe, and proxy boundary probe as evidence.
Do not restart shared dashboards from a design task unless a dedicated reviewed
release/restart task authorizes it.

## Acceptance Checklist

- Deployment topologies are explicit and do not assume Cloudflare.
- Backup, restore/PITR, upgrades, rollback, observability, performance budgets,
  secrets, TLS, least privilege, and disaster recovery are covered.
- Shared read-only dashboard and shared write API are independent switches.
- Release gates are defined before shared-team runtime support.
- The model adds no runtime deployment, server implementation, public exposure,
  dashboard write, provider-send, approval, merge, deploy, release, or
  live-operation authority.
