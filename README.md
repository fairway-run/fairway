# fairway

**traffic control for coding agents**

Fairway coordinates multiple coding agents working in parallel on a single repository: task queue, state machine, lane / worktree management, handoff / evidence / review chain, session tracking, and a live local dashboard.

The name comes from maritime traffic control — fairways are navigable channels for vessels under VTS coordination. Agents transit worktree lanes under fairway's coordination.

## Status

Pre-alpha. The design is settled (see [docs/design/](docs/design/)); implementation begins in week 1. Not yet usable.

## What it is

- A single Go binary (`fairway`) plus an embedded SQLite store.
- A CLI for claiming tasks, recording handoffs / evidence / reviews, managing sessions.
- A local web dashboard for watching lanes, backlog, and activity in real time.
- Config-driven: roles, branches, worktree paths, review routing, and the state machine are all defined in `.fairway/config.toml`.

## What it is not

- Not a workflow engine (Temporal, Cadence).
- Not an IAM tool.
- Not a CI runner.
- Not an LLM provider abstraction — fairway dispatches to whatever agent you run inside a worktree; it does not spawn agents itself.

## Quickstart (sketched — not yet runnable)

```bash
fairway init                  # scaffold .fairway/config.toml and the SQLite DB
$EDITOR .fairway/config.toml  # define roles, branches, worktree root, review routes
fairway worktree setup        # create per-role branches and worktrees
fairway dashboard             # open http://127.0.0.1:7878 in your browser
fairway ready                 # list tasks ready for your role to claim
fairway claim T-001
fairway set-status T-001 done
```

See [docs/quickstart.md](docs/quickstart.md) for the full walkthrough.

## Documentation

Start here:

- [AGENTS.md](AGENTS.md) — orientation for any agent (Claude, Codex, Gemini, human) working in this repo
- [Architecture](docs/architecture.md) — components, data flow, package layout
- [Product](docs/product.md) — vision, principles, roadmap, anti-goals

Design:

- [Scope and non-goals](docs/design/scope.md)
- [Concepts](docs/design/concepts.md)
- [Release cuts](docs/design/release-cuts.md)
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

Examples:

- [GPUaaS-style 5-lane config](examples/gpuaas-config.toml)

## License

Apache-2.0. See [LICENSE](LICENSE).
