# CLI verb surface

```
fairway init                                            # scaffold .fairway/config.toml + DB
fairway ready [--in <epic-id>] [--priority <n>]         # list tasks ready for the caller's role (sorted by priority, sequence, created_at)
fairway claim <task-id>                                 # transition todo → in_progress, assign owner
fairway claim --in <epic-id>                            # claim next ready descendant within an epic
fairway add <task-id> --title <t> [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--role <r>] [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>] [--target-paths <csv>] [--review-domains <csv>] [--risk-level <r>] [--migration-type <t>]
fairway spawn --id <task-id> --title <t> [--kind <k>] [--child | --sibling | --parent <id> | --root] [--from-task <id>] [--priority <n>] [--force] [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>] [--target-paths <csv>] [--review-domains <csv>] [--risk-level <r>] [--migration-type <t>]
fairway update <task-id> [--title <t>] [--notes <text>] [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--dependencies <a,b,c>] [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>] [--target-paths <csv>] [--review-domains <csv>] [--risk-level <r>] [--migration-type <t>]
fairway tree <task-id> [--depth <n>]                    # print descendant tree
fairway set-status <task-id> <state> [--reason <text>] [--reopen]
fairway record evidence <task-id> --command-text <text> --result <pass|fail|partial|skipped|blocked> [--artifact <path>] [--artifact-type <type>] [--duration-seconds <n>] [--notes <text>]
fairway record guard-report <task-id> --guard <name> [--mode <report_only|warning|blocking>] [--finding <text>]... [--false-positive <text>]... [--allowed-debt <text>]... [--graduation-criteria <text>] [--artifact <path>] [--result <result>]
fairway record handoff <task-id> --to <role> --payload <text-or-@file>
fairway record review <task-id> --reviewer <role-or-user> --verdict <approve|changes|reject> [--reason <text>] [--commit <sha>]
fairway route review <task-id> [--reviewer <role>] [--path <path>]... [--reason <text>] # mark pending review
fairway merge-ready <task-id> [--base <ref>]          # verify evidence/review/handoff/git/profile gates
fairway review checkout <task-id> [--source-role <role>] # create/reset named review branch
fairway session upsert --role <role> [--id <id>] [--lane <lane>] [--backend <name>] [--provider <name>] [--task-id <id>] [--pid <pid>]
fairway session status [--all]
fairway session end <session-id> [--status <ended|failed|stale>] [--reason <text>] [--exit-code <n>]
fairway session reconcile [--dry-run]
fairway session launch --role <role> [--backend <shell|tmux|zellij>] [--provider <name>] [--task-id <id>] # adapter; optional
fairway worktree setup | status | prune [--force]
fairway task-detail <task-id>
fairway status-report | health-report | timing-report
fairway dispatch-plan [--role <role>] [--limit <n>]
fairway git-check [--base <ref>]
fairway preflight [--role <role>] [--base <ref>]       # validate current worktree before ready/claim
fairway coordinator preflight | status | tick
fairway readiness report [--profile <name>] [--gap-limit <n>]
fairway adoption artifact [--catalog <path>] [--route <path>]... [--limit <n>] [--gap-limit <n>]
fairway parity artifact [--catalog <path>] [--route <path>]... [--limit <n>] [--gap-limit <n>]
fairway packet context <task-id> --goal <text> --owner <role> --acceptance <text>
fairway packet bugfix <task-id> --bug-summary <text> --root-cause <text> [--owning-layer <text>] --proof-command <cmd> --regression-coverage <text> [--residual-risk <text>]
fairway packet watcher <watch-id> --owner <role-or-lane> --process <text> --command <cmd> --success <text> --failure <text>
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
fairway tracker link <task-id> --provider <jira|linear> --external-id <id> [--url <url>]
fairway tracker links
fairway tracker reconcile [--dry-run]
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
- The caller's role is determined by (in order): `--as <role>` flag, `FAIRWAY_ROLE` env var, current worktree's configured role, prompt if ambiguous.
- Evidence records are command-oriented. `artifact` is optional so "no-op" checks,
  skipped checks, and blocked checks can still leave an auditable row.
- `--state-once` is for legacy migration only. Subsequent imports update task
  definitions but never overwrite DB-owned execution state.
- Task metadata flags (`--profile`, `--owning-domain`, `--source-paths`, etc.)
  are stored on task definitions and also supported by YAML/JSON imports.
- `session launch` is an adapter command. The queue/session model works even
  when agents are launched manually.
- `coordinator tick` composes reports and recommendations. It does not claim,
  merge, or mutate tasks automatically.
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
