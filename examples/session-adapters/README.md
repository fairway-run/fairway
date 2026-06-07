# Session Adapter Examples

Fairway stores session visibility in its DB, but provider/process launch remains
outside the core queue. These examples show how a repo can wrap tmux, zellij,
Codex, Claude, Gemini, or a plain shell and then register the session with
`fairway session upsert`.

They are intentionally small reference scripts. Copy the shape into your own
repo and tune provider flags, prompt text, and transcript paths locally.

## Expected Contract

A launcher should:

1. choose a configured role and worktree,
2. start the provider process or terminal pane,
3. call `fairway session upsert` with `--id`, `--role`, `--branch`,
   `--worktree`, `--backend`, `--provider`, `--task-id`, `--status running`,
   and the best available process or pane metadata,
4. capture or tee transcript output to a file and pass that path with
   `--transcript`,
5. record an initial checkpoint when the session is associated with a task,
6. call `fairway session end` when the process exits, if the launcher owns the
   process lifecycle.

Core Fairway commands do not require these launchers. They only make session
visibility and stale-session reconciliation more useful.

For dashboard wall accuracy, `in_progress` task state is not enough. The
launcher or watcher must keep the session row associated with the current task
and emit lifecycle checkpoints or provider events at start, wait/stale/failure,
and completion.

## tmux transcript bridge

`tmux.sh` is provider-neutral. It starts a tmux pane, pipes pane output to a
transcript file, registers the session, and records an initial checkpoint when a
task ID is provided.

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_INITIAL_PROMPT="$(cat prompts/platform-map.md)" \
examples/session-adapters/tmux.sh orchestrator PF-001
```

For GPUaaS-style Claude lanes, use the Fairway role as the coordination owner
and provider as an informational label:

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_TRANSCRIPT=".fairway/transcripts/gpuaas-D-arch-PF-001.log" \
examples/session-adapters/tmux.sh D-arch PF-001
```

Reconcile stale panes with:

```bash
fairway session reconcile --dry-run
fairway session reconcile
```

## Provider runtime event adapter

`provider-event.sh` is a provider-neutral watcher adapter convention for
delegated sessions that are not directly visible as a local process. A Codex,
Claude, Gemini, or shell-specific monitor can call it whenever the external
runtime state changes.

It always refreshes the Fairway session row, then maps provider state into
Fairway facts:

| Runtime state | Fairway action |
|---|---|
| `started` | record an `active` checkpoint |
| `running` | upsert session metadata only |
| `waiting_on_approval` | record an `awaiting_input` checkpoint |
| `waiting_on_input` | record an `awaiting_input` checkpoint |
| `completed` | record a `done` checkpoint plus evidence, or a handoff with `--handoff-to` |
| `failed` | record an `awaiting_input` checkpoint with the failure summary |
| `stale` / `no_progress` | mark the session stale and record an `awaiting_input` checkpoint |

Active delegated sessions should emit provider events at start, whenever they
wait/block/stale, and on completion. `running` is only a metadata refresh; it is
not enough to make the delegated session visible as active work.

Example delegated Codex-style thread:

```bash
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id thread-abc123 \
  --role backend \
  --task-id FW-109 \
  --state started \
  --summary "delegated Codex thread started" \
  --transcript .fairway/transcripts/codex-thread-abc123.log
```

Example approval wait:

```bash
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id thread-abc123 \
  --role backend \
  --task-id FW-109 \
  --state waiting_on_approval \
  --summary "approval needed for go test ./..." \
  --transcript .fairway/transcripts/codex-thread-abc123.log
```

Example completion that records evidence without changing task status:

```bash
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id thread-abc123 \
  --role backend \
  --task-id FW-109 \
  --state completed \
  --summary "provider watcher adapter implemented" \
  --artifact dist/provider-event-smoke.log
```

Task ownership, terminal status gates, review gates, and merge readiness remain
Fairway responsibilities. Provider watchers should only feed sessions,
checkpoints, evidence, and handoffs.

