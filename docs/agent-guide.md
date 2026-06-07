# Agent Guide

Fairway is built for coding agents working in parallel. This guide is the
operator-facing contract for an agent that is already inside a repo with
Fairway configured.

## First Rule

The Fairway DB is the execution source of truth. Do not edit queue state files,
SQLite rows, or generated dashboard artifacts directly. Use `fairway` commands
so claims, evidence, handoffs, reviews, sessions, checkpoints, and audit history
stay consistent.

## Start Of Session

```bash
fairway config validate
fairway preflight --role <role>
fairway session upsert --role <role> --provider <codex|claude|gemini|shell>
fairway ready
```

For tmux-backed lanes, especially provider sessions that cannot be inspected by
the host application directly, register enough metadata for coordination:

```bash
fairway session upsert \
  --id <session-id> \
  --role <role> \
  --provider <codex|claude|gemini|shell> \
  --backend tmux \
  --name <tmux-session-name> \
  --tmux-pane <session:window.pane> \
  --transcript <path-to-transcript> \
  --task-id <task-id> \
  --pid <pid> \
  --worktree <path> \
  --branch <branch>
fairway checkpoint record <task-id> \
  --state active \
  --owner <role> \
  --summary "Started tmux-backed provider lane; transcript: <path-to-transcript>"
```

The example tmux adapter performs the same registration, transcript capture,
and initial checkpoint in one provider-neutral command:

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_TRANSCRIPT=".fairway/transcripts/claude-<role>-<task-id>.log" \
examples/session-adapters/tmux.sh <role> <task-id>
```

Fairway coordination should work through task state, evidence, handoffs,
checkpoints, and session records. Provider-specific chat history is useful, but
it is not the coordination source of truth.

Durable lane, replaceable provider attachment: a Fairway lane or track is the
durable coordination identity. Provider sessions are replaceable execution
attachments. A long-lived provider session may carry useful working memory, but
the lane can move between Codex, Claude, Gemini, tmux, or shell without changing
task identity, ownership, checkpoints, evidence, reviews, or merge gates.

## Active Work Visibility

Dashboard wall visibility is driven by both task state and session state. A task
marked `in_progress` tells Fairway the task is claimed. A running
`agent_sessions` row tells Fairway which provider attachment is actively working
that task. Agents must keep both current.

Use this order when starting or switching to a task:

```bash
# 1. Register or refresh the provider attachment.
fairway session upsert \
  --id <stable-session-id> \
  --role <provider-role> \
  --provider <codex|claude|gemini|shell> \
  --backend <codex-thread|tmux|zellij|shell> \
  --task-id <task-id> \
  --status running \
  --worktree <path> \
  --branch <branch>

# 2. Claim the task, or keep the existing claim if the coordinator already did.
fairway --as <task-owner-role> claim <task-id>

# 3. Record the active checkpoint, or emit a provider-event "started" event.
fairway checkpoint record <task-id> \
  --state active \
  --owner <provider-role> \
  --summary "<provider> session <stable-session-id> active on <task-id>"

