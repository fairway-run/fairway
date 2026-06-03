# fairway

**traffic control for coding agents**

Fairway coordinates multiple coding agents working in parallel on a single repository: task queue, state machine, lane / worktree management, handoff / evidence / review chain, session tracking, and a live local dashboard.

The name comes from maritime traffic control — fairways are navigable channels for vessels under VTS coordination. Agents transit worktree lanes under fairway's coordination.

## Status

Usable local prototype. Core CLI, SQLite store, migrations, worktrees, sessions,
packets, checkpoints, regression-pack helpers, tracker links, workstream profile
config, dashboard, and release packaging are implemented and covered by smoke
tests.

Current focus: make Fairway a generic coordination layer with configurable
workstream profiles, while proving GPUaaS as the first adoption example. See
[workstream profiles](docs/design/workstream-profiles.md), the
[GPUaaS adoption track](docs/design/gpuaas-arc-adoption.md), and the
[parity runbook](docs/assessment/gpuaas-parity-runbook.md).

Still maturing: provider-specific session launchers, richer dashboard
mutations, real Jira/Linear API adapters, Homebrew tap publishing after the
first release tag, and a future Postgres runtime adapter.

## What it is

- A single Go binary (`fairway`) plus an embedded SQLite store.
- A CLI for claiming tasks, recording handoffs / evidence / reviews, managing sessions.
- A local web dashboard for watching lanes, backlog, and activity in real time.
- Config-driven: roles, branches, worktree paths, review routing, workstream profiles, packet templates, and the state machine are all defined in `.fairway/config.toml`.

## What it is not

- Not a workflow engine (Temporal, Cadence).
- Not an IAM tool.
- Not a CI runner.
- Not an LLM provider abstraction — fairway dispatches to whatever agent you run inside a worktree; it does not spawn agents itself.

## Quickstart

Install the local development binary:

```bash
make install                  # writes ~/.local/bin/fairway by default
```

```bash
fairway init                  # scaffold .fairway/config.toml and the SQLite DB
fairway help                  # print the command summary (-h/--help also work)
$EDITOR .fairway/config.toml  # define roles, branches, worktree root, review routes
fairway config validate
fairway worktree setup        # create per-role branches and worktrees
fairway dashboard             # open http://127.0.0.1:7878 in your browser
fairway import tasks.yaml     # or fairway add T-001 --title ... --role ...
fairway ready                 # list tasks ready for your role to claim
fairway claim T-001
fairway record evidence T-001 --command-text "go test ./..." --result pass
fairway set-status T-001 done
fairway adoption artifact     # summarize routes, gate evaluation, health, and evidence gaps
```

See [docs/quickstart.md](docs/quickstart.md) for the full walkthrough.

## Documentation

Start here:

- [AGENTS.md](AGENTS.md) — orientation for any agent (Claude, Codex, Gemini, human) working in this repo
- [Agent guide](docs/agent-guide.md) — practical command flow for agents using fairway
- [Architecture](docs/architecture.md) — components, data flow, package layout
- [Product](docs/product.md) — vision, principles, roadmap, anti-goals
- [Workstream profile guide](docs/workstream-profile-guide.md) — user-facing guide for profile config, gates, and adoption artifacts

Design:

- [Scope and non-goals](docs/design/scope.md)
- [Concepts](docs/design/concepts.md)
- [Release cuts](docs/design/release-cuts.md)
- [Implementation roadmap](docs/design/implementation-roadmap.md)
- [GPUaaS / ARC adoption](docs/design/gpuaas-arc-adoption.md)
- [Workstream profiles](docs/design/workstream-profiles.md)
- [Schema](docs/design/schema.md)
- [State machine](docs/design/state-machine.md)
- [GPUaaS extraction](docs/design/gpuaas-extraction.md)
- [Hierarchy (epics, stories, spawn)](docs/design/hierarchy.md)
- [Coordinator loop](docs/design/coordinator-loop.md)
- [Context packets](docs/design/context-packets.md)
- [Regression packets](docs/design/regression-packets.md)
- [Watchers](docs/design/watchers.md)
- [Checkpoints](docs/design/checkpoints.md)
- [Review lanes](docs/design/review-lanes.md)
- [Session launch](docs/design/session-launch.md)
- [Postgres adapter](docs/design/postgres-adapter.md)
- [Issue tracker integrations](docs/design/issue-tracker-integrations.md)
- [Worktrees](docs/design/worktrees.md)
- [Dashboard](docs/design/dashboard.md)
- [Multi-project mode](docs/design/multi-project.md)
- [CLI surface](docs/design/cli.md)
- [Open questions](docs/design/open-questions.md)

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
