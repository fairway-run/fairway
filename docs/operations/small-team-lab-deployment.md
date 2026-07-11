# Small-Team Lab Deployment

This runbook packages a local small-team Fairway deployment for a Mac mini,
GitLab lab host, or equivalent single internal control-room machine. It is an
operator procedure for read-only dashboard/server visibility and local CLI
coordination. It does not authorize release, public exposure, production
migration, dashboard writes, provider sends, approvals, merges, deploys, or
live operations.

Use this runbook when a small team needs one durable Fairway host with explicit
paths, status readback, logs, backups, and rollback evidence.

Before configuring a durable host, run the disposable lifecycle rehearsal from
the Fairway source tree:

```bash
bash scripts/ci/small_team_readonly_pilot.sh
```

The harness creates a temporary Git repository and Fairway state, imports the
product backlog, validates config and diagnostics, proves backup/restore,
starts the managed loopback read-only API, exercises status/tasks/task detail/
reports/waits readback, verifies writes remain disabled, stops the process, and
records bounded artifacts under `.fairway-pilot-artifacts`. CI runs the same
harness on an independent runner and publishes the artifact packet.

## Boundary

Supported in this runbook:

- local Fairway binary installation or release binary placement;
- repo-local or operator-owned config and data directories;
- read-only dashboard or read-only server/API on loopback;
- optional trusted proxy/tunnel in front of a loopback read-only origin;
- pid/log/status/version readback;
- backup and restore rehearsal;
- smoke checks for wall, board, reports, and read-only API endpoints.

Not supported here:

- root-owned application data under `/`;
- unauthenticated non-loopback Fairway origin;
- shared write API promotion;
- Postgres runtime switch;
- public dashboard exposure changes;
- provider prompt sending, approval, merge, deploy, release, or live-operation
  authority from the dashboard/server.

## Host Layout

Create an operator-owned directory tree. Do not store app data directly under
the root filesystem and do not rely on the launch shell's current directory.

```bash
export FAIRWAY_HOME="$HOME/fairway-lab"
export FAIRWAY_BIN="$FAIRWAY_HOME/bin/fairway"
export FAIRWAY_REPO="$HOME/dev/fairway"
export FAIRWAY_PROJECT="$HOME/dev/your-repo"
export FAIRWAY_CONFIG="$FAIRWAY_PROJECT/.fairway/config.toml"
export FAIRWAY_STATE="$FAIRWAY_PROJECT/.fairway"
export FAIRWAY_LOG_DIR="$FAIRWAY_HOME/logs"
export FAIRWAY_BACKUP_DIR="$FAIRWAY_HOME/backups"

mkdir -p "$FAIRWAY_HOME/bin" "$FAIRWAY_LOG_DIR" "$FAIRWAY_BACKUP_DIR"
```

For a release binary, place the exact binary under `$FAIRWAY_HOME/bin` and
record its version and checksum. For source builds, build outside the consumer
repo and copy the binary into the lab path:

```bash
cd "$FAIRWAY_REPO"
go build -o "$FAIRWAY_BIN" ./cmd/fairway
"$FAIRWAY_BIN" version
shasum -a 256 "$FAIRWAY_BIN"
```

Record the binary path, version, source commit, and checksum as Fairway evidence
before using the host for team coordination.

## Config

Keep the Fairway DB path under the project `.fairway` directory or another
operator-owned path. Avoid absolute paths owned by root or a transient launch
agent.

```toml
[fairway]
project_name = "lab-project"
db_path = ".fairway/state.db"
main_branch = "main"

[dashboard]
listen = "127.0.0.1:7878"
read_only = true
auto_open = false

[server]
enabled = true
mode = "read_only"
listen = "127.0.0.1:7880"
read_only = true
write_enabled = false
identity_mode = "no_edge_local"
allowed_roles = ["viewer"]
```

Run config validation from the consumer repo:

```bash
cd "$FAIRWAY_PROJECT"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" config validate
```