# 4. Confirm the session/task link is visible.
fairway session status --all
```

If step 2 returns "already claimed" and the claim belongs to the expected lane,
continue with steps 3 and 4. If it belongs to another lane, stop and hand off or
ask the coordinator to reassign it.

When a provider switches tasks, upsert the same session ID with the new
`--task-id`, record a completion, blocked, or handoff checkpoint for the old
task, then record an `active` checkpoint for the new task. Do not leave a
running session pointed at stale work.

The provider role and task owner role can differ. For example, an orchestrator
Codex thread may temporarily execute a backend task. In that case, keep the task
owned by `backend` for routing/review, but register the session with
`--role orchestrator --provider codex --task-id <backend-task-id>`. The session
row explains who is attached; the task definition explains who owns the work.

If an active provider is not registered, the wall can show an in-progress task
without a live session. That is a coordination gap. Fix it by upserting the
session and recording an active checkpoint; do not assume the dashboard can infer
provider state from task status alone.

Short direct coordinator/orchestrator work is the exception. A coordinator may
briefly work a task without registering a provider session when the work is
expected to finish in one short burst, the task has a fresh checkpoint naming
the active owner, and the task will be closed, reset, blocked, or handed off
before the burst ends. In that case the wall may show `in_progress without
session`; read it as intentionally un-attached only while the checkpoint is
fresh. High-risk stabilization, UAT, production-readiness, delegated provider
work, tmux/Claude/Codex external work, or anything expected to span multiple
checkpoints must register a provider session and emit a `started` provider
event.

## Delegated Provider Sessions

When one agent delegates work to another provider session, the delegating agent
must keep the coordination loop explicit. Starting or steering a child session
is not enough; the parent needs a watcher or heartbeat that notices when the
child needs attention.

Minimum delegation checklist:

1. register the provider session with `fairway session upsert`,
2. associate it with the current Fairway task,
3. immediately feed a `started` event through `provider-event.sh` so the
   delegated session creates an `active` checkpoint,
4. feed provider runtime state through `provider-event.sh` or equivalent
   Fairway commands,
5. record `awaiting_input` checkpoints for approvals, questions, failures, or
   stale/no-progress states,
6. record a completion checkpoint plus evidence or handoff when the delegated
   session completes,
7. leave task status, review approval, and merge readiness to normal Fairway
   gates.

Approval-sensitive steps are explicit coordination events. If a delegated
session reaches a step that may require human approval or coordinator-side
execution, such as staging, committing, pushing, dependency installation,
remote changes, privileged commands, or destructive cleanup, it should not wait
silently in provider chat. It must:

1. record an `awaiting_input` checkpoint with the exact blocked operation,
2. notify the coordinator session with the command it wants to run, the
   verification already completed, and the current git/Fairway state,
3. wait for the coordinator to either perform the operation, grant permission,
   or redirect the task,
4. reconcile after notification so it does not create duplicate commits,
   pushes, reviews, or status changes.

For Codex-backed sessions, coordinator notification is usually a follow-up
message to the owning Codex thread. For tmux/Claude/Gemini/shell lanes, use the
provider watcher, transcript bridge, or manual checkpoint plus the team’s
chosen coordinator channel. In all cases, the Fairway checkpoint is the durable
signal; provider chat is only the transport.

When a task stalls on current vendor, platform, or provider behavior, do not
burn hours inside one provider session. After one serious local evidence pass,
consult a second current-info source such as Gemini, web search, vendor docs, or
another agent with relevant context. This is especially useful for Apple
signing/notarization, Cloudflare, Pomerium, GitHub/GitLab runners, Homebrew
policy changes, Kubernetes/kind, container registries, MAAS/LXD, OpenClaw,
Keycloak, and provider-specific network or deployment behavior.

The second source is advisory, not task authority. Validate the finding locally
or against the target environment, then record a Fairway checkpoint or evidence
row with the original symptom, the source consulted, the confirmed
interpretation or rejected hypothesis, and the next action. If the finding is a
real platform prerequisite, block the exact task only when no safe progress
remains; otherwise create a scoped follow-up and continue the next ready task.

Provider-specific watchers, such as a Codex thread monitor, should live outside
Fairway core. Their job is to read provider runtime state and translate it into
provider-neutral Fairway facts:

- `waiting_on_approval` or `waiting_on_input` becomes a checkpoint with
  `state=awaiting_input` and a summary of the requested action.
- `completed` becomes a `done` checkpoint plus evidence or a handoff, then the
  owning task can move through normal Fairway gates.
- `failed` becomes a blocked checkpoint or task status with the failure reason.
- stale/no-progress sessions become stale session records or checkpoints, not
  silent background work.

For Codex-style delegated threads, the operating pattern is:

```bash
# 1. Register the delegated session.
fairway session upsert \
  --id <codex-thread-id> \
  --role <role> \
  --provider codex \
  --backend codex-thread \
  --task-id <task-id> \
  --worktree <path> \
  --branch <branch>

