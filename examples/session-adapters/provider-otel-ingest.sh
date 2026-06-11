#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
usage: provider-otel-ingest.sh [--input <otlp-json>] [--task-id <id>] [--role <role>] [--provider <name>] [--dry-run]

Reads OTLP JSON logs, metrics, or traces from --input or stdin, extracts
provider usage counts and safe attribution metadata, and emits normalized
Fairway usage records through `fairway record usage`.

Required task context can come from OTel attributes (`fairway.task_id`,
`fairway.session_id`, `fairway.role`) or from explicit flags.

Options:
  --input <path>       OTLP JSON file. Default: stdin
  --task-id <id>       Default Fairway task id when not present in attributes
  --session-id <id>    Default Fairway session id when not present in attributes
  --role <role>        Default Fairway role when not present in attributes
  --provider <name>    Default provider when not present in attributes
  --phase <phase>      Default usage phase when not present in attributes
  --dry-run            Print Fairway commands instead of executing them

Set FAIRWAY_BIN to choose the Fairway executable. Default: fairway.
USAGE
}

input="-"
default_task_id=""
default_session_id=""
default_role=""
default_provider=""
default_phase=""
dry_run=false
fairway_bin="${FAIRWAY_BIN:-fairway}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input="${2:?--input requires a value}"; shift 2 ;;
    --task-id) default_task_id="${2:?--task-id requires a value}"; shift 2 ;;
    --session-id) default_session_id="${2:?--session-id requires a value}"; shift 2 ;;
    --role) default_role="${2:?--role requires a value}"; shift 2 ;;
    --provider) default_provider="${2:?--provider requires a value}"; shift 2 ;;
    --phase) default_phase="${2:?--phase requires a value}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ "$input" != "-" && ! -f "$input" ]]; then
  echo "input file not found: $input" >&2
  exit 2
fi

ruby -rjson -rtime - "$input" "$fairway_bin" "$default_task_id" "$default_session_id" "$default_role" "$default_provider" "$default_phase" "$dry_run" <<'RUBY'
input, fairway_bin, default_task_id, default_session_id, default_role, default_provider, default_phase, dry_run = ARGV
raw = input == "-" ? STDIN.read : File.read(input)
root = JSON.parse(raw)

SENSITIVE_KEY = /(prompt[_-]?(text|content|message)|transcript|secret|password|cookie|api[_-]?key|bearer|authorization|raw[_-]?body|tool[_-]?body|input[_-]?text|output[_-]?text|content|completion[_-]?(text|content|message)|message)/i

def otel_value(value)
  return nil unless value.is_a?(Hash)
  value.each do |key, raw|
    next if raw.nil?
    return raw if %w[stringValue intValue doubleValue boolValue bytesValue].include?(key)
    return raw["values"].map { |item| otel_value(item) } if key == "arrayValue" && raw.is_a?(Hash)
    return attrs_to_hash(raw["values"] || []) if key == "kvlistValue" && raw.is_a?(Hash)
  end
  nil
end

def attrs_to_hash(attrs)
  out = {}
  Array(attrs).each do |attr|
    next unless attr.is_a?(Hash)
    key = attr["key"].to_s
    next if key.empty?
    out[key] = otel_value(attr["value"])
  end
  out
end

def merge_attrs(*sets)
  sets.each_with_object({}) { |set, out| set.each { |k, v| out[k] = v unless v.nil? || v == "" } }
end

