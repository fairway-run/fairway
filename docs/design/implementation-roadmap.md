# Implementation Roadmap

This is the working implementation ledger. `release-cuts.md` defines ship
scope; this file tracks the fuller path from the current standalone prototype
to a useful v1.

## Already Implemented

- Core task queue: `init`, `import`, `add`, `update`, `ready`, `claim`,
  `set-status`, `task-detail`, `tree`.
- Audit rows for state transitions, evidence, handoffs, and reviews.
- Review routing, no-self-review enforcement, and `merge-ready`.
- Git safety surfaces: `git-check`, `preflight`.
- Role worktrees: `worktree setup/status/prune`.
- Agent sessions: `session upsert/status/end`.
- Coordinator reports: `coordinator preflight/status/tick`.
- Checkpoints and context packets: `checkpoint record/status/stale`,
  `packet context`.
- SQLite backup/export and JSON output for core read commands.
- Read-only dashboard with SSE refresh, role grouping, health badges, task
  detail, and activity feed.

## Next Implementation Order

1. `fairway spawn`
   - Create discovered work without losing parent/epic context.
   - Require caller-supplied task IDs; fairway v1 does not auto-generate IDs.
   - Resolve current task from `--from-task`, `FAIRWAY_TASK_ID`, or the live
     session for the caller role.

2. Review lane commands
   - `fairway review checkout <task-id> [--source-role <role>]`.
   - Use configured review branch naming and existing worktree/git helpers.

3. Session reconciliation
   - `fairway session reconcile [--dry-run]`.
   - Mark missing PIDs as `stale` or `failed` without touching task claims.

4. Dispatch planning
   - `fairway dispatch-plan`.
   - Compose ready tasks, live sessions, dirty worktrees, stale checkpoints,
     and pending reviews into a suggested next-action list.

5. Packet completion
   - `packet bugfix`.
   - `packet watcher`.
   - `regression-pack list/show/validate`.

6. Watcher lifecycle
   - `watcher start/finish/status`.
   - Reuse checkpoints and evidence rows where possible.

7. Dashboard v0.2 visibility
   - Sessions strip.
   - Worktree dirty/branch status.
   - Latest checkpoints and stale checkpoint badges.
   - Merge-ready and review-route indicators.
   - Epic rollups.

8. Registry and multi-project
   - `register`, `unregister`, `projects`.
   - Multi-project dashboard mode backed by the local registry.

9. Tracker integration boundary
   - Jira/Linear link storage.
   - Dry-run import/reconcile/export.
   - Keep fairway DB authoritative for agent execution state.

10. Adapter and release hardening
    - `session launch` adapter contract for shell/tmux/zellij.
    - `db compat --backend postgres`.
    - CI/release packaging, checksums, Homebrew path.

## Release Hardening Notes

- `.goreleaser.yaml` builds darwin/linux amd64/arm64 archives and emits a
  checksums file.
- `.github/workflows/release.yml` runs on `v*` tags and creates a draft GitHub
  release.
- Homebrew tap publishing is intentionally left off until the v0.1 CLI is cut
  and the binary name/artifact layout has settled.

## Current Priority

Finish v0.2 CLI parity before broadening dashboard or external tracker work.
The CLI must stay complete enough that the dashboard remains observational
rather than privileged.
