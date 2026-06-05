# Quickstart

This is a runnable local flow for trying Fairway in a repository with a small
set of agent lanes.

Public docs: [fairway.run](https://fairway.run).

## Install

Tagged releases publish signed/notarized macOS artifacts and update the
Homebrew cask:

```bash
brew tap fairway-run/tap
brew install --cask fairway            # macOS
# or download from the releases page
```

Until the first tagged release is cut, install from source:

```bash
go install github.com/fairway-run/fairway/cmd/fairway@latest
# or, from a checkout:
make install
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

[[roles]]
name = "orchestrator"
branch = "agent/orchestrator"
```

For architecture, release, migration, or security tracks, add a workstream
profile. Profiles are optional; they give Fairway names for task kinds,
dashboard groups, review domains, adoption route samples, readiness gates, and
packet template fields without baking one project's workflow into the binary.

```toml
[[workstream_profiles]]
name = "platform-foundation"
task_kinds = ["architecture-map", "boundary-guard", "facade"]
dashboard_groups = ["architecture maps", "boundary guards", "facades"]
review_domains = ["architecture", "security"]
route_samples = ["doc/api/openapi.yaml", "cmd/api/routes.go"]

[[workstream_profiles.gates]]
name = "security-review"
mode = "advisory"
task_kinds = ["facade"]
evidence_type = "security-review"
required_evidence_count = 1
accepted_results = ["pass", "partial"]
artifact_required = true

[[packet_templates]]
profiles = ["platform-foundation"]
name = "architecture-map"
required_fields = ["scope", "current_owner", "target_owner", "migration_risk", "acceptance"]
optional_fields = ["source_doc"]
```

Validate after changing config:

```bash
fairway config validate
```

Before closing a task, handoff, deploy, or UAT run, use the workflow guard:

```bash
fairway workflow check
fairway workflow check --mode deploy --require-clean --require-pushed
```

The first command warns about dirty docs/code, unpushed commits, and active
Fairway reconciliation findings. The deploy mode requires clean pushed source
so CI has a chance to run before deploy evidence is recorded.

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
  profile: service-extraction
  owning_domain: orders
  owning_layer: api
  source_paths: ["internal/orders"]
  target_paths: ["cmd/api/routes/orders.go"]
  review_domains: ["architecture", "backend"]
  risk_level: medium
  migration_type: endpoint
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

If you are installing from a local checkout, `make install` writes
`~/.local/bin/fairway` by default. Override with
`make install PREFIX=/opt/homebrew` or `make install BINDIR=/some/bin` when
needed.

```bash
fairway dashboard
```

Opens `http://127.0.0.1:7878`. Leave it open on a second monitor.

For an operator dashboard that should survive the launching terminal or agent
thread, use the detached lifecycle commands:

```bash
fairway dashboard start
fairway dashboard status
fairway dashboard restart
fairway dashboard stop
```

Detached mode writes `.fairway/dashboard.pid` and `.fairway/dashboard.log` by
default.

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

If the task's kind belongs to a configured workstream profile, `merge-ready`
also evaluates that profile's gates. Missing `blocking` profile gates fail the
check; missing advisory/report-only gates are printed as warnings.

In the UI worktree:

```bash
fairway ready                 # T-002 is now ready (T-001 reached done)
fairway claim T-002
fairway record handoff T-002 --to backend --payload "Need an example payload for /v1/orders before finalizing the form."
```

## What you see in the dashboard

- The track summary shows total, ready, in-progress, blocked, done, and
  workstream counts.
- Workstream progress cards group tasks by profile and kind when profile
  metadata is present.
- Profile gate readiness shows whether evidence-backed gates are satisfied
  across the track, with links to missing tasks.
- The lanes strip updates within a second of each transition.
- The activity feed shows the chain: claim -> evidence -> done; claim ->
  handoff, with kind and row-count filters for busy tracks.
- Task tables are bounded by a row limit so large queues stay usable; narrow
  filters or raise the table limit when you need more detail.
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

For architecture-heavy workstreams, start from the generic
[`examples/platform-foundation-queue.yaml`](../examples/platform-foundation-queue.yaml)
fixture to see profile-aware task metadata in context. For planning or
documentation tasks, prefer an `orchestrator` owner with architecture,
governance, security, ops, or frontend listed as review domains. That keeps
coordination ownership separate from review approval and avoids self-review
when review routes point back to the architecture lane.

## Adoption Readiness

When you are trying Fairway in an existing project, generate an adoption
artifact before switching your team over:

```bash
fairway adoption artifact --catalog examples/regression-packs.yaml
fairway --json adoption artifact --route cmd/api/routes.go
```

The artifact summarizes task counts, ready work, configured gates, workstream
profile gates, evidence-backed gate evaluation, review-route samples,
regression-pack validation, health, and evidence gaps. If no `--route` flags are
provided, Fairway samples paths from the configured `route_samples`. See
[workstream-profile-guide.md](workstream-profile-guide.md) for profile design
guidance.

If the project does not have regression packs yet, either add a tiny catalog for
the workstream or omit `--catalog` and treat the regression-pack section as
future coverage. The adoption artifact is still useful for profile gates,
review routes, evidence gaps, and worktree health.

## Adoption References

Fairway is generic, but it was hardened against real multi-agent stabilization
work. The example configs remain useful when you want a larger reference
configuration with architecture, ops, frontend, backend, and governance lanes.

Use
[`examples/gpuaas-a-b-c-d-e-config.toml`](../examples/gpuaas-a-b-c-d-e-config.toml)
for the exact A/B/C/D/E lane mapping, platform-foundation workstream profile,
named advisory gates, packet templates, and
[`examples/gpuaas-regression-packs.yaml`](../examples/gpuaas-regression-packs.yaml)
to verify Fairway accepts GPUaaS-style per-environment regression blocking.

Historical adoption notes are kept in [archive/](archive/README.md), not the
main getting-started path.
