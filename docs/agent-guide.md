# Agent Guide

Fairway is built for coding agents working in parallel. This guide is the
operator-facing contract for an agent that is already inside a repo with
Fairway configured.

Canonical definitions live in [Concepts](design/concepts.md). This guide uses
those terms procedurally and does not create alternate meanings for task,
session, decision, evidence, review, role, lane, or promotion.

If Fairway is not configured yet, stop here and use the
[quickstart](quickstart.md). It proves one bounded task, decision, evidence row,
closeout, and readback before introducing the advanced coordination model on
this page. Do not make a new adopter learn sessions, lanes, worktrees, watchers,
or shared-team operation before that first result.

## Cold Start In Consumer Repos

When `fairway init` is run in a consumer repository, it writes
`.fairway/AGENTS.md` as the local agent breadcrumb. That file is intentionally
short: it states the Fairway execution source of truth, the start-of-session
ritual, role resolution order, session registration expectation, and where to
find this full guide for the installed Fairway version.

Agents landing cold in a repo should first look for `.fairway/AGENTS.md`. Repo
maintainers should paste the bootstrap block printed by `fairway init` into the
root `AGENTS.md`, `CLAUDE.md`, or equivalent provider entrypoint so external
agents are directed to the Fairway contract before editing.

Re-running `fairway init` preserves an edited `.fairway/AGENTS.md`. Use
`fairway init --refresh-agent-contract` only when intentionally replacing the
local breadcrumb with the current generated contract.

For agents without a Fairway source checkout or network access, the installed
binary carries this guide:

```bash
fairway agent-guide
fairway agent-guide --path
fairway agent-guide --output .fairway/agent-guide.md
```

## First Rule

The Fairway DB is the execution source of truth. Do not edit queue state files,
SQLite rows, or generated dashboard artifacts directly. Use `fairway` commands
so claims, evidence, handoffs, reviews, sessions, checkpoints, and audit history
stay consistent.

## Execution Surface Limits

Provider surfaces are replaceable execution attachments, and some surfaces have
local sandbox limits. A Desktop-hosted provider may read the repo and edit
workspace files but still fail at git or cache boundaries. Treat these as
execution-surface findings, not as task logic failures.

Known symptoms:

```text
fatal: Unable to create '.git/index.lock': Operation not permitted
open ~/Library/Caches/go-build/...: operation not permitted
browser launch or local capability probe fails only on one provider surface
```

Use the right surface instead of burning provider turns on a known boundary:

- for Go commands from sandboxed Desktop surfaces, set
  `GOCACHE=/tmp/fairway-go-cache`;
- for reviewed git stage/commit/push boundaries, use a tmux or SSH lane started
  outside the Desktop sandbox and capture the output back into Fairway
  evidence;
- for browser, SSH, Kubernetes, or filesystem-sensitive work, run a non-live
  capability probe on the exact execution surface and retire failed surfaces
  for that task/scope until replacement proof exists.

The intended split is:

```text
Fairway decides and records.
Desktop threads coordinate and review.
tmux/SSH/provider lanes execute only the approved command boundary.
```

Do not repeatedly debug `.git/index.lock` in a Desktop provider after verifying
there is no stale lock and a normal terminal can write the index. Route the
approved command boundary to the configured git lane, record the commit SHA,
rerun `merge-ready`, and continue.

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

For prompt-file based lanes that do not need tmux, use `session launch` as the
repeatable provider attachment. Dry-run first so the coordinator can see the
provider command, prompt file, transcript path, worktree, branch, and session
metadata before anything starts:

```bash
fairway session launch \
  --role <role> \
  --provider <codex|claude|gemini|shell> \
  --task-id <task-id> \
  --prompt-file prompts/<track>/<task-id>.md \
  --transcript .fairway/transcripts/<provider>-<role>-<task-id>.log \
  --command "<provider-command>" \
  --dry-run
```

Run the same command without `--dry-run` after review. It records the session
and an initial checkpoint, but it does not claim the task or mark it done.