# 2. Run a provider adapter or heartbeat outside Fairway core. A provider
# monitor can call the generic event adapter whenever runtime state changes.
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id <codex-thread-id> \
  --role <role> \
  --task-id <task-id> \
  --state waiting_on_approval \
  --summary "<short reason>" \
  --transcript <path-to-transcript>

# 3. If no adapter is available, record the equivalent Fairway fact manually.
fairway checkpoint record <task-id> \
  --state awaiting_input \
  --owner <role> \
  --summary "Delegated Codex thread is waiting on approval: <short reason>"
```

When the coordinator handles the blocked operation, it should notify the
delegated session with the resulting commit, push, or command outcome and tell
the delegated session whether to stop, continue, or only report final summary.
That avoids duplicate git operations while still keeping the implementation
session’s working context intact.

The generic event adapter supports `started`, `running`, `waiting_on_approval`,
`waiting_on_input`, `completed`, `failed`, `stale`, and `no_progress`. It
refreshes the session record first, then records
the mapped checkpoint, evidence, handoff, or stale-session event. Use `started`,
waiting/stale/failure states, and `completed` as mandatory lifecycle events for
active delegated sessions; use `running` only as a metadata refresh.

Do not make Fairway depend on Codex, Claude, Gemini, or any provider API.
Fairway should expose the session/checkpoint/evidence surfaces; provider
watchers should feed those surfaces.

## CI Monitor Utility Rule

CI, deploy, smoke, and UAT polling should be delegated to watcher utilities
instead of long-running agent conversations:

```text
Agents do not poll CI. Watchers poll CI and emit Fairway handbacks.
Agents act on handbacks.
```

Agents should start or link the monitor, record the expected wait window, then
switch to safe non-conflicting work or pause. The monitor utility should poll,
record heartbeat/checkpoint state, attach evidence, close the monitor session,
and emit a handback or resume-needed finding. Agents act on those handbacks;
they should not spend provider tokens repeatedly checking the same pipeline
status.

Before launching separate CI runs for multiple small tasks, check whether they
can be grouped into a work batch with one branch, one validation command set,
one review path, and one CI/deploy-run. Use separate runs only when ownership,
rollback, risk, sequencing, or failure diagnosis requires it.

## Provider Usage Accounting

Provider usage is attribution and planning telemetry, not a completion gate.
Record counts and metadata only. Do not store prompts, transcripts, secrets,
model inputs, generated content, messages, cookies, or provider API tokens as
usage metadata. Unknown values stay unknown; do not report unavailable token
counts as `0`.

Adapters should emit usage through the generic Fairway command path:

```bash
fairway record usage <task-id> \
  --provider codex \
  --session-id <fairway-session-id> \
  --external-session-id <codex-thread-id> \
  --role <role> \
  --phase implementation \
  --source provider_reported \
  --confidence exact \
  --input-tokens <n> \
  --cached-input-tokens <n> \
  --output-tokens <n> \
  --total-tokens <n>
```

If the provider only exposes running totals, record start/end snapshots:

```bash
fairway record usage <task-id> \
  --provider codex \
  --source derived_snapshot \
  --confidence estimated \
  --started-token-snapshot <n> \
  --completed-token-snapshot <n>
```

If usage is unavailable but the session should still be attributed, record the
provider and session identity with `--source unknown --confidence unknown` and
omit numeric fields.

`examples/session-adapters/provider-event.sh` accepts the same usage fields,
for example:

```bash
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id <codex-thread-id> \
  --role <role> \
  --task-id <task-id> \
  --state completed \
  --summary "implemented requested slice" \
  --usage-source provider_reported \
  --usage-confidence exact \
  --usage-phase implementation \
  --input-tokens <n> \
  --cached-input-tokens <n> \
  --output-tokens <n> \
  --total-tokens <n>
