# CLI verb surface

```
fairway init                                            # scaffold .fairway/config.toml + DB
fairway ready [--in <epic-id>] [--priority <n>]         # list tasks ready for the caller's role (sorted by priority, sequence, created_at)
fairway claim <task-id>                                 # transition todo → in_progress, assign owner
fairway claim --in <epic-id>                            # claim next ready descendant within an epic
fairway add <task-id> --title <t> [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--role <r>] [--acceptance <text>]... [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>]... [--target-paths <csv>]... [--review-domains <csv>]... [--risk-level <r>] [--migration-type <t>]
fairway spawn --id <task-id> --title <t> [--kind <k>] [--child | --sibling | --parent <id> | --root] [--from-task <id>] [--priority <n>] [--force] [--acceptance <text>]... [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>]... [--target-paths <csv>]... [--review-domains <csv>]... [--risk-level <r>] [--migration-type <t>]
fairway update <task-id> [--title <t>] [--notes <text>] [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--dependencies <a,b,c>] [--acceptance <text>]... [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>]... [--target-paths <csv>]... [--review-domains <csv>]... [--risk-level <r>] [--migration-type <t>]
fairway tree <task-id> [--depth <n>]                    # print descendant tree
fairway set-status <task-id> <state> [--reason <text>] [--commit <sha>] [--reopen]
fairway record evidence <task-id> --command-text <text> --result <pass|fail|partial|skipped|blocked> [--artifact <path>] [--artifact-type <type>] [--duration-seconds <n>] [--notes <text>]
fairway record guard-report <task-id> --guard <name> [--mode <report_only|warning|blocking>] [--finding <text>]... [--false-positive <text>]... [--allowed-debt <text>]... [--graduation-criteria <text>] [--artifact <path>] [--result <result>]
fairway record handoff <task-id> --to <role> --payload <text-or-@file>
fairway record review <task-id> --reviewer <role-or-user> --verdict <approve|changes|reject> [--reason <text>] [--commit <sha>]
fairway record usage <task-id> --provider <name> [--session-id <id>] [--external-session-id <id>] [--role <role>] [--phase <phase>] [--source <provider_reported|derived_snapshot|manual|unknown>] [--confidence <exact|estimated|unknown>] [--input-tokens <n>] [--cached-input-tokens <n>] [--output-tokens <n>] [--total-tokens <n>]
fairway route review <task-id> [--reviewer <role>] [--path <path>]... [--reason <text>] # mark pending review
fairway merge-ready <task-id> [--base <ref>]          # verify evidence/review/handoff/git/profile gates
fairway review checkout <task-id> [--source-role <role>] # create/reset named review branch
fairway session upsert --role <role> [--id <id>] [--lane <lane>] [--backend <name>] [--provider <name>] [--task-id <id>] [--pid <pid>] [--monitor-kind <kind>] [--automation-id <id>] [--external-run-id <id>] [--poll-command <cmd>] [--manual-until <date-or-rfc3339>]
fairway session status [--all]
fairway session end <session-id> [--status <ended|failed|stale>] [--reason <text>] [--exit-code <n>]
fairway session reconcile [--dry-run]
fairway reconcile active [--dry-run]                    # report stale/unattended active work across sessions, tasks, evidence, and checkpoints
fairway session launch --role <role> [--backend <shell|tmux|zellij>] [--provider <name>] [--task-id <id>] [--prompt-file <path>|--prompt <text>] [--transcript <path>] [--command <provider-command>] [--dry-run] # adapter; optional
fairway worktree setup | status | prune [--force]
fairway task-detail <task-id>                          # includes missing required review domains before merge-ready
fairway status-report | health-report | timing-report
fairway coordinator plan [--ready-limit <n>] [--recommendation-limit <n>] [--allow-utility-monitor] # deterministic dry-run next-action plan
fairway coordinator tick                               # prints the same plan in daily tick form
fairway usage report [--by <provider|task|epic|role|day|kind|phase>] [--task-id <id>] [--since-duration <duration>]
fairway batch create <batch-id> --title <t> [--task <id>]... [--branch <b>] [--worktree <path>] [--validation-command <cmd>]... [--review-domain <domain>]... [--rollback-criteria <text>] [--split-criteria <text>] [--expected-ci <text>] [--deploy-run-id <id>] [--pipeline-id <id>]
fairway batch add <batch-id> <task-id>...
fairway batch remove <batch-id> <task-id>...
fairway batch evidence <batch-id> --command-text <cmd> --result <pass|fail|partial|skipped|blocked> [--artifact <path>] [--artifact-type <type>] [--notes <text>] [--map-to-tasks=false]
fairway batch link <batch-id> [--deploy-run-id <id>] [--pipeline-id <id>]
fairway batch show <batch-id>
fairway batch list
fairway dispatch-plan [--role <role>] [--limit <n>]
fairway git-check [--base <ref>]
fairway preflight [--role <role>] [--base <ref>]       # validate current worktree before ready/claim
fairway workflow check [--mode <task|close|deploy>] [--task-id <id>] [--require-clean] [--require-pushed] # guard task/review/deploy workflow boundaries
fairway workflow closeout <task-id> [--dry-run] [--apply] [--preserve-branch-reason <reason>] # report lane branch/worktree/session closeout debt
fairway audit work-coverage [--since-ref <ref> | --since-duration <duration>] [--task-id <id>] [--dry-run] # advisory coverage audit for commits, task metadata, evidence, and reviews
fairway audit ci-learning [--task-id <id>] [--template] # classify failed CI/deploy/smoke/UAT evidence and follow-up coverage
fairway release verify --version <vX.Y.Z> --tag <vX.Y.Z> --ci-status <status> --docs-status <status> --signing-status <status> --notary-status <status> --release-state <public|draft> --asset <url=status> --homebrew-version <vX.Y.Z> --homebrew-tap-commit <sha> --brew-fetch-status <status>
fairway coordinator preflight | status | tick
fairway readiness report [--profile <name>] [--gap-limit <n>]
fairway adoption artifact [--catalog <path>] [--route <path>]... [--limit <n>] [--gap-limit <n>]
fairway parity artifact [--catalog <path>] [--route <path>]... [--limit <n>] [--gap-limit <n>]
fairway packet context <task-id> --goal <text> --owner <role> --acceptance <text>
fairway packet bugfix <task-id> --bug-summary <text> --root-cause <text> [--owning-layer <text>] --proof-command <cmd> --regression-coverage <text> [--residual-risk <text>]
fairway packet watcher <watch-id> --owner <role-or-lane> --process <text> --command <cmd> --success <text> --failure <text>
fairway packet release-run <task-id> --version <vX.Y.Z> --tag <vX.Y.Z> --source-sha <sha> --release-notes <path-or-status> --changelog-state <text> --ci-status <status> --docs-status <status> --signing-status <status> --notary-status <status> --release-url <url> --homebrew-tap-commit <sha> [--verification-command <cmd>]...
fairway packet template <name> <task-id> --field <key=value>...
fairway packet architecture-map <task-id> --scope <text> --current-owner <role> --target-owner <role> --migration-risk <text> [--source-doc <path>]... --acceptance <text>
fairway packet boundary-guard <task-id> --guard-intent <text> [--finding <text>]... [--false-positive <text>]... --graduation-criteria <text> [--proof-command <cmd>]...
fairway packet vertical-slice <task-id> --target-seam <text> --old-path <path> --new-path <path> --adapter <text> [--proof-command <cmd>]... --rollback-plan <text>
fairway regression-pack list [--catalog <path>]
fairway regression-pack show <pack-id> [--catalog <path>]
fairway regression-pack validate [<catalog-path>]
fairway watcher start <watch-id> --task <task-id> [--owner <role-or-lane>] [--process <text>] [--command <cmd>] [--success <text>] [--failure <text>]
fairway watcher finish <watch-id> --result <pass|fail|blocked> [--artifact <path-or-url>] [--duration-seconds <n>] [--notes <text>]
fairway watcher status [--include-done]
fairway checkpoint record <task-id> --summary <text> [--state <state>] [--owner <role-or-lane>] [--target-close-by <date>] [--artifact <path>]
fairway checkpoint status [--all]
fairway checkpoint stale [--before <date>] [--all]
fairway prune-stale                                     # remove state rows for deleted task definitions
fairway db backup | export
fairway db migrate [--dry-run]
fairway db compat --backend postgres [--print-ddl | --apply-ddl]
fairway import <yaml-or-json-path> [--state-once]        # accepts a task list or {tasks: [...]} envelope; state-once seeds legacy status once
fairway config validate
fairway dashboard [--no-open] [--listen <addr>] [--multi] # foreground server
fairway dashboard start [--listen <addr>] [--multi] [--open] [--pid-file <path>] [--log-file <path>]
fairway dashboard stop [--pid-file <path>] [--log-file <path>]
fairway dashboard restart [--listen <addr>] [--multi] [--open] [--pid-file <path>] [--log-file <path>]
fairway dashboard status [--listen <addr>] [--multi] [--pid-file <path>] [--log-file <path>]
fairway tui [--once]                                    # interactive ready/claim/status/detail/status-update/evidence/readiness loop
fairway tracker providers
fairway tracker configure <plane|jira|linear> [--url <url>] [--workspace <slug>] [--project <id-or-slug>] [--team <key>] [--dry-run]
fairway tracker import <plane|jira|linear> [--query <filter>] [--parent <task-id>] [--dry-run]
fairway tracker link <task-id> --provider <plane|jira|linear> --external-id <id> [--url <url>]
fairway tracker links
fairway tracker export-status <task-id> [--provider <plane|jira|linear>] [--external-id <id>] [--dry-run]
fairway tracker resolve --provider <plane|jira|linear> [--external-id <id>] [--url <url>]
fairway tracker reconcile [--dry-run]
fairway tracker plane export [--task-id <task-id>] [--limit <n>]
fairway tracker plane import --fixture examples/tracker-adapters/plane/evaluation-workspace.yaml
fairway tracker plane comment --task-id <task-id> [--external-id <plane-issue-id>]
fairway register [--name <n>]                           # add current project to ~/.fairway/registry.toml
fairway unregister [<name>]                             # remove from registry
fairway projects                                        # list registered projects
fairway version
```

