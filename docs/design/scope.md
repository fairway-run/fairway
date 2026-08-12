# Scope

## Identity

- **Name:** fairway
- **Tagline:** durable coordination for collaborative and delegated engineering
- **Metaphor:** maritime traffic control. Fairways preserve navigable channels,
  shared facts, and authority boundaries while different vessels enter, leave,
  wait, and continue under their own control.

## What fairway is

A standalone Go binary plus an embedded SQLite store that preserves the
cross-run engineering record as humans, agents, and utilities move between
collaborative problem-solving and bounded delegated execution. It operates
across an existing delivery lifecycle rather than defining a new SDLC phase.
It provides:

- A task queue with definitions (immutable) and execution state (mutable).
- A configurable state machine for task lifecycle.
- Lane / worktree management for per-role isolation.
- A decision / handoff / evidence / review chain so a diagnosis can evolve,
  bounded work can be delegated, and claims can be challenged with an audit
  trail.
- Session lifecycle tracking (PID, tmux pane, heartbeats).
- Status, health, timing, task-detail, merge-readiness, and snapshot reports derived from the store.
- Coordinator preflight/status/tick surfaces, context packets, watcher packets, and task checkpoints for bounded parallel work.
- Workflow regression pack catalog validation and bug-fix review packet rendering.
- A local web dashboard for live observation.

## What fairway is not

- **Not a workflow engine.** No DAG executor, no compensating transactions, no durable timers. If you need Temporal or Cadence semantics, use Temporal or Cadence.
- **Not a software-development methodology.** Fairway does not prescribe
  lifecycle phases, ceremonies, team topology, or a universal definition of
  feature completeness. Projects retain those choices.
- **Not an IAM tool.** No identity provider, no permissions model beyond OS user attribution.
- **Not a CI runner.** Fairway records that work was done; it does not run pipelines.
- **Not an LLM provider abstraction.** Fairway does not spawn agents, proxy
  provider API calls, manage provider credentials, or make provider output
  authoritative. It coordinates whatever agent process you run inside a
  worktree. Future advisory-provider adapters may suggest bounded actions from
  Fairway facts, but Fairway must validate those suggestions against task
  state, review gates, risk, and configured policy.
- **Not a multi-repo federation layer (v1).** Single repo, single SQLite DB per repo. Future work may add federation.

## Out of scope for v1

- Postgres backend (schema is designed to be portable; adapter is v2+; compatibility harness is planned).
- Multi-repo federation.
- LLM provider integration as a required core dependency. Optional advisory
  adapters are future work and must remain replaceable, bounded, and validated;
  see [Coordination intelligence](coordination-intelligence.md).
- Webhook / event emission for external systems.
- Authn / authz beyond OS user attribution.
- Broad write actions from the dashboard. The dashboard has only narrow,
  CSRF-protected, audited mutations for claim and non-terminal status changes;
  terminal state changes and review/evidence writes remain CLI-first.
- Provider launch orchestration as a core requirement. Provider/session launchers may exist as adapters, but the queue works without them.
- Required issue tracker integration. Jira/Linear/GitHub Issues adapters are
  planned, but fairway must remain useful without any external tracker.

## Audience

Solo developers and small teams using coding agents across one repository,
including work that is sequential, collaborative, delegated, or parallel.
Fairway is designed to be useful at one human plus a few replaceable execution
attachments; it should not get in the way at that scale.