```

For provider-supported OpenTelemetry, use the generic OTel bridge. It accepts
OTLP JSON logs, metrics, or traces from stdin or `--input`, maps only structural
usage metadata, and emits `fairway record usage`.

```bash
examples/session-adapters/provider-otel-ingest.sh \
  --input dist/provider-otel.json \
  --task-id <task-id> \
  --role <role> \
  --provider <codex|claude|gemini|shell> \
  --dry-run
```

Remove `--dry-run` after checking the generated command. If the Fairway binary
is not on `PATH`, set `FAIRWAY_BIN=/path/to/fairway`. Task context should come
from OTel resource attributes when possible:

- `fairway.task_id`
- `fairway.session_id`
- `fairway.role`
- `fairway.track`
- `fairway.phase`
- `fairway.usage.source`
- `fairway.usage.confidence`

Provider-specific OTel mappings, such as Codex `response.completed` and Claude
Code token metrics, should plug into this bridge or call `fairway record usage`
with already-normalized fields. Do not enable prompt, tool-body, raw API body,
auth-token, transcript, or generated-content telemetry for usage accounting.

For Codex specifically, use the Codex adapter rather than reading private Codex
SQLite/auth/log/transcript files:

```bash
examples/session-adapters/codex-usage-adapter.sh \
  --mode auto \
  --input dist/codex-usage.jsonl \
  --task-id <task-id> \
  --session-id <fairway-session-id> \
  --role <role> \
  --dry-run
```

The adapter supports Codex-shaped OTel JSON, `codex exec --json` /
newline-delimited JSON with `turn.completed.usage`, and explicit snapshot
mode:

```bash
examples/session-adapters/codex-usage-adapter.sh \
  --mode snapshot \
  --task-id <task-id> \
  --session-id <fairway-session-id> \
  --started-token-snapshot <n> \
  --completed-token-snapshot <n>
```

Remove `--dry-run` only after confirming the generated command contains counts
and metadata, not prompt text or generated content.

For Claude Code, enable usage-only OTel and keep content logging disabled:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_RESOURCE_ATTRIBUTES="fairway.task_id=<task-id>,fairway.session_id=<session-id>,fairway.role=<role>,fairway.phase=implementation"
unset OTEL_LOG_USER_PROMPTS
unset OTEL_LOG_RAW_API_BODIES
```

Then feed exported OTLP JSON through:

```bash
examples/session-adapters/provider-otel-ingest.sh \
  --input dist/claude-code-otel.json \
  --provider claude \
  --task-id <task-id> \
  --role <role> \
  --dry-run
```

Claude Code token and cost metrics map into Fairway usage records; cost is
preserved only as safe metadata for later planning, not pricing or gating.

Use `fairway task-detail <task-id>` or `fairway usage report --by provider` to
inspect recorded usage. Rollups are available by `provider`, `task`, `epic`,
`role`, `day`, `kind`, and `phase`.

If you need machine-readable output, put global flags before the subcommand:

```bash
fairway --json ready
fairway --json task-detail T-001
fairway --json workflow check --mode deploy
```

Use `FAIRWAY_ROLE=<role>` or `--as <role>` when the current worktree cannot be
resolved to one configured role.

If your repo uses workstream profiles, read the configured profile before
claiming profile-shaped work:

```bash
fairway config validate
fairway adoption artifact --limit 5 --gap-limit 5
```

The adoption artifact shows configured gate modes, named profile gates, route
samples, and evidence-backed gate evaluation. `fairway merge-ready` also checks
the profile gates for the target task: missing `blocking` gates fail readiness,
while missing `advisory` and `report_only` gates appear as warnings. Treat
advisory gates as evidence expectations, not as optional background noise.

Profile-shaped work should carry task metadata when the coordinator or dashboard
needs architecture context:

