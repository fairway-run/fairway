# GPUaaS Parity and Gap Assessment - 2026-05-29

## Purpose

Assess Fairway against the current GPUaaS agent/process operating model before
new implementation work starts.

This is intentionally an assessment, not an implementation plan. The goal is to
avoid rebuilding what already exists, identify what is only documented or still
GPUaaS-specific, and turn the real gaps into focused Fairway work later.

## Summary

Fairway already contains much more of the GPUaaS agent operating model than the
top-level README implies. The README still describes Fairway as pre-alpha and
"not yet usable," but the implementation and roadmap show a substantial usable
core:

- task queue and mutable state,
- task hierarchy,
- evidence, handoff, and review records,
- review routing and no-self-review enforcement,
- merge-readiness and git checks,
- role worktrees,
- session tracking,
- coordinator preflight/status/tick reports,
- context, bugfix, and watcher packets,
- watcher lifecycle,
- regression-pack catalog helpers,
- checkpoints,
- dashboard,
- TUI basics,
- project registry and multi-project dashboard mode,
- SQLite migrations and Postgres compatibility check,
- release packaging configuration.

The biggest remaining issue is not absence of design. It is parity clarity:
GPUaaS still has shell/Ruby scripts and Makefile targets that encode current
behavior. Fairway must prove it can replace those behaviors before GPUaaS
switches over.

## Sources Reviewed

Fairway:

- `README.md`
- `AGENTS.md`
- `docs/archive/gpuaas-extraction.md`
- `docs/design/implementation-roadmap.md`
- `docs/design/release-cuts.md`
- `docs/design/cli.md`
- `docs/design/dashboard.md`
- `docs/config-reference.md`
- `examples/gpuaas-config.toml`
- `cmd/fairway/main.go`
- `internal/store/store.go`

GPUaaS references that Fairway is extracting:

- `scripts/ops/agent_queue_store.rb`
- `scripts/ops/agent_queue.sh`
- `scripts/ops/agent_role_config.sh`
- `scripts/ops/orchestrate_agents.sh`
- `scripts/ops/agent_context_packet.sh`
- `scripts/ops/agent_watch_task.sh`
- `scripts/ops/bugfix_review_packet.sh`
- `scripts/ops/workflow_regression_packs.sh`
- `scripts/ops/agent_session_adapter.sh`
- `scripts/ops/codex_subagent_session_bridge.sh`
- `doc/governance/Agent_Orchestrator_v2_Coordinated_Execution.md`
- `doc/governance/Agent_Execution_Quality_and_Context_Model_v1.md`
- `doc/governance/Multi_Agent_Lane_Worktrees_v1.md`

Verification:

- `go test ./...` passes in Fairway on 2026-05-29.

## What Fairway Already Has

