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

## Rules

- A failed launch must not claim a task.
- Launch adapters may record sessions, but they do not bypass `fairway claim`.
- Provider commands are config or adapter behavior, never core queue behavior.
- `--dry-run` prints the command/environment without starting a process.