```bash
fairway add T-010 \
  --title "Map platform evidence ownership" \
  --role arch \
  --kind architecture-map \
  --profile platform-foundation \
  --owning-domain platform \
  --owning-layer service \
  --source-paths cmd/api,packages/services \
  --review-domains architecture,backend \
  --risk-level medium \
  --migration-type ownership-map
```

When you spawn follow-up work, Fairway inherits the parent task metadata unless
you override a metadata flag explicitly. Keep those fields accurate; they drive
review routing, readiness reports, and dashboard workstream grouping.

## Work Batches

Use work batches when several granular tasks share the same implementation and
validation surface. The task remains the accountability unit; the batch is the
branch/worktree/CI/deploy-run/evidence unit.

Batch by shared validation surface when tasks have the same domain, touched
contracts, review domains, rollback behavior, and proof commands. Do not batch
when ownership, review routing, rollback, live-environment risk, or dependency
order need independent proof.

Typical coordinator flow:

```bash
fairway batch create BATCH-001 \
  --title "Platform facade validation slice" \
  --branch feature/platform-facade \
  --worktree ../worktrees/platform-facade \
  --task PF-101 \
  --task PF-102 \
  --validation-command "go test ./..." \
  --validation-command "npm test" \
  --review-domain arch \
  --review-domain backend \
  --rollback-criteria "revert the shared branch if API contract changes fail" \
  --split-criteria "split if frontend and backend failures need different owners" \
  --expected-ci "GitHub Actions platform validation"
```

After shared validation, record the batch evidence once and map it to member
tasks:

```bash
fairway batch evidence BATCH-001 \
  --command-text "go test ./... && npm test" \
  --result pass \
  --artifact https://ci.example/runs/123 \
  --artifact-type ci
fairway batch link BATCH-001 --pipeline-id gha-123 --deploy-run-id deploy-456
fairway batch show BATCH-001
```

`batch evidence` maps proof to every member task by default. Only use
`--map-to-tasks=false` when the evidence is a batch note and does not satisfy
any individual task acceptance check. Individual tasks still move through
normal `set-status`, review, profile gates, and `merge-ready`; a passing batch
does not close tasks automatically.

Before review or deploy boundaries, run:

```bash
fairway audit work-coverage --dry-run --since-duration 24h
```

The audit reports `work_batch_candidate` when several related same-domain tasks
have separate CI/deploy evidence and are not already in a batch. Treat that as
a signal to batch future work when it shares validation, not as a retroactive
requirement to rewrite history.

## Workflow Guard

Use `workflow check` at task, review, and deploy boundaries. It keeps the
operating model short by turning the repeated manual checks into one command.

```bash
# Normal task boundary: warns on dirty files and unpushed commits.
fairway workflow check

# Close/review boundary: fail if the task slice is not committed.
fairway workflow check --mode close --require-clean

# Deploy/UAT boundary: require clean pushed source and active-work
# reconciliation before creating deploy evidence.
fairway workflow check --mode deploy --require-clean --require-pushed

# Coverage boundary: check whether commits, files, evidence, and reviews map
# back to Fairway task metadata.
fairway audit work-coverage --since-ref main --dry-run

# Learning boundary: classify failed CI/deploy/smoke/UAT evidence and confirm
# actionable failures have follow-up tasks.
fairway audit ci-learning --template
```

The command reports:

- dirty docs/code and the right next action;
- commits that have not been pushed, so CI has not run;
- missing upstream tracking;
- active reconciliation findings such as `in_progress` work without a session
  or evidence without a status decision;
- deploy-run guidance for CI/CD/UAT attempts.

`audit work-coverage` is advisory. It catches commits that do not mention or
map to a task, changed files outside task `source_paths` / `target_paths`,
evidence that still needs a status decision, done tasks without required
evidence, and missing review-domain approvals. Run it before review handoff,
deploy/UAT attempts, and release readiness checks.