Secrets are not required for loopback read-only mode. If a proxy, API token, or
future notifier is used, store secret material in the operating environment or
secret manager, not in `.fairway/config.toml`, task evidence, logs, or docs.

## Start Read-Only Dashboard

Use explicit pid and log files for each process. This makes lifecycle readback
independent of the launch terminal.

```bash
cd "$FAIRWAY_PROJECT"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" dashboard restart \
  --listen 127.0.0.1:7878 \
  --read-only \
  --pid-file "$FAIRWAY_STATE/fairway-dashboard-7878.pid" \
  --log-file "$FAIRWAY_STATE/fairway-dashboard-7878.log" \
  --no-open

"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" dashboard status \
  --listen 127.0.0.1:7878 \
  --read-only \
  --pid-file "$FAIRWAY_STATE/fairway-dashboard-7878.pid" \
  --log-file "$FAIRWAY_STATE/fairway-dashboard-7878.log"
```

If a local full-access dashboard is needed for the operator on the same host,
bind it to a separate loopback port and separate pid/log files:

```bash
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" dashboard restart \
  --listen 127.0.0.1:7879 \
  --pid-file "$FAIRWAY_STATE/fairway-dashboard-7879.pid" \
  --log-file "$FAIRWAY_STATE/fairway-dashboard-7879.log" \
  --no-open
```

Do not expose the full-access dashboard through a tunnel or shared proxy.

## Start Read-Only Server/API

FW-269/FW-270 read-only server mode is loopback-only unless a later reviewed
deployment task authorizes a different network boundary.

```bash
cd "$FAIRWAY_PROJECT"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" server start \
  --read-only \
  --listen 127.0.0.1:7880 \
  --pid-file "$FAIRWAY_STATE/fairway-server-7880.pid.json" \
  --log-file "$FAIRWAY_STATE/fairway-server-7880.log"

"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" server status \
  --listen 127.0.0.1:7880 \
  --pid-file "$FAIRWAY_STATE/fairway-server-7880.pid.json" \
  --log-file "$FAIRWAY_STATE/fairway-server-7880.log"
```

The managed lifecycle detaches the server, writes explicit version/config/DB
readback, and refuses to stop or restart a PID unless the live process matches
the lifecycle record's binary, read-only command shape, and per-launch identity
token. Use `server logs --tail <n>` for bounded local log readback and `server
restart --read-only` after a reviewed binary or config replacement. The server
must not be bound to `0.0.0.0`, a LAN address, a Tailscale address, or a public
interface by this runbook.

## Optional Proxy Boundary

For shared read-only dashboard viewing, put Cloudflare Access, Pomerium,
Tailscale Funnel, VPN, SSH tunnel, or another identity-aware proxy in front of
the loopback origin. The proxy is the shared-access boundary; Fairway dashboard
still remains read-only.

Minimum proxy evidence:

- proxy hostname and policy owner;
- origin target, expected loopback port, and firewall posture;
- identity policy or access group;
- public probe status, such as HTTP 302 to identity provider or access-denied
  response when unauthenticated;
- confirmation that the full-access dashboard port is not proxied.

Trusted proxy identity is advisory unless a later verifier task implements and
enables runtime verification.

## Smoke Checks

Run local smoke checks after every start, restart, binary replacement, or host
reboot:

```bash
"$FAIRWAY_BIN" version
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" config validate
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" ready
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" reconcile active --dry-run
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" dashboard status \
  --listen 127.0.0.1:7878 \
  --read-only \
  --pid-file "$FAIRWAY_STATE/fairway-dashboard-7878.pid"

curl -fsS -o /dev/null -w 'wall %{http_code} %{time_starttransfer}\n' http://127.0.0.1:7878/
curl -fsS -o /dev/null -w 'board %{http_code} %{time_starttransfer}\n' http://127.0.0.1:7878/board
curl -fsS -o /dev/null -w 'reports %{http_code} %{time_starttransfer}\n' http://127.0.0.1:7878/reports
curl -fsS http://127.0.0.1:7880/api/v1/status
curl -fsS http://127.0.0.1:7880/api/v1/tasks
```