## Granularity guardrail

`fairway spawn --child` prints a warning when invoked from a leaf-kind task (`kind` in `{task, bug, spike}`):

```
Warning: T-042 is a leaf task (kind=task). Children of leaf tasks usually
indicate execution sub-steps, which belong in your own notes rather than
fairway. If T-099 is genuinely new work the orchestrator should see,
prefer --sibling. To suppress this warning, pass --child --force.
```

The warning is informational, not blocking. See [hierarchy.md](hierarchy.md) for the granularity principle.

## Conventions

- Task IDs are positional; never flagged.
- Default task IDs match `^[A-Z]+-[0-9]+$`; v1 does not auto-generate IDs.
- `--payload @path/to/file` reads the file; otherwise the value is treated as inline text.
- All commands respect `--db <path>` and `--config <path>` overrides.
- Commands exit non-zero on validation failure. Pass `--json` for machine-readable error output.
- Grouped commands accept `help`, `-h`, and `--help` after the group name, for
  example `fairway session --help` or `fairway dashboard help`.
- The caller's role is determined by (in order): `--as <role>` flag, `FAIRWAY_ROLE` env var, current worktree's configured role, prompt if ambiguous.
- Evidence records are command-oriented. `artifact` is optional so "no-op" checks,
  skipped checks, and blocked checks can still leave an auditable row.