`audit ci-learning` is advisory. It turns failed CI, deploy, smoke, and UAT
evidence into a learning record: failure class, root cause, missed gate,
expected local reproduction command, owner, and follow-up task. It also checks
that actionable failures have a `CI-FIX-*`, `CD-FIX-*`, `OPS-FIX-*`,
`HARNESS-FIX-*`, `UAT-BUG-*`, or `DOC-FIX-*` task.

For Fairway release attempts, create one release-run task/checkpoint and render
a release-run packet before tagging:

```bash
fairway packet release-run <release-task-id> \
  --version vX.Y.Z \
  --tag vX.Y.Z \
  --source-sha "$(git rev-parse HEAD)" \
  --release-notes docs/release-notes.md \
  --changelog-state "CHANGELOG.md updated" \
  --ci-status pass \
  --docs-status pass \
  --signing-status pass \
  --notary-status pass \
  --release-url "https://github.com/fairway-run/fairway/releases/tag/vX.Y.Z" \
  --homebrew-tap-commit <tap-commit-sha> \
  --verification-command "brew fetch --cask --force fairway-run/tap/fairway"
```

After the GitHub release and Homebrew tap update are observable, run
`fairway release verify`. Do not mark a release as Homebrew-usable while the
GitHub release is still a draft; asset URLs can return 404 until the draft is
published.

For deploy work, create one deploy-run task for the attempt and create
`CI-FIX-*`, `CD-FIX-*`, `UAT-BUG-*`, `OPS-FIX-*`, `HARNESS-FIX-*`, or
`DOC-FIX-*` follow-ups only for actionable findings.

## Claim Work

```bash
fairway claim T-001
fairway task-detail T-001
```

Claiming moves a ready task to `in_progress` and records the owner/branch.
If another agent wins the claim first, Fairway returns an already-claimed error;
do not keep working that task unless the coordinator reassigns it.

For epic-sized work, claim the next ready descendant:

```bash
fairway claim --in E-001
```

## During Work

Keep local notes however your agent runtime prefers, but record durable facts in
Fairway:

```bash
fairway record evidence T-001 \
  --command-text "go test ./..." \
  --result pass \
  --artifact dist/test.log \
  --artifact-type test
```

Use `pass`, `fail`, `partial`, `skipped`, or `blocked` honestly. A skipped or
blocked check is better than undocumented silence.

If work becomes blocked:

```bash
fairway set-status T-001 blocked --reason "waiting for API fixture"
```

Blocked transitions require a reason in the default config.

## Reconciliation Checkpoint

After every significant work burst, reconcile Fairway state before leaving the
track:

```bash
fairway session status
fairway status-report
fairway ready
fairway reconcile active --dry-run
```

Use `fairway session reconcile --dry-run` when you specifically want to inspect
session-local cleanup such as dead PIDs, missing tmux panes, or stale sessions.
Use `fairway reconcile active --dry-run` for the broader end-of-burst check.

The expected end-of-burst state is:

- active sessions are zero, or each running session is intentionally attached to
  a non-terminal task,
- `in_progress` contains only work that is actively owned or deliberately left
  open with a fresh checkpoint,
- tasks with pass evidence are moved to `done` or have a documented reason they
  remain open,
- tasks with fail, partial, skipped, or blocked evidence are moved to `blocked`,
  reset to `todo`, or split into explicit follow-up work,
- parent/backlog tasks are not left `in_progress` unless the parent itself has a
  current rollup artifact or checkpoint.

Do not use `todo` to hide meaningful partial progress without recording a
handoff, checkpoint, or follow-up task. If the project config supports extended
states such as `needs_followup`, `partial`, `waiting_for_prereq`, or `stale`,
use those states consistently; otherwise use `blocked` with a clear reason or
create a follow-up task and close/reset the parent.

## Side Work

Do not split your assigned task into Fairway subtasks for ordinary execution
steps. Use local scratch notes for that.

