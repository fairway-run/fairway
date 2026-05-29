#!/usr/bin/env bash
set -euo pipefail

role="${1:?usage: zellij.sh <role> [task-id]}"
task_id="${2:-}"
provider="${FAIRWAY_PROVIDER:-codex}"
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
worktree="$(pwd)"
session_id="zellij-${role}-$(date +%Y%m%dT%H%M%S)"
session_name="fairway-${role}"

zellij --session "$session_name" action new-pane --cwd "$worktree" --name "$role" -- "${SHELL:-bash}" -lc 'echo "$FAIRWAY_INITIAL_PROMPT"; exec "${SHELL:-bash}" -l'

args=(session upsert --id "$session_id" --role "$role" --branch "$branch" --worktree "$worktree" --backend zellij --provider "$provider" --name "$session_name")
if [[ -n "$task_id" ]]; then
  args+=(--task-id "$task_id")
fi
fairway "${args[@]}"

echo "$session_name"