Fairway coordination should work through task state, evidence, handoffs,
checkpoints, and session records. Provider-specific chat history is useful, but
it is not the coordination source of truth.

## Thread Steering Vs Fairway Notification

Fairway handoffs, Fairway notifications, and Codex Desktop thread steering are
different operations.

Definitions:

| Term | Meaning |
|---|---|
| Fairway handoff recorded | A durable task handoff/checkpoint exists in Fairway. This does not prove a provider thread received a prompt. |
| Fairway notification recorded | Fairway has a notification row or adapter event. This is proof of routing state, not proof of review or completion. |
| Thread steered | A prompt was actually sent into an existing provider thread using a host tool such as `send_message_to_thread`. |
| Thread checked | The target thread was read after steering, and its response/status was reconciled back into Fairway. |

Do not claim "sent to the thread" unless the host tool accepted the message for
that thread. If only Fairway was updated, say "Fairway handoff recorded;
thread/manual steering still required."

When the host environment exposes desktop thread tools, use this sequence:

1. Discover tool availability before claiming capability. For Codex Desktop,
   look for `send_message_to_thread`, `read_thread`, and `list_threads`.
2. Confirm the target thread id when needed with `list_threads`.
3. Send the exact prompt with the thread messaging tool.
4. Record a Fairway handoff, notification, or checkpoint that includes the
   target thread id, role/domain, task id, and prompt summary. Use
   `--state thread_steered` only after direct thread tooling accepts the
   message.
5. Later read the same thread to determine whether it is working, waiting,
   blocked, complete, or asking for input.
6. Record the result back into Fairway as evidence, review, checkpoint,
   notification acknowledgement, or handoff. Provider chat is not durable
   authority.

When thread tools are not exposed:

1. Record the Fairway handoff/checkpoint/notification normally.
   Use `--state handoff_recorded` when Fairway state was updated but no
   provider/thread delivery proof exists.
2. Produce a clearly labeled manual relay block:

   ```text
   Manual thread relay required
   target_thread: <thread-id>
   role_or_domain: <review|ops|security|backend|frontend|architecture|orchestrator>
   task: <fairway-task-id>
   prompt:
   <exact text to paste>
   ```

3. Do not claim the target thread was steered.
4. Continue other non-conflicting ready work if available; otherwise leave an
   `awaiting_input` checkpoint naming the missing manual/thread relay.

Review steering prompts should include the repo path, task id, commit or
worktree path, changed files, validation already run, requested review domains,
and the expected verdict format: approve or changes requested with concrete
blockers.

Durable lane, replaceable provider attachment: a Fairway lane or track is the
durable coordination identity. Provider sessions are replaceable execution
attachments. A long-lived provider session may carry useful working memory, but
the lane can move between Codex, Claude, Gemini, tmux, or shell without changing
task identity, ownership, checkpoints, evidence, reviews, or merge gates.

For long-running tracks, keep a local untracked working memory file under
`tmp-ux/`. Record the current objective, ordered task list, active task, last
completed task and commit, validation commands, required reviews, and the next
action after CI, review, wait, or handback. Do not commit these files unless the
coordinator explicitly converts one into a public assessment or runbook.

Use the active backlog selected by `.fairway/config.toml` as the implementation
queue. In this repository that is
`docs/roadmap/fairway-product-backlog.yaml`. Treat `examples/*.yaml` and
`docs/archive/*.yaml` as source material or provenance only. If a candidate task
from those files becomes active work, promote it into the active backlog and
import/reconcile the DB before claiming it.

When reviewing a task, check Fairway DB visibility before treating a YAML text
search miss as a blocker. The runtime DB is authoritative for task status,
evidence, handoffs, notifications, reviews, sessions, checkpoints, batches, and
usage, and it can contain valid tasks that have not yet been exported back to a
configured YAML queue file. Use:

```bash
fairway task-detail <task-id>
fairway list --status todo --status in_progress --status blocked --status done
fairway reconcile active --dry-run
fairway db export .fairway/fairway-state-snapshot.json
```

