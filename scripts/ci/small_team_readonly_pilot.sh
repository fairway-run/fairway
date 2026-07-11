#!/usr/bin/env bash
set -euo pipefail

repo_root="${FAIRWAY_SOURCE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
artifacts="${FAIRWAY_PILOT_ARTIFACTS:-$repo_root/.fairway-pilot-artifacts}"
listen="${FAIRWAY_PILOT_LISTEN:-127.0.0.1:17884}"
pilot_root="$(mktemp -d "${TMPDIR:-/tmp}/fairway-small-team-pilot.XXXXXX")"
project="$pilot_root/project"
binary="${FAIRWAY_BIN:-$pilot_root/fairway}"
pid_file="$project/.fairway/server.pid.json"
log_file="$project/.fairway/server.log"
config="$project/.fairway/config.toml"
server_started=false

cleanup() {
  if [[ "$server_started" == true && -x "$binary" ]]; then
    "$binary" --config "$config" server stop \
      --listen "$listen" --pid-file "$pid_file" --log-file "$log_file" \
      >"$artifacts/cleanup-stop.txt" 2>&1 || true
  fi
  rm -rf "$pilot_root"
}
trap cleanup EXIT

rm -rf "$artifacts"
mkdir -p "$artifacts" "$project"

if [[ -z "${FAIRWAY_BIN:-}" ]]; then
  (cd "$repo_root" && go build -o "$binary" ./cmd/fairway)
fi

git -C "$project" init -q
git -C "$project" config user.name "Fairway Pilot"
git -C "$project" config user.email "pilot@example.invalid"

(cd "$project" && "$binary" init) >"$artifacts/init.txt"
(cd "$project" && "$binary" import \
  "$repo_root/docs/roadmap/fairway-product-backlog.yaml") \
  >"$artifacts/import.txt"
git -C "$project" add .fairway/AGENTS.md
git -C "$project" commit -qm "chore: initialize disposable Fairway pilot"

git -C "$repo_root" rev-parse HEAD >"$artifacts/source-sha.txt"
"$binary" version >"$artifacts/version.txt"
shasum -a 256 "$binary" >"$artifacts/binary-sha256.txt"
"$binary" --config "$config" config validate >"$artifacts/config-validate.txt"
doctor_status=0
"$binary" --config "$config" doctor --format json \
  --dashboard-read-only "" --dashboard-full "" \
  >"$artifacts/doctor.json" || doctor_status=$?
printf '%s\n' "$doctor_status" >"$artifacts/doctor-exit-status.txt"
"$binary" --config "$config" reconcile active --dry-run \
  >"$artifacts/reconcile-before.txt"
"$binary" --config "$config" delivery report --since 168h --format json \
  >"$artifacts/delivery-before.json"

mkdir -p "$artifacts/backup"
"$binary" --config "$config" db backup "$artifacts/backup/state.db" \
  >"$artifacts/backup/backup.txt"
"$binary" --config "$config" db export "$artifacts/backup/export.json" \
  >"$artifacts/backup/export.txt"
cp "$config" "$artifacts/backup/config.toml"
"$binary" --config "$artifacts/backup/config.toml" \
  --db "$artifacts/backup/state.db" ready >"$artifacts/restore-ready.txt"
"$binary" --config "$artifacts/backup/config.toml" \
  --db "$artifacts/backup/state.db" reconcile active --dry-run \
  >"$artifacts/restore-reconcile.txt"

"$binary" --config "$config" server start --read-only \
  --listen "$listen" --pid-file "$pid_file" --log-file "$log_file" \
  >"$artifacts/server-start.txt"
server_started=true

"$binary" --json --config "$config" server status \
  --listen "$listen" --pid-file "$pid_file" --log-file "$log_file" \
  >"$artifacts/server-status.json"
curl -fsS -w '\nHTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$listen/api/v1/status" | tee "$artifacts/api-status.txt" >/dev/null
curl -fsS -o "$artifacts/api-tasks.json" \
  -w 'HTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$listen/api/v1/tasks" >"$artifacts/api-tasks-timing.txt"
curl -fsS -o "$artifacts/api-task-fw-284.json" \
  -w 'HTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$listen/api/v1/tasks/FW-284" >"$artifacts/api-task-fw-284-timing.txt"
curl -fsS -o "$artifacts/api-reports-summary.json" \
  -w 'HTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$listen/api/v1/reports/summary" \
  >"$artifacts/api-reports-summary-timing.txt"
"$binary" --json --config "$config" review-waits list --task FW-284 \
  >"$artifacts/review-waits.json"
"$binary" --config "$config" task-detail FW-284 \
  >"$artifacts/task-detail.txt"
"$binary" --config "$config" server logs --tail 80 \
  --listen "$listen" --pid-file "$pid_file" --log-file "$log_file" \
  >"$artifacts/server-log-tail.txt"

grep -q '"mode":"read_only"' "$artifacts/api-status.txt"
grep -q '"read_only":true' "$artifacts/api-status.txt"
grep -q '"writes_enabled":false' "$artifacts/api-status.txt"

"$binary" --config "$config" server stop \
  --listen "$listen" --pid-file "$pid_file" --log-file "$log_file" \
  >"$artifacts/server-stop.txt"
server_started=false
"$binary" --json --config "$config" server status \
  --listen "$listen" --pid-file "$pid_file" --log-file "$log_file" \
  >"$artifacts/server-status-after-stop.json"
test ! -e "$pid_file"

"$binary" --config "$config" delivery report --since 168h --format json \
  >"$artifacts/delivery-after.json"
"$binary" --config "$config" reconcile active --dry-run \
  >"$artifacts/reconcile-after.txt"

printf 'promote_read_only\n' >"$artifacts/recommendation.txt"
printf 'small-team read-only pilot passed; artifacts=%s\n' "$artifacts"
