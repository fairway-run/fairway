<p align="center">
  <img src="assets/logo-lockup.svg" alt="fairway" width="240">
</p>

# Fairway

**Coordination control plane for multi-agent engineering work.**

Fairway is a local-first coordination tool for teams running multiple coding
agents in parallel on one repository. Traffic-control lane coordination was the
original primitive: each lane has a task queue, state machine, worktree,
evidence trail, handoff/review flow, session record, and live dashboard. The
product has grown from that primitive into a coordination control plane for
ownership, evidence, reviews, readiness, risk, usage attribution, utility
handbacks, and release posture.

Use it when the problem is not "can an agent write code?" but "can several
agents work at once without losing ownership, context, review state, or proof of
what changed?"

## Status

Usable local prototype. Core CLI, SQLite store, migrations, worktrees, sessions,
packets, checkpoints, regression-pack helpers, tracker links, workstream profile
config, dashboard, and release packaging are implemented and covered by smoke
tests.

Current focus: make Fairway easy to adopt outside this repository: public docs,
Homebrew distribution, Docusaurus portal, workflow guards that reduce manual
process, and clearer agent operating guidance. The GPUaaS work remains the
first real adoption case study, but Fairway is not GPUaaS-specific.

Still maturing: provider-specific session launchers, richer dashboard
mutations, real Jira/Linear API adapters, first tagged release execution through
the configured Homebrew cask pipeline, and a future Postgres runtime adapter.

## Why It Exists

Multi-agent engineering breaks down when coordination lives in chat threads:

- two agents unknowingly touch the same boundary,
- a task is marked active but no provider is actually running,
- evidence exists but the task never gets closed,
- a reviewer cannot tell which domain still needs approval,
- long UAT or deployment work leaves stale state behind.

Fairway keeps those facts in a local execution store instead of depending on
provider memory, issue tracker comments, or CI logs as the coordination source
of truth.

## What It Is

- A single Go binary (`fairway`) plus an embedded SQLite store.
- A CLI for claiming tasks, recording handoffs / evidence / reviews, managing sessions.
- A local web dashboard with wall, board, diagnostics, and task-detail flows.
- Config-driven: roles, branches, worktree paths, review routing, workstream profiles, packet templates, and the state machine are all defined in `.fairway/config.toml`.
- Tracker-friendly: Plane, Jira, Linear, GitHub Issues, and similar tools can
  mirror planning context, but Fairway remains authoritative for local execution
  state, evidence, reviews, sessions, and merge readiness.

## What It Is Not

- Not a workflow engine (Temporal, Cadence, Argo Workflows).
- Not an IAM tool.
- Not a CI runner.
- Not an issue tracker replacement for stakeholder planning.
- Not an LLM provider abstraction. Fairway records provider-neutral sessions
  and adapter-supplied usage metadata; providers remain external.
- Not an autonomous approval, claim, merge, push, deploy, or cleanup engine.
- Not a transcript, prompt, secret, provider-token, or credential store by
  default.
- Not a token-cost gate. Usage accounting is advisory planning telemetry, not a
  task completion gate.

## Quickstart

Install from source while the first tagged binary release is being prepared:

```bash
go install github.com/fairway-run/fairway/cmd/fairway@latest
# or, from a checkout:
make install                  # writes ~/.local/bin/fairway by default
```

Tagged releases publish signed/notarized macOS artifacts and update the
Homebrew cask:

```bash
brew tap fairway-run/tap
brew install --cask fairway
```

```bash
fairway init                  # scaffold .fairway/config.toml and the SQLite DB
fairway help                  # print the command summary (-h/--help also work)
$EDITOR .fairway/config.toml  # define roles, branches, worktree root, review routes
fairway config validate
fairway worktree setup        # create per-role branches and worktrees
fairway dashboard             # open the dashboard at http://127.0.0.1:7878
fairway import tasks.yaml     # or fairway add T-001 --title ... --role ...
fairway ready                 # list tasks ready for your role to claim
fairway claim T-001
fairway record evidence T-001 --command-text "go test ./..." --result pass
fairway workflow check          # docs/code/commit/push/session guard
fairway set-status T-001 done
fairway adoption artifact     # summarize routes, gate evaluation, health, and evidence gaps
```