Escalate a YAML miss only when it indicates real config/import/export drift or
when the reviewer needs a portable queue artifact that has not been provided.
Do not block an implementation solely because `rg <task-id> <queue-source.yaml>`
does not find a DB-visible task.

Product boundaries are explicit: Fairway coordinates work; it does not
auto-claim, auto-approve, auto-merge, auto-push, deploy, perform destructive
cleanup, store provider credentials/transcripts/prompts by default, or gate
completion on provider usage. See
[design/product-boundaries.md](design/product-boundaries.md) before adding a
new adapter, controller, tracker, usage, or release automation path.

For approval-gated consumer critical flows, use the Fairway template in
[design/consumer-critical-flow-governance.md](design/consumer-critical-flow-governance.md).
The template keeps the durable rule explicit: flow map before implementation,
non-live preflight before live window, bounded retry before causal reset, and
Fairway evidence before handoff. Consumer repos own product scripts, fixtures,
runbooks, and evidence contracts; Fairway owns the reusable coordination state,
packets, waits, notifications, and review/handback evidence.

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

Provider and utility adapters must read existing session state from
`fairway --json session status --all`, not from human table output. If the JSON
cannot be parsed, a matching session is missing required fields, the session is
attached to another task, or the session is already terminal, the adapter must
refuse the event and leave task state unchanged.

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

`fairway reconcile active` checks that provider sessions have the matching
lifecycle checkpoint for their attached task. Running or started sessions need
an `active` checkpoint that names the Fairway session id, external session id,
or transcript artifact. Waiting, failed, stale, and no-progress sessions need
an `awaiting_input` checkpoint. Completed sessions need a `done` checkpoint.
The finding is advisory and provider-neutral; it tells the coordinator which
Fairway checkpoint is missing without polling provider APIs.

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

Use the provider-neutral CI monitor adapter when a project wrapper can supply a
poll command:

```bash
examples/session-adapters/ci-monitor.sh \
  --task-id <task-id> \
  --batch-id <batch-id> \
  --monitor-kind ci \
  --external-run-id <pipeline-or-run-id> \
  --poll-command "<command that prints success/failure status>" \
  --source-sha "$(git rev-parse HEAD)" \
  --manual-until <date-or-rfc3339> \
  --artifact <pipeline-url-or-log> \
  --dry-run
```

Remove `--dry-run` after checking the generated Fairway commands. The adapter
records monitor session proof, watcher lifecycle, heartbeats, evidence, and the
final `reconcile active --dry-run` handback. On failed, timeout, or stale runs
it recommends a follow-up prefix such as `CI-FIX`, `CD-FIX`, `UAT-BUG`,
`OPS-FIX`, `HARNESS-FIX`, or `DOC-FIX`; it does not create tasks unless a
project wrapper chooses to do that explicitly.

Before launching separate CI runs for multiple small tasks, check whether they
can be grouped into a work batch with one branch, one validation command set,
one review path, and one CI/deploy-run. Use separate runs only when ownership,
rollback, risk, sequencing, or failure diagnosis requires it.

Do not treat every provider thread branch as a remote CI branch. Local
worktrees and scratch branches are isolation tools. Remote pushes are promotion
events and need an explicit push intent such as `main-validation`,
`integration`, `review`, `release`, `backup`, or `exception`. In the normal
flow, worker/provider threads validate locally and hand off to the coordinator
or reviewer/merge lane; that lane merges to the configured main branch and
pushes one integrated batch for CI.

## Tool-First Operating Rule

Do not route deterministic, repetitive, or pollable work through an agent just
because an agent can do it. Prefer this order:

1. utility or script for deterministic/pollable work,
2. report or classifier for repetitive summarization,
3. lightweight provider session for bounded implementation or docs,
4. high-context agent/reviewer for judgment, architecture, release, or risk.

Fairway should make tool output easy to consume. A utility should emit enough
structured state for Fairway to record:

- task id, batch id, role, and provider/tool name;
- command or external run id;
- started, heartbeat, completed, failed, timeout, or stale state;
- evidence artifact paths or URLs;
- recommended next action;
- whether a human/agent decision is required.