Use Fairway only when the orchestrator needs to see the work:

```bash
fairway spawn --id T-099 --title "Fix discovered billing route regression" --sibling
```

For long-running side tracks, create a packet and checkpoint:

```bash
fairway packet context T-001 \
  --goal "finish API contract" \
  --owner backend \
  --acceptance "contract tests pass"

fairway checkpoint record T-001 \
  --state active \
  --owner backend \
  --summary "waiting on API fixture"
```

Watcher work should use watcher packets and lifecycle records:

```bash
fairway packet watcher W-001 --owner C-ops/watch --process ci \
  --command "gh run watch" --success "green" --failure "red"
fairway watcher start W-001 --task T-001 --owner C-ops/watch --process ci
fairway watcher finish W-001 --result pass --artifact dist/ci.log
```

Before leaving a CI/deploy/UAT/provider monitor active, prove that a real
watcher exists. Record the automation id, PID, tmux pane, external run id plus
polling command, or a manual checkpoint window with an explicit expiry. If a
monitor task only creates Fairway session/task rows and no backing heartbeat or
bounded checkpoint exists, reset or close it before ending the work burst. That
state is stale bookkeeping, not live monitoring.

When the final monitored item completes, hand control back to the work loop.
Close the deploy-run/watch tasks and monitor sessions, then record or send the
next action: push held branches, start the next ready task, request review, or
state that no ready work remains. A monitor heartbeat finishing successfully is
not the same thing as the overall track being complete.

For Codex/Claude/tmux/provider-backed coordinators, this handback should be a
real continuation prompt to the owning coordinator session whenever ready work
remains. The monitor should not only delete its heartbeat and exit. Include the
working-memory path, Fairway config path, completed monitor summary, and this
instruction:

```text
The monitored CI/deploy/UAT window is complete. Read the current working memory
file, check Fairway status and ready tasks, and continue with the next
non-conflicting task unless a documented stop condition applies. Record a
checkpoint explaining the selected next action.
```

If the monitor cannot send that provider prompt, record a `resume_needed`
checkpoint or finding with the coordinator session id and the next ready task
summary. Clean Fairway state plus ready work is not enough; the execution lane
also needs a continuation signal. `fairway reconcile active` reports the
fallback condition as `monitor_completion_resume_needed` when all monitors are
closed, no active sessions/watchers remain, and ready work is still queued. The
dashboard diagnostics tab shows the same finding.

Use provider-neutral session fields for monitor proof:

```bash
fairway session upsert \
  --id ci-monitor-T-001 \
  --role ops/watch \
  --backend ci-monitor \
  --task-id T-001 \
  --status running \
  --monitor-kind ci \
  --automation-id gha-heartbeat-T-001

fairway session upsert \
  --id deploy-run-T-002 \
  --role ops/watch \
  --backend deploy-monitor \
  --task-id T-002 \
  --status running \
  --monitor-kind deploy \
  --external-run-id deploy-123 \
  --poll-command "gh run view deploy-123"
```

For a short manual monitor, record an active checkpoint with
`--target-close-by <date>`. After that date, `fairway reconcile active` reports
the monitor as `monitor_session_without_backing_proof` unless another backing
proof is attached. The dashboard diagnostics tab shows the same active
reconciliation finding.

Platform-foundation work should use the narrower packet type that matches the
task. If a repo defines `[[packet_templates]]`, use those fields as the packet
contract. Generic templates render with `fairway packet template <name>`:

```bash
fairway packet template architecture-map T-010 \
  --field scope="route ownership" \
  --field current_owner=mixed \
  --field target_owner=D-arch \
  --field migration_risk="route moves can hide auth regressions" \
  --field acceptance="owners and review routes are explicit"
```

Built-in packet commands remain available for common profiles:

