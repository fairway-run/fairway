#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
usage: codex-usage-adapter.sh [--input <path>] --task-id <id> [options]

Maps Codex-supported usage surfaces into Fairway provider usage records.

Supported modes:
  auto        Detect OTel JSON or codex exec --json / NDJSON input. Default.
  otel        Forward Codex-shaped OTLP JSON to provider-otel-ingest.sh.
  exec-json   Read codex exec --json JSON/NDJSON and map turn.completed.usage.
  snapshot    Record caller-supplied start/end token snapshots.

Options:
  --input <path>       Input file. Default: stdin for auto/otel/exec-json
  --task-id <id>       Fairway task id receiving attribution
  --session-id <id>    Fairway session id
  --external-session-id <id>
  --role <role>        Fairway role/lane
  --phase <phase>      Usage phase. Default: implementation
  --model <model>      Model label for snapshot/manual paths
  --mode <mode>        auto, otel, exec-json, or snapshot
  --source <source>    provider_reported, derived_snapshot, manual, unknown
  --confidence <c>     exact, estimated, or unknown
  --started-token-snapshot <n>
  --completed-token-snapshot <n>
  --dry-run            Print Fairway commands instead of executing them

Set FAIRWAY_BIN to choose the Fairway executable. Default: fairway.
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
input="-"
task_id=""
session_id=""
external_session_id=""
role=""
phase="implementation"
model=""
mode="auto"
source=""
confidence=""
started_token_snapshot=""
completed_token_snapshot=""
dry_run=false
fairway_bin="${FAIRWAY_BIN:-fairway}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input="${2:?--input requires a value}"; shift 2 ;;
    --task-id) task_id="${2:?--task-id requires a value}"; shift 2 ;;
    --session-id) session_id="${2:?--session-id requires a value}"; shift 2 ;;
    --external-session-id) external_session_id="${2:?--external-session-id requires a value}"; shift 2 ;;
    --role) role="${2:?--role requires a value}"; shift 2 ;;
    --phase) phase="${2:?--phase requires a value}"; shift 2 ;;
    --model) model="${2:?--model requires a value}"; shift 2 ;;
    --mode) mode="${2:?--mode requires a value}"; shift 2 ;;
    --source) source="${2:?--source requires a value}"; shift 2 ;;
    --confidence) confidence="${2:?--confidence requires a value}"; shift 2 ;;
    --started-token-snapshot) started_token_snapshot="${2:?--started-token-snapshot requires a value}"; shift 2 ;;
    --completed-token-snapshot) completed_token_snapshot="${2:?--completed-token-snapshot requires a value}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$mode" in
  auto|otel|exec-json|snapshot) ;;
  *) echo "unsupported mode: $mode" >&2; exit 2 ;;
esac

if [[ -z "$task_id" ]]; then
  usage
  exit 2
fi

if [[ "$input" != "-" && ! -f "$input" ]]; then
  echo "input file not found: $input" >&2
  exit 2
fi

