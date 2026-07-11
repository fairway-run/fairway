# AI Cloud consumer gap audit

Date: 2026-07-11

## Purpose

AI Cloud is Fairway's largest active consumer. This audit converts recurring
consumer-side coordination friction into Fairway product requirements. It does
not treat old AI Cloud failures as current product failures or authorize changes
to AI Cloud task state.

## Evidence reviewed

- AI Cloud active reconciliation: clean.
- AI Cloud ready queue and stale wait projection.
- Review notification mapping failures for `product` and `app-developer`.
- `audit failure-routing`, which currently reports 932 failed evidence rows and
  573 missing follow-ups, including historical and closeout-command noise.
- `automation candidates`, including repeated CI, UAT, docs, security, and
  notification patterns.
- AI Cloud git status, where managed binaries under `.fairway/bin` appear as
  untracked project files.
- AI Cloud pinned Fairway v0.1.11 binary versus newer source capabilities.

## Product gaps

### Reviewer mapping fails too late

Active tasks can name domains such as `product` and `app-developer` that have no
configured reviewer route. Fairway discovers this only after a wait becomes
`notification_failed` with `mapping_required`.

`FW-297` adds a preflight that checks domains used by active tasks against role,
route, and provider-target configuration before work reaches review wait.

### Historical waits remain expensive to triage

The wait surface correctly preserves immutable history, but old review and
completion-handback rows remain operationally noisy. Resolving them one by one
is disproportionate and can obscure current blockers.

`FW-298` adds lifecycle-aware projection and bounded bulk acknowledgement or
supersede decisions. It must preserve the underlying facts and preview every
mutation.

### Failure routing overstates actionable debt

The current AI Cloud report treats some passing closeout commands, superseded
evidence, and failures with existing follow-ups as new missing-gate findings.
The resulting volume is too high to use as an operator queue.

`FW-299` makes classification aware of task state, later accepted evidence,
supersede facts, and existing follow-up tasks. Raw evidence remains available;
only the actionable projection changes.

### Managed binaries dirty consumer repositories

AI Cloud stores release binaries under `.fairway/bin`, which appears as
untracked source unless every consumer adds a local ignore rule. Runtime binary
retention should not become repository metadata maintenance.

`FW-300` moves the default to a user/OS cache or installs an explicit ignore
rule while retaining exact version, path, upgrade, rollback, and cleanup
readback.

### Consumer capability drift is not explicit

AI Cloud currently pins Fairway v0.1.11 while current source has newer commands.
Operators can inspect versions, but there is no single report that says whether
the pinned and running binaries satisfy the project's required capabilities.

`FW-301` adds a non-mutating capability/minimum-version readiness report. It
must not upgrade binaries automatically.

## Existing capabilities that already cover observed needs

The audit did not create duplicate tasks for these areas:

- repeated command detection is already covered by `automation candidates`;
- common-path command reduction is covered by `FW-290` through `FW-295`;
- CI and external utility monitoring already have watcher/session adapters;
- completion handback supersede and generic wait acknowledgement already exist;
- cross-project read-only reporting and shared-team API/store work already have
  implemented or reviewed tracks.

## Recommended order

1. `FW-299`: reduce false-positive operational debt first.
2. `FW-297`: prevent new unroutable review waits.
3. `FW-298`: clean historical wait projection with immutable decisions.
4. `FW-300`: remove consumer worktree pollution.
5. `FW-301`: make binary/capability drift explicit.

This epic is separate from the common-path epic. `FW-290` reduces actions in a
normal task lifecycle; `FW-296` improves the accuracy and maintainability of a
large consumer deployment.
