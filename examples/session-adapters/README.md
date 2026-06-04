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
4. capture or tee transcript output to a file and pass that path with
   `--transcript`,
5. record an initial checkpoint when the session is associated with a task,
6. call `fairway session end` when the process exits, if the launcher owns the
   process lifecycle.

Core Fairway commands do not require these launchers. They only make session
visibility and stale-session reconciliation more useful.

## tmux transcript bridge

`tmux.sh` is provider-neutral. It starts a tmux pane, pipes pane output to a
transcript file, registers the session, and records an initial checkpoint when a
task ID is provided.

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_INITIAL_PROMPT="$(cat prompts/platform-map.md)" \
examples/session-adapters/tmux.sh orchestrator PF-001
```

For GPUaaS-style Claude lanes, use the Fairway role as the coordination owner
and provider as an informational label:

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_TRANSCRIPT=".fairway/transcripts/gpuaas-D-arch-PF-001.log" \
examples/session-adapters/tmux.sh D-arch PF-001
```

Reconcile stale panes with:

```bash
fairway session reconcile --dry-run
fairway session reconcile
```
