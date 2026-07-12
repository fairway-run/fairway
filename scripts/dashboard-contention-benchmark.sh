#!/usr/bin/env bash
set -euo pipefail

binary=""
config=""
db=""
out=""
read_only_port=7896
full_port=7897
samples=5

usage() {
  echo "usage: $0 --binary <path> --config <path> --db <path> --out <dir> [--read-only-port <port>] [--full-port <port>] [--samples <n>]" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary) binary="$2"; shift 2 ;;
    --config) config="$2"; shift 2 ;;
    --db) db="$2"; shift 2 ;;
    --out) out="$2"; shift 2 ;;
    --read-only-port) read_only_port="$2"; shift 2 ;;
    --full-port) full_port="$2"; shift 2 ;;
    --samples) samples="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

for value in binary config db out; do
  if [[ -z "${!value}" ]]; then
    echo "--${value//_/-} is required" >&2
    usage
    exit 2
  fi
done
[[ -x "$binary" ]] || { echo "binary is not executable: $binary" >&2; exit 2; }
[[ -f "$config" ]] || { echo "config not found: $config" >&2; exit 2; }
[[ -f "$db" ]] || { echo "database not found: $db" >&2; exit 2; }
[[ "$samples" =~ ^[1-9][0-9]*$ ]] || { echo "--samples must be positive" >&2; exit 2; }

mkdir -p "$out"
read_only_log="$out/read-only.log"
full_log="$out/full.log"
latency_csv="$out/latency.csv"
process_csv="$out/process.csv"
summary="$out/summary.txt"
read_only_pid=""
full_pid=""
sse_pid=""

cleanup() {
  [[ -z "$sse_pid" ]] || kill "$sse_pid" 2>/dev/null || true
  [[ -z "$read_only_pid" ]] || kill "$read_only_pid" 2>/dev/null || true
  [[ -z "$full_pid" ]] || kill "$full_pid" 2>/dev/null || true
  [[ -z "$read_only_pid" ]] || wait "$read_only_pid" 2>/dev/null || true
  [[ -z "$full_pid" ]] || wait "$full_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

wait_for_dashboard() {
  local url="$1"
  for _ in $(seq 1 100); do
    if curl -fsS -o /dev/null "$url"; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

"$binary" --config "$config" dashboard --listen "127.0.0.1:$read_only_port" --no-open --read-only >"$read_only_log" 2>&1 &
read_only_pid=$!
"$binary" --config "$config" dashboard --listen "127.0.0.1:$full_port" --no-open >"$full_log" 2>&1 &
full_pid=$!
wait_for_dashboard "http://127.0.0.1:$read_only_port/board"
wait_for_dashboard "http://127.0.0.1:$full_port/board"
kill -0 "$read_only_pid"
kill -0 "$full_pid"

echo "phase,sample,http_code,ttfb_seconds,total_seconds" >"$latency_csv"
measure_board() {
  local phase="$1"
  local index
  for index in $(seq 1 "$samples"); do
    curl -sS -o /dev/null -w "$phase,$index,%{http_code},%{time_starttransfer},%{time_total}\n" \
      "http://127.0.0.1:$full_port/board?benchmark=$phase-$index-$(date +%s%N)" >>"$latency_csv"
  done
}

measure_board baseline
curl -N -sS --max-time 20 "http://127.0.0.1:$read_only_port/events" >"$out/sse.out" 2>"$out/sse.err" &
sse_pid=$!
sleep 2
kill -0 "$sse_pid"
measure_board sse

echo "sample,read_only_cpu,read_only_rss_kib,full_cpu,full_rss_kib" >"$process_csv"
for index in $(seq 1 10); do
  read -r ro_cpu ro_rss <<<"$(ps -p "$read_only_pid" -o %cpu=,rss= | xargs)"
  read -r full_cpu full_rss <<<"$(ps -p "$full_pid" -o %cpu=,rss= | xargs)"
  echo "$index,$ro_cpu,$ro_rss,$full_cpu,$full_rss" >>"$process_csv"
  sleep 1
done

# Same URI twice per process demonstrates that snapshots are process-local.
for port in "$read_only_port" "$full_port"; do
  for index in 1 2; do
    curl -sS -o /dev/null -w "cache-$port,$index,%{http_code},%{time_starttransfer},%{time_total}\n" \
      "http://127.0.0.1:$port/board?benchmark=cache-isolation" >>"$latency_csv"
  done
done

baseline_avg=$(awk -F, '$1=="baseline" {sum+=$5; count++} END {if (count) printf "%.6f", sum/count}' "$latency_csv")
sse_avg=$(awk -F, '$1=="sse" {sum+=$5; count++} END {if (count) printf "%.6f", sum/count}' "$latency_csv")
degradation=$(awk -v base="$baseline_avg" -v active="$sse_avg" 'BEGIN {if (base>0) printf "%.2f", ((active-base)/base)*100; else print "0.00"}')

{
  echo "binary=$binary"
  echo "version=$($binary version)"
  echo "config=$config"
  echo "db=$db"
  echo "read_only_pid=$read_only_pid"
  echo "full_pid=$full_pid"
  echo "read_only_port=$read_only_port"
  echo "full_port=$full_port"
  echo "samples=$samples"
  echo "baseline_average_seconds=$baseline_avg"
  echo "sse_average_seconds=$sse_avg"
  echo "sse_degradation_percent=$degradation"
  echo "journal_mode=$(sqlite3 "$db" 'PRAGMA journal_mode;')"
  echo "wal_autocheckpoint=$(sqlite3 "$db" 'PRAGMA wal_autocheckpoint;')"
  echo "integrity_check=$(sqlite3 "$db" 'PRAGMA integrity_check;')"
  echo "db_bytes=$(stat -f %z "$db" 2>/dev/null || stat -c %s "$db")"
  if [[ -f "$db-wal" ]]; then
    echo "wal_bytes=$(stat -f %z "$db-wal" 2>/dev/null || stat -c %s "$db-wal")"
  else
    echo "wal_bytes=0"
  fi
  sqlite3 -separator = "$db" "SELECT 'rows_tasks', COUNT(*) FROM task_definitions UNION ALL SELECT 'rows_evidence', COUNT(*) FROM task_evidence UNION ALL SELECT 'rows_history', COUNT(*) FROM task_state_history UNION ALL SELECT 'rows_checkpoints', COUNT(*) FROM task_checkpoints UNION ALL SELECT 'rows_sessions', COUNT(*) FROM agent_sessions UNION ALL SELECT 'rows_reviews', COUNT(*) FROM task_reviews UNION ALL SELECT 'rows_notifications', COUNT(*) FROM task_notifications;"
} >"$summary"

cat "$summary"
