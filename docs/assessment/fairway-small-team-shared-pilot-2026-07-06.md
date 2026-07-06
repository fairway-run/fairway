# Fairway Small-Team Shared Pilot - 2026-07-06

## Decision

Repeat the pilot before promoting shared-team Fairway as supported.

The loopback read-only shared surface is usable for status, task, and report
readback. It preserves the intended authority boundary: no write API, no public
exposure, no dashboard mutation, no provider-send authority, no approval
authority, no merge/deploy/release/live-operation authority.

The pilot is not enough to claim supported shared-team operation yet. It was
run by the architecture-control operator from the local repository, not by a
second operator on the Mac mini GitLab lab host. The next pilot should be
performed by a non-authoring operator using the packaged lab runbook.

## Scope

Task: `FW-275`

Pilot mode: local loopback read-only shared surface.

Participating roles:

- architecture-control operator: starts the local shared surface, captures
  readback, records evidence, and closes the assessment;
- governance reviewer: verifies the operating model and process claims;
- ops reviewer: verifies the runbook shape, fallback path, and runtime
  operability;
- security reviewer: verifies loopback-only exposure and no hidden authority.

Allowed commands:

```bash
fairway server --read-only --listen 127.0.0.1:<port>
fairway task-detail <task-id>
fairway review-waits list --task <task-id>
fairway rough-edge add --task <task-id> ...
fairway rough-edge list --task <task-id>
fairway delivery report --since 168h
fairway record evidence ...
fairway record review ...
fairway workflow check
fairway reconcile active --dry-run
```

Forbidden actions:

- non-loopback/shared/public bind without a separately reviewed deployment
  task;
- `api-write-pilot` enablement;
- dashboard-originated write, review approval, merge, deploy, release,
  provider-send, dashboard restart, or public exposure changes;
- storing secrets, bearer tokens, cookies, raw provider transcripts, raw tool
  bodies, or private payload dumps in evidence.

Fallback:

Use the local CLI and SQLite store directly if the read-only server is stopped,
unavailable, or not trusted by the operator. The shared surface is visibility
only for this pilot.

## Readback Evidence

The local read-only server was started on `127.0.0.1:17880` and queried through
the API skeleton.

Evidence artifact:

- `.fairway/artifacts/fw-275-small-team-pilot-20260706/read-only-server-smoke-v2.txt`

Observed readback:

```text
status: project=fairway mode=read_only read_only=true writes_enabled=false
tasks: count=209 first=FW-143 status=done role=arch
FW-275: status=in_progress role=governance
summary: total=209 todo=2 ready=1 in_progress=1 blocked=6 done=200
POST /api/v1/tasks/FW-275/evidence: 501, write pilot not enabled
0.0.0.0 read-only bind: rejected before listen
```

This proves that the shared surface can support one status/task/report readback
workflow without hidden write authority.

## Rough Edges

Recorded with `fairway rough-edge add` on `FW-275`:

| Owner | Severity | Decision | Summary |
| --- | --- | --- | --- |
| ops | medium | fix-now | Pilot needs a packaged one-command local lab start/status script before non-authoring operators can repeat read-only shared-surface checks. |
| governance | medium | defer | Pilot evidence is from architecture-control/operator dry run; repeat with a second human operator before promoting shared-team support. |
| security | low | defer | Loopback read-only mode is safe for local pilot; trusted proxy or non-loopback exposure still needs separate identity/deployment review. |

## Delivery Metrics

Evidence artifact:

- `.fairway/artifacts/fw-275-small-team-pilot-20260706/delivery-report-168h.json`

Seven-day delivery report highlights at pilot time:

| Metric | Value |
| --- | ---: |
| Completed tasks | 37 |
| Blocked opened / resolved | 16 / 15 |
| Review records | 152 |
| Review approvals / changes requested | 115 / 37 |
| Same-lane review mappings | 42 |
| Notifications / wakes | 141 / 141 |
| Handoffs | 8 |
| Reopen/retry count | 3 |
| Review usefulness ratio | 0.24 |

Interpretation:

- The shared read-only surface helps status discovery but does not remove review
  mapping or human approval waits by itself.
- The most useful next shared-team improvement is a repeatable operator runbook
  and status readback, not broader write authority.
- Repeated review-change loops remain a stronger improvement signal than
  adding more reviewers by default.

## Recommendation

Repeat the pilot on the Mac mini GitLab lab host before promotion.

The next pilot should:

1. Start the read-only server using the packaged lab runbook and durable
   pid/log paths.
2. Have a non-authoring operator open status, tasks, one task detail, and
   reports through the shared surface.
3. Record one review/wait/evidence readback flow without giving the dashboard
   or server hidden mutation authority.
4. Capture rough edges with owner, severity, fix-now/defer decision, and
   artifact reference.
5. Compare delivery/process metrics before and after the pilot window.
6. Decide whether to promote read-only shared mode, repeat with fixes, or block
   shared-team support.

Do not promote `api-write-pilot` or non-loopback exposure from this pilot.
