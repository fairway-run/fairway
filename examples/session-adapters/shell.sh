#!/usr/bin/env bash
set -euo pipefail

role="${1:?usage: shell.sh <role> [task-id]}"
task_id="${2:-}"
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
worktree="$(pwd)"
session_id="shell-${role}-$$"

args=(session upsert --id "$session_id" --role "$role" --branch "$branch" --worktree "$worktree" --backend shell --provider shell --pid "$$")
if [[ -n "$task_id" ]]; then
  args+=(--task-id "$task_id")
fi
fairway "${args[@]}"

finish() {
  fairway session end "$session_id" --reason normal --exit-code "$?" >/dev/null 2>&1 || true
}
trap finish EXIT

exec "${SHELL:-bash}" -l
