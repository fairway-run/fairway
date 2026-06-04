#!/usr/bin/env bash
set -euo pipefail

role="${1:?usage: tmux.sh <role> [task-id]}"
task_id="${2:-}"
provider="${FAIRWAY_PROVIDER:-codex}"
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
worktree="$(pwd)"
session_id="tmux-${role}-$(date +%Y%m%dT%H%M%S)"
session_name="fairway-${role}"
transcript="${FAIRWAY_TRANSCRIPT:-.fairway/transcripts/${session_id}.log}"
provider_command="${FAIRWAY_PROVIDER_COMMAND:-${SHELL:-bash} -l}"

mkdir -p "$(dirname "$transcript")"
: > "$transcript"

launch_cmd='if [[ -n "${FAIRWAY_INITIAL_PROMPT:-}" ]]; then printf "%s\n" "$FAIRWAY_INITIAL_PROMPT"; fi; exec ${FAIRWAY_PROVIDER_COMMAND:-${SHELL:-bash} -l}'
FAIRWAY_PROVIDER_COMMAND="$provider_command" tmux new-session -d -s "$session_name" -c "$worktree" "${SHELL:-bash}" -lc "$launch_cmd"
pane_id="$(tmux display-message -p -t "$session_name" '#{pane_id}')"
pane_pid="$(tmux display-message -p -t "$pane_id" '#{pane_pid}')"
printf -v transcript_q '%q' "$transcript"
tmux pipe-pane -o -t "$pane_id" "cat >> $transcript_q"

args=(session upsert --id "$session_id" --role "$role" --branch "$branch" --worktree "$worktree" --backend tmux --provider "$provider" --name "$session_name" --tmux-pane "$pane_id" --pid "$pane_pid" --transcript "$transcript")
if [[ -n "$task_id" ]]; then
  args+=(--task-id "$task_id")
fi
fairway "${args[@]}"

if [[ -n "$task_id" ]]; then
  fairway checkpoint record "$task_id" \
    --state active \
    --owner "$role" \
    --summary "Started tmux-backed ${provider} lane; transcript: ${transcript}" \
    --artifact "$transcript"
fi

echo "$session_name $pane_id $transcript"