Agents should act on those recorded facts instead of re-running status checks
or re-summarizing raw logs in conversation. If a repeated workflow cannot be
captured by a utility yet, create a Fairway backlog task for the missing tool
surface rather than adding more provider-routing rules.

Use the generic utility event adapter for deterministic checks that are not
CI/deploy polling loops:

```bash
examples/session-adapters/utility-event.sh \
  --task-id <task-id> \
  --batch-id <batch-id> \
  --utility-name codegen-drift \
  --utility-kind codegen \
  --command "make codegen-check" \
  --external-run-id <run-or-scan-id> \
  --source-sha "$(git rev-parse HEAD)" \
  --artifact dist/codegen-drift.log \
  --state completed \
  --recommended-next-action "continue review; generated artifacts are clean"
```

Supported utility states are `started`, `heartbeat`, `completed`, `failed`,
`timeout`, and `stale`. Terminal utility events record checkpoint state,
evidence, watcher/session closure, and a `reconcile active --dry-run` handback.
Use `--decision-required` when a human or agent must choose the next action.

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
newline-delimited JSON with `turn.completed.usage`, including
`reasoning_output_tokens`, and explicit snapshot mode:

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

If your repo uses rule packs, treat them as reusable operating knowledge for
the task. Rule packs do not approve work by themselves. They identify applicable
rules, expected evidence, review domains, recommended commands, and stop
conditions. Record selected rules and non-applicable rationale as evidence when
the project requires it.

When adopting a new project rule source, follow the checklist in
`docs/design/rule-packs.md#project-adoption-checklist`: verify the local path,
choose advisory/blocking/disabled mode deliberately, check review-domain
vocabulary, run `fairway config validate` and `fairway rules validate`, add CI
validation, and start advisory before promoting a source to blocking.

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
  --source-paths docs/api \
  --review-domains architecture,backend \
  --review-domains governance \
  --acceptance "current owners are mapped" \
  --acceptance "target owners and review routes are mapped" \
  --risk-level medium \
  --migration-type ownership-map
```

`--acceptance`, `--source-paths`, `--target-paths`, and `--review-domains` are
repeatable. The path/domain flags also accept comma-separated values, so use
repeated flags when it keeps long task definitions readable. `task-detail`
renders acceptance checks as separate bullets and path/domain metadata as
flattened lists in the order supplied.

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

# Close/review boundary: fail if the task slice is not committed and the lane
# has unresolved branch/worktree/session closeout debt.
fairway workflow check --mode close --task-id <task-id> --require-clean
fairway workflow closeout <task-id> --dry-run

# Deploy/UAT boundary: require clean pushed source and active-work
# reconciliation before creating deploy evidence.
fairway workflow check --mode deploy --require-clean --require-pushed

# Coverage boundary: check whether commits, files, evidence, and reviews map
# back to Fairway task metadata.
fairway audit work-coverage --since-ref main --dry-run

# Learning boundary: classify failed CI/deploy/smoke/UAT evidence and confirm
# actionable failures have follow-up tasks.
fairway audit ci-learning --template

# Coordination-design boundary: check whether docs, command examples, and
# incident lessons map to Fairway backlog tasks.
fairway audit docs-backlog

# Process-intelligence boundary: measure whether review/gate overhead is
# improving speed, quality, or safety.
fairway delivery report --since 168h

# Automation boundary: find repeated deterministic work before it becomes
# recurring LLM scheduling or bookkeeping.
fairway automation candidates --since 168h
```

The command reports:

- dirty docs/code and the right next action;
- commits that have not been pushed, so CI has not run;
- missing upstream tracking;
- active reconciliation findings such as `in_progress` work without a session
  or evidence without a status decision;
- deploy-run guidance for CI/CD/UAT attempts;
- lane closeout findings such as missing reviews, active sessions/watchers,
  dirty worktrees, unmerged branches, remote branch leftovers, and explicit
  branch preservation reasons.

