#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
usage: ci-monitor.sh --task-id <task-id> --external-run-id <id> --poll-command <cmd> [options]

Provider-neutral utility adapter for CI/deploy/smoke/UAT monitoring. The
adapter records Fairway session proof, watcher lifecycle, checkpoints,
evidence, and a coordinator handback without keeping an agent conversation
active.

Options:
  --task-id <id>                    Fairway task receiving monitor evidence.
  --batch-id <id>                   Optional Fairway work batch id for notes.
  --role <role>                     Monitor owner. Default: ops/watch.
  --utility-name <name>             Utility/backend label. Default: ci-monitor.
  --monitor-kind <ci|deploy|smoke|uat|docs|ops>
                                    Monitor kind. Default: ci.
  --external-run-id <id>            CI/deploy/smoke/UAT run id or URL.
  --automation-id <id>              Optional backing scheduler/heartbeat id.
  --poll-command <cmd>              Command used to poll the external run.
  --source-sha <sha>                Source commit SHA being monitored.
  --manual-until <date|rfc3339>     Expected completion window for manual proof.
  --artifact <path-or-url>          Evidence artifact path or URL.
  --success-regex <regex>           Poll output success regex. Default: success|passed|green.
  --failure-regex <regex>           Poll output failure regex. Default: fail|failed|error|red.
  --interval-seconds <n>            Poll interval. Default: 30.
  --timeout-seconds <n>             Overall timeout. Default: 1800.
  --result <pass|fail|timeout|stale>
                                    Deterministic one-shot result for wrappers/tests.
  --dry-run                         Print Fairway commands without executing them.
  -h, --help                        Show this help.
USAGE
}

fairway_bin="${FAIRWAY_BIN:-fairway}"
task_id=""
batch_id=""
role="ops/watch"
utility_name="ci-monitor"
monitor_kind="ci"
external_run_id=""
automation_id=""
poll_command=""
source_sha=""
manual_until=""
artifact=""
success_regex="success|passed|green"
failure_regex="fail|failed|error|red"
interval_seconds=30
timeout_seconds=1800
forced_result=""
dry_run=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --task-id) task_id="${2:?--task-id requires a value}"; shift 2 ;;
    --batch-id) batch_id="${2:?--batch-id requires a value}"; shift 2 ;;
    --role) role="${2:?--role requires a value}"; shift 2 ;;
    --utility-name) utility_name="${2:?--utility-name requires a value}"; shift 2 ;;
    --monitor-kind) monitor_kind="${2:?--monitor-kind requires a value}"; shift 2 ;;
    --external-run-id) external_run_id="${2:?--external-run-id requires a value}"; shift 2 ;;
    --automation-id) automation_id="${2:?--automation-id requires a value}"; shift 2 ;;
    --poll-command) poll_command="${2:?--poll-command requires a value}"; shift 2 ;;
    --source-sha) source_sha="${2:?--source-sha requires a value}"; shift 2 ;;
    --manual-until) manual_until="${2:?--manual-until requires a value}"; shift 2 ;;
    --artifact) artifact="${2:?--artifact requires a value}"; shift 2 ;;
    --success-regex) success_regex="${2:?--success-regex requires a value}"; shift 2 ;;
    --failure-regex) failure_regex="${2:?--failure-regex requires a value}"; shift 2 ;;
    --interval-seconds) interval_seconds="${2:?--interval-seconds requires a value}"; shift 2 ;;
    --timeout-seconds) timeout_seconds="${2:?--timeout-seconds requires a value}"; shift 2 ;;
    --result) forced_result="${2:?--result requires a value}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ -z "$task_id" || -z "$external_run_id" || -z "$poll_command" ]]; then
  usage
  exit 2
fi

case "$monitor_kind" in
  ci|deploy|smoke|uat|docs|ops) ;;
  *) echo "unsupported monitor kind: $monitor_kind" >&2; exit 2 ;;
esac

case "$forced_result" in
  ""|pass|fail|timeout|stale) ;;
  *) echo "unsupported result: $forced_result" >&2; exit 2 ;;
esac

sanitize_id() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9_.:-' '-'
}

session_id="$(sanitize_id "${utility_name}-${monitor_kind}-${external_run_id}")"
watch_id="$session_id"
artifact_value="$artifact"
if [[ -z "$artifact_value" ]]; then
  artifact_value="$external_run_id"
fi

note_parts=("external_run=${external_run_id}")
if [[ -n "$batch_id" ]]; then note_parts+=("work_batch=${batch_id}"); fi
if [[ -n "$source_sha" ]]; then note_parts+=("source_sha=${source_sha}"); fi
notes="$(printf '%s ' "${note_parts[@]}")"
notes="${notes% }"

followup_prefix_for_kind() {
  case "$1" in
    ci) echo "CI-FIX" ;;
    deploy) echo "CD-FIX" ;;
    smoke) echo "HARNESS-FIX" ;;
    uat) echo "UAT-BUG" ;;
    docs) echo "DOC-FIX" ;;
    ops) echo "OPS-FIX" ;;
    *) echo "OPS-FIX" ;;
  esac
}

