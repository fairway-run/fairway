# Product Boundaries

Fairway is the engineering control and accountability layer for agent-driven
delivery. It keeps accountable intent, material decisions, evidence,
independent judgment, and promotion state explicit while agents and external
engineering systems perform the work.

Coordination is a Fairway capability, not a transfer of authority. Sessions,
lanes, handoffs, waits, notifications, dashboards, and orchestrators can make
work visible and resumable. They do not become approval, risk acceptance,
merge, deploy, release, credential, or live-operation authority.

This page is the canonical product boundary. Other docs link here instead of
restating or weakening it.

## Responsibility And Authority

| Surface | May do | Must not imply |
|---|---|---|
| CLI and store | Record explicit task, decision, evidence, review, and promotion facts | That recording a claim proves or approves it |
| Dashboard | Read and explain current Fairway state | Privileged approval, send, merge, deploy, or live authority |
| Coordinator and watchers | Compute deterministic waits, reminders, and bounded next actions | Autonomous ownership transfer or consequential execution |
| Provider adapter | Attach a provider session and report bounded metadata or delivery proof | That provider output is provenance, approval, or policy |
| Reviewer | Record an attributable verdict within configured scope | Authority outside the review domain or over external systems |
| Human operator | Execute explicitly authorized external action and record evidence | That Fairway itself performed or authorized the action |

## Fairway Will Do

- Track tasks, ownership, state transitions, evidence, handoffs, reviews,
  sessions, watchers, worktrees, batches, and closeout.
- Surface stale, unsafe, review-gated, approval-gated, utility-gated, or
  incomplete work.
- Serve a shared read-only dashboard suitable for exposure through an
  identity-aware proxy or tunnel.
- Recommend next actions through CLI reports, dashboard diagnostics, and
  dry-run controller plans.
- Record provider-neutral usage metadata when an adapter supplies it.
- Record supply-chain provenance metadata that links tasks, sessions,
  checkpoints, evidence, reviews, commits, release verification, and safe
  artifact references without becoming a compiler/runtime dependency.
- Coordinate deterministic utilities such as CI monitors, codegen drift checks,
  release asset checks, and registry freshness checks.
- Integrate with planning systems such as Plane, Jira, Linear, and GitHub
  Issues while keeping local execution state in Fairway.

## Fairway Will Not Do

- Auto-claim ready tasks or silently transfer ownership between lanes.
- Auto-approve reviews or waive required review domains.
- Auto-merge branches or auto-push commits.
- Auto-delete local branches, worktrees, or task state without an explicit
  operator command.
- Perform destructive, production-impacting, credential, or approval-gated
  actions without an explicit stop condition and operator decision.
- Become a CI runner. Fairway records CI evidence and monitor handbacks; CI
  systems still execute CI.
- Become an IAM or public web gateway. Shared dashboard mode blocks writes, but
  the user/project owns domains, identity providers, Access policies, tunnels,
  and JWT/header verification.
- Become a workflow/DAG engine. Fairway coordinates human-paced engineering
  lanes; it does not replace Temporal, Cadence, Argo Workflows, or similar
  systems.
- Replace external planning tools. Plane, Jira, Linear, and GitHub Issues can
  mirror roadmap or stakeholder context; they do not own Fairway execution
  state.
- Become an LLM provider abstraction. Fairway records sessions and usage
  metadata supplied by adapters; providers remain external.
- Become a compiler, package manager, artifact signer, SBOM system,
  attestation authority, or runtime dependency. Fairway may export provenance
  metadata for those systems to reference, but source/build/release integrity
  remains with the project supply-chain tooling.
- Store prompts, transcripts, secrets, provider credentials, or private auth
  material by default.
- Gate task completion on token cost, provider spend, or model choice. Usage
  accounting is an operational planning signal, not a completion gate.
- Encode one project's taxonomy as core product grammar. Prefixes such as
  `CI-FIX-*`, `CD-FIX-*`, `UAT-BUG-*`, `OPS-FIX-*`, `HARNESS-FIX-*`, or
  `DOC-FIX-*` belong to workstream profiles and project conventions. Fairway
  may surface or recommend them when a profile/adapter uses that taxonomy, but
  the prefix itself does not own status, review, evidence, or release
  semantics.

## Controller Rule

The orchestration controller is advisory by default. It may classify state,
recommend actions, start explicitly configured utility monitors, and emit
provider continuation prompts when judgment is needed.

It must not silently claim, approve, merge, delete, push, deploy, or mutate
production-impacting state. Any future apply path must name the exact mutation,
show the dry-run plan, and keep stop conditions visible.

Structured advisory recommendations must pass `fairway advisory validate`
before they are recorded or used for handoff context. Validation checks the
action enum, target role, confidence, risk flags, cited Fairway facts, and
provider wake routability warnings. Accepted recommendations may be recorded as
`advisory-recommendation` evidence only; they do not approve reviews, accept
risk, claim tasks, merge, push, deploy, run live actions, or mutate
environments.

## Usage Accounting Rule

Provider usage records are for planning and retrospection:

- which provider or model was expensive,
- which task shapes consume more tokens,
- which work should be moved to deterministic utilities,
- which workflows should be batched.

Usage records must not include prompts, transcripts, secrets, or provider auth
material. Fairway may report counts, phases, roles, task IDs, provider names,
confidence, and source metadata.

## Provenance Rule

Fairway provenance is metadata and linkage, not content capture. Provenance
exports may include task IDs, state history, checkpoint summaries, evidence
references, review verdicts, session lifecycle, provider usage counts, commit
SHAs, release tags, and verification outcomes. They must not include raw
secrets, provider auth state, raw prompt bodies by default, private
transcripts, raw tool bodies, or generated-content dumps.

See [supply-chain-provenance.md](supply-chain-provenance.md) for the model.

## Adapter Rule

Adapters are edge contracts. Core Fairway remains provider-neutral.

- Provider adapters attach agent sessions and optional usage metadata.
- Utility adapters attach deterministic work such as CI polling.
- Tracker adapters mirror planning context to or from external issue trackers.
- Advisory provider adapters may be declared in config as bounded recommendation
  sources. They are metadata and validation inputs only; they do not make
  Fairway an LLM provider abstraction or grant execution authority.

Adapters may feed Fairway evidence, sessions, checkpoints, usage, and links.
They do not decide task status, review approval, merge readiness, or release
promotion on their own.

Provider thread steering is also an adapter boundary. Fairway may record that a
handoff was recorded, a notification was delivered, or a provider thread was
actually steered, but only the active provider/app surface can prove the direct
thread capability existed. If a session does not expose tools such as
`send_message_to_thread` and `read_thread`, agents must record Fairway state and
route the manual relay through the coordinator/control track. A delivered or
steered notification is a resume signal, not authority to approve, merge, push,
deploy, release, or close work.
For review-gated tasks, handoff-only or failed notification state is treated as
notification-blocked rather than proof that a reviewer is actively waiting.