- `--state-once` is for legacy migration only. Subsequent imports update task
  definitions but never overwrite DB-owned execution state.
- Task metadata flags (`--profile`, `--owning-domain`, `--source-paths`, etc.)
  are stored on task definitions and also supported by YAML/JSON imports.
  `--acceptance`, `--source-paths`, `--target-paths`, and `--review-domains`
  are repeatable. List flags may also contain comma-separated values; repeated
  list flags are flattened in the order supplied and rendered as task metadata
  in `task-detail`.
- `session launch` is an adapter command. The queue/session model works even
  when agents are launched manually. The shell-backed path can feed a prompt
  file into a provider command, record a transcript path, and upsert a session
  without claiming the task. Prompt and transcript paths are relative to the
  launch worktree unless absolute. Use `--dry-run` to inspect the command,
  prompt file, transcript, and session metadata without mutation.
- `session reconcile` handles session-local cleanup such as missing PID/tmux
  panes and stale sessions attached to terminal tasks. `reconcile active` is
  broader: it reports unattended `in_progress` tasks, evidence without a status
  decision, stale checkpoints, parent tasks left active without direct rollup
  work, and monitor sessions without backing automation/process/external-run
  proof or a fresh bounded manual checkpoint. It also reports
  `monitor_completion_resume_needed` when monitors are complete, no active
  sessions/watchers remain, and ready work is waiting for the coordinator loop.
  Provider sessions attached to tasks must also have a matching lifecycle
  checkpoint: `active` for started/running, `awaiting_input` for waiting,
  failed, stale, or no-progress, and `done` for completed.
- `workflow check` composes git and active-work checks into the operating-model
  guard. It warns on dirty docs/code, unpushed commits, missing upstreams, and
  active reconciliation findings. Use `--mode deploy` before deploy/UAT work;
  it requires a clean committed SHA and pushed commits so CI can run. Use
  `--mode close --task-id <id>` before moving a lane to new implementation
  work; it includes the lane closeout report for the task. Use `--require-clean`
  or `--require-pushed` when the current boundary should fail instead of warn.
- `workflow closeout <task-id>` is advisory by default. It reports task status,
  commit association, CI/deploy/UAT evidence, review-domain completeness,
  active sessions/watchers, branch merge state, remote branch presence,
  worktree cleanliness, and explicit branch preservation reasons. `--apply`
  deletes only a verified merged `origin/<branch>` remote branch when the
  closeout report has no blockers. It does not delete local branches or
  worktrees.
