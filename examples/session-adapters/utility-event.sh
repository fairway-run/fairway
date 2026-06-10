#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
usage: utility-event.sh --task-id <task-id> --utility-name <name> --state <state> --command <cmd> [options]

Provider-neutral utility event adapter for deterministic, repetitive, or
pollable work that should not keep an agent conversation active. Examples:
codegen drift checks, release asset checks, registry/image freshness checks,
stale branch scans, CI/deploy/smoke/UAT monitors, and docs portal checks.

States:
  started
  heartbeat
  completed
  failed
  timeout
  stale

Options:
  --task-id <id>                  Fairway task receiving utility state/evidence.
  --batch-id <id>                 Optional Fairway work batch id for notes.
  --role <role>                   Utility owner. Default: ops/watch.
  --utility-name <name>           Utility/backend label, e.g. codegen-drift.
  --utility-kind <kind>           Utility kind, e.g. codegen, release-asset, registry.
                                  Default: utility.
  --command <cmd>                 Command or poll operation the utility runs.
  --external-run-id <id>          External run/check id or URL.
  --automation-id <id>            Optional backing scheduler/heartbeat id.
  --source-sha <sha>              Source commit SHA being checked.
  --manual-until <date|rfc3339>   Expected completion window for manual proof.
  --artifact <path-or-url>        Evidence/checkpoint artifact path or URL.
  --artifact-type <type>          Evidence artifact type. Default: <utility-kind>_utility.
  --result <pass|fail|blocked>    Evidence/watcher result override for terminal states.
  --recommended-next-action <txt> Human/agent next action for handback.
  --decision-required             Mark handback as requiring human/agent decision.
  --dry-run                       Print Fairway commands without executing them.
  -h, --help                      Show this help.
USAGE
}

fairway_bin="${FAIRWAY_BIN:-fairway}"
task_id=""
batch_id=""
role="ops/watch"
utility_name=""
utility_kind="utility"
command_text=""
external_run_id=""
automation_id=""
source_sha=""
manual_until=""
artifact=""
artifact_type=""
result_override=""
recommended_next_action=""
decision_required=false
state=""
dry_run=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --task-id) task_id="${2:?--task-id requires a value}"; shift 2 ;;
    --batch-id) batch_id="${2:?--batch-id requires a value}"; shift 2 ;;
    --role) role="${2:?--role requires a value}"; shift 2 ;;
    --utility-name) utility_name="${2:?--utility-name requires a value}"; shift 2 ;;
    --utility-kind) utility_kind="${2:?--utility-kind requires a value}"; shift 2 ;;
    --command) command_text="${2:?--command requires a value}"; shift 2 ;;
    --external-run-id) external_run_id="${2:?--external-run-id requires a value}"; shift 2 ;;
    --automation-id) automation_id="${2:?--automation-id requires a value}"; shift 2 ;;
    --source-sha) source_sha="${2:?--source-sha requires a value}"; shift 2 ;;
    --manual-until) manual_until="${2:?--manual-until requires a value}"; shift 2 ;;
    --artifact) artifact="${2:?--artifact requires a value}"; shift 2 ;;
    --artifact-type) artifact_type="${2:?--artifact-type requires a value}"; shift 2 ;;
    --result) result_override="${2:?--result requires a value}"; shift 2 ;;
    --recommended-next-action) recommended_next_action="${2:?--recommended-next-action requires a value}"; shift 2 ;;
    --decision-required) decision_required=true; shift ;;
    --state) state="${2:?--state requires a value}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ -z "$task_id" || -z "$utility_name" || -z "$state" || -z "$command_text" ]]; then
  usage
  exit 2
fi

case "$state" in
  started|heartbeat|completed|failed|timeout|stale) ;;
  *) echo "unsupported utility state: $state" >&2; exit 2 ;;
esac

case "$result_override" in
  ""|pass|fail|blocked) ;;
  *) echo "unsupported result: $result_override" >&2; exit 2 ;;
esac

case "$state" in
  completed|failed|timeout|stale)
    if [[ -z "$artifact" && -z "$external_run_id" ]]; then
      echo "terminal utility events require --artifact or --external-run-id as backing evidence" >&2
      exit 2
    fi
    ;;
esac

sanitize_id() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9_.:-' '-'
}

run_id="$external_run_id"
if [[ -z "$run_id" ]]; then
  run_id="${task_id}-${utility_kind}"
fi
session_id="$(sanitize_id "${utility_name}-${utility_kind}-${run_id}")"
watch_id="$session_id"
if [[ -z "$artifact_type" ]]; then
  artifact_type="${utility_kind}_utility"
fi
artifact_value="$artifact"
if [[ -z "$artifact_value" ]]; then
  artifact_value="$run_id"
fi

note_parts=("utility=${utility_name}" "utility_kind=${utility_kind}" "state=${state}")
if [[ -n "$external_run_id" ]]; then note_parts+=("external_run=${external_run_id}"); fi
if [[ -n "$batch_id" ]]; then note_parts+=("work_batch=${batch_id}"); fi
if [[ -n "$source_sha" ]]; then note_parts+=("source_sha=${source_sha}"); fi
if [[ "$decision_required" == true ]]; then note_parts+=("decision_required=true"); fi
if [[ -n "$recommended_next_action" ]]; then note_parts+=("next_action=${recommended_next_action}"); fi
notes="$(printf '%s ' "${note_parts[@]}")"
notes="${notes% }"

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
  "$fairway_bin" session status --all 2>/dev/null | awk -v id="$session_id" -v field="$field" '
    $1 == id {
      if (field == "status") print $3;
      if (field == "task") print $5;
      exit
    }
  '
}

