# Session Adapter Examples

Fairway stores session visibility in its DB, but provider/process launch remains
outside the core queue. These examples show how a repo can wrap tmux, zellij,
Codex, Claude, Gemini, or a plain shell and then register the session with
`fairway session upsert`.

They are intentionally small reference scripts. Copy the shape into your own
repo and tune provider flags, prompt text, and transcript paths locally.

## Expected Contract

A launcher should:

1. choose a configured role and worktree,
2. start the provider process or terminal pane,
3. call `fairway session upsert` with `--id`, `--role`, `--branch`,
   `--worktree`, `--backend`, `--provider`, and the best available process or
   pane metadata,
4. call `fairway session end` when the process exits, if the launcher owns the
   process lifecycle.

Core Fairway commands do not require these launchers. They only make session
visibility and stale-session reconciliation more useful.