| Capability | Current Fairway State | Notes |
|---|---|---|
| Task definitions and mutable state | Implemented | `task_definitions`, `task_state`, import/add/update/ready/claim/set-status/task-detail. |
| Hierarchy | Implemented | `add`, `spawn`, `tree`; supports parent/child/sibling/root patterns. |
| Dependencies and ready queue | Implemented | `ready`, dependency-aware task selection, priority/sequence ordering. |
| Evidence records | Implemented | `record evidence`; command/result/artifact/duration/notes model. |
| Handoff records | Implemented | `record handoff`; payload model for role transitions. |
| Review records | Implemented | `record review`; approval/changes/reject plus commit/reason. |
| Review routing | Implemented | Configured route rules and `route review`; no-self-review is documented and implemented in roadmap scope. |
| Merge readiness | Implemented | `merge-ready`; gates can require evidence/review/handoff. |
| Git consistency and preflight | Implemented | `git-check`, `preflight`; branch/worktree safety surfaces. |
| Worktree management | Implemented | `worktree setup/status/prune`. |
| Session tracking | Implemented | `session upsert/status/end/reconcile/launch`. |
| Coordinator reports | Implemented | `coordinator preflight/status/tick`, `dispatch-plan`, status/health/timing reports. |
| Context packets | Implemented | `packet context`. |
| Bug-fix packets | Implemented | `packet bugfix`. |
| Watcher packets and lifecycle | Implemented | `packet watcher`, `watcher start/finish/status`. |
| Checkpoints | Implemented | `checkpoint record/status/stale`. |
| Regression-pack helpers | Implemented | `regression-pack list/show/validate`. |
| Dashboard | Implemented | Read-only dashboard, SSE, role grouping, task detail, sessions/worktrees/checkpoints/watchers. Roadmap says first dashboard mutation path exists for claim. |
| Multi-project registry | Implemented | `register`, `unregister`, `projects`, dashboard multi-project mode. |
| SQLite store and migrations | Implemented | Embedded migrations, backup/export, compatibility checks. |
| Postgres compatibility harness | Partially implemented | `db compat --backend postgres`; not the runtime adapter. |
| Tracker links | First pass | Local links and dry-run reconcile for Jira/Linear; provider adapters deferred. |
| Release packaging | First pass | GoReleaser config and tag-triggered GitHub release workflow. |

## What Exists in GPUaaS But Needs Parity Proof

These are not necessarily missing from Fairway. They are behaviors GPUaaS
depends on, so Fairway needs a parity check or migration proof before replacing
the current scripts.

| GPUaaS Behavior | Current GPUaaS Source | Fairway Status | Needed Assessment/Proof |
|---|---|---|---|
| Queue YAML validation | `make queue-ready`, `agent_queue_validate.sh` | Likely covered by config/import validation | Prove Fairway import catches invalid roles, statuses, dependencies, and bad task metadata. |
| Queue-to-git consistency | `make queue-git-check` | `git-check` exists | Compare outputs against current GPUaaS repo state and role branches. |
| Ready queue ordering | `queue-ready` | `ready` exists | Run both on copied task data and compare ordering by role/priority/dependencies. |
| Claim/state transitions | `queue-claim`, `queue-set-status` | `claim`, `set-status` exist | Prove state transitions and blocked reason behavior match GPUaaS policy. |
| Evidence-before-done discipline | queue store gates | Configurable gates exist | Configure gates to GPUaaS policy and test done/merge-ready behavior. |
| Handoff records | `queue-record-handoff` | `record handoff` exists | Confirm payload fields cover GPUaaS handoff minimum fields. |
| Review records and routing | `queue-route-review`, `queue-record-review` | `route review`, `record review` exist | Verify path routing for GPUaaS critical paths: API, architecture, governance, security, billing, provisioning, terminal, CI. |
| Session visibility | session adapter and Codex subagent bridge | `session` commands exist | Confirm Fairway can represent tmux/zellij/manual/Codex API sessions without provider-specific coupling. |
| Watcher lanes | `agent_watch_task.sh` | watcher commands exist | Confirm watcher records cover command, success/failure, allowed fixes, duration, artifact, and close condition. |
| Context packets | `agent_context_packet.sh` | `packet context` exists | Compare generated packet against GPUaaS required fields. |
| Bug-fix packet | `bugfix_review_packet.sh` | `packet bugfix` exists | Ensure root cause, owning layer, proof command, regression coverage, residual risk are all present. |
| Workflow regression packs | `workflow_regression_packs.sh`, YAML catalog | regression-pack commands exist | Import/validate GPUaaS pack catalog or define adapter format. |
| Coordinator tick | `orchestrator-tick` | coordinator commands exist | Compare preflight/status/tick output with GPUaaS coordinator expectations. |
| Dashboard usefulness | none equivalent except scripts/reports | Fairway dashboard exists | Validate with real GPUaaS task volume and role branches. |
| Postgres compatibility | GPUaaS spike docs/scripts | `db compat` exists | Decide whether this remains harness-only or becomes a runtime adapter later. |

