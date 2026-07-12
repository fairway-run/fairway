# Worktrees

Fairway can use one git worktree per role, with paths and naming controlled by project configuration.

See [concepts.md](concepts.md) for the canonical distinction between role and
lane. In short: a role is the configured responsibility; a lane is an active
execution slot or display label under a role.

## Layout

```
projects/
├── myrepo/                 # the primary checkout
└── worktrees/              # configured via [worktrees] root
    ├── myrepo-backend/     # role "backend", branch "agent/backend"
    ├── myrepo-ui/          # role "ui", branch "agent/ui"
    └── myrepo-ops/         # role "ops", branch "agent/ops"
```

Configured by:

```toml
[worktrees]
root = "../worktrees"
naming = "{repo}-{role}"

[[roles]]
name = "backend"
branch = "agent/backend"
```

`{repo}` resolves to the basename of the primary checkout. `{role}` resolves to the role name. Templates are simple substitution — no expression language.

## Commands

- `fairway worktree setup` — for each role in config, ensure the branch exists (created from `[fairway] main_branch` if not) and the worktree is checked out.
- `fairway worktree status` — list worktrees, branches, last commit, dirty state.
- `fairway worktree prune` — remove worktrees for roles no longer in config (with confirmation).

These are conveniences. Fairway never blocks you from using `git worktree` directly.

## Why one worktree per role

- Each agent has its own working directory and branch — no `git stash` games.
- The dashboard can show per-lane last-commit and dirty state by `cd`-ing into the worktree.
- Branches stay long-lived per role; PRs merge `agent/<role>` → `main` (or whatever the project's main branch is — also configurable).

## Out of scope

- Worktrees on remote machines.
- Sparse checkouts.
- Auto-rebase against main (left to user discretion).
