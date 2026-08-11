# Architecture

## High-level component diagram

```
                  ┌──────────────────────────────────┐
                  │           fairway CLI            │
                  │         (cmd/fairway)            │
                  └──────────────┬───────────────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
        ▼                        ▼                        ▼
  ┌──────────┐            ┌───────────┐           ┌──────────────┐
  │  config  │            │   state   │           │  dashboard   │
  │  (TOML)  │            │ (machine) │           │ (HTTP + SSE) │
  └────┬─────┘            └─────┬─────┘           └──────┬───────┘
       │                        │                        │
       │              ┌─────────▼─────────┐              │
       └─────────────►│       store       │◄─────────────┘
                      │   (SQLite + Go    │
                      │     migrations)   │
                      └─────────┬─────────┘
                                │
                  ┌─────────────┼─────────────┐
                  ▼             ▼             ▼
              ┌────────┐   ┌─────────┐   ┌────────┐
              │  git   │   │ session │   │ report │
              │worktree│   │ tracker │   │ render │
              └────────┘   └─────────┘   └────────┘
```

Coding runtimes remain outside Fairway core. Provider/session adapters attach
runtime facts through validated Fairway commands and store methods. A future
optional Seaway adapter follows the same edge pattern:

```text
Fairway task/session/evidence APIs
              ^
              | correlated, sourced run facts
              |
optional Fairway-Seaway adapter
              |
              | public versioned run contract
              v
Seaway admission/events/results
```

The adapter owns version negotiation, correlation, cursors, deduplication, and
translation. It does not read either product's database, merge their state
machines, or gain task, review, cancellation, or promotion authority. See
[Optional Seaway integration](design/seaway-integration.md).

## Package layout

`cmd/fairway/` — CLI entrypoint. Thin: parses args via cobra, dispatches to `internal/*`. No business logic.

`internal/config/` — TOML loader and validator. Knows nothing about the DB; consumed by other packages.

`internal/store/` — SQLite schema, migrations (embedded via `//go:embed`), low-level queries. Exposes typed methods like `ClaimTask`, `RecordEvidence`, `Snapshot`. Holds the only `*sql.DB` instance. Threads `project_id` through every read and write — callers never pass it.

`internal/state/` — state machine. Pure logic. Takes a config and a transition request; returns valid/invalid plus the row to write into history. No DB access.

`internal/session/` — session lifecycle. PID detection, tmux pane detection (via `tmux display -p`), heartbeats.

`internal/git/` — git shellouts for worktree setup, branch creation, status, last-commit lookup. Uses `os/exec`; no libgit2.

`internal/report/` — status / health / timing / dispatch report generators. Reads from store; renders text or JSON.

`internal/coordinator/` — composed preflight/status/tick logic. Calls config,
store, git, session, and report packages; does not mutate tasks automatically.

`internal/packet/` — context, bugfix, and watcher packet rendering. Produces
Markdown or JSON artifacts from task/config inputs.

`internal/dashboard/` — HTTP server, HTML templates, SSE. Embeds `assets/` (HTMX, CSS) via `//go:embed`. Read-only views over the store. Supports both single-project and multi-project (`ATTACH DATABASE`) data sources via a swappable view layer.

`internal/registry/` — reads / writes `~/.fairway/registry.toml`. Used by `fairway register`, `fairway projects`, and the multi-project dashboard. The only fairway code that touches paths outside the current project.

## Data flow: `fairway claim T-042`

1. CLI parses args, resolves role (worktree path → config), loads config.
2. Opens the store via `internal/store`.
3. Calls `state.Validate(currentStatus, target, config.States)`.
4. On valid: store opens `BEGIN IMMEDIATE`, performs a guarded `UPDATE task_state ... WHERE claimant IS NULL`, inserts `task_state_history`, then commits. A losing concurrent claim returns `ErrAlreadyClaimed`.
5. CLI prints confirmation.
6. Dashboard SSE pollers (1Hz) pick up the new history row within ~1s and push it to connected clients.

## Data flow: `fairway merge-ready T-042`

1. CLI resolves the task, configured base branch, and review routes.
2. `internal/git` computes changed files for the task ref against the base ref.
3. `internal/store` checks configured gates: evidence rows, handoff rows, and
   approved review rows for all matched review routes.
4. `internal/git` verifies the ref is based on the configured base branch and
   the working tree is clean.
5. CLI prints a merge-ready summary or the missing gate(s).

## Data flow: `fairway dashboard`

1. CLI loads config + store.
2. Starts HTTP server on `[dashboard] listen`.
3. Routes:
   - `GET /` — lanes strip + backlog (server-rendered).
   - `GET /tasks/:id` — task detail.
   - `GET /partials/backlog` — HTMX partial.
   - `GET /events` — SSE stream of history merges.
4. Each request opens a short-lived read transaction.

## Concurrency model

- One `*sql.DB` per process.
- WAL mode enabled at open (`PRAGMA journal_mode=WAL`).
- All writes wrapped in transactions.
- Claim uses `BEGIN IMMEDIATE` plus a guarded update so two claimers cannot both
  win on SQLite.
- The dashboard is fully read-only against the store; it never holds locks across requests.

## Build & distribution

- `go build ./cmd/fairway` → single static binary.
- Pure Go, no CGO (via `modernc.org/sqlite`) — cross-compilation is trivial.
- Targets: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`.
- `goreleaser` produces archives + checksums per tag.
- Homebrew tap once v0.1 stabilizes.

## What lives where — quick reference

| Concern | Package |
|---|---|
| TOML parsing | `internal/config` |
| State transition validation | `internal/state` |
| Task DB writes | `internal/store` |
| Migration runner | `internal/store/migrations` |
| tmux / PID detection | `internal/session` |
| Optional runtime integration | Edge adapter using public Fairway and runtime contracts; not a core package |
| Worktree shellouts | `internal/git` |
| Coordinator preflight/status/tick | `internal/coordinator` |
| Context, bugfix, and watcher packet rendering | `internal/packet` |
| HTML templates, SSE | `internal/dashboard` |
| Text / JSON report rendering | `internal/report` |
| Project registry (`~/.fairway/registry.toml`) | `internal/registry` |
| CLI command wiring | `cmd/fairway` |
