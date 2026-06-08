# Review Lanes

Review lanes are role worktrees used for reviewing another lane's branch. GPUaaS
found that detached `FETCH_HEAD` reviews are easy to lose and hard to repeat, so
fairway should document named review branches as the default pattern.

## Principles

- Review is routed by config, not by whoever is idle.
- The claimant cannot approve their own work.
- Review branches are named and disposable.
- Reviewer/merge lanes can own the integration step when the orchestrator is
  not doing it: verify the scratch branch, merge locally into the configured
  main branch, and push the integrated main branch for CI.
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

## Reviewer/Merge Lane

When execution is split across disposable provider threads, the merge lane is
the control point that prevents remote branch sprawl:

1. fetch or inspect the worker branch locally,
2. run focused validation and review,
3. record the Fairway review verdict,
4. merge or squash into the configured main branch locally,
5. push the main branch or approved integration branch with push intent
   `main-validation` or `integration`,
6. record CI/deploy evidence and close out the worker branch.

The merge lane should not push every provider thread branch just to get CI.
Worker branches are scratch by default; the merged batch is the remote
validation unit unless a review, release, backup, or exception intent is
recorded.

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