run_cmd() {
  if [[ "$dry_run" == true ]]; then
    printf '+'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

if [[ "$mode" == "snapshot" ]]; then
  usage_args=("$fairway_bin" record usage "$task_id" --provider codex --phase "$phase")
  if [[ -n "$session_id" ]]; then usage_args+=(--session-id "$session_id"); fi
  if [[ -n "$external_session_id" ]]; then usage_args+=(--external-session-id "$external_session_id"); fi
  if [[ -n "$role" ]]; then usage_args+=(--role "$role"); fi
  if [[ -n "$model" ]]; then usage_args+=(--model "$model"); fi
  usage_args+=(--source "${source:-derived_snapshot}" --confidence "${confidence:-estimated}")
  if [[ -n "$started_token_snapshot" ]]; then usage_args+=(--started-token-snapshot "$started_token_snapshot"); fi
  if [[ -n "$completed_token_snapshot" ]]; then usage_args+=(--completed-token-snapshot "$completed_token_snapshot"); fi
  run_cmd "${usage_args[@]}"
  exit 0
fi

tmp_input=""
if [[ "$input" == "-" ]]; then
  tmp_input="$(mktemp)"
  cat > "$tmp_input"
  input="$tmp_input"
fi
cleanup() {
  if [[ -n "$tmp_input" ]]; then
    rm -f "$tmp_input"
  fi
}
trap cleanup EXIT

if [[ "$mode" == "auto" ]]; then
  if ruby -rjson -e 'root = JSON.parse(File.read(ARGV[0])); exit(root.is_a?(Hash) && (root["resourceLogs"] || root["resourceMetrics"] || root["resourceSpans"]) ? 0 : 1)' "$input" 2>/dev/null; then
    mode="otel"
  else
    mode="exec-json"
  fi
fi

if [[ "$mode" == "otel" ]]; then
  otel_args=("$script_dir/provider-otel-ingest.sh" --input "$input" --task-id "$task_id" --provider codex --phase "$phase")
  if [[ -n "$session_id" ]]; then otel_args+=(--session-id "$session_id"); fi
  if [[ -n "$role" ]]; then otel_args+=(--role "$role"); fi
  if [[ "$dry_run" == true ]]; then otel_args+=(--dry-run); fi
  FAIRWAY_BIN="$fairway_bin" "${otel_args[@]}"
  exit 0
fi

ruby -rjson - "$input" "$fairway_bin" "$task_id" "$session_id" "$external_session_id" "$role" "$phase" "$model" "${source:-provider_reported}" "${confidence:-exact}" "$dry_run" <<'RUBY'
input, fairway_bin, task_id, default_session_id, default_external_session_id, default_role, default_phase, default_model, default_source, default_confidence, dry_run = ARGV
raw = File.read(input)

SENSITIVE_USAGE_KEY = /(prompt|transcript|secret|password|cookie|api[_-]?key|bearer|authorization|raw[_-]?body|tool[_-]?body|input[_-]?text|output[_-]?text|content|completion[_-]?(text|content|message)|message)/i

def parse_events(raw)
  stripped = raw.strip
  return [] if stripped.empty?
  begin
    parsed = JSON.parse(stripped)
    return parsed if parsed.is_a?(Array)
    return [parsed]
  rescue JSON::ParserError
    events = []
    raw.each_line do |line|
      line = line.strip
      next if line.empty?
      begin
        events << JSON.parse(line)
      rescue JSON::ParserError
        next
      end
    end
    events
  end
end

def dig_any(hash, paths)
  paths.each do |path|
    cur = hash
    path.each do |key|
      cur = cur[key] if cur.is_a?(Hash)
    end
    return cur unless cur.nil? || cur == ""
  end
  nil
end

def int_string(value)
  return "" if value.nil? || value == ""
  Integer(value).to_s
rescue ArgumentError, TypeError
  ""
end

def first_string(*values)
  values.each do |value|
    return value.to_s unless value.nil? || value.to_s.empty?
  end
  ""
end

def usage_hash(event)
  type = first_string(event["type"], event["event"], event["event_name"], event["name"])
  if type == "event_msg" && dig_any(event, [["payload", "type"]]) == "token_count"
    usage = dig_any(event, [["payload", "info", "last_token_usage"]])
    return usage if usage.is_a?(Hash)
  end
  usage = dig_any(event, [
    ["turn", "completed", "usage"],
    ["turn.completed", "usage"],
    ["turn.completed.usage"],
    ["response", "completed", "usage"],
    ["response.completed", "usage"],
    ["response", "usage"],
    ["usage"]
  ])
  return nil unless usage.is_a?(Hash)
  return usage if type.empty? || type.include?("turn.completed") || type.include?("response.completed") || type.include?("completed") || event.key?("usage")
  nil
end

commands = []
parse_events(raw).each do |event|
  next unless event.is_a?(Hash)
  usage = usage_hash(event)
  next unless usage
  usage.keys.each do |key|
    raise "refusing sensitive Codex usage key #{key.inspect}" if key =~ SENSITIVE_USAGE_KEY
  end
  args = [fairway_bin, "record", "usage", task_id, "--provider", "codex"]
  fields = {
    "--session-id" => first_string(event["fairway_session_id"], event["session_id"], default_session_id),
    "--external-session-id" => first_string(event["thread_id"], event["conversation_id"], event["session_id"], event["request_id"], default_external_session_id),
    "--role" => first_string(event["role"], default_role),
    "--phase" => first_string(event["phase"], default_phase),
    "--source" => default_source,
    "--confidence" => default_confidence,
    "--model" => first_string(event["model"], dig_any(event, [["response", "model"], ["turn", "model"]]), default_model),
    "--input-tokens" => int_string(first_string(usage["input_tokens"], usage["prompt_tokens"], usage["input"])),
    "--cached-input-tokens" => int_string(first_string(usage["cached_input_tokens"], usage["cache_read_input_tokens"], dig_any(usage, [["input_token_details", "cached_tokens"], ["input_token_details", "cache_read"]]))),
    "--output-tokens" => int_string(first_string(usage["output_tokens"], usage["completion_tokens"], usage["output"])),
    "--reasoning-tokens" => int_string(first_string(usage["reasoning_tokens"], usage["reasoning_output_tokens"], dig_any(usage, [["output_token_details", "reasoning_tokens"]]))),
    "--total-tokens" => int_string(first_string(usage["total_tokens"], usage["total"]))
  }
  fields.each { |flag, value| args += [flag, value] unless value.empty? }
  commands << args
end

commands.each do |args|
  if dry_run == "true"
    puts "+ " + args.map { |arg| arg.include?(" ") ? arg.inspect : arg }.join(" ")
  else
    system(*args) || exit($?.exitstatus || 1)
  end
end
RUBY
