# GPUaaS Extraction

Fairway is a standalone extraction of the agent queue/orchestration model proven
inside GPUaaS. The GPUaaS implementation remains the reference behavior during
the rewrite, but repo-specific policy must become configuration or adapters.

GPUaaS itself grew out of earlier experiments in
[`subashram/poiesis`](https://github.com/subashram/poiesis). Poiesis was a
provider-backed workflow engine with contract generation, planning, developer,
reviewer, QA, and red-team agents. Fairway does not inherit that provider
runtime, but it does keep the coordination lessons that survived into GPUaaS.

## Source Material

The extraction is informed by these GPUaaS files:

| Source | Reuse | Convert or drop |
|---|---|---|
| `scripts/ops/agent_queue_store.rb` | Store schema, state transitions, evidence, handoffs, reviews, sessions, reports, merge readiness checks. | Hardcoded roles, branch names, review regexes, DB path, and YAML state sync. |
| `scripts/ops/agent_queue.sh` | Operator command shape and preflight flow. | Makefile wrapper names and GPUaaS-specific branch freshness checks. |
| `scripts/ops/agent_role_config.sh` | Role to branch/provider/worktree mapping concept. | Specific role names, providers, and path templates. |
| `scripts/ops/orchestrate_agents.sh` | Coordinator preflight/status/tick composition. | GPUaaS-specific Make targets and track policy defaults. |
| `scripts/ops/agent_context_packet.sh` | Bounded side-agent context packet shape. | Environment-specific fields and shell wrapper implementation. |
| `scripts/ops/agent_watch_task.sh` | Watcher packet shape and typed evidence expectations. | GitLab/demo-specific examples. |
| `scripts/ops/bugfix_review_packet.sh` | Root-cause, owning-layer, proof, and regression handoff packet. | Environment-variable wrapper. |
| `scripts/ops/workflow_regression_packs.sh` | Workflow regression pack list/show/validate behavior. | GPUaaS-specific pack catalog contents. |
| `scripts/ops/agent_queue_postgres_compat.sh` | Postgres compatibility harness shape. | Spike DDL as production migration. |
| `scripts/ops/agent_session_adapter.sh` | Optional shell/tmux/zellij launch adapter and preflight checks. | Provider command templates as core behavior. |
| `scripts/ops/codex_subagent_session_bridge.sh` | Session records for coordinator-spawned subagents. | Codex-specific bridge as core behavior. |
| `doc/governance/Agent_Queue_Structured_Store_v1.md` | Source-of-truth split and schema rationale. | None. |
| `doc/governance/Agent_Queue_State_And_Telemetry_Hardening_v1.md` | Boundary hardening, evidence enforcement, blocked reasons, timing, health, and session reconciliation. | GPUaaS-specific queue task IDs and rollout order. |
| `doc/governance/Agent_Orchestration_Tool_Extraction_Boundary_v1.md` | Generic core vs adapter boundary. | None. |
| `doc/governance/Agent_Parallel_Track_Control_v1.md` | Checkpoint discipline, side-track limits, context packets, and drift rules. | Separate `track_checkpoints` identity model. |
| `doc/governance/Agent_Watcher_Lanes_v1.md` | Watcher-lane operating contract and artifact type taxonomy. | GPUaaS-specific CI/deploy examples. |
| `doc/governance/Multi_Agent_Lane_Worktrees_v1.md` | Persistent lane worktrees, named review branches, and provider-as-execution-choice guidance. | GPUaaS role names and release branch policy. |
| `doc/governance/Testing_Standards.md` and `Workflow_Regression_Packs.yaml` | Workflow pack definition, bug-fix definition of done, and regression evidence expectations. | GPUaaS product workflows. |
| `doc/governance/Agent_Queue_Postgres_Adapter_Semantics_v1.md` | Shared-store compatibility contract and transaction rules. | Ruby implementation details and spike DDL. |
| `subashram/poiesis` | Contract-first task shape, dependency-ready task selection, human approval, QA/red-team evidence, and "context over hardcoded agents." | Provider API calls, automated agent execution loop, generated artifacts directory model, and specialist-agent runtime. |

## Extraction Boundary

Generic fairway core:

- task definitions and mutable task state,
- configurable state machine,
- ready / claim / set-status transitions,
- append-only state history,
- handoff, evidence, and review records,
- merge readiness and git consistency checks,
- session lifecycle and stale-session reconciliation,
- optional session launch adapters,
- coordinator preflight/status/tick,
- context and watcher packet renderers,
- bug-fix packets and workflow regression pack catalog helpers,
- task checkpoints,
- status, health, timing, task-detail, and snapshot reports,
- worktree setup and branch hygiene.

Config or adapter layer:

- role names and lane count,
- branch naming,
- provider labels and launch commands,
- worktree path templates,
- review routing,
- task source format,
- acceptance-check conventions,
- evidence and review gates.
- session launch backends and provider commands.

Out of core:

- LLM provider management,
- transcript capture,
- CI execution,
- product-specific release policy,
- GPUaaS-specific task IDs, roles, paths, and commands.

## GPUaaS Operations Kept

The queue model alone was not enough in GPUaaS. The day-to-day operating loop
also needed:

- `coordinator preflight/status/tick` to compose queue, git, session,
  checkpoint, and health checks before dispatch,
- `context packets` to bound side-agent work,
- `watcher packets` for CI/deploy/smoke monitoring,
- `task checkpoints` to replace vague active side tracks,
- `workflow regression packs` and `bugfix review packets` for root-cause and
  regression discipline,
- named review branches instead of detached review checkouts,
- pre-claim worktree checks for branch, cleanliness, and base freshness,
- optional session launch/bridge adapters for shell, tmux, zellij, and
  coordinator-spawned subagents.

Fairway should implement these as generic commands and adapters. The GPUaaS
Makefile target names are compatibility examples, not the core API.

## Poiesis Lessons Kept

Poiesis had a useful framing: specialization comes from context, not from
hardcoded agents. Fairway keeps that by treating roles and providers as config,
and by making task notes, acceptance checks, contracts, and evidence the durable
context an agent receives.

Poiesis also made contract boundaries explicit through `input_contract`,
`output_contract`, and `acceptance_criteria`. Fairway folds those into generic
task fields instead of requiring a separate contract agent:

- `notes` carries background, assumptions, and contract text,
- `acceptance_checks` carries executable or human-verifiable checks,
- `task_evidence` records QA, red-team, test, screenshot, or review artifacts,
- `task_reviews` records human or role-based approval.

Poiesis had automated rework loops between developer, reviewer, QA, and red-team
agents. Fairway intentionally does not run those loops in core. If teams want
automated iteration later, it should be an adapter that claims a task, records
each review/QA/red-team pass as evidence, and leaves fairway as the state and
audit authority.

Poiesis encouraged planner-generated atomic tasks. Fairway keeps the useful
part, small reviewable tasks with explicit dependencies, but the orchestrator
owns granularity. Agents should not explode an assigned task into internal
subtasks inside fairway.

## Non-Negotiable Lessons

The DB is authoritative for mutable execution facts. Imports may seed task
definitions and, when explicitly requested for migration, initial state. Imports
must not continuously sync or overwrite DB-owned fields such as status, owner,
branch, completed time, commit, review state, evidence, or session telemetry.

Operational facts are not derivable from Git after the fact. Fairway therefore
keeps append-only records for transitions, evidence, handoffs, reviews, and
sessions, and exposes backup/export commands for human-readable handoff and
incident review.

Provider behavior is intentionally optional. Fairway may record that a role uses
`codex`, `claude`, `gemini`, or `shell`, but the queue store must work even when
agents are launched manually.

Automated QA, red-team, and reviewer outputs are evidence, not separate state
machines. A future runner can add those records, but merge readiness and task
health should still derive from fairway's DB tables.

## Parity Targets

Before GPUaaS switches from the Ruby queue to fairway, compare fairway output
against a copied GPUaaS queue DB and task file for:

- ready queue ordering,
- claim and set-status transitions,
- evidence-before-done behavior when enabled,
- blocked reason validation,
- review routing,
- merge readiness gates,
- git consistency checks,
- task-detail output,
- status / timing / health reports,
- session status and reconciliation,
- coordinator tick and preflight summaries,
- watcher packet rendering and close evidence,
- checkpoint stale detection,
- review branch checkout flow,
- workflow regression pack validation and bug-fix packet rendering,
- Postgres compatibility harness output,
- JSON/YAML snapshot export.