If a proxy is configured, also record the unauthenticated public boundary probe.
For Cloudflare Access this is commonly a `302` to the Access login flow or an
access-denied response, depending on policy.

## Backup And Restore

Take a backup before binary replacement, config changes, shared-team pilots, or
host maintenance:

```bash
backup_id="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$FAIRWAY_BACKUP_DIR/$backup_id"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" db backup "$FAIRWAY_BACKUP_DIR/$backup_id/state.db"
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" db export "$FAIRWAY_BACKUP_DIR/$backup_id/export.json"
cp "$FAIRWAY_CONFIG" "$FAIRWAY_BACKUP_DIR/$backup_id/config.toml"
"$FAIRWAY_BIN" version > "$FAIRWAY_BACKUP_DIR/$backup_id/version.txt"
git -C "$FAIRWAY_PROJECT" rev-parse HEAD > "$FAIRWAY_BACKUP_DIR/$backup_id/project-head.txt"
```

Restore rehearsal:

```bash
restore_dir="$(mktemp -d /tmp/fairway-restore.XXXXXX)"
cp "$FAIRWAY_BACKUP_DIR/$backup_id/state.db" "$restore_dir/state.db"
cp "$FAIRWAY_CONFIG" "$restore_dir/config.toml"
"$FAIRWAY_BIN" --config "$restore_dir/config.toml" --db "$restore_dir/state.db" config validate
"$FAIRWAY_BIN" --config "$restore_dir/config.toml" --db "$restore_dir/state.db" ready
"$FAIRWAY_BIN" --config "$restore_dir/config.toml" --db "$restore_dir/state.db" reconcile active --dry-run
```

Record the backup path, restore directory, command output, and cleanup action as
evidence. Delete restore scratch space after evidence capture if it contains
local task metadata that should not persist.

## Stop And Rollback

Stop dashboard processes with their explicit pid files:

```bash
"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" dashboard stop \
  --listen 127.0.0.1:7878 \
  --pid-file "$FAIRWAY_STATE/fairway-dashboard-7878.pid" \
  --log-file "$FAIRWAY_STATE/fairway-dashboard-7878.log"

"$FAIRWAY_BIN" --config "$FAIRWAY_CONFIG" server stop \
  --listen 127.0.0.1:7880 \
  --pid-file "$FAIRWAY_STATE/fairway-server-7880.pid.json" \
  --log-file "$FAIRWAY_STATE/fairway-server-7880.log"
```

Rollback after a binary/config change:

1. stop the affected dashboard/server process;
2. restore the previous binary path or config file;
3. if state changed, restore the latest reviewed DB backup;
4. start the process with explicit pid/log files;
5. rerun smoke checks;
6. record evidence with old/new version, backup id, status output, and any
   residual blocker.

Rollback does not automatically merge divergent writes. If multiple write
surfaces were active, create a separate reconciliation task.

## Evidence Checklist

Record the following for each lab deployment or restart:

- host class and owner, for example `Mac mini GitLab lab`;
- Fairway binary path, version, source SHA, and checksum;
- Fairway config path and DB path;
- dashboard/server listen addresses and read-only/write mode;
- pid/log files and status readback;
- smoke command output for wall, board, reports, API status, and API tasks;
- proxy/no-edge boundary probe;
- backup id and restore rehearsal result;
- cleanup or rollback action.

## Known Limits

- This runbook packages local/single-host read-only operation. It does not make
  shared write mode generally supported.
- It does not replace the FW-259 shared-team operations model or the future
  Postgres/server-backed implementation review gates.
- It does not authorize a release or restart shared production dashboards.
- It does not provide a public endpoint hardening checklist for write-capable
  APIs.
