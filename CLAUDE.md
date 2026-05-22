# CLAUDE.md

This file orients Claude Code (or any Claude API client) working in this repository. Substantive guidance lives in [AGENTS.md](AGENTS.md) — read that first.

## Claude-specific notes

- Prefer the Edit tool over Bash `sed` for file changes.
- Use the Read tool before Edit (required by the harness).
- When creating commits, follow [docs/governance/commits.md](docs/governance/commits.md), including the `Co-Authored-By` trailer.
- Parallelize independent reads; serialize when one call depends on another.

## Role detection

Resolve role in this order:

1. Current worktree path matched against `[worktrees]` in config.
2. `FAIRWAY_ROLE` env var.
3. Ask the user if both are absent.

Do not assume a default role.
