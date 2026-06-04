# Agent Guide

Fairway is built for coding agents working in parallel. This guide is the
operator-facing contract for an agent that is already inside a repo with
Fairway configured.

## First Rule

The Fairway DB is the execution source of truth. Do not edit queue state files,
SQLite rows, or generated dashboard artifacts directly. Use `fairway` commands
so claims, evidence, handoffs, reviews, sessions, checkpoints, and audit history
stay consistent.

## Start Of Session

```bash
fairway config validate
fairway preflight --role <role>
fairway session upsert --role <role> --provider <codex|claude|gemini|shell>
fairway ready
```

For tmux-backed lanes, especially provider sessions that cannot be inspected by
the host application directly, register enough metadata for coordination:

```bash
fairway session upsert \
  --id <session-id> \
  --role <role> \
  --provider <codex|claude|gemini|shell> \
  --backend tmux \
  --name <tmux-session-name> \
  --tmux-pane <session:window.pane> \
  --transcript <path-to-transcript> \
  --task-id <task-id> \
  --pid <pid> \
  --worktree <path> \
  --branch <branch>
fairway checkpoint record <task-id> \
  --state active \
  --owner <role> \
  --summary "Started tmux-backed provider lane; transcript: <path-to-transcript>"
```

The example tmux adapter performs the same registration, transcript capture,
and initial checkpoint in one provider-neutral command:

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_TRANSCRIPT=".fairway/transcripts/claude-<role>-<task-id>.log" \
examples/session-adapters/tmux.sh <role> <task-id>
```

Fairway coordination should work through task state, evidence, handoffs,
checkpoints, and session records. Provider-specific chat history is useful, but
it is not the coordination source of truth.

Durable lane, replaceable provider attachment: a Fairway lane or track is the
durable coordination identity. Provider sessions are replaceable execution
attachments. A long-lived provider session may carry useful working memory, but
the lane can move between Codex, Claude, Gemini, tmux, or shell without changing
task identity, ownership, checkpoints, evidence, reviews, or merge gates.

## Delegated Provider Sessions

When one agent delegates work to another provider session, the delegating agent
must keep the coordination loop explicit. Starting or steering a child session
is not enough; the parent needs a watcher or heartbeat that notices when the
child needs attention.

Provider-specific watchers, such as a Codex thread monitor, should live outside
Fairway core. Their job is to read provider runtime state and translate it into
provider-neutral Fairway facts:

- `waiting_on_approval` or `waiting_on_input` becomes a checkpoint with
  `state=awaiting_input` and a summary of the requested action.
- `completed` becomes evidence or a handoff, then the owning task can move
  through normal Fairway gates.
- `failed` becomes a blocked checkpoint or task status with the failure reason.
- stale/no-progress sessions become stale session records or checkpoints, not
  silent background work.

For Codex-style delegated threads, the operating pattern is:

```bash
# 1. Register the delegated session.
fairway session upsert \
  --id <codex-thread-id> \
  --role <role> \
  --provider codex \
  --backend codex-thread \
  --task-id <task-id> \
  --worktree <path> \
  --branch <branch>

# 2. Run a provider adapter or heartbeat outside Fairway core. A provider
# monitor can call the generic event adapter whenever runtime state changes.
examples/session-adapters/provider-event.sh \
  --provider codex \
  --backend codex-thread \
  --external-session-id <codex-thread-id> \
  --role <role> \
  --task-id <task-id> \
  --state waiting_on_approval \
  --summary "<short reason>" \
  --transcript <path-to-transcript>

# 3. If no adapter is available, record the equivalent Fairway fact manually.
fairway checkpoint record <task-id> \
  --state awaiting_input \
  --owner <role> \
  --summary "Delegated Codex thread is waiting on approval: <short reason>"
```

The generic event adapter supports `running`, `waiting_on_approval`,
`waiting_on_input`, `completed`, `failed`, `stale`, and `no_progress`. It
refreshes the session record first, then records the mapped checkpoint,
evidence, handoff, or stale-session event.

Do not make Fairway depend on Codex, Claude, Gemini, or any provider API.
Fairway should expose the session/checkpoint/evidence surfaces; provider
watchers should feed those surfaces.

If you need machine-readable output, put global flags before the subcommand:

```bash
fairway --json ready
fairway --json task-detail T-001
```

Use `FAIRWAY_ROLE=<role>` or `--as <role>` when the current worktree cannot be
resolved to one configured role.

If your repo uses workstream profiles, read the configured profile before
claiming profile-shaped work:

```bash
fairway config validate
fairway adoption artifact --limit 5 --gap-limit 5
```

The adoption artifact shows configured gate modes, named profile gates, route
samples, and evidence-backed gate evaluation. `fairway merge-ready` also checks
the profile gates for the target task: missing `blocking` gates fail readiness,
while missing `advisory` and `report_only` gates appear as warnings. Treat
advisory gates as evidence expectations, not as optional background noise.

Profile-shaped work should carry task metadata when the coordinator or dashboard
needs architecture context:

```bash
fairway add T-010 \
  --title "Map platform evidence ownership" \
  --role arch \
  --kind architecture-map \
  --profile platform-foundation \
  --owning-domain platform \
  --owning-layer service \
  --source-paths cmd/api,packages/services \
  --review-domains architecture,backend \
  --risk-level medium \
  --migration-type ownership-map
