# Fairway Small-Team Repeat Pilot - 2026-07-10

## Decision

Promote bounded read-only small-team operation as supported in `v0.1.10`.

The supported shape is one operator-controlled Fairway host using the managed
loopback read-only server lifecycle, explicit config/database/pid/log paths,
backup and restore evidence, and local CLI fallback. An identity-aware proxy or
SSH/VPN tunnel may provide reviewed remote read access while the Fairway origin
remains loopback-only.

This decision does not promote shared writes, a non-loopback Fairway origin,
trusted-proxy runtime identity, dashboard mutation, provider-send authority, or
a Postgres runtime store.

## Evidence Model

The repeat combined two independent evidence channels:

1. the earlier architecture-control operator pilot in
   `fairway-small-team-shared-pilot-2026-07-06.md`, which exercised the
   read-only API and authority boundary manually; and
2. the clean-state lifecycle harness
   `scripts/ci/small_team_readonly_pilot.sh`, executed locally and by GitHub
   Actions on a fresh Ubuntu runner.

The clean runner is independent of the implementation workstation and rebuilds
the operator state from an empty Git repository. It is stronger regression
proof than another unversioned shell transcript, while the earlier manual pilot
continues to provide the operator-usability evidence.

## Clean-Runner Result

- Source SHA: `500af2b` (pilot harness portability fix included).
- GitHub Actions run: `29138050301` initially proved the lifecycle and exposed
  that Ubuntu initialized the disposable repository on `master` while Fairway
  config expected `main`.
- Owning fix: the harness now creates the disposable repository with
  `git init -b main`.
- Final clean GitHub Actions run: `29138090621`, passed at `500af2b` with the
  lifecycle artifact uploaded.
- Local repeat artifact: `/tmp/fairway-fw284-local-pilot-v3`.
- Local recommendation: `promote_read_only`.

The artifact packet includes:

- source SHA, binary version, and SHA-256 checksum;
- generated config and imported task-state initialization;
- config validation and doctor diagnostics;
- SQLite backup/export and restored-store readiness/reconciliation;
- managed start/status/log/stop readback with explicit pid/log paths;
- status, tasks, `FW-284` detail, report summary, and review-wait API/CLI
  readback;
- endpoint response timings;
- assertions for `mode=read_only`, `read_only=true`, and
  `writes_enabled=false`;
- after-stop status, pid cleanup, and final reconciliation.

## Findings And Fixes

| Finding | Result |
| --- | --- |
| The original packet was untracked while requiring the working checkout to remain at the prior lifecycle commit. | Fixed by committing the packet and building the reviewed lifecycle SHA from a detached worktree. |
| A prose-only operator sequence could drift from CI. | Fixed by adding one reusable lifecycle harness used locally and in CI. |
| `doctor` probes optional tools outside the read-only pilot scope. | Harness records full diagnostics and disables dashboard probes; capability-specific findings remain visible. |
| Ubuntu initialized the disposable repository on `master`, conflicting with generated `main` config. | Fixed at the harness boundary with explicit `git init -b main`. |

No product-authority or data-integrity defect was found in the read-only server
lifecycle, backup/restore path, or API read models.

## Supported Boundary

Supported:

- managed `server start|status|logs|stop|restart` in read-only mode;
- loopback origin on an operator-controlled host;
- status, task list/detail, and report-summary read APIs;
- explicit binary/version/config/database/pid/log readback;
- SQLite backup/export and restore rehearsal;
- local CLI fallback;
- reviewed read-only proxy/tunnel access where the origin stays isolated.

Preview or unsupported:

- append-only and guarded write APIs;
- non-loopback Fairway origin binding;
- trusted-proxy/JWT identity verification by Fairway;
- Postgres as the active runtime store or automated SQLite cutover;
- dashboard mutation, provider sends, approvals, merge, deploy, release, or
  live-operation authority.

## Recommendation

Release `v0.1.10` with the bounded read-only small-team support statement above.
Keep write-capable server modes and server-backed storage behind separately
reviewed pilots and do not advertise them as supported team operation.