`audit work-coverage` is advisory. It catches commits that do not mention or
map to a task, changed files outside task `source_paths` / `target_paths`,
evidence that still needs a status decision, done tasks without required
evidence, and missing review-domain approvals. Run it before review handoff,
deploy/UAT attempts, and release readiness checks.

`audit docs-backlog` is advisory. It scans coordination design docs for task
ids, task path coverage, documented Fairway command examples, and known
coordination topics. Run it after incident retrospectives, design reviews, and
large coordination-model updates so doc-only capabilities, stale completed
tasks, and consumer-specific lessons that should become Fairway product tasks
do not remain hidden in chat or local notes. The audit does not change task,
review, merge-ready, or release state.

`delivery report` is advisory. It measures delivery velocity and process
overhead from existing Fairway task transitions, evidence, reviews, handoffs,
notifications, and wait projections. Use it when tuning review profiles or
process pilots to compare overhead with outcomes such as defects caught,
rework, blocked time, cycle time, and avoided unsafe actions. It does not make
metrics into review, merge, deploy, or release gates.

`automation candidates` is advisory. It groups repeated command, evidence, and
notification patterns and recommends a likely implementation surface such as a
script, Fairway CLI command, dashboard panel, watcher, or packet template. Use
it to apply the manual-once, checklist-second, automate-third rule. It does not
auto-create tasks or mutate workflow.

`audit ci-learning` is advisory. `audit failure-routing` is the same read model
with known-failure routing help text and a `failure_routing_ok` human status
label. It turns failed CI, deploy, smoke, UAT, and
coordination evidence into a learning record: failure class, root cause, missed
gate, expected local reproduction command, owner, owning domain/layer, evidence
artifact path, suggested follow-up prefix/kind, and forbidden actions until
review. It recognizes artifact contracts, provider API/4xx behavior, browser
surface failures, setup gates, callback gaps, redaction findings, uncommitted
reviewed files, and undelivered review handoffs. It also checks that actionable
failures have a `CI-FIX-*`, `CD-FIX-*`, `OPS-FIX-*`, `HARNESS-FIX-*`,
`UAT-BUG-*`, or `DOC-FIX-*` task. The report recommends follow-ups only; task
creation still requires an explicit operator command, dry-run/apply workflow,
or configured policy.

`advisory validate` checks optional advisory-provider output before anyone
treats it as useful coordination input:

```bash
fairway advisory validate T-001 \
  --action render_packet \
  --target-role backend \
  --confidence 0.74 \
  --rationale "retry packet should be refreshed from recorded task facts" \
  --cited-fact "task:T-001 status=blocked" \
  --record-evidence
```

Accepted actions are advisory only: `inspect_task`, `route_review`,
`record_evidence`, `refresh_memory`, `render_packet`, `create_follow_up`,
`wake_provider`, `run_preflight`, and `record_checkpoint`. Risk flags require
`--requires-human`; cited facts must point at Fairway task/evidence/review/
checkpoint/session/handoff/notification facts. `--record-evidence` writes
`advisory-recommendation` evidence only. It does not approve reviews, accept
risk, claim work, merge, push, deploy, run live actions, or mutate an
environment.

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

## Shared Dashboard Access

If the dashboard is exposed to teammates through a tunnel or identity-aware
proxy, run it in shared read-only mode. Read-only dashboard access is for
observation, review context, and coordination visibility; it is not permission
to mutate tasks. Record task changes, evidence, reviews, handoffs, and release
actions through the Fairway CLI from a trusted local worktree.

Do not trust proxy identity headers unless the origin is reachable only through
that trusted proxy/tunnel and JWT/header verification is in place. See
[dashboard-sharing.md](design/dashboard-sharing.md).

## Lane Closeout Rule

Fairway task completion is not the same as lane completion. The older lane
worktree model had an important invariant:

```text
finish task -> review/merge -> clean lane -> next task
```

Fairway must preserve that invariant. A lane should not claim the next
implementation task just because the current task is marked `done`. A lane is
ready for the next task only when its current task or batch has a recorded
closeout decision:

