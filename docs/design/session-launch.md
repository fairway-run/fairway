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
  [--dry-run]
```

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
| running | keep session `running`; refresh heartbeat/checkpoint only when useful |
| waiting on approval | checkpoint `awaiting_input` with requested command/action |
| waiting on input | checkpoint `awaiting_input` with the question or missing input |
| completed | record evidence or handoff; task status changes still use normal gates |
| failed | checkpoint or task `blocked` with reason, depending on owner action needed |
| no progress beyond threshold | checkpoint `awaiting_input` or mark session `stale` |

This keeps Fairway provider-neutral while still making provider-specific runtime
events visible to coordinators and dashboards.

## Rules

- A failed launch must not claim a task.
- Launch adapters may record sessions, but they do not bypass `fairway claim`.
- Provider commands are config or adapter behavior, never core queue behavior.
- `--dry-run` prints the command/environment without starting a process.
- Provider runtime watchers are adapters. They may write session, checkpoint,
  evidence, or handoff rows, but Fairway core must not depend on provider APIs.