## Real Gaps To Address

| Gap | Why It Matters | Proposed Fairway Work |
|---|---|---|
| README is stale | It understates implementation maturity and will mislead users and agents. | Update README status to reflect implemented v0.2-like capabilities and current limitations. |
| GPUaaS parity harness missing | We need confidence before replacing GPUaaS scripts. | Add a documented `gpuaas-parity` procedure using copied GPUaaS queue/task data and expected-output comparisons. |
| GPUaaS config example is generic | Example uses `backend/ui/ops/arch/governance`; GPUaaS uses `A-backend/B-ui/C-ops/D-arch/E-governance`. | Add a GPUaaS exact config example or migration notes mapping both naming schemes. |
| Import/migration story needs a concrete dry run | GPUaaS queue state currently lives in YAML plus SQLite/script conventions. | Add a migration guide from GPUaaS `Agent_Work_Queue.yaml` and queue DB snapshot to Fairway. |
| Review routing parity needs explicit fixtures | GPUaaS critical path routing is high risk. | Add review route fixture tests for API, governance, architecture, CI, auth, billing, provisioning, terminal, node-agent paths. |
| Packet parity needs fixture tests | Context/bugfix/watcher packets are central to agent discipline. | Add golden tests comparing Fairway packet output to GPUaaS-required fields. |
| Workflow regression pack format needs adoption | GPUaaS has a real catalog; Fairway helper must support it cleanly. | Add GPUaaS catalog fixture and validation test, or document required adapter transformation. |
| Dashboard mutation model incomplete | Dashboard is useful, but operational teams may expect claim/status/review from UI later. | Keep CLI source of truth; expand dashboard mutations slowly with CSRF/audit and parity tests. |
| Provider/session adapters are intentionally shallow | Good boundary, but users need examples. | Add shell/tmux/zellij/Codex-bridge examples without making provider management core. |
| Postgres adapter is compatibility-only | Multi-user/shared operations may eventually need server DB. | Keep SQLite as v1 runtime; define v2 Postgres runtime criteria separately. |
| Release maturity unclear | GoReleaser exists, but first real release/tap path is still pending. | Cut v0.1/v0.2 release, then add Homebrew tap once artifact layout settles. |

## Proposed Next Work Pack

Do not start by adding new features. Start with parity and documentation cleanup.

1. **Update Fairway README maturity**
   - Make it clear what is implemented and what is not.
   - Avoid "not yet usable" if core commands are passing tests.

2. **Create GPUaaS exact config**
   - Add `examples/gpuaas-a-b-c-d-e-config.toml`.
   - Use GPUaaS branch names and worktree conventions:
     `agent/A-backend`, `agent/B-ui`, `agent/C-ops`, `agent/D-arch`,
     `agent/E-governance`.

3. **Create GPUaaS parity runbook**
   - Inputs: copied `Agent_Work_Queue.yaml`, optional queue DB snapshot,
     role branches, and representative dirty/clean worktrees.
   - Outputs: ready ordering comparison, git-check comparison, route-review
     comparison, packet comparison, merge-ready comparison.

4. **Add fixture tests for GPUaaS-critical behaviors**
   - review routing,
   - evidence/review/handoff gates,
   - context/bugfix/watcher packet fields,
   - regression-pack validation,
   - state transition/block reason policy.

5. **Run Fairway against GPUaaS copy**
   - Do not mutate the live GPUaaS queue first.
   - Use exported/copy data.
   - Record mismatches as Fairway tasks.

## Recommendation

Fairway should become the reusable execution-control layer for GPUaaS agents,
but GPUaaS should not switch until parity is proven.

The near-term goal is therefore:

```text
document -> configure -> import copy -> compare -> fix parity gaps -> pilot in one lane
```

Only after that should GPUaaS replace the current Ruby/shell queue scripts with
Fairway commands.
