# Quickstart

This is a runnable local flow for the current Fairway prototype.

## Install

Once release binaries ship:

```bash
brew install subashram/tap/fairway     # macOS
# or download from the releases page
```

For now:

```bash
go install github.com/subashram/fairway/cmd/fairway@latest
```

## Bootstrap a repo

```bash
cd /path/to/your/repo
fairway init
$EDITOR .fairway/config.toml
```

A minimal config:

```toml
[fairway]
main_branch = "main"

[[roles]]
name = "backend"
branch = "agent/backend"

[[roles]]
name = "ui"
branch = "agent/ui"
```

## Set up lanes

```bash
fairway worktree setup
```

Creates the configured branches (off `main_branch`) and checks them out into the configured worktree root.

## Define tasks

Create tasks via YAML import:

```bash
fairway import tasks.yaml
```

`tasks.yaml`:

```yaml
- id: T-001
  title: Wire up /v1/orders endpoint
  role: backend
  notes: |
    Returns the user's recent orders.
    Acceptance:
      - integration test passes
      - p95 < 100ms
  acceptance_checks:
    - "go test ./internal/orders/..."
- id: T-002
  title: Build the orders list page
  role: ui
  dependencies: [T-001]
```

## Run the dashboard

```bash
fairway dashboard
```

Opens `http://127.0.0.1:7878`. Leave it open on a second monitor.

## A typical loop

In the backend worktree:

```bash
fairway ready                 # what is available for me?
fairway claim T-001
# ... do the work, commit ...
fairway record evidence T-001 --command-text "go test ./..." --result pass --artifact internal/orders/orders_test.go --artifact-type test
fairway set-status T-001 done
fairway route review T-001 --path internal/orders/orders.go
fairway merge-ready T-001
```

In the UI worktree:

```bash
fairway ready                 # T-002 is now ready (T-001 reached done)
fairway claim T-002
fairway record handoff T-002 --to backend --payload "Need an example payload for /v1/orders before finalizing the form."
```

## What you see in the dashboard

- The lanes strip updates within a second of each transition.
- The activity feed shows the chain: claim → evidence → done; claim → handoff.
- Health badges flag the unacknowledged handoff to backend after one hour.

Long-running side work should use packets and checkpoints:

```bash
fairway packet context T-001 --goal "finish API contract" --owner backend --acceptance "tests pass"
fairway checkpoint record T-001 --state active --owner backend --summary "waiting on API fixture"
fairway checkpoint status
```

See [coordinator-loop.md](design/coordinator-loop.md),
[context-packets.md](design/context-packets.md), and
[checkpoints.md](design/checkpoints.md).

## GPUaaS Parity

GPUaaS migration work should start from
[gpuaas-parity-runbook.md](assessment/gpuaas-parity-runbook.md), not from a live
queue cut-over. Use
[`examples/gpuaas-a-b-c-d-e-config.toml`](../examples/gpuaas-a-b-c-d-e-config.toml)
for the exact A/B/C/D/E lane mapping and
[`examples/gpuaas-regression-packs.yaml`](../examples/gpuaas-regression-packs.yaml)
to verify Fairway accepts GPUaaS-style per-environment regression blocking.