## CI/deploy/UAT monitor utility

`ci-monitor.sh` is a provider-neutral watcher utility for external CI, deploy,
smoke, UAT, docs, or ops runs. It does not call GitHub, GitLab, Plane, Codex,
Claude, or any provider API directly. A project-specific wrapper supplies the
poll command and external run id; the utility translates the result into
Fairway session proof, watcher lifecycle, checkpoints, evidence, and a
coordinator handback.

Dry-run a monitor before running it:

```bash
examples/session-adapters/ci-monitor.sh \
  --task-id T-001 \
  --batch-id BATCH-001 \
  --monitor-kind ci \
  --external-run-id gha-123 \
  --poll-command "gh run view gha-123 --json conclusion --jq .conclusion" \
  --source-sha "$(git rev-parse HEAD)" \
  --manual-until 2026-06-07 \
  --artifact https://github.com/org/repo/actions/runs/123 \
  --dry-run
```

Run it as a utility process after checking the generated commands:

```bash
examples/session-adapters/ci-monitor.sh \
  --task-id T-001 \
  --batch-id BATCH-001 \
  --monitor-kind ci \
  --external-run-id gha-123 \
  --poll-command "gh run view gha-123 --json conclusion --jq .conclusion" \
  --success-regex "success" \
  --failure-regex "failure|cancelled|timed_out" \
  --interval-seconds 30 \
  --timeout-seconds 1800 \
  --artifact https://github.com/org/repo/actions/runs/123
```

For deterministic wrappers and tests, pass `--result pass|fail|timeout|stale`.
On failure or timeout the adapter records failed/blocked evidence and prints a
recommended follow-up prefix: `CI-FIX`, `CD-FIX`, `UAT-BUG`, `OPS-FIX`,
`HARNESS-FIX`, or `DOC-FIX`, depending on `--monitor-kind`. It recommends by
default; it does not create tasks automatically.

After closing the watcher and session, the adapter runs
`fairway reconcile active --dry-run` as the reusable handback. If ready work
remains, that report surfaces `monitor_completion_resume_needed`.

## Provider Usage Adapters

`provider-otel-ingest.sh` is the provider-neutral usage bridge. It accepts OTLP
JSON logs, metrics, or traces from stdin or `--input`, extracts only structural
usage metadata, and emits `fairway record usage`.

```bash
examples/session-adapters/provider-otel-ingest.sh \
  --input dist/provider-otel.json \
  --task-id FW-125 \
  --role backend \
  --dry-run
```

When a project needs a non-default Fairway config, set `FAIRWAY_BIN` to an
executable wrapper path, not to a shell command with embedded arguments:

```bash
tmpdir=$(mktemp -d /tmp/fairway-wrapper.XXXXXX)
cat > "$tmpdir/fairway" <<'EOF'
#!/usr/bin/env bash
cd /path/to/fairway
exec go run ./cmd/fairway --config /path/to/project/.fairway/config.toml "$@"
EOF
chmod +x "$tmpdir/fairway"

FAIRWAY_BIN="$tmpdir/fairway" examples/session-adapters/provider-otel-ingest.sh \
  --input dist/provider-otel.json \
  --task-id FW-125
```

`codex-usage-adapter.sh` is the Codex-specific wrapper. It supports Codex-shaped
OTel JSON, `codex exec --json` / newline-delimited JSON with
`turn.completed.usage`, and caller-supplied token snapshots:

```bash
examples/session-adapters/codex-usage-adapter.sh \
  --mode exec-json \
  --input dist/codex-exec.jsonl \
  --task-id FW-124 \
  --session-id codex-fw-124 \
  --role backend \
  --dry-run
```

The Codex adapter does not read private Codex SQLite files, auth files,
transcripts, prompts, logs, or generated content. Remove `--dry-run` only after
confirming the generated command records counts and safe metadata only.
