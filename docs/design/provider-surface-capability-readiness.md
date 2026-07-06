# Provider Surface Capability Readiness

Fairway provider sessions are replaceable execution attachments. Some tasks
also depend on the concrete local surface behind that attachment: a browser
that can launch, a shell with the right CLI tools, a Kubernetes context, SSH
access, local filesystem permissions, or a usable Fairway session/checkpoint.

Provider-surface capability readiness records those local prerequisites before
high-risk work starts. It is coordination and review evidence only. It does not
approve reviews, authorize live execution, mutate production, send provider
prompts, or carry credentials.

## Capability State

Capability state is scoped to a task or operation, not to a provider globally.

| State | Meaning |
| --- | --- |
| `unknown` | No current proof exists for the task/scope. |
| `pass` | The exact surface passed the required non-live probe. |
| `fail` | The probe failed with a sanitized finding. |
| `retired` | The surface is no-go for the task/scope until replaced. |
| `superseded` | A newer reviewed surface replaced the previous one. |
| `expired` | Proof aged out or the surface changed. |
| `waived` | A bounded approved waiver exists with owner, expiry, risk, and compensating evidence. |

For live windows, first-write gates, and disposable preflights, `unknown`,
`fail`, `retired`, and `expired` are blocking states unless a bounded waiver is
explicitly recorded and reviewed.

## Evidence Shape

The read model should be able to display, at minimum:

- task id and operation scope;
- provider/session/surface id, role, owner, worktree, and branch;
- capability class such as browser launch, CLI access, Kubernetes target
  access, SSH/tmux access, filesystem permission, or Fairway coordination;
- exact probe command or helper mode;
- result state, timestamp, expiry, and artifact path;
- mutation and credential boundary booleans;
- sanitized failure class for non-pass results;
- replacement surface requirement when the surface is retired or failed;
- required review domains;
- statement that the record is not approval, merge, deploy, mutation, release,
  or dashboard send authority.

Artifacts must be sanitized. They must not persist credentials, tokens,
cookies, OTP material, private keys, bearer material, raw page bodies, or raw
response headers.

## Doctor Diagnostics

`fairway doctor` is the local read-only capability diagnostic surface for common
agent execution blockers. It checks Fairway config and DB paths, git worktree
state, stale `.git/index.lock` guidance for tmux/CLI fallback, Go cache posture,
required CLI tools, dashboard reachability, and Fairway session readback. Each
row reports `pass`, `warn`, or `fail`, an owner, a suggested command, an
optional evidence path, and the boundary it blocks, such as task work, release,
dashboard restart, provider capability probes, git boundary, or shared-team
pilot.

Doctor output is evidence and triage only. It must not approve reviews, start
providers, push, deploy, restart dashboards, mutate environments, run live
operations, or expose credentials. Use JSON output when a coordinator or agent
needs compact structured diagnostics.

## Coordinator And Dashboard Projection

Coordinator plan and dashboard task detail should project provider-surface
readiness for approval-gated operations:

- required capability list;
- current state per capability;
- current allowed execution surface;
- retired/no-go surfaces for the same task/scope;
- replacement surface and proof requirement;
- stale or expired proof warnings;
- suggested next CLI action.

The dashboard remains read-only. It may show capability readiness and no-go
state, but it must not send prompts, approve reviews, authorize execution,
mutate environments, or change task status.

## Retirement Rule

When a required non-live probe fails, Fairway should make the surface no-go for
that task/scope until a replacement surface passes the required proof. A
retirement record should include:

- failed surface id;
- capability and sanitized failure class;
- artifact path;
- forbidden future use for the task/scope;
- replacement surface requirement;
- next owner and next action.

Live-operation packets and review readbacks should name both the retired
surface and the replacement surface when a prior attempt failed on provider
capability.

## Regression Case

The GPUaaS MFA drill loop exposed this requirement. One drill-operator surface
failed direct headed Chrome launch and CDP loopback before browser navigation,
credential submission, or Keycloak mutation. A replacement operator-v2 surface
passed a non-live direct headed Chrome launch probe with no navigation, no
credential submission, no Keycloak mutation, and sanitized artifacts.

That distinction must live in Fairway state and review packets, not only in
provider chat.