```bash
fairway packet architecture-map T-010 \
  --scope "route ownership" \
  --current-owner mixed \
  --target-owner D-arch \
  --migration-risk "route moves can hide auth regressions" \
  --source-doc doc/architecture/platform-foundation/ownership.md \
  --acceptance "owners and review routes are explicit"

fairway packet boundary-guard T-011 \
  --guard-intent "report imports across package boundaries" \
  --finding "cmd/api imports billing internals" \
  --false-positive "generated client code" \
  --graduation-criteria "zero critical findings for two releases" \
  --proof-command "go test ./..."

fairway packet vertical-slice T-012 \
  --target-seam "platform evidence facade" \
  --old-path cmd/api/evidence.go \
  --new-path packages/services/platform/evidence.go \
  --adapter "thin route adapter" \
  --proof-command "go test ./cmd/api ./packages/services/platform" \
  --rollback-plan "revert adapter wiring"
```

## Handoffs

When work crosses a role boundary, hand it off instead of reaching across:

```bash
fairway record handoff T-001 \
  --to ui \
  --payload "Backend contract is ready; see dist/openapi-check.txt"
```

Use `--payload @path/to/file` for longer handoffs.

## Review

Route review based on changed paths:

```bash
fairway route review T-001 --path cmd/api/routes.go --path doc/api/openapi.draft.yaml
```

Reviewers record a verdict:

```bash
fairway record review T-001 \
  --reviewer governance \
  --verdict approve \
  --reason "route and evidence look good"
```

Use `changes` rather than `approve` when more work is required. No agent should
approve its own work.

## Finish Work

Before marking done, record the evidence that proves the acceptance checks:

```bash
fairway record evidence T-001 --command-text "go test ./..." --result pass
fairway set-status T-001 done
fairway merge-ready T-001
```

If gates fail, fix the missing evidence/review/handoff or record why the task is
not ready. Do not force a green story into the DB.

## External Tracker Mirrors

Plane, Jira, Linear, and similar tools are planning mirrors, not execution
stores. Use tracker commands to render or link planning context, but do not let
external issue state drive Fairway task status, sessions, evidence, reviews, or
merge gates.

For the Plane spike, set local environment variables and run dry-run commands:

```bash
export PLANE_BASE_URL=http://localhost:8088
export PLANE_WORKSPACE=fairway-eval
export PLANE_PROJECT=FWPLANE

fairway tracker plane export --task-id FW-122
fairway tracker plane import --fixture examples/tracker-adapters/plane/evaluation-workspace.yaml
fairway tracker plane comment --task-id FW-122 --external-id FWPLANE-122
```

`fairway tracker plane --apply` paths are intentionally unsupported in the
spike. Plane tokens must come from environment or OS credential storage and must
not be committed.

End your session when the runtime exits:

```bash
fairway session end <session-id> --reason normal --exit-code 0
```

## What Not To Do

- Do not edit Fairway DB rows by hand.
- Do not keep working after losing a claim.
- Do not switch roles by changing branches inside a role worktree.
- Do not create Fairway subtasks for private implementation steps.
- Do not self-review.
- Do not mark `done` without evidence, even when the config allows it.
- Do not rely on Jira, Linear, GitHub Issues, or a chat thread as the execution
  source of truth. Link them if useful; keep execution state in Fairway.

## Useful Commands

```bash
fairway ready
fairway task-detail <task-id>
fairway status-report
fairway health-report
fairway dispatch-plan --role <role>
fairway checkpoint status
fairway session status
fairway session reconcile --dry-run
fairway dashboard start
fairway dashboard status
```

See [design/cli.md](design/cli.md) for the complete command surface.

Fairway sessions are records created by `fairway session upsert` or
`fairway session launch`. Host applications may show their own subagent history
or worker list; those entries are not Fairway sessions unless they were
registered with Fairway. Use `fairway session status` for live Fairway-tracked
lanes, and `fairway session reconcile --dry-run` before assuming a host-app
sidebar count represents active Fairway work.