See [docs/quickstart.md](docs/quickstart.md) for the full walkthrough.

The dashboard has one route model: `/` is the wall view for lane-level
coordination, `/board` is the sortable/filterable operator board,
`/board?tab=diagnostics` shows sessions/worktrees/watchers/checkpoints, and
`/tasks/<id>` opens task detail.

## Documentation

Public docs are available at [fairway.run](https://fairway.run).

Start here:

- [Quickstart](docs/quickstart.md) — first local project setup
- [Product](docs/product.md) — vision, principles, roadmap, anti-goals
- [Governed agentic engineering](docs/governed-agentic-engineering.md) — the operating model Fairway supports
- [Product boundaries](docs/design/product-boundaries.md) — what Fairway coordinates and what it deliberately does not do
- [Backlog sources](docs/design/backlog-sources.md) — active backlog, archive, examples, and runtime DB authority
- [Agent guide](docs/agent-guide.md) — practical command flow for agents using Fairway
- [Dashboard](docs/design/dashboard.md) — wall, board, diagnostics, reports, and task detail
- [Rule packs](docs/design/rule-packs.md) — reusable operating knowledge that Fairway can load, recommend, and turn into evidence expectations
- [Release notes](docs/release-notes.md) — current release candidate scope and known limits

For maintainers and repository agents:

- [AGENTS.md](AGENTS.md) — orientation for any agent (Claude, Codex, Gemini, human) working in this repo
- [Architecture](docs/architecture.md) — components, data flow, package layout
- [Workstream profile guide](docs/workstream-profile-guide.md) — user-facing guide for profile config, gates, and adoption artifacts

Core Design:

- [Scope and non-goals](docs/design/scope.md)
- [Product boundaries](docs/design/product-boundaries.md)
- [Backlog sources](docs/design/backlog-sources.md)
- [Concepts](docs/design/concepts.md)
- [Release cuts](docs/design/release-cuts.md)
- [Implementation roadmap](docs/design/implementation-roadmap.md)
- [Workstream profiles](docs/design/workstream-profiles.md)
- [Rule packs](docs/design/rule-packs.md)
- [Schema](docs/design/schema.md)
- [State machine](docs/design/state-machine.md)
- [Hierarchy (epics, stories, spawn)](docs/design/hierarchy.md)
- [Coordinator loop](docs/design/coordinator-loop.md)
- [Context packets](docs/design/context-packets.md)
- [Regression packets](docs/design/regression-packets.md)
- [Watchers](docs/design/watchers.md)
- [Checkpoints](docs/design/checkpoints.md)
- [Review lanes](docs/design/review-lanes.md)
- [Session launch](docs/design/session-launch.md)
- [Provider usage accounting](docs/design/provider-usage-accounting.md)
- [Work batch model](docs/design/work-batch-model.md)
- [Postgres adapter](docs/design/postgres-adapter.md)
- [Issue tracker integrations](docs/design/issue-tracker-integrations.md)
- [Worktrees](docs/design/worktrees.md)
- [Dashboard](docs/design/dashboard.md)
- [Multi-project mode](docs/design/multi-project.md)
- [CLI surface](docs/design/cli.md)

Archive:

- [Archived decision logs and adoption notes](docs/archive/README.md)

Governance:

- [Coding standards](docs/governance/coding-standards.md)
- [Testing](docs/governance/testing.md)
- [Review guards](docs/governance/review-guards.md)
- [Commits](docs/governance/commits.md)
- [Release](docs/governance/release.md)

Reference:

- [Config reference](docs/config-reference.md)
- [Quickstart](docs/quickstart.md)
- [Workstream profile guide](docs/workstream-profile-guide.md)

Examples:

- [GPUaaS-style 5-lane config](examples/gpuaas-config.toml)
- [GPUaaS exact A/B/C/D/E config](examples/gpuaas-a-b-c-d-e-config.toml)
- [GPUaaS regression-pack fixture](examples/gpuaas-regression-packs.yaml)
- [Generic platform-foundation queue](examples/platform-foundation-queue.yaml)
- [Session adapter examples](examples/session-adapters/)

## License

Apache-2.0. See [LICENSE](LICENSE).
