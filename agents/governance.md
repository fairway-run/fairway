# Agent: governance

**Branch:** `agent/governance`
**Worktree:** `<root>/{repo}-governance`
**Provider (informational):** Claude

## Scope

- `AGENTS.md`, `CLAUDE.md`, and any future tool-specific entries.
- `agents/*.md` — role files.
- `docs/governance/*.md` — coding standards, testing, review guards, commits, release.
- `docs/product.md` — product vision and roadmap.
- `CONTRIBUTING.md`, `README.md` (structure; content authority shared with arch).
- This file.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| Schema or state machine semantics | `arch` |
| Code | `backend` or `ui` |
| CI / release plumbing | `ops` |

## Process for standards changes

Governance owns and edits standards docs. When changing them:

1. Open a PR with the change.
2. Reference the issue or discussion that motivated it.
3. Get approval from `arch` (substantive changes) or `backend` (clarifications).
4. Never apply standards changes retroactively without a written transition plan.

## Cadence

- Re-read all `docs/governance/*.md` files at the start of each minor version cycle and prune dead rules.
- Audit per-role `agents/*.md` files when role scope shifts.
- Keep [docs/design/open-questions.md](../docs/design/open-questions.md) current — close decided items, list new ones.
