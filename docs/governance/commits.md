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

## Amending vs new commits

- Never amend a commit you have pushed to a shared branch.
- For your own `agent/<role>` branch, amend freely before opening a PR.
- After a PR is open, prefer new commits — they make review easier.