validate_session_boundary() {
  local existing_status
  local existing_task_id
  existing_status="$(existing_session_field status)"
  existing_task_id="$(existing_session_field task)"
  if [[ -n "$existing_task_id" && "$existing_task_id" != "$task_id" ]]; then
    echo "session/task mismatch for ${session_id}: existing task ${existing_task_id}, event task ${task_id}" >&2
    exit 2
  fi
  case "$existing_status:$state" in
    ended:*|failed:*|stale:*)
      echo "refusing utility event ${state} for terminal session ${session_id} (${existing_status})" >&2
      exit 2
      ;;
  esac
}

upsert_session() {
  upsert_args=("$fairway_bin" session upsert
    --id "$session_id"
    --role "$role"
    --provider utility
    --backend "$utility_name"
    --name "$run_id"
    --task-id "$task_id"
    --status running
    --monitor-kind "$utility_kind"
    --external-run-id "$run_id"
    --poll-command "$command_text")
  if [[ -n "$automation_id" ]]; then upsert_args+=(--automation-id "$automation_id"); fi
  if [[ -n "$manual_until" ]]; then upsert_args+=(--manual-until "$manual_until"); fi
  run_cmd "${upsert_args[@]}"
}

record_checkpoint() {
  local checkpoint_state="$1"
  local summary="$2"
  checkpoint_args=("$fairway_bin" checkpoint record "$task_id"
    --state "$checkpoint_state"
    --owner "$role"
    --summary "$summary"
    --artifact "$artifact_value")
  if [[ -n "$manual_until" && "$checkpoint_state" == "active" ]]; then
    checkpoint_args+=(--target-close-by "$manual_until")
  fi
  run_cmd "${checkpoint_args[@]}"
}

start_watcher() {
  run_cmd "$fairway_bin" watcher start "$watch_id" \
    --task "$task_id" \
    --owner "$role" \
    --process "$utility_kind" \
    --command "$command_text" \
    --success "pass|clean|success|fresh|present" \
    --failure "fail|drift|missing|stale|error"
}

ensure_watcher_started() {
  start_watcher || true
}

finish_watcher() {
  local result="$1"
  run_cmd "$fairway_bin" watcher finish "$watch_id" --result "$result" --artifact "$artifact_value" --notes "$notes"
}

record_evidence() {
  local result="$1"
  run_cmd "$fairway_bin" record evidence "$task_id" \
    --command-text "$command_text" \
    --result "$result" \
    --artifact "$artifact_value" \
    --artifact-type "$artifact_type" \
    --notes "$notes"
}

finish_session() {
  local status="$1"
  local reason="$2"
  local exit_code="$3"
  run_cmd "$fairway_bin" session end "$session_id" --status "$status" --reason "$reason" --exit-code "$exit_code"
}

emit_handback() {
  local result="$1"
  echo "utility_handback=${utility_name} kind=${utility_kind} task=${task_id} result=${result} decision_required=${decision_required}"
  if [[ -n "$recommended_next_action" ]]; then
    echo "recommended_next_action=${recommended_next_action}"
  fi
  run_cmd "$fairway_bin" reconcile active --dry-run
}

validate_session_boundary
upsert_session

case "$state" in
  started)
    start_watcher
    record_checkpoint active "${utility_name} started ${utility_kind} utility for ${task_id}; ${notes}"
    ;;
  heartbeat)
    record_checkpoint active "${utility_name} heartbeat for ${utility_kind} utility on ${task_id}; ${notes}"
    ;;
  completed)
    result="$result_override"
    if [[ -z "$result" ]]; then result="pass"; fi
    ensure_watcher_started
    record_checkpoint done "${utility_name} completed ${utility_kind} utility for ${task_id}: ${result}; ${notes}"
    record_evidence "$result"
    finish_watcher "$result"
    finish_session ended "${utility_kind} utility completed: ${result}" 0
    emit_handback "$result"
    ;;
  failed)
    result="$result_override"
    if [[ -z "$result" ]]; then result="fail"; fi
    ensure_watcher_started
    record_checkpoint awaiting_input "${utility_name} failed ${utility_kind} utility for ${task_id}; ${notes}"
    record_evidence "$result"
    finish_watcher "$result"
    finish_session failed "${utility_kind} utility failed" 1
    emit_handback "$result"
    ;;
  timeout|stale)
    result="$result_override"
    if [[ -z "$result" ]]; then result="blocked"; fi
    ensure_watcher_started
    record_checkpoint awaiting_input "${utility_name} ${state} ${utility_kind} utility for ${task_id}; ${notes}"
    record_evidence "$result"
    finish_watcher "$result"
    finish_session stale "${utility_kind} utility ${state}" 124
    emit_handback "$state"
    ;;
esac
