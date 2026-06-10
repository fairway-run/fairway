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
  --usage-source <source> Usage source: provider_reported, derived_snapshot, manual, unknown
  --usage-confidence <c>  Usage confidence: exact, estimated, unknown
  --usage-phase <phase>   Usage attribution phase
  --usage-model <model>   Provider model label
  --started-token-snapshot <n>
  --completed-token-snapshot <n>
  --input-tokens <n>
  --cached-input-tokens <n>
  --uncached-input-tokens <n>
  --output-tokens <n>
  --reasoning-tokens <n>
  --total-tokens <n>
  --elapsed-seconds <n>
  --dry-run               Print Fairway commands without executing them
USAGE
}

fairway_bin="${FAIRWAY_BIN:-fairway}"
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
usage_source=""
usage_confidence=""
usage_phase=""
usage_model=""
started_token_snapshot=""
completed_token_snapshot=""
input_tokens=""
cached_input_tokens=""
uncached_input_tokens=""
output_tokens=""
reasoning_tokens=""
total_tokens=""
elapsed_seconds=""
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
    --usage-source) usage_source="${2:?--usage-source requires a value}"; shift 2 ;;
    --usage-confidence) usage_confidence="${2:?--usage-confidence requires a value}"; shift 2 ;;
    --usage-phase) usage_phase="${2:?--usage-phase requires a value}"; shift 2 ;;
    --usage-model) usage_model="${2:?--usage-model requires a value}"; shift 2 ;;
    --started-token-snapshot) started_token_snapshot="${2:?--started-token-snapshot requires a value}"; shift 2 ;;
    --completed-token-snapshot) completed_token_snapshot="${2:?--completed-token-snapshot requires a value}"; shift 2 ;;
    --input-tokens) input_tokens="${2:?--input-tokens requires a value}"; shift 2 ;;
    --cached-input-tokens) cached_input_tokens="${2:?--cached-input-tokens requires a value}"; shift 2 ;;
    --uncached-input-tokens) uncached_input_tokens="${2:?--uncached-input-tokens requires a value}"; shift 2 ;;
    --output-tokens) output_tokens="${2:?--output-tokens requires a value}"; shift 2 ;;
    --reasoning-tokens) reasoning_tokens="${2:?--reasoning-tokens requires a value}"; shift 2 ;;
    --total-tokens) total_tokens="${2:?--total-tokens requires a value}"; shift 2 ;;
    --elapsed-seconds) elapsed_seconds="${2:?--elapsed-seconds requires a value}"; shift 2 ;;
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

if [[ "$runtime_state" == "completed" && -z "$artifact" && -z "$handoff_to" ]]; then
  echo "completed provider events require --artifact or --handoff-to; transcript alone is not completion evidence" >&2
  exit 2
fi

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

existing_session_field() {
  local field="$1"
  if [[ "$dry_run" == true ]]; then
    return 0
  fi
  local json
  if ! json="$("$fairway_bin" --json session status --all 2>/dev/null)"; then
    echo "failed to read Fairway session status for ${session_id}" >&2
    exit 2
  fi
  local value
  if ! value="$(printf '%s' "$json" | ruby -rjson -e '
field = ARGV.fetch(0)
id = ARGV.fetch(1)
begin
  sessions = JSON.parse(STDIN.read)
rescue JSON::ParserError => e
  warn "failed to parse Fairway session JSON: #{e.message}"
  exit 3
end
exit 0 if sessions.nil?
unless sessions.is_a?(Array)
  warn "Fairway session JSON must be an array"
  exit 3
end
session = sessions.find { |item| item.is_a?(Hash) && item["id"] == id }
exit 0 if session.nil?
key = case field
      when "status" then "status"
      when "task" then "task_id"
      else field
      end
unless session.key?(key)
  warn "Fairway session #{id} missing #{key}"
  exit 3
end
value = session[key]
print value unless value.nil?
' "$field" "$session_id")"; then
    echo "refusing provider event because Fairway session status could not be parsed" >&2
    exit 2
  fi
  printf '%s' "$value"
}

existing_status="$(existing_session_field status)"
existing_task_id="$(existing_session_field task)"
if [[ -n "$existing_task_id" && "$existing_task_id" != "$task_id" ]]; then
  echo "session/task mismatch for ${session_id}: existing task ${existing_task_id}, event task ${task_id}" >&2
  exit 2
fi
case "$existing_status:$runtime_state" in
  ended:*|failed:*|stale:*)
    echo "refusing provider event ${runtime_state} for terminal session ${session_id} (${existing_status})" >&2
    exit 2
    ;;
