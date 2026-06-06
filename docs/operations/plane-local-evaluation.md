# Plane Local Evaluation

This runbook sets up Plane as a local planning surface for Fairway tracker
adapter evaluation. It does not make Plane the Fairway execution store. Fairway
remains authoritative for task state, sessions, evidence, handoffs, reviews,
checkpoints, and merge readiness.

Plane setup changes over time. Use the official Plane Docker Compose installer
as the source of truth:

- Plane Docker Compose docs: https://developers.plane.so/self-hosting/methods/docker-compose
- Plane releases: https://github.com/makeplane/plane/releases

## Goal

Evaluate whether Plane works as a product/external-team planning surface for
Fairway tasks before implementing a generic tracker adapter contract.

The evaluation should answer:

- Which Plane concepts map cleanly to Fairway task definitions?
- Which Plane concepts should remain planning-only?
- What status/comment/export operations are useful without mutating Fairway
  execution state?
- What adapter behavior should generalize to Jira and Linear?

## Prerequisites

- Docker Desktop on macOS or Docker Engine plus Docker Compose on Linux.
- At least 2 vCPU and 4 GB RAM for a small local evaluation instance. Plane
  recommends more memory for sustained use.
- A local directory outside the Fairway repo for Plane runtime files, for
  example `~/dev/plane-eval`.

Check Docker Compose:

```bash
docker compose version
```

## Runtime Directory

Do not run Plane runtime files inside this repository. Keep generated compose
files, environment files, volumes, cookies, exports, and tokens outside the repo:

```bash
mkdir -p ~/dev/plane-eval
cd ~/dev/plane-eval
```

If you temporarily place local runtime files under the Fairway checkout, use a
directory named `plane-eval-local/`; it is ignored by git.

## Install Or Refresh Plane

Use the official setup script from the latest Plane release:

```bash
cd ~/dev/plane-eval
curl -fsSL -o setup.sh https://github.com/makeplane/plane/releases/latest/download/setup.sh
chmod +x setup.sh
./setup.sh
```

Choose the install action in the interactive menu. The setup script downloads
Plane's compose and environment files into a generated Plane app directory.

Record the exact version or source used:

```bash
git ls-remote --tags https://github.com/makeplane/plane.git | tail -20
grep -E 'PLANE|VERSION|IMAGE' plane-app/plane.env 2>/dev/null || true
```

If the generated files use a different directory name, substitute that directory
for `plane-app` below.

## Configure Local Ports

Open the generated environment file and set a local-only port if `80` is not
available:

```bash
$EDITOR plane-app/plane.env
```

Recommended local evaluation defaults:

```text
LISTEN_HTTP_PORT=8088
```

Do not commit `plane.env` or any copied environment file. It may contain local
secrets, generated keys, SMTP settings, or cookies.

## Start Plane

Use the generated setup script menu, or run Compose from the generated app
directory:

```bash
cd ~/dev/plane-eval/plane-app
docker compose up -d
docker compose ps
```

Expected local URL:

```text
http://localhost:8088
```

If you kept `LISTEN_HTTP_PORT=80`, use `http://localhost`.

## Startup Verification

Verify the service is reachable before creating evaluation data:

```bash
curl -I http://localhost:8088
docker compose ps
docker compose logs --tail=100
```

Expected result:

- `curl` returns an HTTP response from Plane.
- Compose services are running or healthy.
- Logs do not show repeated database, migration, or worker restart errors.

## Initial Admin Account

Plane may require initial browser setup for the first admin account. This is the
only expected manual browser step.

Do not commit:

- admin email/password,
- API tokens,
- session cookies,
- browser exports,
- `plane.env`,
- database dumps.

Use a local-only evaluation account.

## Create Fairway Evaluation Workspace

In Plane, create:

- Workspace: `fairway-eval`
- Project: `Fairway Tracker Adapter Evaluation`
- Optional cycle: `FW-120 Local Evaluation`
- Optional modules:
  - `Planning Mirror`
  - `Execution Boundary`
  - `Follow-up Taxonomy`
  - `Release Readiness`

Use `examples/tracker-adapters/plane/evaluation-workspace.yaml` as the seed
manifest for representative issues, labels, modules, and comments. The file is
not a Plane API payload; it is a stable evaluation contract that can later drive
an adapter dry-run.

## Representative Data

Create representative Plane issues for:

- a Fairway epic or parent task,
- a normal implementation task,
- a boundary/review-heavy task,
- a monitor/deploy-run bookkeeping task,
- `CI-FIX`, `CD-FIX`, `UAT-BUG`, `OPS-FIX`, `HARNESS-FIX`, and `DOC-FIX`
  follow-ups,
- a task with evidence links,
- a task with a blocked/residual-risk comment,
- a task with review-domain labels.

Use comments to test status export:

```text
Fairway execution summary:
- local task: FW-120
- state: in_progress
- owner/lane: ops
- evidence: docs/operations/plane-local-evaluation.md
- boundary: Plane is planning-only; Fairway DB owns execution state.
```

## Mapping Evaluation

Record mapping results in the Plane project and in any follow-up Fairway docs.

| Fairway concept | Plane concept to evaluate | Notes |
|---|---|---|
| task id | issue identifier, label, or custom field | Keep Fairway ID visible and stable. |
| title | issue name | Direct mapping. |
| notes | issue description | Good for planning context, not execution truth. |
| parent | epic/module/parent issue | Evaluate hierarchy limits. |
| status | issue state | Planning mirror only; must not mutate Fairway state implicitly. |
| priority | issue priority | Import/export mapping can be config-driven. |
| role | label or custom field | Useful for routing views. |
| owning domain | label/module/custom field | Useful for product grouping. |
| kind | issue type or label | Must stay provider-neutral. |
| review domains | labels/checklist/custom field | Planning signal, not approval truth. |
| acceptance checks | description checklist | Importable as task definition text. |
| evidence links | comment or attachment link | Export summary only; Fairway evidence rows remain authoritative. |
| follow-up taxonomy | labels and issue key prefixes | Useful for reports and planning. |

## Planning-Only Boundary

Plane may own:

- roadmap grouping,
- stakeholder discussion,
- issue labels and modules,
- project/cycle planning,
- broad priority and product intent,
- external team comments.

Plane must not silently mutate:

- Fairway task status,
- local task owner/claimant,
- active provider session,
- checkpoint state,
- evidence result,
- review approval,
- merge-ready or release-ready gates.

Any future adapter write must be an explicit operator action with a dry-run
preview first.

## Shutdown

```bash
cd ~/dev/plane-eval/plane-app
docker compose down
```

This stops containers while preserving named volumes.

## Reset

Only reset when you intentionally want to discard evaluation data:

```bash
cd ~/dev/plane-eval/plane-app
docker compose down -v
```

If you created browser-only seed data and need to preserve it, export notes from
Plane before resetting.

## Repeatability Checklist

- Docker Compose version recorded.
- Plane release/setup source recorded.
- Local URL recorded.
- Startup verification commands pass.
- Workspace/project/cycle/modules created.
- Evaluation issues match
  `examples/tracker-adapters/plane/evaluation-workspace.yaml`.
- Mapping table filled with clean mappings and planning-only concepts.
- No secrets, tokens, cookies, env files, or dumps committed.
- Follow-up notes identify requirements for FW-121 provider-neutral tracker
  adapter contract.