def each_event(root)
  events = []
  Array(root["resourceLogs"]).each do |resource_log|
    resource = attrs_to_hash(resource_log.dig("resource", "attributes"))
    Array(resource_log["scopeLogs"]).each do |scope_log|
      Array(scope_log["logRecords"]).each do |record|
        attrs = merge_attrs(resource, attrs_to_hash(record["attributes"]))
        body = otel_value(record["body"])
        attrs["event.name"] ||= record["eventName"] || attrs["name"] || (body if body.is_a?(String))
        attrs["event.timestamp"] ||= record["timeUnixNano"] || record["observedTimeUnixNano"]
        events << attrs
      end
    end
  end

  Array(root["resourceSpans"]).each do |resource_span|
    resource = attrs_to_hash(resource_span.dig("resource", "attributes"))
    Array(resource_span["scopeSpans"]).each do |scope_span|
      Array(scope_span["spans"]).each do |span|
        attrs = merge_attrs(resource, attrs_to_hash(span["attributes"]))
        attrs["event.name"] ||= span["name"]
        attrs["event.timestamp"] ||= span["endTimeUnixNano"] || span["startTimeUnixNano"]
        events << attrs
      end
    end
  end

  Array(root["resourceMetrics"]).each do |resource_metric|
    resource = attrs_to_hash(resource_metric.dig("resource", "attributes"))
    Array(resource_metric["scopeMetrics"]).each do |scope_metric|
      Array(scope_metric["metrics"]).each do |metric|
        name = metric["name"].to_s
        points = metric.dig("sum", "dataPoints") || metric.dig("gauge", "dataPoints") || []
        Array(points).each do |point|
          attrs = merge_attrs(resource, attrs_to_hash(point["attributes"]))
          value = point["asInt"] || point["asDouble"]
          token_type = (attrs["token.type"] || attrs["gen_ai.token.type"] || attrs["claude_code.token.type"]).to_s.tr("-", "_")
          if name =~ /token.*usage|usage.*token/
            case token_type
            when "input", "prompt"
              attrs["gen_ai.usage.input_tokens"] = value
            when "cache_read", "cached_input", "cached"
              attrs["gen_ai.usage.cached_input_tokens"] = value
            when "cache_creation"
              attrs["gen_ai.usage.cache_creation_tokens"] = value
            when "output", "completion"
              attrs["gen_ai.usage.output_tokens"] = value
            when "reasoning", "reasoning_output"
              attrs["gen_ai.usage.reasoning_tokens"] = value
            when "total"
              attrs["gen_ai.usage.total_tokens"] = value
            else
              attrs[name] = value
            end
          else
            attrs[name] = value
          end
          attrs["event.name"] ||= name
          attrs["event.timestamp"] ||= point["timeUnixNano"] || point["startTimeUnixNano"]
          events << attrs
        end
      end
    end
  end

  if events.empty? && root.is_a?(Hash)
    events << root
  end
  events
end

def first(attrs, keys)
  keys.each do |key|
    value = attrs[key]
    return value.to_s unless value.nil? || value.to_s.empty?
  end
  ""
end

def int_value(attrs, keys)
  raw = first(attrs, keys)
  return "" if raw.empty?
  Integer(raw).to_s
rescue ArgumentError
  ""
end

def timestamp_value(attrs)
  raw = first(attrs, ["fairway.completed_at", "event.timestamp", "timeUnixNano", "timestamp"])
  return "" if raw.empty?
  if raw =~ /^\d{16,}$/
    Time.at(raw.to_i / 1_000_000_000.0).utc.iso8601(9)
  else
    raw
  end
end

def safe_metadata(attrs)
  out = {}
  {
    "request_id" => ["gen_ai.operation.name", "request.id", "provider.request_id", "claude_code.request.id", "claude_code.api.request.id", "otel.span_id"],
    "track" => ["fairway.track"],
    "query_source" => ["claude_code.query.source", "query.source"],
    "cache_creation" => ["gen_ai.usage.cache_creation_tokens", "cache_creation_tokens", "claude_code.usage.cache_creation_tokens", "claude_code.token.cache_creation", "claude_code.api.request.cache_creation_input_tokens"],
    "cost" => ["gen_ai.usage.cost", "claude_code.cost.usage", "claude_code.cost.usd", "cost"]
  }.each do |target, keys|
    value = first(attrs, keys)
    next if value.empty?
    raise "refusing sensitive OTel usage metadata key #{target.inspect}" if target =~ SENSITIVE_KEY
    out[target] = value
  end
  attrs.keys.each do |key|
    raise "refusing sensitive OTel attribute #{key.inspect}" if key =~ SENSITIVE_KEY
  end
  out
