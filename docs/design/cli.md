# CLI verb surface

## Common work path

`fairway work` is progressive disclosure over the existing task, session,
checkpoint, evidence, and review records. It is not a second workflow or store.

`work start` requires an explicit task ID. In one SQLite transaction it moves a
`todo` task to `in_progress`, attaches or refreshes a stable provider session,
and records an active checkpoint. Repeating the command for the same active
owner/session is safe. Blocked and terminal tasks fail closed; they require the
existing explicit status/reopen commands and their reasons or gates.

`work status` gives the compact current task state and durable fact counts.
Without a task ID it uses only unambiguous existing task/session environment
inference. `--explain` names the underlying records and detailed inspection
command. Neither command records evidence, synthesizes reviews, or authorizes
merge, deploy, release, credentials, public exposure, or live operations.

Material implementation choices use the task decision model documented in
[`task-decision-memory.md`](task-decision-memory.md). Until the first-class
command lands, record the concise decision through bounded task evidence or a
checkpoint and reference it from track memory when it has cross-task value. Do
not store raw prompts, chain-of-thought, transcripts, or tool bodies as a
substitute for a decision.

```
fairway init [--refresh-agent-contract]                # scaffold .fairway/config.toml + DB + .fairway/AGENTS.md
fairway agent-guide [--path | --output <path>]          # print or write the embedded offline agent guide
fairway ready [--in <epic-id>] [--priority <n>]         # list tasks ready for the caller's role (sorted by priority, sequence, created_at)
fairway claim <task-id>                                 # transition todo → in_progress, assign owner
fairway claim --in <epic-id>                            # claim next ready descendant within an epic
fairway add <task-id> --title <t> [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--role <r>] [--acceptance <text>]... [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>]... [--target-paths <csv>]... [--review-domains <csv>]... [--risk-level <r>] [--migration-type <t>]
fairway spawn --id <task-id> --title <t> [--kind <k>] [--child | --sibling | --parent <id> | --root] [--from-task <id>] [--priority <n>] [--force] [--acceptance <text>]... [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>]... [--target-paths <csv>]... [--review-domains <csv>]... [--risk-level <r>] [--migration-type <t>]
fairway update <task-id> [--title <t>] [--notes <text>] [--kind <k>] [--parent <id>] [--priority <n>] [--sequence <n>] [--dependencies <a,b,c>] [--acceptance <text>]... [--profile <p>] [--owning-domain <d>] [--owning-layer <l>] [--source-paths <csv>]... [--target-paths <csv>]... [--review-domains <csv>]... [--risk-level <r>] [--migration-type <t>]
fairway tree <task-id> [--depth <n>]                    # print descendant tree
fairway list [--status <state[,state]>]... [--role <role>] [--ready] # list tasks by status with dependency readiness summary
fairway set-status <task-id> <state> [--reason <text>] [--commit <sha>] [--reopen]
fairway record evidence <task-id> --command-text <text> --result <pass|fail|partial|skipped|blocked> [--artifact <path>] [--artifact-type <type>] [--duration-seconds <n>] [--notes <text>]
fairway record guard-report <task-id> --guard <name> [--mode <report_only|warning|blocking>] [--finding <text>]... [--false-positive <text>]... [--allowed-debt <text>]... [--graduation-criteria <text>] [--artifact <path>] [--result <result>]
fairway record handoff <task-id> --to <role> --payload <text-or-@file>
fairway record completion-handback <task-id> --to <role> --next-action <text> [--completion-state <state>] [--evidence <path>]... [--approval-boundary <text>] [--provider <name>] [--target <thread-or-adapter>] [--state <handoff_recorded|notification_delivered|thread_steered|notification_failed>] [--reason <text>]
fairway record completion-handback-supersede <task-id> --handoff-id <id> --reason <text> [--replacement-handoff-id <id>] [--evidence <path>]
fairway record review <task-id> --reviewer <role-or-user> [--domain <review-domain>] --verdict <approve|changes|reject> [--reason <text>] [--commit <sha>]
fairway record usage <task-id> --provider <name> [--session-id <id>] [--external-session-id <id>] [--role <role>] [--phase <phase>] [--source <provider_reported|derived_snapshot|manual|unknown>] [--confidence <exact|estimated|unknown>] [--input-tokens <n>] [--cached-input-tokens <n>] [--output-tokens <n>] [--total-tokens <n>]
fairway route review <task-id> [--reviewer <role>] [--path <path>]... [--reason <text>] # mark pending review
fairway merge-ready <task-id> [--base <ref>]          # verify evidence/review/handoff/git/profile gates
fairway review checkout <task-id> [--source-role <role>] # create/reset named review branch
fairway review-waits list [--blocking] [--task <task-id>] [--stale] # read-only review wait projection
fairway review-waits wake [--task <task-id>] [--send]              # fixed-template wake prompts for parked provider threads
fairway review-policy report [--profile <name>]                   # review profile overhead/outcome report
fairway work start <task-id> [--session-id <id>] [--role <role>] [--provider <name>] [--backend <name>] [--external-run-id <id>] [--summary <text>]
fairway work status [<task-id>] [--explain]
fairway session upsert --role <role> [--id <id>] [--lane <lane>] [--backend <name>] [--provider <name>] [--task-id <id>] [--pid <pid>] [--monitor-kind <kind>] [--automation-id <id>] [--external-run-id <id>] [--poll-command <cmd>] [--manual-until <date-or-rfc3339>]
fairway session status [--all]
fairway session end <session-id> [--status <ended|failed|stale>] [--reason <text>] [--exit-code <n>]
fairway session reconcile [--dry-run]
fairway lane start --role <role> [--session-id <id>] [--task-id <id>] [--backend <local|shell|tmux>] [--pid <pid>] [--tmux-pane <pane>] [--transcript <path>]
fairway lane status [--session-id <id>] [--all]
fairway lane logs --session-id <id> [--tail <n>]
fairway lane stop --session-id <id> [--status <ended|failed|stale>] [--reason <text>] [--exit-code <n>]
fairway reconcile active [--dry-run]                    # report stale/unattended active work across sessions, tasks, evidence, and checkpoints
fairway session launch --role <role> [--backend <shell|tmux|zellij>] [--provider <name>] [--task-id <id>] [--prompt-file <path>|--prompt <text>] [--transcript <path>] [--command <provider-command>] [--dry-run] # adapter; optional
fairway worktree setup | status | prune [--force]
fairway task-detail <task-id>                          # includes missing required review domains before merge-ready
fairway status-report | health-report | timing-report
fairway completion-handback-report [--include-closed] [--format human|markdown]
fairway coordinator plan [--ready-limit <n>] [--recommendation-limit <n>] [--allow-utility-monitor] # deterministic dry-run next-action plan
fairway coordinator tick [--completion-handback-wake] [--task <task-id>] [--send] # daily plan plus optional stale completion-handback wake prompts
fairway usage report [--by <provider|task|epic|role|day|kind|phase|model>] [--task-id <id>] [--since-duration <duration>]
fairway usage cost-report [--by <provider|task|epic|role|day|kind|phase|model>] [--task-id <id>] [--since-duration <duration>] [--forecast-days <n>] [--format human|markdown]
fairway delivery report --since <duration> [--profile <name>] [--format text|json] # read-only delivery velocity and process overhead report
fairway delivery resources [--type <type>] [--project <project>] [--stale] [--format text|json] # read-only typed delivery resource projection
fairway automation candidates --since <duration> [--threshold <n>] [--format text|json] # read-only repeated-work automation candidate report
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
fairway doctor [--dashboard-read-only <addr>] [--dashboard-full <addr>] [--format text|json] # read-only local capability diagnostics
fairway workflow check [--mode <task|close|deploy>] [--task-id <id>] [--require-clean] [--require-pushed] # guard task/review/deploy workflow boundaries
fairway workflow closeout <task-id> [--dry-run] [--apply] [--preserve-branch-reason <reason>] # report lane branch/worktree/session closeout debt
fairway audit work-coverage [--since-ref <ref> | --since-duration <duration>] [--task-id <id>] [--dry-run] # advisory coverage audit for commits, task metadata, evidence, and reviews
fairway audit ci-learning [--task-id <id>] [--template] # classify failed CI/deploy/smoke/UAT evidence and follow-up coverage
fairway audit notifications [--task <task-id>] [--all] # read-only provider notification lifecycle audit across waits and handbacks
fairway audit docs-backlog [--doc <path>]... # advisory coordination docs-to-backlog coverage audit
fairway advisory adapters [--include-disabled] # read-only configured advisory provider adapter list
fairway advisory validate <task-id> --action <action> --target-role <role> --confidence <0..1> --rationale <text> --cited-fact <fact>... [--provider <adapter>] [--requires-human] [--risk-flag <flag>]... [--record-evidence]
fairway notify notifiers [--include-disabled] # read-only configured external notifier list
fairway notify dry-run --notifier <name> --task <task-id> --domain <domain> [--template <name>] [--target <target>] [--record-intent]
fairway notify send --notifier <name> --task <task-id> --domain <domain> [--template <name>] [--target <label>]
fairway release verify --version <vX.Y.Z> --tag <vX.Y.Z> --ci-status <status> --docs-status <status> --signing-status <status> --notary-status <status> --release-state <public|draft> --asset <url=status> --homebrew-version <vX.Y.Z> --homebrew-tap-commit <sha> --brew-fetch-status <status>
fairway coordinator preflight | status | tick
fairway readiness report [--profile <name>] [--gap-limit <n>]
fairway adoption artifact [--catalog <path>] [--route <path>]... [--limit <n>] [--gap-limit <n>]
fairway parity artifact [--catalog <path>] [--route <path>]... [--limit <n>] [--gap-limit <n>]
fairway packet context <task-id> --goal <text> --owner <role> --acceptance <text>
fairway packet bugfix <task-id> --bug-summary <text> --root-cause <text> [--owning-layer <text>] --proof-command <cmd> --regression-coverage <text> [--residual-risk <text>]
fairway packet retry <task-id> --kind <preflight|live-operation> --source-sha <sha> --operator-surface <surface> --artifact-dir <path> --evidence-contract <text>... --allowed-action <text>... --forbidden-action <text>... --expires-at <time-or-window> --prior-failure-closure <text> [--next-action <text>]
fairway packet watcher <watch-id> --owner <role-or-lane> --process <text> --command <cmd> --success <text> --failure <text>
fairway packet release-run <task-id> --version <vX.Y.Z> --tag <vX.Y.Z> --source-sha <sha> --release-notes <path-or-status> --changelog-state <text> --ci-status <status> --docs-status <status> --signing-status <status> --notary-status <status> --release-url <url> --homebrew-tap-commit <sha> [--verification-command <cmd>]...
fairway packet template <name> <task-id> --field <key=value>... [--instantiate-waits] [--child-task <id=field>]...
fairway packet rules <task-id>
fairway packet architecture-map <task-id> --scope <text> --current-owner <role> --target-owner <role> --migration-risk <text> [--source-doc <path>]... --acceptance <text>
fairway packet boundary-guard <task-id> --guard-intent <text> [--finding <text>]... [--false-positive <text>]... --graduation-criteria <text> [--proof-command <cmd>]...
fairway packet vertical-slice <task-id> --target-seam <text> --old-path <path> --new-path <path> --adapter <text> [--proof-command <cmd>]... --rollback-plan <text>
fairway regression-pack list [--catalog <path>]
fairway regression-pack show <pack-id> [--catalog <path>]
fairway regression-pack validate [<catalog-path>]
fairway recipe extract --task <task-id> --name <name> [--output <path>] [--input <text>]... [--forbidden-action <text>]... [--closeout-rule <text>]...
fairway recipe render --recipe <path> --task <task-id> [--field <key=value>]... [--format markdown|json]
fairway recipe list [--dir <path>] [--format text|json]
fairway contract agent-output [--schema <schema-or-name>] [--format text|json]
fairway watcher start <watch-id> --task <task-id> [--owner <role-or-lane>] [--process <text>] [--command <cmd>] [--success <text>] [--failure <text>]
fairway watcher finish <watch-id> --result <pass|fail|blocked> [--artifact <path-or-url>] [--duration-seconds <n>] [--notes <text>]
fairway watcher status [--include-done]
fairway rules validate <rule-pack-dir>                 # validate local rule-pack metadata and report groups/findings
fairway rules evidence-types                           # list evidence types from loaded packs, profile gates, and recorded evidence
fairway rules match <task-id>                          # show selected, disabled, and non-applicable rules for a task
fairway checkpoint record <task-id> --summary <text> [--state <state>] [--owner <role-or-lane>] [--target-close-by <date>] [--artifact <path>]
fairway checkpoint status [--all]
fairway checkpoint stale [--before <date>] [--all]
fairway memory show [--track <track-id>]
fairway memory update --track <track-id> [--title <text>] [--purpose <text>] [--operating-mode <text>] [--active-scope <text>] [--current-objective <text>] [--decision <text>]... [--blocker <text>]... [--open-question <text>]... [--next-action <text>]... [--source-checkpoint-id <id>]... [--source-evidence-id <id>]... [--source-review-id <id>]...
fairway memory append --track <track-id> [fields]
fairway memory packet --track <track-id> [--for <provider-or-surface>]
fairway memory stale [--older-than <duration>]
fairway wait add --task <task-id> --track <track-id> --on <condition> [--kind <kind>] [--target <target>] [--deadline <time>] [--deadline-source <origin>] [--action <action>] [--reason <text>] [--suggested-command <cmd>]
fairway wait ack <wait-id> [--reason <text>] [--actor <role-or-track>]
fairway wait list [--task <task-id>] [--stale] [--kind <kind>]
fairway wait tick [--task <task-id>] [--stale] [--kind <kind>]
fairway wait wake [--task <task-id>] [--kind <kind>] [--send]
fairway live-window record <task-id> --phase <phase> [--next-owner <role>] [--next-action <action>] [--authorization-state <state>] [--prompt <text>] [--command <cmd>] [--missed-deadline-action <action>] [--target-close-by <date>] [--artifact <path>]
fairway live-window status [--task <task-id>]
fairway live-window control-room [--task <task-id>] [--stale]
fairway live-window retry-budget record <task-id> --meaningful-failures <n> --coordination-failures <n> --budget <n> [--reset-task <task-id>] [--reset-reason <text>]
fairway live-window retry-budget status [--task <task-id>]
fairway prune-stale                                     # remove state rows for deleted task definitions
fairway db backup | export
fairway db migrate [--dry-run]
fairway db compat --backend postgres [--print-ddl | --apply-ddl]
fairway db rehearsal --backend postgres [--out <dir>] [--apply-dsn-env <env>] [--postgres-schema <schema>]
fairway import <yaml-or-json-path> [--state-once]        # accepts a task list or {tasks: [...]} envelope; state-once seeds legacy status once
fairway config validate
fairway dashboard [--no-open] [--listen <addr>] [--multi] # foreground server
fairway dashboard start [--listen <addr>] [--multi] [--open] [--pid-file <path>] [--log-file <path>]
fairway dashboard stop [--pid-file <path>] [--log-file <path>]
fairway dashboard restart [--listen <addr>] [--multi] [--open] [--pid-file <path>] [--log-file <path>]
fairway dashboard status [--listen <addr>] [--multi] [--pid-file <path>] [--log-file <path>]
fairway server --read-only [--listen <addr>]              # shared-team read-only API skeleton
fairway server --mode api-write-pilot --write             # guarded shared-team write API pilot
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

`fairway help`, `fairway -h`, and `fairway --help` print a grouped command
summary by workflow area. `fairway <command> --help` exits successfully with
concise command usage for top-level commands, including file-taking commands
such as `fairway import --help`; longer examples live in `fairway agent-guide`
and the docs.

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
- Do not infer noun-style subcommands that are not in the command summary.
  Fairway currently keeps core task operations as top-level verbs such as
  `add`, `spawn`, `update`, `tree`, `ready`, and `task-detail`; there is no
  grouped `task` command. Dependency metadata is set with
  `fairway update <task-id> --dependencies <a,b,c>` and inspected through
  `fairway task-detail <task-id>`, `fairway tree`, or readiness output. Watcher
  rows are inspected with `fairway watcher status [--include-done]`; there is
  no `fairway watcher list` command.
- `fairway ready` prints claimable ready tasks. If no ready tasks are claimable
  but todo tasks remain in the filtered scope, it prints a blocker summary by
  dependency-blocked, review-gated, approval-gated, profile-gated,
  session-gated, or unknown category with a suggested inspection command.
  `fairway --json ready` returns a report object with `tasks`,
  `claimable_count`, `non_ready_todo_count`, and `blocker_categories`.
- The caller's role is determined by (in order): `--as <role>` flag, `FAIRWAY_ROLE` env var, current worktree's configured role, prompt if ambiguous.
- Evidence records are command-oriented. `artifact` is optional so "no-op" checks,
  skipped checks, and blocked checks can still leave an auditable row.
- `--state-once` is for legacy migration only. Subsequent imports update task
  definitions but never overwrite DB-owned execution state.
- Task metadata flags (`--profile`, `--owning-domain`, `--source-paths`,
  `--tag`, etc.)
  are stored on task definitions and also supported by YAML/JSON imports.
  `--acceptance`, `--source-paths`, `--target-paths`, `--review-domains`, and
  `--tag` are repeatable. List flags may also contain comma-separated values;
  repeated list flags are flattened in the order supplied and rendered as task
  metadata in `task-detail`. Tags are generic grouping metadata and may be
  simple strings such as `production-readiness` or key:value strings such as
  `environment:cloudflare`.
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
- `lane start|status|logs|stop` is a bounded lifecycle surface for local or
  tmux provider/helper lanes. `lane start` upserts an existing session record
  and records an `active` checkpoint when `--task-id` is supplied; it does not
  launch a provider, send a prompt, claim review authority, or mutate project
  state beyond session/checkpoint facts. `lane status` projects process/pane
  readback such as `running`, `missing_process`, `missing_tmux_pane`,
  `unsupported_remote`, or terminal session state. `lane logs` reads a local
  transcript path from the session, requires the file to stay under the session
  worktree after symlink resolution, redacts common credential markers before
  display, and does not store log content. `lane stop` records session closeout
  and a `done` checkpoint; it does not kill remote runtimes. Unknown remote
  backends fail closed until a later reviewed runtime adapter defines their
  trust boundary.
- `reconcile active` permits bounded active evidence capture for approved live
  operations only while the task has a running session and a fresh `active`
  checkpoint with `--target-close-by` still open. This allows gate/runtime
  evidence during an operation window without treating it as abandoned work. It
  does not close the task: once the window expires, or if the session,
  checkpoint, or closeout marker is missing, evidence recorded after activation
  again reports `status_decision_required` until the operator sets the task to
  `done`, `blocked`, `todo`, or another configured closeout state.
- `live-window record` is a typed checkpoint helper for repeated exact-window
  live-operation loops. It records compatibility phases such as
  `packet-prepared`, `reviews-routed`, `approvals-readback`,
  `gate-authorized`, `gate-running`, `closeout`, and `next-decision`, plus
  control-room phases `packet_ready`, `approvals_ready`,
  `execution_authorized`, `operator_running`, `closeout_required`, `done`, and
  `blocked`. Rows can include the next owner, next action, authorization state,
  exact prompt or command, missed-deadline action, deadline, and artifact path.
  The command writes a normal `task_checkpoints` row; there is no second wait
  or phase store.
- `live-window status`, `live-window control-room`, and `coordinator plan`
  project the latest phase so the control thread can see where the loop is
  parked without polling provider chat. `control-room --stale` filters to
  missed-deadline handoffs such as approved-but-not-executed windows. This is
  intentionally a token-burn reduction surface: Fairway holds routine
  scheduling and handoff state so LLM/provider turns can focus on judgment,
  implementation, review, and exception handling. The canonical design model is
  `docs/design/live-operation-control-room.md`.
- `live-window retry-budget record|status` records the current bounded rerun
  state as a normal checkpoint. Meaningful live-operation failures count
  against the budget; coordination-only failures are visible but do not exhaust
  the product retry budget. Once meaningful failures reach the budget, `packet
  retry` fails closed until an existing Fairway causal reset task and reset
  reason are recorded. This does not authorize another live window; it only
  makes the retry/reset decision explicit.
- `review-policy report` summarizes configured and built-in review-profile
  pilots and blocking policies. Built-in defaults distinguish reversible
  evidence-led work from irreversible, live, and release boundaries, and expose
  `prototype-first` for uncertain reversible product/UX work. Prototype-first
  tasks should record `prototype-artifact`, `owner-usage-proof`,
  `prototype-gap-list`, and `stabilization-decision` evidence before
  stabilization or boundary exit. The report compares review overhead with
  Fairway outcome signals such as defects caught, rework-reduction signals,
  blocked tasks, completed tasks, and avoided unsafe actions. Advisory profiles
  with overhead but no useful outcomes should be narrowed or removed rather
  than promoted to blocking defaults. It also reports loop-detected
  causal-reset recommendations when repeated meaningful failures, same-layer
  fixes, or approvals without end-to-end flow progress show that another retry
  needs a failure chain, real unknowns, proof-before-retry, and a lighter
  safe-boundary review plan.
- `record evidence` treats `artifact_type` values `screenshot`, `video`,
  `browser-trace`, and `uat` as UX media evidence. `task-detail` reports a
  `ux media evidence` summary so operators can see whether user-visible work
  was actually exercised. These rows are references to redacted artifacts only:
  do not store raw secrets, auth tokens, provider-private transcripts,
  arbitrary prompt bodies, or unredacted user data in Fairway evidence.
- `memory show|update|append|packet|stale` provides first-class track memory
  for coordinator and provider resume packets. Track memory stores curated
  operating summaries, blockers, decisions, next actions, and numeric Fairway
  source fact references. It does not store raw chat transcripts, prompt bodies,
  generated content, provider credentials, cookies, tokens, or private provider
  database state. `memory packet` renders a compact packet from the memory row
  plus current Fairway tasks, sessions, and checkpoints so new provider
  attachments can resume without polling chat.
- `wait add|ack|list|tick|wake` manages generic parked-work waits while keeping
  Fairway as the state source of truth. `wait add` records a structured
  checkpoint fact for parked work such as repeated handoffs, live-window
  closeout, non-review actor waits, or external control-loop waits. The wait
  checkpoint preserves the configured deadline and `deadline_source` so stale
  output explains the ack-timeout origin. `wait ack` records an acknowledgement
  checkpoint without deleting history. `wait list|tick|wake` projects those
  durable wait checkpoints together with review waits, completion handbacks,
  provider/session and checkpoint plan actions, live-window handoffs, monitor
  actions, approvals, and stale track memory. `wait tick` is a dry-run/operator
  visibility surface.
  `wait wake` renders fixed-template wake prompts for stale or failed
  task-backed waits and, with `--send`, records bounded delivery or
  `notification_failed` evidence through existing task notification rows with a
  stable dedupe signature. Missing provider-target mappings are visible in
  dry-run output as `target_action=mapping_required`; `--send` records
  `notification_failed` with `action=mapping_required` instead of claiming
  delivery. These commands do not run a durable timer, execute DAG steps,
  approve reviews, claim tasks, mutate environments, or perform live actions.
  Dashboard projections remain display-only; wake delivery stays in
  CLI/coordinator/provider-adapter surfaces.
- `record completion-handback` is a typed closeout helper for delegated work.
  It writes a normal handoff plus a linked notification row. The handoff payload
  records the next actor, next safe action, optional completion outcome,
  evidence paths, and approval boundary; the notification state records whether
  provider/thread delivery was accepted, failed, or only recorded in Fairway.
  Supported completion outcomes are `done`, `reviewed`, `merge-ready`,
  `blocked-with-follow-up`, `monitor-completed`, `live-window-closeout`, and
  `live-window-next-decision`. Pending cross-role completion handbacks block
  terminal closeout until delivery or failure proof is recorded and age by
  `[coordinator].notification_ack_timeout` in `coordinator plan`/task detail.
  A `live-window closeout` or `next-decision` checkpoint with no handback is
  projected as a closeout-to-next-owner wait so repeated live windows do not
  depend on polling chat. This does not authorize approval, merge, push, deploy,
  wake delivery, or any dashboard mutation.
- `record completion-handback-supersede` records immutable evidence that an
  older completion handback is obsolete. It keeps the original handoff,
  notification rows, reason, replacement handoff id, evidence path, and
  timestamp visible in history while excluding the old handback from active
  coordinator and notification-audit waits. For non-terminal tasks, the command
  refuses to hide an unresolved handback unless a replacement handback is named
  or the task already has an explicit `blocked` status decision.
- `coordinator tick --completion-handback-wake` renders fixed stale
  completion-handback wake prompts from the same coordinator plan projection.
  With `--send`, it records a bounded `task_notifications` row on the
  `coordinator` domain using a stable wake signature, suppresses duplicate
  successful wake signatures, and reports missing provider-target mappings as
  `target_action=mapping_required`. When sent, missing mappings record
  `notification_failed` with `action=mapping_required` rather than delivery.
  It selects only stale handbacks/closeouts and suppresses terminal tasks; fresh
  waits remain visible in plan/task detail without wake delivery. The dashboard
  remains read-only and never calls this send path.
- `workflow check` composes git and active-work checks into the operating-model
  guard. It warns on dirty docs/code, unpushed commits, missing upstreams, and
  active reconciliation findings. Use `--mode deploy` before deploy/UAT work;
  it requires a clean committed SHA and pushed commits so CI can run. Use
  `--mode close --task-id <id>` before moving a lane to new implementation
  work; it includes the lane closeout report for the task. Use `--require-clean`
  or `--require-pushed` when the current boundary should fail instead of warn.
  Cleanliness output separates true `dirty_paths` from
  `allowed_local_artifacts`, which are configured or evidence-referenced
  untracked operational artifact paths.
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
- `audit ci-learning` and `audit failure-routing` are advisory by default. They
  share the same deterministic read model; `failure-routing` uses
  failure-routing help text and a `failure_routing_ok` human status label for
  operator clarity. They classify failed CI, deploy, smoke, UAT, and known
  coordination evidence as
  missed local gate, missed review gate, CI-environment-only, flaky
  runner/cache, approval-gated blocker, artifact contract, provider API,
  browser surface, setup gate, callback missing, redaction finding, commit
  boundary, or undelivered handoff. The report recommends scoped follow-up task
  prefixes/kinds, owning domain/layer, evidence artifact paths, and forbidden
  actions until review; it does not create tasks unless an explicit
  operator/configured apply path does so. Use `--template` to render a learning
  artifact for review or release notes.
- `audit notifications [--task <id>] [--all]` is a read-only lifecycle report
  over existing review waits, completion handbacks, generic waits, coordinator
  plan rows, handoffs, and `task_notifications`. It reports task, source,
  target domain or role, provider target, handoff id, latest notification id,
  stale age, expected next action, mapping-required target gaps, and a
  fixed-template recovery command. By default it suppresses terminal, resolved,
  delivered, superseded, and other
  non-actionable acknowledgement rows, but it still shows unresolved completion
  handbacks when their latest linked notification is `acknowledged`,
  `review_acknowledged`, or `review_recorded` without delivery proof. `--all`
  includes the suppressed historical rows for audit. The command does not send
  provider messages, approve reviews, close tasks, or create a second wait
  store.
- `audit docs-backlog [--doc <path>]...` is a read-only coordination
  docs-to-backlog coverage report. It scans coordination design docs for
  Fairway task ids, task `source_paths` / `target_paths` coverage, documented
  command examples, and known coordination topics such as review waits,
  completion handbacks, live-operation control, track memory, safe iteration,
  process overhead, and repeated-work automation. Findings are advisory and do
  not change task, review, merge-ready, or release state.
- `delivery report --since <duration> [--profile <name>] [--format text|json]`
  is a read-only delivery velocity and process overhead report. It uses existing
  task transitions, evidence, reviews, handoffs, notifications, and review-wait
  projections to report completed tasks, blocked time, review-wait time,
  first-evidence-to-done time, review approvals versus changes requested,
  notification/wake and handoff counts, approval loops, reopen/retry count,
  outcome-source buckets, defect-source rows, work-batch rollups, and loop
  signals. `outcome_source` is the broader evidence/source classifier;
  `defect_source` is populated only when a review requests changes/rejects or
  non-pass evidence (`fail`, `blocked`, or `partial`) indicates where an issue
  was discovered. Passing tests, preflight, deploy, or UAT proof can contribute
  outcome evidence without being counted as defect discovery. Teams use these
  metrics to identify where extra review, preflight, UAT, tests, packet
  templates, or automation actually reduced defects or blocked time. It is
  advisory by default and does not approve reviews, mutate task status, merge,
  deploy, or gate release.
- `delivery resources [--type <type>] [--project <project>] [--stale]
  [--format text|json]` projects typed delivery resources from existing tasks
  and evidence. Resource classes include environments, dashboards, docs portals,
  binaries, release artifacts, CI pipelines, preflight packets, and rehearsal
  targets. Rows expose owner, state, provenance, last verified commit/version
  when evidence includes them, required evidence, blockers, and the next safe
  action. The command is read-only and does not deploy, restart dashboards,
  publish docs, cut releases, approve reviews, merge, or perform live
  operations. The detailed model is in
  [delivery-resources.md](delivery-resources.md).
- `rough-edge add --task <task-id> --owner <role> --severity <low|medium|high|critical> --decision <fix-now|defer> --summary <text> [--expires <date>] [--artifact <path>]`
  records a rough edge found while actually using the product as structured
  `rough-edge` evidence on the task. It captures owner, severity, fix-now/defer
  decision, optional expiry, summary, and a linked artifact reference without
  ingesting artifact contents. Expiry accepts RFC3339Nano, RFC3339, or
  `YYYY-MM-DD`; invalid expiry input is rejected rather than silently stored as
  non-expiring feedback.
- `rough-edge list [--task <task-id>] [--owner <role>] [--expired] [--format text|json]`
  projects the owner rough-edge queue from existing evidence rows. It is
  read-only and does not create backlog tasks, mutate status, approve reviews,
  send notifications, merge, deploy, or change dashboard authority.
- `provenance report [--task <task-id>|--since <duration>] [--format text|markdown|json]`
  is a metadata-only supply-chain provenance export over existing Fairway task
  metadata, evidence refs, review refs, checkpoints, sessions, usage counts,
  handoffs, commit refs, and release refs. It excludes raw prompts, private
  transcripts, raw tool bodies, generated-content dumps, credentials, and
  secrets, applying bounded redaction to sensitive-looking refs before
  rendering.
- `provenance prompt-packet --task <task-id> [--format markdown|json]`
  renders a bounded task packet from the same provenance read model. It carries
  objective, scope, acceptance, source facts, validation gates, evidence refs,
  review refs, and forbidden actions, but it does not authorize execution,
  review approval, merge, push, deploy, release, or dashboard mutation.
- `recipe extract|render|list` promotes completed tasks into reusable
  recipe/context packets. Recipes are JSON files, normally under
  `.fairway/recipes`, that reference source facts, evidence refs, validation
  gates, expected evidence, forbidden actions, and closeout rules. They reject
  unsupported schemas, raw prompts, transcripts, tool bodies,
  generated-content dumps, secret-like values, unsafe privacy warnings,
  unsafe substitution values, and recipes without source facts. `recipe render`
  substitutes task-specific fields into a bounded Markdown or JSON packet; it
  does not create tasks, approve, merge, deploy, release, wake providers, or
  mutate dashboard state.
- `provenance manifest --path <file>... [--format text|json]` builds a
  content-free SHA-256 manifest over selected evidence or provenance exports.
  It reports missing artifacts, changed hashes, and privacy-rejected path names
  without embedding artifact contents. It is tamper-evidence for review and
  release packets, not proof that a change was benign or malicious.
- `automation candidates --since <duration> [--threshold <n>] [--format text|json]`
  is a read-only repeated-work report. It detects repeated deterministic
  command, evidence, and notification patterns, then reports frequency, recent
  task ids, representative commands/artifacts, likely owner, estimated
  coordination cost, suggested surface, and recommended action. It does not
  auto-create tasks or mutate workflow.
- `advisory adapters` lists configured advisory provider adapters. It is a
  read-only config inspection surface; it does not invoke providers, store
  prompts/transcripts, wake threads, approve work, or mutate task state.
- `advisory validate` checks a structured recommendation before it is trusted
  or recorded. The contract contains `provider`, `action`, `task_id`,
  `target_role`, `confidence`, `requires_human`, `rationale`, `risk_flags`, and
  cited Fairway facts. Allowed actions are `inspect_task`, `route_review`,
  `record_evidence`, `refresh_memory`, `render_packet`, `create_follow_up`,
  `wake_provider`, `run_preflight`, and `record_checkpoint`. When `--provider`
  names a configured adapter, validation also checks adapter mode and
  `allowed_actions`. Risk flags require `--requires-human`; cited facts must
  name Fairway facts such as `task:<id>`, `evidence:<id>`, `review:<id>`,
  `checkpoint:<id>`, `session:<id>`, `handoff:<id>`, or `notification:<id>`.
  `--record-evidence` writes only an `advisory-recommendation` evidence row; it
  does not approve, merge, deploy, claim, wake providers, mutate environments,
  or replace review/gate checks.
- `notify notifiers`, `notify dry-run`, and `notify send` are the external
  notifier surfaces. `notifiers` is read-only config inspection. `dry-run`
  renders a bounded notification request from a fixed template label. With
  `--record-intent`, it writes a notification row with state `intent` and
  template metadata only. `send` requires an explicitly configured
  `mode = "send"` notifier and records `sent` followed by
  `notification_delivered` or `notification_failed`. Destinations and webhook
  bearer tokens are read from environment variables at send time and are not
  stored. The command does not approve, review, merge, deploy, wake providers
  outside the configured adapter, or give the dashboard send or mutation
  authority.
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
- `completion-handback-report` derives idle-time metrics from existing
  completion handbacks, notification rows, checkpoints, task status, and
  `[coordinator].notification_ack_timeout`. It reports task/workstream closeout
  latency, open/stale counts, and next-decision timing for retrospective
  evidence. It intentionally avoids per-person productivity scoring and excludes
  terminal tasks unless `--include-closed` is passed. Use global `--json` for
  structured output or `--format markdown` for a report artifact.
- Utility adapters such as `examples/session-adapters/utility-event.sh` and
  `examples/session-adapters/ci-monitor.sh` are shell-level conventions over
  existing Fairway commands, not new core subcommands. They should record
  provider-neutral utility sessions, checkpoints, evidence, watcher lifecycle,
  and handback output without making task status, review, or merge decisions.
- `adoption artifact` is the generic readiness report. It uses configured
  workstream profile `route_samples` when no `--route` flags are provided and
  reports named profile gates plus evidence-backed gate evaluation. `parity
  artifact` remains a compatibility alias for GPUaaS-style comparisons.
- `doctor` runs read-only local capability diagnostics before or during agent
  work. It reports config and DB path checks, git worktree state, stale
  `.git/index.lock` guidance, Go cache posture, required CLI tools, dashboard
  reachability, and Fairway session readback as structured pass/warn/fail rows.
  Rows include owner, suggested command, evidence path where applicable, and
  boundary labels such as task work, release, dashboard restart, git boundary,
  provider capability probes, or shared-team pilot. The command does not mutate
  task state, start providers, approve work, push, deploy, restart dashboards,
  run live operations, or expose secrets. Use `--json` or `--format json` for
  agent-consumed output.
- `merge-ready` evaluates profile gates and selected rule-pack requirements for
  the target task. Missing `blocking` profile gates or blocking rule evidence
  fail readiness; missing `advisory` and `report_only` gates or advisory rule
  evidence are reported as warnings. If task metadata declares
  `review_domains`, `merge-ready` also requires an approved review whose
  reviewer matches each domain. `workflow check --mode close --task-id <id>`
  reports the same selected rule evidence gaps during lane closeout.
- `readiness report` evaluates configured profile gates across a workstream or
  all profiles. Missing blocking gates make the report fail in human mode;
  `--json` returns the full report for automation.
- `dashboard` without a subcommand runs in the foreground. Use `dashboard start`,
  `dashboard stop`, `dashboard restart`, and `dashboard status` for a detached
  local dashboard. Detached lifecycle commands do not open a browser unless
  `--open` is passed. They write `.fairway/dashboard.pid` and
  `.fairway/dashboard.log` by default; multi-project mode uses
  `.fairway/dashboard-multi.*`.
- `server --read-only` runs the first shared-team API skeleton. The FW-269
  surface exposes only GET JSON endpoints:
  `/api/v1/status`, `/api/v1/tasks`, `/api/v1/tasks/<task-id>`, and
  `/api/v1/reports/summary`. It reuses the local Fairway store/read models and
  does not add server writes, dashboard writes, review approval, provider-send,
  merge, deploy, release, public exposure, or live-operation authority. Write
  mode flags and write-capable `[server]` config fail closed. Until FW-270 or a
  later reviewed identity/proxy boundary lands, `fairway server --read-only`
  rejects non-loopback listen addresses such as `0.0.0.0`, LAN/private,
  Tailscale, or public interfaces.
  FW-270 adds a request identity and command-authorization guard for the
  read-only API. Supported identity modes are `no_edge_local`,
  `trusted_proxy_read_only`, `api_token`, `service_account`, and
  `mtls_service_account`; service-account and mTLS modes are fail-closed
  placeholders. The implemented command scope is `read:api`, authorized for
  `viewer` or `admin` roles only. API-token roles must be supported server
  roles and included in `[server].allowed_roles`; bearer token failures return
  bounded errors and do not echo submitted or configured token values. This
  still does not add any shared write API.
- `server start --read-only`, `server status`, `server logs`, `server stop`,
  and `server restart --read-only` provide the bounded local lab lifecycle for
  that loopback read-only API. The lifecycle writes
  `.fairway/server-read-only.pid.json` and `.fairway/server-read-only.log` by
  default; explicit `--pid-file` and `--log-file` paths are supported. Status
  reports the binary, version, config, DB, address, and lifecycle paths. The
  JSON pid record includes a non-secret per-launch identity token, and stop or
  restart refuses to signal a process unless its command line matches that
  token, binary, and read-only server shape. Stale records are removed, while
  occupied addresses without a matching record remain `unknown` and fail
  closed. Managed lifecycle never accepts write mode or a non-loopback listen
  address.
- `server --mode api-write-pilot --write` runs the shared-team write API pilot
  when `[server]` is configured for `api-write-pilot`, API-token identity, and a
  command-scoped write role. FW-271 added
  `POST /api/v1/tasks/<task-id>/evidence` and
  `POST /api/v1/tasks/<task-id>/checkpoints`; FW-272 adds guarded
  `POST /api/v1/tasks/<task-id>/status` and
  `POST /api/v1/tasks/<task-id>/reviews`. All writes require JSON,
  `Idempotency-Key`, project-scope matching, command-scoped authorization, and
  unsafe private-data marker rejection. Status writes require
  `expected_status`; review writes require `admin` or matching
  `reviewer:<domain>`. Reviewer-domain tokens cannot override the stored
  reviewer identity; `admin` may explicitly record a reviewer override while
  audit remains bound to the authenticated admin actor. This still does not add
  dashboard-originated mutation, provider sends, merge, deploy, release, public
  exposure, or live-operation authority.
- `packet bugfix`, `packet retry`, platform-foundation packets,
  `packet template`, `packet rules`, and `regression-pack` are quality
  surfaces. They render and validate review context; they do not execute
  product test suites or authorize live execution. `packet retry` renders
  bounded preflight or live-operation retry packets from a task id, source SHA,
  operator surface, artifact directory, evidence contract, allowed actions,
  forbidden actions, expiry/window, and prior-failure closure. When a
  `live-window retry-budget` checkpoint exists, retry packets include the
  iteration count, meaningful failure count, coordination-only failure count,
  budget, and reset task/reference. If the budget is exhausted without an
  existing reset task and reset reason, packet rendering stops with a
  causal-reset requirement. Packet rendering explicitly grants no hidden
  approval. `packet rules <task-id>` renders
  selected and non-applicable rule-pack context, required evidence, recommended
  commands, review domains, rationale, and residual-risk/stop-condition fields;
  record that packet as evidence explicitly when it is used for review or
  handoff. `packet template environment-deploy-preflight` is available as a
  built-in reusable deploy rehearsal packet even when a project has not added a
  local template. `--instantiate-waits` records generic
  `environment-rehearsal` waits for route readback, worker access, smoke,
  rollback, and evidence-contract checks. `--child-task <id=field>` creates an
  explicit child workflow-guard task for a named check field. These
  instantiation modes record coordination state only; they do not run deploy
  commands, approve work, accept live authorization, mutate environments, or
  grant dashboard send/write authority. Projects may still configure their own
  `packet template` entries for reusable deploy rehearsal packets such as
  [environment-deploy-preflight.md](environment-deploy-preflight.md).
- `db compat --backend postgres` prints or validates the reviewed compatibility
  DDL sketch for a future Postgres adapter. `--apply-ddl` remains intentionally
  unimplemented.
- `db rehearsal --backend postgres --out <dir>` creates a disposable rehearsal
  packet from the current SQLite store: SQLite backup, source/rehearsal exports,
  Postgres compatibility report/DDL, read-model equivalence report, manifest,
  and rollback instructions. It opens the SQLite backup as the rehearsal source
  and compares deterministic task/evidence/review/handoff/history counts. It
  does not switch the runtime store, restart dashboards, publish a release, or
  authorize public/shared write exposure. With `--apply-dsn-env <env>`, the
  command also uses `psql` to drop and recreate only the named
  `--postgres-schema` inside the disposable Postgres database named by that
  environment variable, applies the compatibility DDL, imports a bounded
  snapshot of tasks/state/history/evidence/handoffs/reviews/sessions, and writes
  `postgres-apply.sql`, `postgres-import.sql`, and `postgres-readback.json`.
  The schema must be a simple `fairway_`-prefixed name; reserved or common
  schemas such as `public`, `pg_catalog`, `information_schema`, `pg_toast`, and
  all `pg_` schemas are rejected before any drop statement is generated. The
  DSN value must be a `postgres://` or `postgresql://` URL. It is read from the
  environment, split into libpq environment variables, not passed in `psql`
  argv, and not written to the manifest. This proof remains a disposable
  compatibility rehearsal, not adapter parity, production migration, cutover
  readiness, or shared-team store enablement.
- `contract agent-output` prints the versioned JSON contract catalog for
  agent-consumed Fairway output. The first catalog schema is
  `fairway.agent-output-contracts.v1` and covers task packets, ready queues,
  waits, reviews, evidence requirements, lane status, and closeout handbacks.
  Each contract entry names its schema/version, source command, required
  fields, enum values, compatibility expectations, privacy exclusions, and
  authority limits. The command is read-only and does not create tasks, record
  evidence, approve reviews, send providers, mutate dashboard state, merge,
  deploy, release, or run live operations. Agents should consume `--format
  json` or global `--json`, ignore unknown fields unless a schema says
  otherwise, and treat human-formatted text as non-contractual.
- See [release-cuts.md](release-cuts.md) for the subset of this surface that
  ships in each release.

## Output

- Human format by default: tables, color when stdout is a TTY.
- `--json` for everything that lists or returns structured data.
- Exit codes: `0` success, `1` validation / config error, `2` runtime error (DB, git), `3` not found, `4` invariant violation.
