#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
usage: provider-event.sh --provider <name> --external-session-id <id> --role <role> --task-id <task-id> --state <state> --summary <text> [options]

States:
  started
  running
  waiting_on_approval
  waiting_on_input
  completed
  failed
  stale
  no_progress

Options:
  --backend <label>       Session backend label. Default: provider-session
  --branch <branch>       Branch name. Default: current git branch when available
  --worktree <path>       Worktree path. Default: current directory
  --transcript <path>     Transcript path or URL stored on the session
  --artifact <path>       Evidence/checkpoint artifact path or URL
  --handoff-to <role>     For completed work, record a handoff instead of evidence
  --dry-run               Print Fairway commands without executing them
USAGE
}

provider=""
external_session_id=""
role=""
task_id=""
runtime_state=""
summary=""
backend="provider-session"
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
worktree="$(pwd)"
transcript=""
artifact=""
handoff_to=""
dry_run=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --provider) provider="${2:?--provider requires a value}"; shift 2 ;;
    --external-session-id) external_session_id="${2:?--external-session-id requires a value}"; shift 2 ;;
    --role) role="${2:?--role requires a value}"; shift 2 ;;
    --task-id) task_id="${2:?--task-id requires a value}"; shift 2 ;;
    --state) runtime_state="${2:?--state requires a value}"; shift 2 ;;
    --summary) summary="${2:?--summary requires a value}"; shift 2 ;;
    --backend) backend="${2:?--backend requires a value}"; shift 2 ;;
    --branch) branch="${2:?--branch requires a value}"; shift 2 ;;
    --worktree) worktree="${2:?--worktree requires a value}"; shift 2 ;;
    --transcript) transcript="${2:?--transcript requires a value}"; shift 2 ;;
    --artifact) artifact="${2:?--artifact requires a value}"; shift 2 ;;
    --handoff-to) handoff_to="${2:?--handoff-to requires a value}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ -z "$provider" || -z "$external_session_id" || -z "$role" || -z "$task_id" || -z "$runtime_state" || -z "$summary" ]]; then
  usage
  exit 2
fi

case "$runtime_state" in
  started|running|waiting_on_approval|waiting_on_input|completed|failed|stale|no_progress) ;;
  *) echo "unsupported runtime state: $runtime_state" >&2; exit 2 ;;
esac

session_id="${provider}-${external_session_id}"

run_cmd() {
  if [[ "$dry_run" == true ]]; then
    printf '+'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

upsert_args=(fairway session upsert --id "$session_id" --role "$role" --provider "$provider" --backend "$backend" --name "$external_session_id" --task-id "$task_id" --worktree "$worktree" --branch "$branch")
if [[ -n "$transcript" ]]; then
  upsert_args+=(--transcript "$transcript")
fi
run_cmd "${upsert_args[@]}"

case "$runtime_state" in
  started)
    checkpoint_args=(fairway checkpoint record "$task_id" --state active --owner "$role" --summary "Provider session ${session_id} started: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
  running)
    ;;
  waiting_on_approval)
    checkpoint_args=(fairway checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} waiting on approval: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
  waiting_on_input)
    checkpoint_args=(fairway checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} waiting on input: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
  completed)
    checkpoint_args=(fairway checkpoint record "$task_id" --state done --owner "$role" --summary "Provider session ${session_id} completed: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    if [[ -n "$handoff_to" ]]; then
      run_cmd fairway record handoff "$task_id" --to "$handoff_to" --payload "Provider session ${session_id} completed: ${summary}"
    else
      evidence_args=(fairway record evidence "$task_id" --command-text "provider session ${session_id} completed: ${summary}" --result pass --artifact-type provider-session)
      if [[ -n "$artifact" ]]; then
        evidence_args+=(--artifact "$artifact")
      elif [[ -n "$transcript" ]]; then
        evidence_args+=(--artifact "$transcript")
      fi
      run_cmd "${evidence_args[@]}"
    fi
    ;;
  failed)
    checkpoint_args=(fairway checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} failed: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
  stale|no_progress)
    run_cmd fairway session end "$session_id" --status stale --reason "$runtime_state: $summary"
    checkpoint_args=(fairway checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} ${runtime_state}: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
esac
