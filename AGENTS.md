# AGENTS.md

Canonical orientation for any agent — Claude, Codex, Gemini, or human — working in this repository. Tool-specific entries (`CLAUDE.md`, future `GEMINI.md`, etc.) defer to this file.

## What fairway is

A standalone Go binary that coordinates multiple coding agents working in parallel on a single repository. Read [README.md](README.md) and [docs/design/scope.md](docs/design/scope.md) before touching code.

## How fairway dogfoods itself

Fairway is developed using fairway-style multi-agent workflows. Five role lanes work in parallel on long-lived branches:

| Role | Branch | Scope |
|---|---|---|
| [backend](agents/backend.md) | `agent/backend` | CLI, store, state machine, sessions, git ops, reports |
| [ui](agents/ui.md) | `agent/ui` | Dashboard templates, CSS, HTMX, SSE client behavior |
| [ops](agents/ops.md) | `agent/ops` | CI, releases, packaging, goreleaser |
| [arch](agents/arch.md) | `agent/arch` | Schema, state machine design, cross-cutting APIs |
| [governance](agents/governance.md) | `agent/governance` | Docs, process, this file, governance/ |

Tasks flow through the configured state machine (see [docs/design/state-machine.md](docs/design/state-machine.md)). Cross-role work hands off via `fairway record handoff`. Reviews route via config — see [docs/governance/review-guards.md](docs/governance/review-guards.md).

Workstream profiles in `.fairway/config.toml` may define task kinds, route
samples, named gates, review domains, dashboard groups, and packet templates.
Agents should treat profile gates as the local definition of "ready enough" for
that track. See [docs/design/workstream-profiles.md](docs/design/workstream-profiles.md)
and [docs/config-reference.md](docs/config-reference.md).

## Before you start

Every agent must read:

1. [docs/agent-guide.md](docs/agent-guide.md)
2. [docs/design/release-cuts.md](docs/design/release-cuts.md)
3. [docs/governance/coding-standards.md](docs/governance/coding-standards.md)
4. [docs/governance/testing.md](docs/governance/testing.md)
5. [docs/governance/review-guards.md](docs/governance/review-guards.md)
6. [docs/governance/commits.md](docs/governance/commits.md)
7. Your role file (linked above).

## Operating principles

1. **The DB is the source of truth.** Never edit `task_state` rows out-of-band. Use `fairway` commands.
2. **Stay in your lane.** If a task crosses roles, hand it off — do not reach across.
3. **No self-review.** Reviews are routed by config; you cannot approve your own work.
4. **Evidence over assertion.** Even when `[gates] require_evidence_before_done = false`, prefer recording artifact paths over silence.
5. **One worktree, one role.** Do not switch roles by changing branches inside a worktree.
6. **Do not subdivide assigned tasks into fairway sub-tasks.** Track internal execution steps in your own scratch (todo file, Claude's task list, `WORKLOG.md`). Use `fairway spawn --sibling` only for *genuinely new* work the orchestrator should see. If a task is too big, hand it back to `arch` with a suggested split — do not split it yourself. See [hierarchy.md](docs/design/hierarchy.md#task-granularity-who-decides).
7. **Bound side work with packets and checkpoints.** Long-running side work, watcher work, or newly discovered follow-up needs a context packet and fresh checkpoint rather than an untracked thread summary. See [context-packets.md](docs/design/context-packets.md) and [checkpoints.md](docs/design/checkpoints.md).
8. **Respect the active profile.** For profile-shaped work, use the configured task kinds, packet templates, route samples, and named gates. Do not invent project-specific gates in chat; capture them in config or docs.

## Architecture

See [docs/architecture.md](docs/architecture.md) for the component layout and data flows.

## Product context

See [docs/product.md](docs/product.md) for vision, principles, roadmap, and anti-goals. Read this before proposing scope changes.

## Tool-specific entries

- Claude / Claude Code: [CLAUDE.md](CLAUDE.md)

Add more as needed (`GEMINI.md`, `CODEX.md`); each defers to this file for substantive guidance.
