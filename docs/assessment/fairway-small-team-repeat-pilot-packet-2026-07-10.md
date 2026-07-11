# Fairway Small-Team Repeat Pilot Packet

Date prepared: 2026-07-10

Task: `FW-284`

Required lifecycle commit: `192fda9`

## Purpose

This packet is for a non-authoring operator to repeat the small-team Fairway
read-only pilot on the Mac mini GitLab lab or an equivalent isolated host. The
operator must not be the FW-283 implementation provider or the Architecture
Control operator who ran the first FW-275 pilot.

The run proves operator usability and bounded lifecycle behavior. It does not
authorize shared writes, trusted-proxy runtime promotion, non-loopback binding,
public exposure, provider sends, approvals, merge, deploy, release, Homebrew,
Postgres runtime cutover, or live operations.

## Operator Inputs

Set these paths for the selected host and project:

```bash
export FAIRWAY_SOURCE="$HOME/dev/fairway"
export FAIRWAY_PROJECT="$HOME/dev/fairway"
export FAIRWAY_REVIEWED_SHA="192fda99cd9eb8c415e09ae83382f3ab2af8e13b"
export FAIRWAY_CONFIG="$FAIRWAY_PROJECT/.fairway/config.toml"
export FAIRWAY_LAB_HOME="$HOME/fairway-lab"
export FAIRWAY_BUILD_SOURCE="$FAIRWAY_LAB_HOME/source-$FAIRWAY_REVIEWED_SHA"
export FAIRWAY_BIN="$FAIRWAY_LAB_HOME/bin/fairway-fw284"
export FAIRWAY_STATE="$FAIRWAY_PROJECT/.fairway"
export FAIRWAY_PILOT_ID="fw-284-$(date -u +%Y%m%dT%H%M%SZ)"
export FAIRWAY_ARTIFACTS="$FAIRWAY_STATE/artifacts/$FAIRWAY_PILOT_ID"
export FAIRWAY_LISTEN="127.0.0.1:17884"
export FAIRWAY_PID_FILE="$FAIRWAY_STATE/fairway-server-17884.pid.json"
export FAIRWAY_LOG_FILE="$FAIRWAY_STATE/fairway-server-17884.log"
mkdir -p "$FAIRWAY_LAB_HOME/bin" "$FAIRWAY_ARTIFACTS"
```

Record the operator name, host, UTC start time, and why the operator is
independent of the first pilot in `$FAIRWAY_ARTIFACTS/operator.txt`. Do not
record credentials, tokens, cookies, prompt bodies, transcripts, or raw tool
payloads.

## Preflight And Build

```bash
git -C "$FAIRWAY_SOURCE" fetch https://github.com/fairway-run/fairway.git main
git -C "$FAIRWAY_SOURCE" cat-file -e "$FAIRWAY_REVIEWED_SHA^{commit}"
if [ -e "$FAIRWAY_BUILD_SOURCE" ]; then
  echo "reviewed build source already exists: $FAIRWAY_BUILD_SOURCE" >&2
  exit 1
fi
git -C "$FAIRWAY_SOURCE" worktree add --detach \
  "$FAIRWAY_BUILD_SOURCE" "$FAIRWAY_REVIEWED_SHA"
git -C "$FAIRWAY_BUILD_SOURCE" rev-parse HEAD \
  | tee "$FAIRWAY_ARTIFACTS/source-sha.txt"
test "$(git -C "$FAIRWAY_BUILD_SOURCE" rev-parse HEAD)" = "$FAIRWAY_REVIEWED_SHA"
git -C "$FAIRWAY_BUILD_SOURCE" status --porcelain \
  | tee "$FAIRWAY_ARTIFACTS/source-status.txt"
test ! -s "$FAIRWAY_ARTIFACTS/source-status.txt"

cd "$FAIRWAY_BUILD_SOURCE"
GOCACHE=/tmp/fairway-go-cache go build -o "$FAIRWAY_BIN" ./cmd/fairway
"$FAIRWAY_BIN" version | tee "$FAIRWAY_ARTIFACTS/version.txt"
shasum -a 256 "$FAIRWAY_BIN" | tee "$FAIRWAY_ARTIFACTS/binary-sha256.txt"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" config validate \
  | tee "$FAIRWAY_ARTIFACTS/config-validate.txt"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" doctor --format json \
  > "$FAIRWAY_ARTIFACTS/doctor.json"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" reconcile active --dry-run \
  | tee "$FAIRWAY_ARTIFACTS/reconcile-before.txt"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" delivery report --since 168h --format json \
  > "$FAIRWAY_ARTIFACTS/delivery-before.json"
```

Stop if the source SHA differs, config validation fails, reconciliation reports
an unapproved finding, the selected address is already occupied, or the binary
path is not operator-owned.

## Backup And Restore Proof

