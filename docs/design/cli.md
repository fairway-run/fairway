# CLI verb surface

```
fairway init                                            # scaffold .fairway/config.toml + DB
fairway ready [--in <epic-id>] [--priority <n>]         # list tasks ready for the caller's role (sorted by priority, sequence, created_at)
fairway claim <task-id>                                 # transition todo → in_progress, assign owner
fairway claim --in <epic-id>                            # claim next ready descendant within an epic
fairway add <task-id> --title <t> [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--role <r>]
fairway spawn --title <t> [--kind <k>] [--child | --sibling | --parent <id>] [--priority <n>] [--force]
fairway update <task-id> [--title <t>] [--notes <text>] [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--dependencies <a,b,c>]
fairway tree <task-id> [--depth <n>]                    # print descendant tree
fairway set-status <task-id> <state> [--reason <text>] [--reopen]
fairway record evidence <task-id> --command-text <text> --result <pass|fail|partial|skipped|blocked> [--artifact <path>] [--artifact-type <type>] [--duration-seconds <n>] [--notes <text>]
fairway record handoff <task-id> --to <role> --payload <text-or-@file>
fairway record review <task-id> --reviewer <role-or-user> --verdict <approve|changes|reject> [--reason <text>] [--commit <sha>]
fairway route review <task-id>                          # apply review routing rules from config
fairway merge-ready <task-id> [--ref <ref>] [--base <ref>] # verify evidence/review/handoff/git gates
fairway session upsert | end | status | reconcile
fairway worktree setup | status | prune
fairway task-detail <task-id>
fairway status-report | health-report | timing-report | dispatch-plan
fairway git-check [--base <ref>]
fairway db backup | export
fairway import <yaml-or-json-path> [--state-once]        # bootstrap-only; never overwrites mutable state after initial import
fairway config validate
fairway dashboard [--no-open] [--listen <addr>] [--multi]
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
- `--payload @path/to/file` reads the file; otherwise the value is treated as inline text.
- All commands respect `--db <path>` and `--config <path>` overrides.
- Commands exit non-zero on validation failure. Pass `--json` for machine-readable error output.
- The caller's role is determined by (in order): `--as <role>` flag, `FAIRWAY_ROLE` env var, current worktree's configured role, prompt if ambiguous.
- Evidence records are command-oriented. `artifact` is optional so "no-op" checks,
  skipped checks, and blocked checks can still leave an auditable row.
- `--state-once` is for legacy migration only. Subsequent imports update task
  definitions but never overwrite DB-owned execution state.

## Output

- Human format by default: tables, color when stdout is a TTY.
- `--json` for everything that lists or returns structured data.
- Exit codes: `0` success, `1` validation / config error, `2` runtime error (DB, git), `3` not found, `4` invariant violation.