- task status is decided: done, blocked, todo/reset, or explicit follow-up;
- evidence is attached and acceptance checks are accounted for;
- required reviews are approved, waived with reason, or the task remains
  review-gated;
- the task or batch commit exists and is associated with the work;
- CI/deploy/UAT result is recorded when that work boundary applies;
- branch is merged and deleted from configured remotes, or intentionally
  preserved with a reason;
- role worktree is clean and on the expected branch;
- provider sessions, watchers, and monitor utilities are ended or intentionally
  still running with fresh checkpoints;
- `fairway reconcile active --dry-run` and the relevant workflow check are
  clean or have explicit findings.

Treat this as the lane boundary:

```text
task done != lane done
```

If a branch or worktree must remain after the task is done, record why. Common
valid reasons are review pending, CI pending, blocked dependency, preserved
release branch, follow-up batch, or operator-approved investigation. Branches
left behind without one of those reasons are cleanup debt.

Use the closeout guard before moving a lane to the next implementation task:

```bash
fairway task-detail <task-id>
fairway merge-ready <task-id>
fairway workflow closeout <task-id> --dry-run
fairway workflow check --mode close --task-id <task-id> --require-clean
fairway reconcile active --dry-run
```

Then merge and delete the task branch, or record a preserve reason as evidence
or checkpoint:

```bash
fairway workflow closeout <task-id> --dry-run \
  --preserve-branch-reason "release branch retained until tag cut"
```

`workflow closeout --apply` deletes only a verified merged `origin/<branch>`
remote branch when the closeout report has no blockers. It does not delete
local branches or worktrees; operators still perform or approve those cleanup
commands explicitly.

Remote branch cleanup is downstream of push intent. If a provider thread
created a scratch branch and it was never meant to be reviewed or validated
remotely, do not push it. If it was pushed by exception, closeout must record
why it was pushed and whether it was merged, deleted, or intentionally
preserved.

Use the explicit intent command before pushing a worker branch remotely:

```bash
fairway record push-intent <task-id> \
  --intent main-validation \
  --branch main \
  --remote origin

fairway record push-intent <task-id> \
  --intent exception \
  --branch scratch/<task-id> \
  --remote origin \
  --reason "operator requested remote backup before risky rebase"
```

`workflow closeout` and `workflow check --mode close` report a remote branch
without matching push-intent evidence as closeout debt.

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
fairway list --status todo,in_progress,blocked
fairway watcher status
fairway reconcile active --dry-run
```

Do not guess CLI subcommands from natural language. Check `fairway --help` or
the group help first (`fairway session --help`, `fairway watcher --help`,
`fairway workflow --help`). Use `fairway watcher status [--include-done]` for
watcher rows; there is no `fairway watcher list`. Use `fairway task-detail`,
`fairway tree`, `fairway ready`, `fairway list --status <state>`, and
`fairway update --dependencies` for task and dependency inspection/update;
there is no grouped `task` command or `depends` shortcut in the current CLI.
Use `fairway list --status todo --ready` when an empty or surprising ready queue
needs dependency context.

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

Approved live operations may record gate or runtime evidence while the task is
still `in_progress`, but only with a bounded closeout marker. Keep the provider
session running and record a fresh active checkpoint with `--target-close-by`
covering the operation window:

```bash
fairway checkpoint record <task-id> \
  --state active \
  --owner ops \
  --target-close-by 2026-06-13T03:15:00Z \
  --summary "Provider session <session-id> active; approved live operation window with expected closeout"
```

While that window is open, `fairway reconcile active --dry-run` treats evidence
recorded after activation as active evidence capture, not as final closeout
debt. This is temporary. If the session is missing, the checkpoint is stale, the
checkpoint has no open `target-close-by`, or the window expires, reconciliation
again reports `status_decision_required` until the task is explicitly moved to
`done`, `blocked`, `todo`, or a configured closeout state. Do not use this
pattern to park unbounded live work.

For repeated exact-window live operations, also record the current handshake
phase so the coordinator/control thread can see the loop state without polling
chat. The full architecture is in
`docs/design/live-operation-control-room.md`; the examples below are the CLI
surface for that model:

```bash
fairway live-window record <task-id> \
  --phase packet-prepared \
  --next-owner governance \
  --next-action route exact-window reviews \
  --artifact .fairway/artifacts/<packet>.md

