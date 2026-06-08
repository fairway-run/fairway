# Commits

## Subject line

- 70 characters max.
- Type prefix: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `ops:`.
- Lowercase after the prefix. No trailing period.
- Imperative mood: `feat: add session reconcile command`, not `added`.

Good:

```
feat: add session reconcile command
fix: prevent claimant from reviewing own task
docs: clarify state machine reopen semantics
```

Bad:

```
Added a thing.
WIP
update
```

## Body

- Blank line after subject.
- Wrap at 72 columns.
- Explain *why*, not *what*. The diff shows what.
- Reference issues with `Refs #42` or `Fixes #42`.

## Trailers

For agent-assisted commits, include:

```
Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
```

(or whichever agent made meaningful authorial contributions).

## Merging

- Default: **squash merge** into `main`. The squash commit's subject and body are the PR title and description.
- Long-lived `agent/<role>` branches are kept; never delete on merge.
- Tags are signed when possible; signing is not required for PR commits.

## What not to commit

- Files matching `.gitignore` (state DBs, build artifacts).
- Credentials, tokens, `.env` files.
- Large binary fixtures (> 100KB) unless justified.
- Generated files that can be reproduced by `go generate` — generate them in CI.

## Docs with code

- Prefer committing docs with the code change that makes them true.
- A follow-up `docs:` commit is fine for purely editorial cleanup, but not for
  documenting behavior that reviewers need to evaluate the code safely.
- If a change deliberately leaves docs untouched, say why in the PR or handoff.
- Documentation-only changes should still be committed at the boundary where
  they become true and sanity-checked. Do not leave completed docs as ambient
  worktree changes.

## Push and CI signal

- Remote push is a promotion action, not the default end of every scratch
  provider branch. Push integration-ready commits promptly only when the branch
  has an explicit push intent:

  - `main-validation`: merge locally to the configured main branch and push it
    so normal CI validates the integrated batch.
  - `integration`: shared integration branch used by multiple lanes.
  - `review`: branch intentionally exposed for independent review.
  - `release`: release or promotion branch.
  - `backup`: explicit operator-approved backup/mirror.
  - `exception`: scoped reason recorded in Fairway evidence or checkpoint.

- Provider threads and ad hoc worker branches are scratch execution branches by
  default. They should run local validation, then hand off to the coordinator or
  reviewer/merge lane. They should not push remote branches or trigger remote
  CI unless the push intent above is recorded.
- If the orchestrator is not doing the merge, use a reviewer/merge lane to
  verify, merge locally into the configured main branch, and push that branch.
- CI itself usually does not need a Fairway task; record/link the result as
  task evidence or deploy-run evidence.
- If the active project/profile uses `CI-FIX-*` follow-ups, create one only
  when CI exposes an actionable build, test, lint, generated contract, or
  runner failure. `CI-FIX-*` is a project taxonomy convention, not Fairway core
  grammar.
- Use `fairway workflow check --require-pushed` before handing off work that
  depends on CI having run.

## Deploy and UAT boundaries

- Use `fairway workflow check --mode deploy --require-clean --require-pushed`
  before deploy, smoke, or UAT work.
- Create one deploy-run task per meaningful release/deploy attempt.
- Create scoped finding tasks only for actionable failures. Prefixes such as
  `CI-FIX-*`, `CD-FIX-*`, `UAT-BUG-*`, `OPS-FIX-*`, `HARNESS-FIX-*`, or
  `DOC-FIX-*` are examples from one profile taxonomy. Other projects should
  use the task IDs, kinds, labels, and metadata documented by their active
  workstream profile.

## Amending vs new commits

- Never amend a commit you have pushed to a shared branch.
- For your own `agent/<role>` branch, amend freely before opening a PR.
- After a PR is open, prefer new commits — they make review easier.
