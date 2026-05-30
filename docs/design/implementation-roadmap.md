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
- Contextual spawning, review checkout, session reconciliation, dispatch plans,
  bugfix/watcher packets, watcher lifecycle, and regression-pack catalogs.
- Project registry commands and first-pass multi-project dashboard mode.
- Local tracker links for Jira/Linear and dry-run tracker reconcile.
- SQLite backup/export and JSON output for core read commands.
- Read-only dashboard with SSE refresh, role grouping, health badges, task
  detail, activity feed, sessions, worktrees, checkpoints, watchers, and
  rollups.
- Embedded forward-only migrations and migration compatibility checks.
- Basic TUI mode for headless ready/claim/status/detail workflows.
- First audited dashboard mutation path with CSRF-protected claim.
- Release packaging config and tag-triggered GitHub release workflow.
- GPUaaS parity import path, exact GPUaaS config, parity runbook, adoption
  parity artifact, and agent usage guide.

## Completed v0.2 Track

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

## Remaining Work

1. Expand platform-foundation orchestration support from the
   [GPUaaS / ARC adoption track](gpuaas-arc-adoption.md). Architecture-map,
   boundary-guard, and vertical-slice packets exist, and task metadata now
   covers profile, owning domain/layer, source/target paths, review domains,
   risk, and migration type. The dashboard now surfaces an initial workstream
   grouping over profile/kind metadata, and configured packet templates can
   render profile-specific packets. Dashboard filters now cover profile, kind,
   owning domain, risk, and review domain. Next slices are structured guard
   evidence and release-level readiness reports.
2. Expand configurable [workstream profiles](workstream-profiles.md). The
   initial config shape now exists for profile metadata, route samples, named
   gates, packet templates, template profile scoping, and structured evidence
   requirements. Adoption artifacts now evaluate named gates against task
   evidence rows, and `merge-ready` enforces missing `blocking` gates for the
   target task; dashboard grouping by task kind/profile has started. Next
   slices are structured guard evidence, release-level readiness reports, and
   multi-reviewer readiness.
3. Add multi-reviewer merge readiness for review domains such as architecture,
   security, frontend, ops, and governance.
4. Add Homebrew tap publishing after the first tagged release settles.
5. Expand dashboard mutations beyond claim while keeping CSRF and per-action
   audit on every write.
6. Expand TUI ergonomics beyond the basic command loop.
7. Expand tracker integration from local links/dry-run reporting to provider
   adapters when credentials and API mapping are explicit.