- `set-status` records `commit_sha` for terminal CLI transitions. Pass
  `--commit <sha>` to pin an explicit task commit; otherwise Fairway records
  the current `HEAD` when marking a task terminal.
- `tracker` commands are provider-neutral adapter contract surfaces. `configure`,
  `import`, `export-status`, `resolve`, and `reconcile` are dry-run/advisory
  until a provider adapter explicitly adds an apply path. Tracker planning
  mirrors must not mutate Fairway execution state, sessions, evidence, reviews,
  or merge gates.
- `tracker plane` is the first provider-specific adapter spike. It renders
  Plane issue payloads, fixture import previews, and execution-summary comments
  from local Fairway state. `--apply` is intentionally rejected in the spike;
  Plane credentials come from `PLANE_BASE_URL`, `PLANE_WORKSPACE`,
  `PLANE_PROJECT`, and eventually `PLANE_API_TOKEN`.
- `audit work-coverage` is advisory by default. It compares commits since a
  base ref or duration window against Fairway task IDs, task `source_paths` /
  `target_paths`, evidence rows, and required review domains. Use it before
  review, deploy, and release boundaries to catch real work that happened
  outside task/evidence/review coverage.
- `audit ci-learning` is advisory by default. It classifies failed CI, deploy,
  smoke, and UAT evidence as missed local gate, missed review gate,
  CI-environment-only, flaky runner/cache, or approval-gated blocker, then
  checks for matching `CI-FIX-*`, `CD-FIX-*`, `OPS-FIX-*`, `HARNESS-FIX-*`,
  `UAT-BUG-*`, or `DOC-FIX-*` follow-up tasks. Use `--template` to render a
  learning artifact for review or release notes.
- `batch` commands model shared implementation and validation units. A batch
  can contain multiple granular tasks that share one branch/worktree,
  validation command set, review domains, CI/deploy-run, and evidence set.
  `batch evidence` maps shared evidence back to each member task by default so
  tasks still close independently while pointing to the same proof. Use
  `--map-to-tasks=false` only for batch-level notes that do not satisfy any
  task acceptance check.
- `release verify` is an advisory release-readiness guard with a non-zero exit
  on release issues. It consumes observed evidence from commands such as
  `gh release view`, `curl`, `brew info`, and `brew fetch`; it does not call
  provider APIs itself. It flags draft GitHub releases with matching Homebrew
  casks, missing release notes/changelog entries, failed asset URL checks,
  missing release status, and failed Homebrew fetch verification.
- `coordinator tick` composes reports and recommendations. It does not claim,
  merge, or mutate tasks automatically.
- Utility adapters such as `examples/session-adapters/utility-event.sh` and
  `examples/session-adapters/ci-monitor.sh` are shell-level conventions over
  existing Fairway commands, not new core subcommands. They should record
  provider-neutral utility sessions, checkpoints, evidence, watcher lifecycle,
  and handback output without making task status, review, or merge decisions.
- `adoption artifact` is the generic readiness report. It uses configured
  workstream profile `route_samples` when no `--route` flags are provided and
  reports named profile gates plus evidence-backed gate evaluation. `parity
  artifact` remains a compatibility alias for GPUaaS-style comparisons.
- `merge-ready` evaluates profile gates for the target task. Missing
  `blocking` gates fail readiness; missing `advisory` and `report_only` gates
  are reported as warnings. If task metadata declares `review_domains`,
  `merge-ready` also requires an approved review whose reviewer matches each
  domain.
- `readiness report` evaluates configured profile gates across a workstream or
  all profiles. Missing blocking gates make the report fail in human mode;
  `--json` returns the full report for automation.
- `dashboard` without a subcommand runs in the foreground. Use `dashboard start`,
  `dashboard stop`, `dashboard restart`, and `dashboard status` for a detached
  local dashboard. Detached lifecycle commands do not open a browser unless
  `--open` is passed. They write `.fairway/dashboard.pid` and
  `.fairway/dashboard.log` by default; multi-project mode uses
  `.fairway/dashboard-multi.*`.
- `packet bugfix`, platform-foundation packets, `packet template`, and
  `regression-pack` are quality surfaces. They render and validate review
  context; they do not execute product test suites.
- `db compat --backend postgres` is a planned adapter harness, not the default
  v1 runtime.
- See [release-cuts.md](release-cuts.md) for the subset of this surface that
  ships in each release.

## Output

- Human format by default: tables, color when stdout is a TTY.
- `--json` for everything that lists or returns structured data.
- Exit codes: `0` success, `1` validation / config error, `2` runtime error (DB, git), `3` not found, `4` invariant violation.