run_cmd() {
  if [[ "$dry_run" == true ]]; then
    printf '+'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

run_shell_poll() {
  if [[ "$dry_run" == true ]]; then
    printf '+ %q -lc %q\n' bash "$poll_command"
    return 0
  fi
  bash -lc "$poll_command"
}

record_start() {
  upsert_args=("$fairway_bin" session upsert
    --id "$session_id"
    --role "$role"
    --provider utility
    --backend "$utility_name"
    --name "$external_run_id"
    --task-id "$task_id"
    --status running
    --monitor-kind "$monitor_kind"
    --external-run-id "$external_run_id"
    --poll-command "$poll_command")
  if [[ -n "$automation_id" ]]; then upsert_args+=(--automation-id "$automation_id"); fi
  if [[ -n "$manual_until" ]]; then upsert_args+=(--manual-until "$manual_until"); fi
  run_cmd "${upsert_args[@]}"

  run_cmd "$fairway_bin" watcher start "$watch_id" \
    --task "$task_id" \
    --owner "$role" \
    --process "$monitor_kind" \
    --command "$poll_command" \
    --success "$success_regex" \
    --failure "$failure_regex"

  checkpoint_args=("$fairway_bin" checkpoint record "$task_id"
    --state active
    --owner "$role"
    --summary "${utility_name} watching ${monitor_kind} run ${external_run_id}; ${notes}")
  if [[ -n "$manual_until" ]]; then checkpoint_args+=(--target-close-by "$manual_until"); fi
  checkpoint_args+=(--artifact "$artifact_value")
  run_cmd "${checkpoint_args[@]}"
}

record_heartbeat() {
  run_cmd "$fairway_bin" checkpoint record "$task_id" \
    --state active \
    --owner "$role" \
    --summary "${utility_name} heartbeat for ${monitor_kind} run ${external_run_id}; still waiting; ${notes}" \
    --artifact "$artifact_value"
}

record_success() {
  run_cmd "$fairway_bin" checkpoint record "$task_id" \
    --state done \
    --owner "$role" \
    --summary "${utility_name} completed ${monitor_kind} run ${external_run_id}: pass; ${notes}" \
    --artifact "$artifact_value"
  run_cmd "$fairway_bin" record evidence "$task_id" \
    --command-text "$poll_command" \
    --result pass \
    --artifact "$artifact_value" \
    --artifact-type "${monitor_kind}_monitor" \
    --notes "$notes"
  run_cmd "$fairway_bin" watcher finish "$watch_id" --result pass --artifact "$artifact_value" --notes "$notes"
  run_cmd "$fairway_bin" session end "$session_id" --status ended --reason "${monitor_kind} monitor passed" --exit-code 0
}

record_failure() {
  prefix="$(followup_prefix_for_kind "$monitor_kind")"
  recommended="${prefix}-<next>"
  failure_notes="${notes} recommended_followup=${recommended}"
  run_cmd "$fairway_bin" checkpoint record "$task_id" \
    --state awaiting_input \
    --owner "$role" \
    --summary "${utility_name} completed ${monitor_kind} run ${external_run_id}: fail; create or link ${recommended}" \
    --artifact "$artifact_value"
  run_cmd "$fairway_bin" record evidence "$task_id" \
    --command-text "$poll_command" \
    --result fail \
    --artifact "$artifact_value" \
    --artifact-type "${monitor_kind}_monitor" \
    --notes "$failure_notes"
  run_cmd "$fairway_bin" watcher finish "$watch_id" --result fail --artifact "$artifact_value" --notes "$failure_notes"
  run_cmd "$fairway_bin" session end "$session_id" --status failed --reason "${monitor_kind} monitor failed; recommended ${recommended}" --exit-code 1
  echo "recommended_followup=${recommended}"
}

record_timeout() {
  result_label="$1"
  prefix="$(followup_prefix_for_kind "$monitor_kind")"
  recommended="${prefix}-<next>"
  timeout_notes="${notes} recommended_followup=${recommended}"
  run_cmd "$fairway_bin" checkpoint record "$task_id" \
    --state awaiting_input \
    --owner "$role" \
    --summary "${utility_name} ${result_label} for ${monitor_kind} run ${external_run_id}; create or link ${recommended}" \
    --artifact "$artifact_value"
  run_cmd "$fairway_bin" record evidence "$task_id" \
    --command-text "$poll_command" \
    --result blocked \
    --artifact "$artifact_value" \
    --artifact-type "${monitor_kind}_monitor" \
    --notes "$timeout_notes"
  run_cmd "$fairway_bin" watcher finish "$watch_id" --result blocked --artifact "$artifact_value" --notes "$timeout_notes"
  run_cmd "$fairway_bin" session end "$session_id" --status stale --reason "${monitor_kind} monitor ${result_label}; recommended ${recommended}" --exit-code 124
  echo "recommended_followup=${recommended}"
}

emit_handback() {
  echo "coordinator_handback=${utility_name} ${monitor_kind} ${external_run_id} result=${1}"
  run_cmd "$fairway_bin" reconcile active --dry-run
}

record_start

result="$forced_result"
if [[ -z "$result" ]]; then
  deadline=$((SECONDS + timeout_seconds))
  result="timeout"
  while (( SECONDS <= deadline )); do
    output="$(run_shell_poll || true)"
    if [[ "$output" =~ $success_regex ]]; then
      result="pass"
      break
    fi
    if [[ "$output" =~ $failure_regex ]]; then
      result="fail"
      break
    fi
    record_heartbeat
    sleep "$interval_seconds"
  done
fi

case "$result" in
  pass)
    record_success
    emit_handback pass
    ;;
  fail)
    record_failure
    emit_handback fail
    ;;
  timeout|stale)
    record_timeout "$result"
    emit_handback "$result"
    ;;
esac
