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

## Rules

- A failed launch must not claim a task.
- Launch adapters may record sessions, but they do not bypass `fairway claim`.
- Provider commands are config or adapter behavior, never core queue behavior.
- `--dry-run` prints the command/environment without starting a process.
