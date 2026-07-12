# Environment Deploy Preflight and Rehearsal Packets

Fairway models environment deploy readiness as packet, evidence, and readiness
state over existing tasks. It does not add a second deploy store and it does not
authorize deploy execution. Product repositories own their deploy scripts,
cluster credentials, and environment-specific proof commands.

## Goals

- Make demo, staging, airgap rehearsal, and production-like promotion handoffs
  visible before user/operator handoff.
- Catch route, runtime, worker-access, smoke, rollback, and evidence-contract
  blockers before an approved window is consumed.
- Record the owner, next action, and missed proof for every blocker as Fairway
  evidence or checkpoint state.
- Keep the model reusable: no consumer repository, infrastructure provider,
  deployment system, or environment-specific fields are required by Fairway.

## Packet Shape

Use a configured `packet template` named `environment-deploy-preflight` or an
equivalent profile-local template. The expected fields are:

- `environment`: target label such as `demo`, `staging`, `airgap-rehearsal`, or
  `production-like`.
- `deploy_kind`: `fresh-install`, `redeploy`, `promotion`, `rollback-drill`, or
  a project-local single-token kind.
- `source_sha`: source commit or artifact version being rehearsed.
- `operator_surface`: CLI, dashboard, runbook, CI job, release lane, or other
  human-facing execution surface.
- `route_readback`: how public/internal routes, DNS, proxy, TLS, auth, or tunnel
  state will be read back.
- `worker_access`: how runner, worker, node, queue, or background-job access
  will be proven.
- `smoke_scope`: app family, API, CLI, browser, callback, or integration smoke
  that must pass before handoff.
- `rollback_plan`: rollback or stop condition proof required before execution.
- `evidence_contract`: artifact paths, redaction rules, and exact result fields
  that must be recorded.
- `next_owner`: role that acts if the packet blocks.
- `next_action`: exact next command, review, approval, or packet update.
- `handoff_deadline`: when the packet becomes stale for handoff.

Optional fields should cover `known_limits`, `manual_checks`,
`forbidden_actions`, `approval_boundary`, `related_batch`, and `release_url`.

Example:

```bash
fairway packet template environment-deploy-preflight DEPLOY-123 \
  --field environment=staging \
  --field deploy_kind=redeploy \
  --field source_sha="$(git rev-parse HEAD)" \
  --field operator_surface="release lane" \
  --field route_readback="curl /healthz through the published route" \
  --field worker_access="runner can read job logs and worker health" \
  --field smoke_scope="API smoke, browser smoke, background worker smoke" \
  --field rollback_plan="previous artifact can be restored and read back" \
  --field evidence_contract=".fairway/artifacts/DEPLOY-123/preflight.md" \
  --field next_owner=ops \
  --field next_action="fix route readback before user handoff" \
  --field handoff_deadline=2026-06-25T22:00:00Z
```

Packet rendering is advisory. It does not run deploy commands, accept approval,
mutate environments, or satisfy readiness by itself.

Fairway includes `environment-deploy-preflight` as a built-in packet template,
so consumers can render the packet before adding project-local TOML. The same
command can optionally instantiate coordination state:

```bash
fairway packet template environment-deploy-preflight DEPLOY-123 \
  --field environment=staging \
  --field deploy_kind=redeploy \
  --field source_sha="$(git rev-parse HEAD)" \
  --field operator_surface="release lane" \
  --field route_readback="curl /healthz through the published route" \
  --field worker_access="runner can read job logs and worker health" \
  --field smoke_scope="API smoke, browser smoke, background worker smoke" \
  --field rollback_plan="previous artifact can be restored and read back" \
  --field evidence_contract=".fairway/artifacts/DEPLOY-123/preflight.md" \
  --field next_owner=ops \
  --field next_action="fix route readback before user handoff" \
  --field handoff_deadline=2026-06-25T22:00:00Z \
  --instantiate-waits \
  --child-task DEPLOY-ROUTE-001=route_readback
```

`--instantiate-waits` records generic `environment-rehearsal` waits for
route-readback, worker-access, smoke, rollback, and evidence-contract checks.
`--child-task <id=field>` creates an explicit child workflow-guard task for a
named check field. Both modes are coordination-only. They do not run commands,
grant approval, mutate environments, satisfy release readiness, or authorize
live/deploy work.

