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
- TUI mode for headless ready/claim/status/detail, status updates, evidence,
  merge-ready, and readiness reports.
- Audited dashboard mutation paths with CSRF-protected claim and non-terminal
  status updates.
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
   owning domain, risk, and review domain. Structured guard reports can be
   recorded as typed evidence, and readiness reports evaluate profile gates
   across a workstream. Multi-reviewer readiness now enforces task-level
   `review_domains` in `merge-ready`.
2. Expand configurable [workstream profiles](workstream-profiles.md). The
   initial config shape now exists for profile metadata, route samples, named
   gates, packet templates, template profile scoping, and structured evidence
   requirements. Adoption artifacts now evaluate named gates against task
   evidence rows, and `merge-ready` enforces missing `blocking` gates for the
   target task; dashboard grouping by task kind/profile has started, and
   task-level `review_domains` are enforced by `merge-ready`.
3. Add Homebrew tap publishing after the first tagged release settles.
4. Expand dashboard mutations beyond claim/status only when parity with CLI
   gates stays explicit.
5. Expand TUI ergonomics beyond the current command loop when usage shows which
   shortcuts matter.
6. Expand tracker integration from local links/dry-run reporting to provider
   adapters when credentials and API mapping are explicit.

## Discovered During GPUaaS Adoption

These are product follow-ups found while using Fairway to coordinate the
GPUaaS platform-foundation and Docusaurus portal tracks. Keep them here until
they graduate into a release cut or an imported Fairway task queue.

The immediate follow-ups are also available as an importable queue in
`examples/fairway-adoption-improvements.yaml` so GPUaaS adoption can continue
while Fairway improvements are assigned as their own bounded workstream.

### Immediate When Needed

1. Dashboard workstream data volume
   - Current state: global search, status/profile/kind/domain/risk/review
     filters, table row limits, and activity kind/row limits are implemented.
   - Next pressure point: when a workstream has several days of task churn, add
     per-workstream pagination or expandable workstream sections so the main
     pane stays scannable without hiding too much context.
   - Trigger: any real adoption track has more than 50 tasks in one
     profile/kind group, or users repeatedly raise `table_limit` to 100+.

2. Activity feed query scaling
   - Current state: dashboard fetches a bounded recent activity window and
     filters in memory.
   - Next pressure point: add store-level filters for activity kind, task id,
     profile, and time window instead of fetching a large mixed feed first.
   - Trigger: activity count regularly exceeds a few hundred rows during active
     tracks, or dashboard first paint slows noticeably.

3. Gate-readiness drill-down
   - Current state: the dashboard shows profile gates, satisfied counts, and
     missing task links.
   - Next pressure point: add a task-detail gate panel so a single task shows
     which profile gates it satisfies, which evidence rows counted, and which
     reason remains open.
   - Trigger: users inspect a task after seeing a gate miss and still need the
     CLI to understand why.

4. SQLite busy handling for burst local writes
   - Current state: multiple Fairway CLI writes issued at the same time can hit
     `SQLITE_BUSY`; serial reruns succeed.
   - Next pressure point: add a small busy timeout/retry policy around store
     writes so shell scripts or orchestrators can record evidence/reviews in
     quick succession without manual retry.
   - Trigger: adoption tracks commonly record several evidence or review rows
     from scripts, or users see database lock errors during normal local use.

5. Prompt-file session launch
   - Current state: `session upsert` can record tmux/zellij sessions and the
     example adapters prove the shape, but launching long agent prompts through
     inline shell/tmux command strings is brittle.
   - Next pressure point: add a prompt-file based adapter path or documented
     launcher convention that writes the prompt to a file, starts the provider
     with that file, captures a transcript, and records the session in one
     repeatable command.
   - Trigger: platform-foundation lanes launch multiple bounded Claude/Codex
     tasks through tmux and need consistent prompt/transcript/session handling.

### Later / Product Polish

1. Saved dashboard views
   - Save common filters such as "docs portal API work", "blocked security
     reviews", or "evidence activity" as URL-addressable views.

2. Dashboard route parity with CLI reports
   - Add read-only dashboard pages for readiness report, adoption artifact, and
     merge-ready detail using the same evaluation logic as the CLI.

3. Provider tracker follow-through
   - Once Jira/Linear adapters are configured, export discovered Fairway
     follow-ups to the planning tool while keeping the Fairway DB authoritative
     for execution state.
