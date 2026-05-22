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
| `doc/governance/Agent_Queue_Structured_Store_v1.md` | Source-of-truth split and schema rationale. | None. |
| `doc/governance/Agent_Queue_State_And_Telemetry_Hardening_v1.md` | Boundary hardening, evidence enforcement, blocked reasons, timing, health, and session reconciliation. | GPUaaS-specific queue task IDs and rollout order. |
| `doc/governance/Agent_Orchestration_Tool_Extraction_Boundary_v1.md` | Generic core vs adapter boundary. | None. |
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

Out of core:

- LLM provider management,
- transcript capture,
- CI execution,
- product-specific release policy,
- GPUaaS-specific task IDs, roles, paths, and commands.

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
- JSON/YAML snapshot export.
