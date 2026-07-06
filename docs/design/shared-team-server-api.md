# Shared-Team Server API And Identity Boundary

FW-257 defines the server/API and identity boundary for future shared-team
Fairway. It is a design model only. It does not implement a server runtime,
public exposure, storage migration, dashboard write authority, provider-send
authority, review approval authority, merge/deploy/live-operation authority, or
release behavior.

The shared-team operating model in
[shared-team-operating-model.md](shared-team-operating-model.md) remains the
source for when a shared control plane is justified. This document answers what
a future server process may expose and how identity must be treated.

## Need For A Server Process

Fairway does not need a server for the default local-first workflow. Local
SQLite plus CLI/dashboard remains the supported baseline.

A server process is justified only when a team needs:

- multiple trusted writers on different machines;
- consistent read models for shared dashboards without copying SQLite files;
- provider/session adapters reporting from remote hosts;
- central audit and backup posture;
- optional external notifier or utility monitor callbacks;
- a single control-room endpoint for a team deployment.

Do not introduce a server process merely to make dashboard routes faster. Route
performance remains a read-model concern handled by batching, fast paths,
snapshot caching, and lazy panels.

## Process Shape

A future shared server should be a long-running Fairway process with explicit
modes:

| Mode | Purpose | Write authority |
|---|---|---|
| `dashboard-read-only` | Shared observation surface behind a trusted proxy. | None. |
| `api-read-only` | Remote CLI/report/dashboard reads. | None. |
| `api-write-pilot` | Limited command-shaped writes for a small team pilot. | Explicit authenticated command API only. |
| `api-write-supported` | Release-gated shared write control plane. | Explicit authenticated command API only. |

The dashboard surface and the API surface must remain separable. Enabling a
server API must not accidentally make the shared dashboard write-capable.

## Implemented Read-Only Skeleton

FW-269 implements only the `api-read-only` skeleton:

```bash
fairway server --read-only [--listen 127.0.0.1:7880]
```