```

When you spawn follow-up work, Fairway inherits the parent task metadata unless
you override a metadata flag explicitly. Keep those fields accurate; they drive
review routing, readiness reports, and dashboard workstream grouping.

## Claim Work

```bash
fairway claim T-001
fairway task-detail T-001
```

Claiming moves a ready task to `in_progress` and records the owner/branch.
If another agent wins the claim first, Fairway returns an already-claimed error;
do not keep working that task unless the coordinator reassigns it.

For epic-sized work, claim the next ready descendant:

```bash
fairway claim --in E-001
```

## During Work

Keep local notes however your agent runtime prefers, but record durable facts in
Fairway:

```bash
fairway record evidence T-001 \
  --command-text "go test ./..." \
  --result pass \
  --artifact dist/test.log \
  --artifact-type test
```

Use `pass`, `fail`, `partial`, `skipped`, or `blocked` honestly. A skipped or
blocked check is better than undocumented silence.

If work becomes blocked:

```bash
fairway set-status T-001 blocked --reason "waiting for API fixture"
```

Blocked transitions require a reason in the default config.

## Side Work

Do not split your assigned task into Fairway subtasks for ordinary execution
steps. Use local scratch notes for that.

Use Fairway only when the orchestrator needs to see the work:

```bash
fairway spawn --id T-099 --title "Fix discovered billing route regression" --sibling
```

For long-running side tracks, create a packet and checkpoint:

```bash
fairway packet context T-001 \
  --goal "finish API contract" \
  --owner backend \
  --acceptance "contract tests pass"

fairway checkpoint record T-001 \
  --state active \
  --owner backend \
  --summary "waiting on API fixture"
```

Watcher work should use watcher packets and lifecycle records:

```bash
fairway packet watcher W-001 --owner C-ops/watch --process ci \
  --command "gh run watch" --success "green" --failure "red"
fairway watcher start W-001 --task T-001 --owner C-ops/watch --process ci
fairway watcher finish W-001 --result pass --artifact dist/ci.log
```

Platform-foundation work should use the narrower packet type that matches the
task. If a repo defines `[[packet_templates]]`, use those fields as the packet
contract. Generic templates render with `fairway packet template <name>`:

```bash
fairway packet template architecture-map T-010 \
  --field scope="route ownership" \
  --field current_owner=mixed \
  --field target_owner=D-arch \
  --field migration_risk="route moves can hide auth regressions" \
  --field acceptance="owners and review routes are explicit"
```

Built-in packet commands remain available for common profiles:

```bash
fairway packet architecture-map T-010 \
  --scope "route ownership" \
  --current-owner mixed \
  --target-owner D-arch \
  --migration-risk "route moves can hide auth regressions" \
  --source-doc doc/architecture/platform-foundation/ownership.md \
  --acceptance "owners and review routes are explicit"

fairway packet boundary-guard T-011 \
  --guard-intent "report imports across package boundaries" \
  --finding "cmd/api imports billing internals" \
  --false-positive "generated client code" \
  --graduation-criteria "zero critical findings for two releases" \
  --proof-command "go test ./..."

fairway packet vertical-slice T-012 \
  --target-seam "platform evidence facade" \
  --old-path cmd/api/evidence.go \
  --new-path packages/services/platform/evidence.go \
  --adapter "thin route adapter" \
  --proof-command "go test ./cmd/api ./packages/services/platform" \
  --rollback-plan "revert adapter wiring"
```

## Handoffs

When work crosses a role boundary, hand it off instead of reaching across:

```bash
fairway record handoff T-001 \
  --to ui \
  --payload "Backend contract is ready; see dist/openapi-check.txt"
```

Use `--payload @path/to/file` for longer handoffs.

## Review

Route review based on changed paths:

```bash
fairway route review T-001 --path cmd/api/routes.go --path doc/api/openapi.draft.yaml
```

Reviewers record a verdict:

```bash
fairway record review T-001 \
  --reviewer governance \
  --verdict approve \
  --reason "route and evidence look good"
```

Use `changes` rather than `approve` when more work is required. No agent should
approve its own work.

## Finish Work

Before marking done, record the evidence that proves the acceptance checks:

```bash
fairway record evidence T-001 --command-text "go test ./..." --result pass
fairway set-status T-001 done
fairway merge-ready T-001
```

If gates fail, fix the missing evidence/review/handoff or record why the task is
not ready. Do not force a green story into the DB.

End your session when the runtime exits:

```bash
fairway session end <session-id> --reason normal --exit-code 0
```

## What Not To Do

- Do not edit Fairway DB rows by hand.
- Do not keep working after losing a claim.
- Do not switch roles by changing branches inside a role worktree.
- Do not create Fairway subtasks for private implementation steps.
- Do not self-review.
- Do not mark `done` without evidence, even when the config allows it.
- Do not rely on Jira, Linear, GitHub Issues, or a chat thread as the execution
  source of truth. Link them if useful; keep execution state in Fairway.

## Useful Commands

```bash
fairway ready
fairway task-detail <task-id>
fairway status-report
fairway health-report
fairway dispatch-plan --role <role>
fairway checkpoint status
fairway session status
fairway session reconcile --dry-run
fairway dashboard start
fairway dashboard status
```

See [design/cli.md](design/cli.md) for the complete command surface.

Fairway sessions are records created by `fairway session upsert` or
`fairway session launch`. Host applications may show their own subagent history
or worker list; those entries are not Fairway sessions unless they were
registered with Fairway. Use `fairway session status` for live Fairway-tracked
lanes, and `fairway session reconcile --dry-run` before assuming a host-app
sidebar count represents active Fairway work.