fairway live-window record <task-id> \
  --phase approvals_ready \
  --next-owner architecture-control \
  --next-action authorize operator handoff \
  --authorization-state "approvals recorded; execution not authorized" \
  --command "fairway live-window record <task-id> --phase execution_authorized" \
  --prompt "Authorize the drill operator for the approved window" \
  --target-close-by 2026-06-13T18:20:00Z \
  --missed-deadline-action "escalate to Architecture Control and reschedule window"

fairway live-window record <task-id> \
  --phase operator_running \
  --next-owner ops \
  --next-action run browser smoke \
  --authorization-state "execution authorized" \
  --target-close-by 2026-06-13T19:20:00Z

fairway live-window status --task <task-id>
fairway live-window control-room --stale
fairway coordinator plan
```

The supported phases are `packet-prepared`, `reviews-routed`,
`approvals-readback`, `gate-authorized`, `gate-running`, `closeout`, and
`next-decision`, plus live-operation control-room phases `packet_ready`,
`approvals_ready`, `execution_authorized`, `operator_running`,
`closeout_required`, `done`, and `blocked`. These records are normal
checkpoints with typed summaries, not a second phase store. Use them to name the
next owner, deadline, authorization state, exact prompt or command, and
missed-deadline behavior after every approval/execution/blocked/done/retry
handoff.

This is also a token-budget boundary. LLM/provider turns should not be used as
the scheduler for routine live-operation waits. Fairway should hold the durable
phase, next actor, deadline, exact action/prompt/command, authorization status,
and missed-deadline behavior so provider turns can focus on judgment,
implementation, review, and exception handling. Optional tmux or zellij
control-room panes should make this state visible without every agent rereading
chat or asking Architecture Control what happened.

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
fairway watcher status --include-done
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

fairway packet retry T-011 \
  --kind preflight \
  --source-sha abc1234 \
  --operator-surface local-shell \
  --artifact-dir .fairway/artifacts/T-011/retry-001 \
  --evidence-contract "preflight output and rollback proof recorded" \
  --allowed-action "run non-live smoke" \
  --forbidden-action "live execution" \
  --expires-at 2026-06-14T21:00:00-05:00 \
  --prior-failure-closure "prior setup failure fixed and reviewed" \
  --next-action "record packet as evidence before retry handoff"

fairway packet vertical-slice T-012 \
  --target-seam "platform evidence facade" \
  --old-path cmd/api/evidence.go \
  --new-path packages/services/platform/evidence.go \
  --adapter "thin route adapter" \
  --proof-command "go test ./cmd/api ./packages/services/platform" \
  --rollback-plan "revert adapter wiring"
```

`packet retry` renders a bounded retry packet only. It does not authorize live
execution, approve risk, or replace Fairway status, review, and gate checks.

## Handoffs

When work crosses a role boundary, hand it off instead of reaching across:

```bash
fairway record handoff T-001 \
  --to ui \
  --payload "Backend contract is ready; see dist/openapi-check.txt"
```

Use `--payload @path/to/file` for longer handoffs.

When a delegated provider is closing a task and the next required action belongs
to another actor, record a completion handback instead of relying on chat state:

```bash
fairway record completion-handback T-001 \
  --to ops \
  --next-action "schedule the next exact-window drill packet" \
  --completion-state blocked-with-follow-up \
  --evidence .fairway/artifacts/T-001/closeout.md \
  --approval-boundary "review-only handback; no deploy authority" \
  --provider codex \
  --target <thread-or-adapter-target> \
  --state thread_steered
```