esac

upsert_args=("$fairway_bin" session upsert --id "$session_id" --role "$role" --provider "$provider" --backend "$backend" --name "$external_session_id" --task-id "$task_id" --worktree "$worktree" --branch "$branch")
if [[ -n "$transcript" ]]; then
  upsert_args+=(--transcript "$transcript")
fi
run_cmd "${upsert_args[@]}"

usage_present=false
for usage_value in "$usage_source" "$usage_confidence" "$usage_phase" "$usage_model" "$started_token_snapshot" "$completed_token_snapshot" "$input_tokens" "$cached_input_tokens" "$uncached_input_tokens" "$output_tokens" "$reasoning_tokens" "$total_tokens" "$elapsed_seconds"; do
  if [[ -n "$usage_value" ]]; then
    usage_present=true
    break
  fi
done
if [[ "$usage_present" == true ]]; then
  usage_args=("$fairway_bin" record usage "$task_id" --provider "$provider" --external-session-id "$external_session_id" --session-id "$session_id" --role "$role")
  if [[ -n "$usage_source" ]]; then usage_args+=(--source "$usage_source"); fi
  if [[ -n "$usage_confidence" ]]; then usage_args+=(--confidence "$usage_confidence"); fi
  if [[ -n "$usage_phase" ]]; then usage_args+=(--phase "$usage_phase"); fi
  if [[ -n "$usage_model" ]]; then usage_args+=(--model "$usage_model"); fi
  if [[ -n "$started_token_snapshot" ]]; then usage_args+=(--started-token-snapshot "$started_token_snapshot"); fi
  if [[ -n "$completed_token_snapshot" ]]; then usage_args+=(--completed-token-snapshot "$completed_token_snapshot"); fi
  if [[ -n "$input_tokens" ]]; then usage_args+=(--input-tokens "$input_tokens"); fi
  if [[ -n "$cached_input_tokens" ]]; then usage_args+=(--cached-input-tokens "$cached_input_tokens"); fi
  if [[ -n "$uncached_input_tokens" ]]; then usage_args+=(--uncached-input-tokens "$uncached_input_tokens"); fi
  if [[ -n "$output_tokens" ]]; then usage_args+=(--output-tokens "$output_tokens"); fi
  if [[ -n "$reasoning_tokens" ]]; then usage_args+=(--reasoning-tokens "$reasoning_tokens"); fi
  if [[ -n "$total_tokens" ]]; then usage_args+=(--total-tokens "$total_tokens"); fi
  if [[ -n "$elapsed_seconds" ]]; then usage_args+=(--elapsed-seconds "$elapsed_seconds"); fi
  run_cmd "${usage_args[@]}"
fi

case "$runtime_state" in
  started)
    checkpoint_args=("$fairway_bin" checkpoint record "$task_id" --state active --owner "$role" --summary "Provider session ${session_id} started: ${summary}")
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
    checkpoint_args=("$fairway_bin" checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} waiting on approval: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
  waiting_on_input)
    checkpoint_args=("$fairway_bin" checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} waiting on input: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
  completed)
    checkpoint_args=("$fairway_bin" checkpoint record "$task_id" --state done --owner "$role" --summary "Provider session ${session_id} completed: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    if [[ -n "$handoff_to" ]]; then
      run_cmd "$fairway_bin" record handoff "$task_id" --to "$handoff_to" --payload "Provider session ${session_id} completed: ${summary}"
    else
      evidence_args=("$fairway_bin" record evidence "$task_id" --command-text "provider session ${session_id} completed: ${summary}" --result pass --artifact-type provider-session --artifact "$artifact")
      run_cmd "${evidence_args[@]}"
    fi
    run_cmd "$fairway_bin" session end "$session_id" --status ended --reason "provider completed"
    ;;
  failed)
    checkpoint_args=("$fairway_bin" checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} failed: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    run_cmd "$fairway_bin" session end "$session_id" --status failed --reason "provider failed: $summary" --exit-code 1
    ;;
  stale|no_progress)
    run_cmd "$fairway_bin" session end "$session_id" --status stale --reason "$runtime_state: $summary"
    checkpoint_args=("$fairway_bin" checkpoint record "$task_id" --state awaiting_input --owner "$role" --summary "Provider session ${session_id} ${runtime_state}: ${summary}")
    if [[ -n "$artifact" ]]; then
      checkpoint_args+=(--artifact "$artifact")
    elif [[ -n "$transcript" ]]; then
      checkpoint_args+=(--artifact "$transcript")
    fi
    run_cmd "${checkpoint_args[@]}"
    ;;
esac
