# Scope

## Identity

- **Name:** fairway
- **Tagline:** traffic control for coding agents
- **Metaphor:** maritime traffic control. Fairways are navigable channels under VTS coordination; agents transit worktree lanes under fairway's coordination.

## What fairway is

A standalone Go binary plus an embedded SQLite store that coordinates multiple coding agents working in parallel on a single repository. It provides:

- A task queue with definitions (immutable) and execution state (mutable).
- A configurable state machine for task lifecycle.
- Lane / worktree management for per-role isolation.
- A handoff / evidence / review chain so work can be passed between roles with an audit trail.
- Session lifecycle tracking (PID, tmux pane, heartbeats).
- Status, health, timing, task-detail, merge-readiness, and snapshot reports derived from the store.
- Coordinator preflight/status/tick surfaces, context packets, watcher packets, and task checkpoints for bounded parallel work.
- Workflow regression pack catalog validation and bug-fix review packet rendering.
- A local web dashboard for live observation.

## What fairway is not

- **Not a workflow engine.** No DAG executor, no compensating transactions, no durable timers. If you need Temporal or Cadence semantics, use Temporal or Cadence.
- **Not an IAM tool.** No identity provider, no permissions model beyond OS user attribution.
- **Not a CI runner.** Fairway records that work was done; it does not run pipelines.
- **Not an LLM provider abstraction.** Fairway does not spawn agents, does not proxy API calls, does not manage credentials. It coordinates whatever agent process you run inside a worktree.
- **Not a multi-repo federation layer (v1).** Single repo, single SQLite DB per repo. Future work may add federation.

## Out of scope for v1

- Postgres backend (schema is designed to be portable; adapter is v2+; compatibility harness is planned).
- Multi-repo federation.
- LLM provider integration.
- Webhook / event emission for external systems.
- Authn / authz beyond OS user attribution.
- Write actions from the dashboard (v0.1 is observation-only; mutations are CLI-only).
- Provider launch orchestration as a core requirement. Provider/session launchers may exist as adapters, but the queue works without them.

## Audience

Solo developers and small teams running 2–6 coding agents in parallel against one repository. Fairway is designed to be useful at one human plus three agents; it should not get in the way at that scale.
