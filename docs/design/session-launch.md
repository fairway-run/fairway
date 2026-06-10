# Session Launch Adapter

Fairway's queue works when agents are launched manually. `fairway session
launch` is an optional convenience adapter for teams that want a consistent way
to start shell, tmux, zellij, or provider-specific sessions.

## Command

```bash
fairway session launch \
  --role <role> \
  [--lane <lane>] \
  [--backend <shell|tmux|zellij>] \
  [--provider <name>] \
  [--task-id <id>] \
  [--prompt-file <path> | --prompt <text>] \
  [--transcript <path>] \
  [--command <provider-command>] \
  [--dry-run]
```

The built-in implementation is intentionally small and provider-neutral. For
`--backend shell`, Fairway can feed a prompt file into a configured provider
command, tee output to a transcript path, and register the resulting session.
Prompt and transcript paths are relative to the launch worktree unless absolute.
If `--prompt` is provided, Fairway writes it to `--prompt-file` or to a generated
`.fairway/prompts/<role>-<task>-<timestamp>.md` path before launch. `--dry-run`
prints the resolved session id, provider command, prompt file, transcript,
worktree, branch, and task metadata without writing prompt files, starting a
process, claiming a task, or recording a session.

## Adapter Contract

Fairway gives the launcher:

- project root,
- role and optional lane,
- worktree path,
- branch,
- optional task ID,
- provider label,
- environment variables derived from config.

The launcher returns:

- session ID,
- backend label,
- session name,
- PID when known,
- tmux pane or zellij pane when applicable,
- transcript path when available,
- start timestamp,
- initial status.

Fairway records the returned data in `agent_sessions`. Transcript contents stay
outside the DB.

## tmux Transcript Bridge

For provider sessions that run inside tmux, use a bridge adapter rather than a
provider-specific core integration. The adapter should:

1. start or attach to a tmux pane,
2. capture pane output to a transcript path with `tmux pipe-pane`,
3. call `fairway session upsert` with provider, role, task id, pane, PID,
   transcript, worktree, and branch,
4. record an initial `fairway checkpoint record` when a task id is associated,
5. rely on `fairway session reconcile` to mark missing PID or tmux-pane
   sessions stale without changing task ownership.

Example:

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_TRANSCRIPT=".fairway/transcripts/claude-platform-map.log" \
examples/session-adapters/tmux.sh orchestrator PF-001
```

Provider labels such as `claude`, `codex`, `gemini`, and `shell` are
informational. Fairway coordinates the lane through session rows, task state,
checkpoints, evidence, handoffs, and reviews.

Durable lane, replaceable provider attachment: the Fairway lane/track is the
stable coordination object. Provider sessions attach execution and runtime
memory to that lane, but they do not define it. A lane can move between Codex,
Claude, Gemini, tmux, or shell without changing task identity, ownership,
checkpoints, evidence, reviews, or merge gates.

## Provider Runtime Watchers

Some providers expose session state that does not map to a local PID or tmux
pane. For example, a delegated Codex thread can enter `waitingOnApproval` or
`waitingOnInput` after the coordinator has already sent the initial prompt.
Fairway core should not poll those provider APIs directly. Instead, provider
runtime watchers should translate provider-specific state into Fairway's generic
coordination surfaces.

Watcher responsibilities:

1. register or update an `agent_sessions` row with a provider label and stable
   external session id,
2. poll or subscribe to provider runtime state outside Fairway core,
3. record checkpoints when the session is waiting on approval, waiting on
   input, stale, failed, or completed,
4. record evidence or handoffs for completed work,
5. leave task ownership, terminal status gates, and merge readiness to Fairway.

Recommended status mapping:

| Provider runtime state | Fairway action |
|---|---|
| started | checkpoint `active` with provider session id and transcript/artifact |
| running | keep session `running`; refresh heartbeat/checkpoint only when useful |
| waiting on approval | checkpoint `awaiting_input` with requested command/action |
| waiting on input | checkpoint `awaiting_input` with the question or missing input |
| completed | checkpoint `done`, then record evidence or handoff; task status changes still use normal gates |
| failed | checkpoint or task `blocked` with reason, depending on owner action needed |
| no progress beyond threshold | checkpoint `awaiting_input` or mark session `stale` |

This keeps Fairway provider-neutral while still making provider-specific runtime
events visible to coordinators and dashboards.

`fairway reconcile active` enforces the lifecycle convention as an advisory
readiness guard. A provider session attached to a task must have a matching
checkpoint: `active` for started/running, `awaiting_input` for waiting,
failed, stale, or no-progress, and `done` for completed. The checkpoint should
name the Fairway session id, external session id, or transcript artifact so the
guard can associate it with the provider attachment. The guard reports the
provider, external session id, role, task id, and expected checkpoint; it never
polls provider APIs or changes task ownership.

The reference convention is `examples/session-adapters/provider-event.sh`. A
provider-specific watcher can translate its local enum into one of Fairway's
generic runtime states and call:

```bash
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id <thread-id> \
  --role <role> \
  --task-id <task-id> \
  --state waiting_on_input \
  --summary "<question or missing input>" \
  --transcript <path-to-transcript>
```

For `started`, the adapter records an `active` checkpoint. For
`waiting_on_approval` and `waiting_on_input`, it records an `awaiting_input`
checkpoint. For `completed`, it records a `done` checkpoint plus evidence by
default or a handoff when `--handoff-to <role>` is provided. Active external
sessions should emit provider events at start, waiting/stale/failure, and
completion so coordinators do not need to remember to poll provider-specific
chat or thread state. The adapter does not claim tasks, set terminal statuses,
approve reviews, or mark work merge-ready.

Provider and utility adapters must validate existing Fairway session state from
structured `fairway --json session status --all` output. They must fail closed
when that JSON cannot be parsed, when a matching session is missing required
fields, when an existing session belongs to a different task, or when an event
tries to continue an ended, failed, or stale session. Human table output is for
operators, not adapter trust-boundary parsing.

## Rules

- A failed launch must not claim a task.
- Launch adapters may record sessions, but they do not bypass `fairway claim`.
- Provider commands are config or adapter behavior, never core queue behavior.
- `--dry-run` prints the command/environment without starting a process.
- `session launch` may record a session and initial checkpoint when `--task-id`
  is provided, but it must not claim the task or mark it terminal.
- Provider runtime watchers are adapters. They may write session, checkpoint,
  evidence, or handoff rows, but Fairway core must not depend on provider APIs.

## Prompt-File Lane Example

For GPUaaS-style platform-foundation lanes, keep the durable lane identity in
Fairway and make the prompt file the repeatable provider attachment:

```bash
fairway session launch \
  --role orchestrator \
  --provider claude \
  --task-id PF-001 \
  --prompt-file prompts/platform-foundation/PF-001.md \
  --transcript .fairway/transcripts/claude-orchestrator-PF-001.log \
  --command "claude" \
  --dry-run
```

After reviewing the dry-run output, run the same command without `--dry-run`.
The launch records the session and an initial active checkpoint, while task
claiming, terminal status changes, evidence, reviews, and merge readiness remain
normal Fairway operations.
