#!/usr/bin/env bash
set -euo pipefail

role="${1:?usage: tmux.sh <role> [task-id]}"
task_id="${2:-}"
provider="${FAIRWAY_PROVIDER:-codex}"
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
worktree="$(pwd)"
session_id="tmux-${role}-$(date +%Y%m%dT%H%M%S)"
session_name="fairway-${role}"

tmux new-session -d -s "$session_name" -c "$worktree" "${SHELL:-bash}" -lc 'echo "$FAIRWAY_INITIAL_PROMPT"; exec "${SHELL:-bash}" -l'
pane_id="$(tmux display-message -p -t "$session_name" '#{pane_id}')"

args=(session upsert --id "$session_id" --role "$role" --branch "$branch" --worktree "$worktree" --backend tmux --provider "$provider" --name "$session_name" --tmux-pane "$pane_id")
if [[ -n "$task_id" ]]; then
  args+=(--task-id "$task_id")
fi
fairway "${args[@]}"

echo "$session_name $pane_id"
