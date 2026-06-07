# Review Lanes

Review lanes are role worktrees used for reviewing another lane's branch. GPUaaS
found that detached `FETCH_HEAD` reviews are easy to lose and hard to repeat, so
fairway should document named review branches as the default pattern.

## Principles

- Review is routed by config, not by whoever is idle.
- The claimant cannot approve their own work.
- Review branches are named and disposable.
- Findings live in fairway review records or artifacts, not random repo-root
  files.
- Model/provider diversity is useful when practical, but provider choice never
  bypasses role ownership or gates.

## Recommended Flow

Inside the reviewer lane worktree:

```bash
git fetch origin
git rebase origin/main
git fetch origin agent/backend
git branch -f review/backend origin/agent/backend
git switch review/backend
fairway record review T-042 --reviewer arch --verdict approve --reason "schema and API impact reviewed"
```

The branch name should be derived from config:

```text
review/{source-role}
```

`fairway route review <task-id>` determines which review role(s) are required.
`fairway merge-ready <task-id>` verifies the required approvals exist before the
coordinator merges.

## Review Provider Sessions

Review-heavy work should use explicit reviewer provider sessions, not only chat
comments or detached local branches. The session attaches to the reviewed task
and uses the reviewer domain as the Fairway role:

```bash
fairway session upsert \
  --id review-arch-T-042 \
  --role arch \
  --provider codex \
  --backend codex-thread \
  --task-id T-042 \
  --branch review/backend \
  --worktree ../fairway-review-arch

fairway checkpoint record T-042 \
  --state active \
  --owner arch \
  --summary "Architecture review started; scope: API contract, schema, and boundary rules"
```

The same shape applies to every review domain:

| Review domain | Typical scope | Session role |
|---|---|---|
| Architecture | ownership boundaries, contracts, ADR fit, migration shape | `arch` or `architecture` |
| Security | auth, secrets, PKI, audit, data exposure, threat model | `security` |
| Ops | deployability, runbooks, observability, rollback, environment impact | `ops` |
| Backend | service/API behavior, schema access, workers, tests | `backend` |
| Frontend | user flows, accessibility, API client behavior, visual regression | `frontend` or `ui` |
| Governance | evidence quality, task boundaries, CI/CD gates, release discipline | `governance` |

Provider choice is independent of review domain. A security review can run in a
Codex thread, a tmux Claude pane, Gemini, or a shell session. Fairway records
the review role, task id, branch, worktree, provider, and backend so the
dashboard can show who is reviewing what.

For tmux-backed reviews:

```bash
FAIRWAY_PROVIDER=claude \
FAIRWAY_PROVIDER_COMMAND="claude" \
FAIRWAY_TRANSCRIPT=".fairway/transcripts/review-security-T-042.log" \
examples/session-adapters/tmux.sh security T-042
```

For shell fallback reviews:

```bash
fairway session upsert \
  --id review-ops-shell-T-042 \
  --role ops \
  --provider shell \
  --backend shell \
  --task-id T-042 \
  --branch review/backend \
  --worktree "$PWD"
```

## Review Handoff

A reviewer session must leave a durable handoff even when the final verdict is
not approval. Minimum handoff contents:

- scope reviewed,
- evidence inspected,
- commands run,
- verdict or blocker,
- next owner/action,
- artifact path when notes are longer than one paragraph.

Use a checkpoint for partial review state:

```bash
fairway checkpoint record T-042 \
  --state awaiting_input \
  --owner security \
  --summary "Security review blocked: missing secret-rotation evidence; next owner ops"
```

Use a review record for the domain verdict:

```bash
fairway record review T-042 \
  --reviewer security \
  --verdict changes \
  --reason "missing secret-rotation rollback evidence"
```

End the reviewer session when the review is handed off:

```bash
fairway session end review-security-T-042 \
  --status ended \
  --reason "changes requested and handoff recorded" \
  --exit-code 0
```

Implementation sessions and review sessions can point at the same task, but
their roles should differ. The task owner remains the implementation owner;
reviewer sessions attach as reviewer roles. If the dashboard cannot yet
distinguish implementer and reviewer sessions directly, use the session id and
role convention `review-<domain>-<task-id>` and track the product gap in the
Fairway backlog.

## Future Command

```bash
fairway review checkout <task-id> [--source-role <role>]
```

This should:

1. resolve the source role branch,
2. fetch it,
3. create or reset `review/{source-role}`,
4. switch the current review worktree to that branch,
5. print the task detail and required review routes.
