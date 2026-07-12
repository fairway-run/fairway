# Fairway five-minute first-value assessment

Date: 2026-07-11  
Task: FW-327  
Result: pass

## Goal

Prove that a new adopter can run the current Fairway source in a clean Git
repository, initialize the local control record, complete one bounded work
item, and inspect its decision and evidence without learning the full
coordination vocabulary.

## Environment

- Source: Fairway repository at commit `32aa564` plus the uncommitted FW-326
  sidebar-only slice, which does not affect the CLI.
- Binary: `/tmp/fairway-fw327`, built with
  `GOCACHE=/tmp/fairway-go-cache go build -o /tmp/fairway-fw327 ./cmd/fairway`.
- Version readback: `0.1.0-dev`, expected for a source build without release
  linker flags.
- Fixture: temporary local Git repository with branch `main`, one empty baseline
  commit, and local test-only Git identity.
- Network: not required.

## Final Command Path

```bash
fairway init
git add .fairway/.gitignore .fairway/AGENTS.md .fairway/config.toml
git commit -m "chore: initialize Fairway"
GOCACHE=/tmp/fairway-go-cache fairway doctor
fairway add FV-001 --title "Verify the local Fairway control record" --role operator
fairway work start FV-001 --session-id first-value --role operator \
  --provider shell --backend shell \
  --summary "Complete the first bounded Fairway record"
fairway decision record FV-001 \
  --decision "Keep first-value proof in Fairway" \
  --trigger "The bounded setup check needs a durable rationale" \
  --alternative "Leave the result only in shell history" \
  --chosen "Record the decision and verification in the local Fairway DB" \
  --reason "The next operator can inspect the claim without provider chat" \
  --risk "Local metadata only; no external action" \
  --validation "fairway task-detail FV-001" \
  --fact-ref "task:FV-001"
fairway work verify FV-001 --command-text "fairway doctor" --result pass
fairway work close FV-001 --session-id first-value
fairway task-detail FV-001
```

Measured execution after the existing-repository baseline: **0.432 seconds**.
This is a machine-executed copy/paste proof, not a claim about every person's
reading speed. It leaves the five-minute budget dominated by installation,
reading, and typing rather than Fairway command latency.

## Readback

- `doctor_ok: true`.
- Task `FV-001` moved `todo -> in_progress -> done`.
- Evidence recorded `fairway doctor` with result `pass`.
- One privacy-bounded decision row was present with its authority disclaimer.
- `work close` reported success and ended session `first-value`.
- `task-detail FV-001` showed status, history, evidence, and decision in one
  readback.
- No dashboard, worktree, reviewer, watcher, provider adapter, or network setup
  was required.

## Confusion Points Found

### Repository without a committed main branch

The first attempt used a newly initialized repository with no commit.
`fairway doctor` failed with `base "main" not found`, correctly blocking task
work. The quickstart now states that the repository needs a committed `main`
baseline.

### Uncommitted Fairway bootstrap files

The second attempt left the files created by `fairway init` uncommitted.
`work verify` succeeded, but `work close` failed because the worktree was dirty.
The quickstart now commits `.fairway/.gitignore`, `.fairway/AGENTS.md`, and
`.fairway/config.toml` immediately after initialization.

### Development version readback

A source build reports `0.1.0-dev` unless release linker flags are supplied.
This is expected and is recorded so it is not mistaken for a release mismatch.

### Decision quality

The first decision is stored as `draft`. The decision explains local setup; it
does not grant approval or promotion authority and does not require independent
assessment for this reversible task.

## Defaults Observed

- SQLite DB: `.fairway/state.db`, ignored by the generated `.gitignore`.
- Main branch: `main`.
- Dashboard: loopback, full-access default at `127.0.0.1:7878` for direct
  invocation; no dashboard was started by the rehearsal.
- Shared server: disabled and read-only by default.
- Review/evidence gates: no review required for the simple default task; work
  close still enforced repository cleanliness.

## Cleanup Proof

All temporary rehearsal repositories and the temporary binary were removed
after their results were summarized. No dashboard, server, provider session,
remote branch, public endpoint, or external resource was created.

## Recommendation

**Pass.** Publish the short path with the two explicit Git prerequisites. Keep
roles, profiles, worktrees, dashboards, shared-team operation, review routing,
and release controls behind progressive links unless a user's work crosses one
of those boundaries.
