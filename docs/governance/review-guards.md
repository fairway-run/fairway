# Review guards

## What gets reviewed

Every change merged to `main` is reviewed. Solo work-in-progress on `agent/<role>` branches is unreviewed by definition; the gate is at the PR back into `main`.

## Pre-merge checks (automated)

Every PR must:

- [ ] `go test ./...` passes.
- [ ] `go test -tags=integration ./...` passes.
- [ ] `golangci-lint run` is clean.
- [ ] `go vet ./...` is clean.
- [ ] `gofmt -s -l .` returns empty.
- [ ] If `internal/store/migrations/` changed: migration applies cleanly to an empty DB and to a fixture DB from the previous version.

These run in CI. PRs cannot merge red.

## Review routing

Routing rules live in `.fairway/config.toml` under `[[review_routes]]`. For fairway's own repo:

```toml
[[review_routes]]
match = "internal/store/**"
reviewer = "arch"

[[review_routes]]
match = "internal/state/**"
reviewer = "arch"

[[review_routes]]
match = "internal/dashboard/assets/**"
reviewer = "ui"

[[review_routes]]
match = "internal/dashboard/templates/**"
reviewer = "ui"

[[review_routes]]
match = ".github/workflows/**"
reviewer = "ops"

[[review_routes]]
match = ".goreleaser.yaml"
reviewer = "ops"

[[review_routes]]
match = "docs/governance/**"
reviewer = "governance"

[[review_routes]]
match = "docs/design/**"
reviewer = "arch"

[[review_routes]]
match = "**"
reviewer = "arch"      # catch-all for the v0.1 cycle
```

Apply with `fairway route review <task-id>`. By default the command derives
changed paths from `main_branch...HEAD` and applies the first matching route.
Operators can pin a route explicitly with `--reviewer <role>` or test routing
against known paths with repeated `--path <path>` flags.

## No self-review

The role that owns a task, or the active claimant identity when present, cannot
review it. Enforced in code when routing and when inserting `task_reviews`.

If the catch-all reviewer is the same as the claimant, route manually:

```bash
fairway record review <task-id> --reviewer <other-role> --verdict approve --reason "manual route away from claimant"
```

## Verdicts

| Verdict | Effect |
|---|---|
| `approve` | Task may transition to a terminal state (subject to `[gates]`). |
| `changes` | `task_state.review_status` becomes `changes_requested`; the task may transition to `blocked` only when the claimant cannot continue without external input. |
| `reject` | Task transitions to `blocked`; requires discussion before re-claiming. |

Verdicts are recorded immutably in `task_reviews`. Revising a verdict means a new row.

## Reopens

A terminal task can be reopened with `fairway set-status --reopen <task-id> <new-state>`. This writes a history row with `reason = "reopen: <text>"`. The dashboard highlights reopened tasks in the activity feed.

## Cross-role changes

A PR touching multiple `[[review_routes]]` matches requires approval from each matched role. The first-match-wins rule for routing applies to *which queue picks it up*; final merge requires all matched roles to approve.

## Time bounds

- A review awaiting verdict for > 24h shows in the dashboard "stale reviews" badge.
- After 72h the original claimant may escalate by handing off to `governance`, who decides whether to reroute or push for resolution.

## Reviewer responsibilities

A reviewer:

- Reads the diff and the task definition.
- Runs the acceptance checks locally when reasonable.
- Verifies the [pre-merge checks](#pre-merge-checks-automated) actually ran (not just claimed).
- Requires exact verification command evidence for CI scripts, release gates,
  workflow YAML/JSON, generated artifacts, deploy/promotion wiring, and docs
  publishing changes. The evidence should name the command that was run and
  attach the relevant artifact or log when available.
- Notes any [coding standards](coding-standards.md) violations.
- Approves, requests changes, or rejects — does not silently leave a PR open.