The command writes a normal handoff and a linked notification row. Use
`--state notification_failed --reason "<why>"` when delivery cannot happen; that
records the failure explicitly so the coordinator can decide the next relay
path. A pending cross-role completion handback (`handoff_recorded`) blocks
terminal closeout until delivery or failure proof is recorded. The handback does
not approve, merge, push, deploy, wake providers from the dashboard, or replace
task status closeout.

Use `--completion-state` for the closeout outcome, not for delivery proof.
Supported outcomes are `done`, `reviewed`, `merge-ready`,
`blocked-with-follow-up`, `monitor-completed`, `live-window-closeout`, and
`live-window-next-decision`. For repeated live-operation loops, a
`live-window closeout` or `next-decision` checkpoint without a completion
handback is surfaced by `fairway coordinator plan` as a closeout-to-next-owner
wait. Pending completion handbacks age by `[coordinator].notification_ack_timeout`
and become stale coordinator actions rather than silent idle work.

To render or record a bounded wake for stale completion handbacks, use the
coordinator tick surface:

```bash
fairway coordinator tick --completion-handback-wake --task <task-id>
fairway coordinator tick --completion-handback-wake --task <task-id> --send --state thread_steered
```

The command uses fixed prompts, stable duplicate-suppression signatures, and
provider targets for the next owner. Missing targets are recorded as
`notification_failed`. Fresh waits and terminal tasks are not woken, and the
dashboard remains read-only.

A handoff is not the same thing as a delivered provider/thread message. When the
coordinator actually sends, attempts, or receives acknowledgement for a provider
notification, record that state separately:

```bash
fairway record notification T-001 \
  --domain ui \
  --provider codex \
  --target <thread-or-adapter-target> \
  --state notification_delivered
```

Use `--state notification_failed --reason "<why>"` when the provider target
could not be contacted. Use `handoff_recorded` when Fairway recorded the
routing state but no provider delivery proof exists, `notification_delivered`
when an adapter or provider confirms delivery, `thread_steered` only when
direct thread tooling accepted the message, `review_acknowledged` when the
target reviewer/control lane confirmed receipt, and `review_recorded` only when
the review was recorded in Fairway.
Notification state never substitutes for `fairway record review`, status
changes, merge, push, deploy, or release gates.

A review-gated task with missing required reviews and only a handoff, a failed
notification, or no delivered reviewer notification is `notification-blocked`,
not normal review wait. `fairway coordinator plan`, task detail, dashboard task
detail, and workflow closeout expose that state so the coordinator retries or
manually relays the reviewer notification before waiting for review.

For parked review waits, use the bounded wake surface rather than writing custom
provider prompts into Fairway state:

```bash
fairway review-waits wake --task <task-id>
fairway review-waits wake --task <task-id> --send --state thread_steered
```

The first command renders the fixed wake prompt without writing notification
state. The second records a provider delivery fact on the `coordinator` domain
after a coordinator/provider adapter has sent or accepted the prompt. Fairway
suppresses duplicate wake signatures and records `notification_failed` if no
wake target is configured. Wake prompts are status-aware: resolved review waits
on blocked, in-progress, todo, or otherwise non-review tasks are review-wait
only and do not authorize `merge-ready` or reviewed-lane closeout. The
dashboard remains read-only and does not send wake prompts.

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

When the reviewer identity must differ from the required review domain, use
`--domain`. For example, an independently assigned reviewer named
`ops-reviewer` can satisfy an `ops` review domain without weakening
no-self-review:

```bash
fairway record review T-001 \
  --reviewer ops-reviewer \
  --domain ops \
  --verdict approve \
  --reason "independent ops review"
```

When every required review domain is approved, `fairway coordinator plan`
surfaces a `review-complete` handback for the coordinator or reviewer/merge
lane. The handback prevents review completion from being trapped in provider
chat, but it does not merge, push, deploy, or release. The coordinator still
runs `fairway merge-ready <task-id>` and performs the configured promotion step
explicitly. If you record delivery of that handback, include the current
`review_signature` from `coordinator plan` or task detail in the notification
reason; commit-only acknowledgement is not enough when the required review set
changes on the same commit.

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
fairway coordinator plan
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