```bash
mkdir -p "$FAIRWAY_ARTIFACTS/backup"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" db backup \
  "$FAIRWAY_ARTIFACTS/backup/state.db"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" db export \
  "$FAIRWAY_ARTIFACTS/backup/export.json"
cp "$FAIRWAY_CONFIG" "$FAIRWAY_ARTIFACTS/backup/config.toml"

"$FAIRWAY_BIN" --config "$FAIRWAY_ARTIFACTS/backup/config.toml" \
  --db "$FAIRWAY_ARTIFACTS/backup/state.db" ready \
  | tee "$FAIRWAY_ARTIFACTS/restore-ready.txt"
"$FAIRWAY_BIN" --config "$FAIRWAY_ARTIFACTS/backup/config.toml" \
  --db "$FAIRWAY_ARTIFACTS/backup/state.db" reconcile active --dry-run \
  | tee "$FAIRWAY_ARTIFACTS/restore-reconcile.txt"
```

The backup is local pilot evidence, not a production retention system.

## Managed Start And Readback

```bash
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" server start \
  --read-only \
  --listen "$FAIRWAY_LISTEN" \
  --pid-file "$FAIRWAY_PID_FILE" \
  --log-file "$FAIRWAY_LOG_FILE" \
  | tee "$FAIRWAY_ARTIFACTS/server-start.txt"

"$FAIRWAY_BIN" --json --config "$FAIRWAY_CONFIG" server status \
  --listen "$FAIRWAY_LISTEN" \
  --pid-file "$FAIRWAY_PID_FILE" \
  --log-file "$FAIRWAY_LOG_FILE" \
  > "$FAIRWAY_ARTIFACTS/server-status.json"

curl -fsS -w '\nHTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$FAIRWAY_LISTEN/api/v1/status" \
  | tee "$FAIRWAY_ARTIFACTS/api-status.txt"
curl -fsS -o "$FAIRWAY_ARTIFACTS/api-tasks.json" \
  -w 'HTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$FAIRWAY_LISTEN/api/v1/tasks" \
  | tee "$FAIRWAY_ARTIFACTS/api-tasks-timing.txt"
curl -fsS -o "$FAIRWAY_ARTIFACTS/api-task-fw-284.json" \
  -w 'HTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$FAIRWAY_LISTEN/api/v1/tasks/FW-284" \
  | tee "$FAIRWAY_ARTIFACTS/api-task-fw-284-timing.txt"
curl -fsS -o "$FAIRWAY_ARTIFACTS/api-reports-summary.json" \
  -w 'HTTP %{http_code} TTFB %{time_starttransfer}s TOTAL %{time_total}s\n' \
  "http://$FAIRWAY_LISTEN/api/v1/reports/summary" \
  | tee "$FAIRWAY_ARTIFACTS/api-reports-summary-timing.txt"

"$FAIRWAY_BIN" --json --config "$FAIRWAY_CONFIG" review-waits list --task FW-284 \
  > "$FAIRWAY_ARTIFACTS/review-waits.json"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" task-detail FW-284 \
  > "$FAIRWAY_ARTIFACTS/task-detail.txt"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" server logs --tail 80 \
  --listen "$FAIRWAY_LISTEN" \
  --pid-file "$FAIRWAY_PID_FILE" \
  --log-file "$FAIRWAY_LOG_FILE" \
  > "$FAIRWAY_ARTIFACTS/server-log-tail.txt"
```

The status response must say `mode=read_only`, `read_only=true`, and
`writes_enabled=false`. The operator must not invoke POST endpoints.

## Stop And Cleanup Readback

```bash
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" server stop \
  --listen "$FAIRWAY_LISTEN" \
  --pid-file "$FAIRWAY_PID_FILE" \
  --log-file "$FAIRWAY_LOG_FILE" \
  | tee "$FAIRWAY_ARTIFACTS/server-stop.txt"

"$FAIRWAY_BIN" --json --config "$FAIRWAY_CONFIG" server status \
  --listen "$FAIRWAY_LISTEN" \
  --pid-file "$FAIRWAY_PID_FILE" \
  --log-file "$FAIRWAY_LOG_FILE" \
  > "$FAIRWAY_ARTIFACTS/server-status-after-stop.json"

test ! -e "$FAIRWAY_PID_FILE"
! lsof -nP -iTCP@"$FAIRWAY_LISTEN" -sTCP:LISTEN

"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" delivery report --since 168h --format json \
  > "$FAIRWAY_ARTIFACTS/delivery-after.json"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" reconcile active --dry-run \
  | tee "$FAIRWAY_ARTIFACTS/reconcile-after.txt"

git -C "$FAIRWAY_SOURCE" worktree remove "$FAIRWAY_BUILD_SOURCE"
test ! -e "$FAIRWAY_BUILD_SOURCE"
```

If any step fails, stop the managed server with the same pid record, preserve
the bounded artifact directory, remove the detached build worktree only after
confirming no process uses its binary, and report the exact failed command.
Never delete or signal a process that Fairway reports as `unknown` or
`mismatch`.

## Operator Handback

The non-authoring operator must provide:

1. operator and host identity without credentials or private payloads;
2. artifact directory and exact source/binary readback;
3. pass/fail for build, config, doctor, backup/restore, managed lifecycle, API
   status/tasks/task-detail/reports, wait/evidence readback, and cleanup;
4. observed startup and endpoint timings;
5. rough edges with owner, severity, and fix-now/defer decision;
6. one recommendation: `promote_read_only`, `repeat_with_fixes`, or `block`.

The implementation track will record that handback as FW-284 evidence. Review
and release preparation happen only after the independent handback is complete.
