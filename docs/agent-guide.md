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
contract even when the current packet command is still built in:

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
fairway dashboard
```

See [design/cli.md](design/cli.md) for the complete command surface.