It serves metadata-only JSON from the existing Fairway store/read models:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/status` | Project and read-only server status. |
| `GET /api/v1/tasks` | Task list rows. |
| `GET /api/v1/tasks/<task-id>` | Task detail with transitions, evidence, handoffs, and reviews. |
| `GET /api/v1/reports/summary` | Project task-status summary. |

The skeleton has no write endpoints. `[server] mode = "read_only"` is the only
enabled mode; write-capable modes and `write_enabled = true` fail config
validation. Because FW-269 has no identity verification or authorization, the
server refuses non-loopback bind addresses. Loopback addresses such as
`127.0.0.1:7880`, `localhost:7880`, and IPv6 loopback are allowed; `0.0.0.0`,
LAN/private, Tailscale, and public interfaces require FW-270 or a later
reviewed identity/proxy/deployment boundary. This slice does not implement
identity verification, API token authorization, shared writes, public exposure,
dashboard write behavior, provider-send authority, review approval authority,
merge/deploy/live-operation authority, or release behavior.

FW-270 adds the first identity and command authorization guard for the
read-only API. The guard supports these configured identity modes:

| Identity mode | FW-270 behavior |
|---|---|
| `no_edge_local` | Local loopback read-only use; actor role is `viewer`. |
| `trusted_proxy_read_only` | Requires `trusted_proxy_verified = true`, a proof header, identity header, and optional issuer/audience header checks. Without verified proof, proxy identity is advisory and cannot authorize reads. |
| `api_token` | Requires `Authorization: Bearer` to match the token in `api_token_env`; accepted token roles must also appear in `allowed_roles`, bearer proof uses constant-time comparison for equal-length values, and only a token fingerprint is used internally. |
| `service_account` / `mtls_service_account` | Accepted config placeholders that fail closed at runtime until proof verification is implemented. |

Authorization is command-scoped. FW-270 implements `read:api` for `viewer` and
`admin` roles only, with allowed roles declared in `[server].allowed_roles`.
Error responses are bounded reason codes/messages and do not echo raw JWTs,
headers, cookies, bearer tokens, API tokens, prompts, transcripts, or provider
private data. This still does not add shared writes, review approval, dashboard
mutation, provider sends, merge, deploy, release, public exposure, or live
operation authority.

## API Families

Read APIs may expose the same information already available through CLI and
dashboard read models:

- task list, ready queue, task detail, and state history;
- evidence, reviews, handoffs, checkpoints, sessions, waits, and notifications;
- review waits, completion handbacks, live-window phases, and generic waits;
- dashboard board/wall/report projections;
- workflow, merge-ready, config, and reconcile diagnostics;
- provenance, delivery, usage, rough-edge, recipe metadata, and automation
  candidate reports.

Write APIs must be command-shaped. Each endpoint should map to one existing or
planned CLI command family:

- task status: claim, set status, update metadata, import definitions;
- evidence and audit: record evidence, checkpoint, handoff, review,
  notification, completion handback, wait add/ack, usage;
- session lifecycle: upsert, heartbeat, end, reconcile;
- configuration-independent utility results: CI/deploy/UAT monitor evidence,
  provider adapter delivery proof, advisory recommendation evidence.

The API should not expose generic SQL, arbitrary row patching, arbitrary prompt
storage, raw artifact upload by default, provider credential storage, or
dashboard-originated mutation endpoints.

## Local-Only Flows

Some flows should remain local-only unless a later task explicitly designs and
reviews them:

- git staging, commit, push, branch cleanup, and worktree cleanup;
- release tagging, GoReleaser publish, Homebrew/tap update, and dashboard
  restart;
- live/deploy/credential operations;
- editing local config files that include paths, tokens, tunnels, or
  deployment-specific secrets;
- raw evidence artifact viewing outside configured local roots.

The shared server may record evidence that these actions happened, but it does
not execute them.

## Authentication

Every write-capable API request must have a verified actor. Supported identity
sources should be explicit deployment choices:

| Profile | Intended use | Notes |
|---|---|---|
| `no-edge-local` | Loopback-only local development. | No remote writes. |
| `trusted-proxy-read-only` | Cloudflare Access, Pomerium, Tailscale, or internal proxy for dashboard/read APIs. | Identity display may be report-only until verifier support exists. |
| `trusted-proxy-write` | Future write-capable server behind a verified proxy. | Requires fail-closed identity verification and role mapping. |
| `api-token` | Provider/utility adapter writes. | Token value comes from environment/secret manager; only token fingerprint/scope is recorded. |
| `mTLS/service-account` | Internal automation in controlled networks. | Service identity must be scoped to explicit command families. |

Identity headers are not trusted unless the origin is reachable only through the
trusted proxy and the configured verifier passes. JWTs, cookies, bearer tokens,
client secrets, private keys, and service tokens must never be stored in Fairway
state, evidence, logs, or committed config.

## Authorization

Authorization should be explicit and command-scoped. Suggested role scopes:

| Scope | Allows | Does not allow |
|---|---|---|
| `viewer` | Read dashboards, reports, task detail, diagnostics. | Any mutation. |
| `operator` | Record evidence, checkpoints, handoffs, waits, sessions for assigned lanes. | Review approval, merge, deploy, release, live execution. |
| `reviewer:<domain>` | Record review verdicts for the named domain. | Self-review, unrelated domains, merge/deploy authority. |
| `coordinator` | Record deterministic waits, notifications, handbacks, advisory evidence, and session reconciliation. | Approve, merge, deploy, live operations, arbitrary provider sends. |
| `adapter:<name>` | Record scoped adapter events such as usage, session lifecycle, notification delivery, or monitor evidence. | Task status changes unless explicitly configured. |
| `admin` | Manage server config, project registration, backup jobs, and auth policy. | Bypass review/merge/deploy/live gates. |

Authorization is not a substitute for Fairway policy. Even an authenticated
admin must still satisfy configured review, workflow, merge-ready, and release
guards.

## CSRF And Token Boundaries

Browser dashboard requests and API clients have different threat models:

- browser forms require CSRF tokens for any future same-origin mutation;
- read-only dashboards must not render mutation forms when read-only mode is
  active;
- API tokens should use `Authorization: Bearer` or mTLS and must not rely on
  browser cookies;
- cross-origin browser writes should be disabled by default;
- CORS must default to deny;
- JSON APIs should require content type checks for write requests;
- API token fingerprints and scopes may be logged; raw token values must not.

If a future dashboard write mode is designed, it must be reviewed separately
and remain disabled for shared read-only deployments.

## Audit Requirements

Every write API must record:

- project ID and task ID when applicable;
- authenticated actor and auth source;
- role/scope used for authorization;
- session ID or adapter name;
- command family and stable idempotency key when supplied;
- before/after state for mutable command families;
- append-only evidence/review/checkpoint/notification IDs for fact rows;
- request result: accepted, rejected, conflict, forbidden, invalid, or failed.

Audit rows must not store raw request bodies when they may include prompts,
secrets, tokens, private transcripts, tool bodies, or generated content dumps.

## Idempotency And Conflict Responses

Write APIs should accept optional idempotency keys for provider/adaptor retry
paths. Idempotent replay returns the original accepted result when the actor,
scope, command, and payload digest match. Mismatched replay fails closed.

Conflict responses should include:

- stable error code;
- current task/state version or updated row timestamp;
- current owner/status/review summary when relevant;
- suggested safe command, such as reload task detail or rerun merge-ready;
- no raw private payload.

FW-258 owns the full concurrency and sync model.

## Deployment Profiles

FW-257 names the API/identity expectations for common profiles; FW-259 owns the
operational runbook.

Cloudflare Access:
: Suitable for read-only dashboard and future verified identity display. Write
  mode requires JWT verification, audience/issuer checks, fail-closed behavior,
  and explicit role mapping.

Pomerium or internal OIDC proxy:
: Suitable when the team already operates an identity-aware proxy. Same rule:
  identity headers are trusted only after signed proof is verified or enforced
  upstream with origin isolation.

No edge / private network:
: Suitable for local lab or airgap team servers. Use loopback, VPN, mTLS, or
  private network controls plus API tokens or service identities. Do not rely on
  obscurity or unauthenticated LAN access for writes.

## Privacy Boundary

The server API must preserve existing Fairway privacy rules:

- no raw prompts or private transcripts by default;
- no raw tool bodies or generated-content dumps;
- no provider auth state, cookies, tokens, service keys, or credentials;
- evidence artifacts remain references unless a separate safe artifact viewer
  explicitly renders them from allowed local roots with redaction;
- recipe and packet APIs validate unsafe markers before rendering.

## Release Gates Before Implementation

A future implementation task must not be considered release-ready until it has:

- command parity tests between local CLI and API write paths;
- authn/authz tests for allowed, forbidden, missing, expired, malformed, and
  wrong-scope identities;
- CSRF/CORS/content-type tests for browser surfaces;
- audit tests that prove raw secrets/tokens/prompts are not recorded;
- read-only dashboard mutation-blocking tests;
- deploy profile docs for Cloudflare/Pomerium/no-edge modes;
- explicit review for arch, backend, security, and ops.

## Acceptance Checklist

- A server is optional and justified only for shared team coordination.
- Read APIs, write APIs, dashboard routes, provider adapter routes, and local
  CLI-only flows are separated.
- Authentication and authorization are command-scoped and auditable.
- Trusted proxy identity is not trusted unless verified or enforced upstream
  with origin isolation.
- CSRF, API tokens, CORS, and content-type boundaries are explicit.
- The model adds no runtime server implementation, public exposure, dashboard
  write authority, provider-send authority, approval, merge, deploy, release,
  or live-operation authority.
