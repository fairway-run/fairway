# GPUaaS Parity Runbook

Use this before replacing GPUaaS queue scripts with Fairway. The goal is to
prove behavior against a copied GPUaaS checkout and record every mismatch as a
Fairway task.

## Inputs

- A disposable GPUaaS checkout or worktree. Do not run parity against the live
  queue first.
- `doc/governance/Agent_Work_Queue.yaml` from GPUaaS.
- Optional copy of `.git/agent_queue/agent_queue.db` for historical comparison.
- `examples/gpuaas-a-b-c-d-e-config.toml` copied to `.fairway/config.toml`.
- `doc/governance/Workflow_Regression_Packs.yaml` or
  `examples/gpuaas-regression-packs.yaml` for catalog validation.

## Setup

```bash
cd /path/to/gpuaas-copy
mkdir -p .fairway
cp /path/to/fairway/examples/gpuaas-a-b-c-d-e-config.toml .fairway/config.toml
fairway config validate
fairway db migrate --dry-run
fairway db migrate
fairway import doc/governance/Agent_Work_Queue.yaml --state-once
```

If the imported GPUaaS YAML uses fields Fairway does not yet understand, stop
and capture the mismatch. Do not massage the source file until the missing
mapping is written down.

## Required Comparisons

| Behavior | GPUaaS Reference | Fairway Command | Pass Criteria |
|---|---|---|---|
| Queue validation | `make queue-ready` / `agent_queue_validate.sh` | `fairway import ... --state-once` and `fairway ready --json` | Invalid roles, dependencies, task IDs, and metadata are rejected or rendered with equivalent operator clarity. |
| Ready ordering | `queue-ready` | `fairway ready --json` | Same ready tasks per role, ordered by dependency completion, priority, and sequence. |
| Queue/git consistency | `make queue-git-check` | `fairway git-check` / `fairway preflight` | Same role branch/worktree risks are visible before dispatch. |
| Claim and transitions | `queue-claim`, `queue-set-status` | `fairway claim`, `fairway set-status` | Double-claim loses atomically; blocked requires a reason; terminal gates behave as configured. |
| Evidence and merge readiness | queue store gates | `fairway record evidence`, `fairway merge-ready` | Evidence-before-done and review/hand-off gates match GPUaaS policy. |
| Review routing | `queue-route-review` | `fairway route review --path ...` | Critical API, architecture, governance, security, billing, provisioning, terminal, CI, and node-agent paths route to the expected lane. |
| Packets | `agent_context_packet.sh`, `bugfix_review_packet.sh`, `agent_watch_task.sh` | `fairway packet context`, `packet bugfix`, `packet watcher` | Generated packets include goal, acceptance, history, evidence, root cause, owning layer, proof command, regression coverage, residual risk, success, and failure fields. |
| Regression packs | `workflow_regression_packs.sh` | `fairway regression-pack validate <catalog>` | GPUaaS catalogs with per-environment blocking maps validate without adapter loss. |
| Coordinator tick | `orchestrator-tick` | `fairway coordinator preflight`, `status`, `tick` | Dispatch blockers, stale sessions, active side tracks, and ready work are all visible. |
| Dashboard usefulness | script output/manual scans | `fairway dashboard` | Real GPUaaS task volume remains scannable by lane, status, stale sessions, checkpoints, watchers, and activity. |

## Evidence To Save

Save command output under a temporary parity artifact directory in the copied
checkout:

```bash
mkdir -p .fairway/parity
fairway ready --json > .fairway/parity/fairway-ready.json
fairway git-check --json > .fairway/parity/fairway-git-check.json
fairway coordinator tick --json > .fairway/parity/fairway-tick.json
fairway regression-pack validate doc/governance/Workflow_Regression_Packs.yaml \
  > .fairway/parity/fairway-regression-packs.txt
```

Record mismatches as Fairway tasks with `fairway add` or `fairway spawn` in the
Fairway repo. GPUaaS should switch over only after the parity artifact set has
no unexplained differences.