## Evidence Contract

Preflight and rehearsal outcomes should be attached to the deploy or release
task with stable artifact types:

- `environment-preflight`: full packet result and readiness summary.
- `environment-blocker`: unresolved blocker with owner and next action.
- `route-readback`: route, DNS, proxy, TLS, auth, or tunnel proof.
- `worker-access`: runner, node, queue, job, or worker proof.
- `app-smoke`: app/API/browser/callback/background smoke proof.
- `rollback-proof`: rollback, stop-condition, or recovery proof.
- `handoff-readiness`: final user/operator handoff packet.

Use `result=pass` only when the evidence proves the stated contract. Use
`result=fail` or `result=blocked` when any blocker remains, and include
`owner=<role>` and `next_action=<text>` in notes or the artifact body. This lets
the dashboard and delivery report show unresolved work before handoff without a
new persistence model. The same facts also project into
[`fairway delivery resources`](delivery-resources.md) as environment,
preflight-packet, or rehearsal-target rows with state, blocker, provenance, and
next-safe-action readback.

Example:

```bash
fairway record evidence DEPLOY-123 \
  --artifact-type environment-blocker \
  --result blocked \
  --artifact .fairway/artifacts/DEPLOY-123/route-blocker.md \
  --notes "owner=ops next_action=repair route publication before handoff"
```

If a preflight blocks after review or approval, record a checkpoint or
completion handback to the next owner. Do not leave the next action in provider
chat only.

## Readiness Profile

Projects can make environment readiness visible through a profile gate rather
than a product-specific command:

```toml
[[workstream_profiles]]
name = "environment-deploy"
task_kinds = ["task", "workflow-guard"]
dashboard_groups = ["deploy", "readiness", "handoff"]
review_domains = ["ops", "architecture"]
route_samples = ["deploy/**", "docs/runbooks/**"]

[[workstream_profiles.gates]]
name = "environment-preflight"
group = "deploy readiness"
mode = "blocking"
evidence_type = "environment-preflight"
required_evidence_count = 1
accepted_results = ["pass"]
artifact_required = true
description = "Environment deploy handoff requires passing preflight evidence."

[[workstream_profiles.gates]]
name = "rollback-proof"
group = "deploy readiness"
mode = "blocking"
evidence_type = "rollback-proof"
required_evidence_count = 1
accepted_results = ["pass"]
artifact_required = true
description = "Promotion-like deploys require rollback proof before handoff."

[[packet_templates]]
profiles = ["environment-deploy"]
name = "environment-deploy-preflight"
required_fields = [
  "environment",
  "deploy_kind",
  "source_sha",
  "operator_surface",
  "route_readback",
  "worker_access",
  "smoke_scope",
  "rollback_plan",
  "evidence_contract",
  "next_owner",
  "next_action",
  "handoff_deadline",
]
optional_fields = ["known_limits", "manual_checks", "forbidden_actions", "approval_boundary", "related_batch", "release_url"]
```

`fairway readiness report --profile environment-deploy` then reports whether
the required evidence exists. Dashboard task detail and reports already display
task tags, evidence, checkpoints, review waits, handoffs, and completion
handbacks, so unresolved environment blockers are visible as long as they are
recorded with the stable artifact types above. `fairway delivery report` also
groups failed, blocked, or partial rehearsal evidence by `packet=<id>` and
`check=<id>` values in evidence notes, so repeated route, worker, smoke,
rollback, or evidence-contract failures can be seen as automation candidates.
`fairway delivery resources` and the dashboard Delivery Resources panel show
the related deploy environment, preflight packet, and rehearsal target as typed
readiness records; those records remain read-only and do not authorize deploy
or rollback.

## Boundaries

- Fairway packet rendering does not authorize live execution, deploy, rollback,
  release, DNS/proxy mutation, credential use, or public exposure.
- The read-only dashboard may display deploy readiness and blockers, but it must
  not send notifications, approve gates, or mutate environments.
- External notifier delivery, when configured, can wake a next owner about a
  preflight blocker, but the notification is not approval or deploy authority.
- Consumer repositories must provide product-specific commands, fixtures,
  credentials, smoke tests, redaction rules, and rollback scripts.