end

def normalize_provider(provider)
  case provider.to_s
  when "claude_code", "claude-code", "anthropic.claude"
    "claude"
  else
    provider.to_s
  end
end

events = each_event(root)
commands = []
events.each do |attrs|
  metadata = safe_metadata(attrs)
  provider = first(attrs, ["fairway.provider", "gen_ai.system", "llm.provider", "ai.provider", "provider", "service.name"])
  provider = normalize_provider(provider)
  provider = default_provider if provider.empty?
  task_id = first(attrs, ["fairway.task_id", "task.id"])
  task_id = default_task_id if task_id.empty?
  next if provider.empty? || task_id.empty?

  args = [fairway_bin, "record", "usage", task_id, "--provider", provider]
  {
    "--session-id" => first(attrs, ["fairway.session_id", "session.id"]),
    "--external-session-id" => first(attrs, ["provider.session_id", "claude_code.session.id", "claude_code.conversation.id", "provider.thread_id", "thread.id", "conversation.id", "gen_ai.conversation.id", "claude_code.request.id", "claude_code.api.request.id", "request.id"]),
    "--role" => first(attrs, ["fairway.role", "role"]),
    "--phase" => first(attrs, ["fairway.phase", "phase"]),
    "--source" => first(attrs, ["fairway.usage.source"]),
    "--confidence" => first(attrs, ["fairway.usage.confidence"]),
    "--completed-at" => timestamp_value(attrs),
    "--model" => first(attrs, ["gen_ai.response.model", "gen_ai.request.model", "model", "llm.model_name", "claude_code.model", "claude_code.api.request.model"]),
    "--input-tokens" => int_value(attrs, ["gen_ai.usage.input_tokens", "input_tokens", "llm.usage.prompt_tokens", "prompt_tokens", "codex.usage.input_tokens", "claude_code.token.input", "claude_code.api.request.input_tokens", "claude_code.api.request.prompt_tokens"]),
    "--cached-input-tokens" => int_value(attrs, ["gen_ai.usage.cached_input_tokens", "gen_ai.usage.input_token_details.cache_read", "cached_input_tokens", "cache_read_input_tokens", "codex.usage.cached_input_tokens", "claude_code.token.cache_read", "claude_code.api.request.cache_read_input_tokens"]),
    "--output-tokens" => int_value(attrs, ["gen_ai.usage.output_tokens", "output_tokens", "llm.usage.completion_tokens", "completion_tokens", "codex.usage.output_tokens", "claude_code.token.output", "claude_code.api.request.output_tokens", "claude_code.api.response.output_tokens", "claude_code.api.request.completion_tokens"]),
    "--reasoning-tokens" => int_value(attrs, ["gen_ai.usage.reasoning_tokens", "gen_ai.usage.reasoning_output_tokens", "reasoning_tokens", "reasoning_output_tokens", "codex.usage.reasoning_tokens", "codex.usage.reasoning_output_tokens", "claude_code.token.reasoning"]),
    "--total-tokens" => int_value(attrs, ["gen_ai.usage.total_tokens", "total_tokens", "llm.usage.total_tokens", "codex.usage.total_tokens", "claude_code.token.total", "claude_code.api.request.total_tokens"]),
    "--elapsed-seconds" => int_value(attrs, ["fairway.elapsed_seconds", "elapsed_seconds"])
  }.each do |flag, value|
    value = default_session_id if flag == "--session-id" && value.empty?
    value = default_role if flag == "--role" && value.empty?
    value = default_phase if flag == "--phase" && value.empty?
    value = "provider_reported" if flag == "--source" && value.empty?
    value = "exact" if flag == "--confidence" && value.empty?
    args += [flag, value] unless value.empty?
  end
  metadata.each { |key, value| args += ["--metadata", "#{key}=#{value}"] }
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
