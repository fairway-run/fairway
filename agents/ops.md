# Agent: ops

**Branch:** `agent/ops`
**Worktree:** `<root>/{repo}-ops`
**Provider (informational):** Gemini

## Scope

- `.github/workflows/` — CI, test, lint, release.
- `.goreleaser.yaml` — cross-compile, archive, checksum.
- Homebrew tap (when v0.1 stabilizes).
- Dependency hygiene: `go.mod` tidy, `go mod download` reproducibility.
- Session-launch backend glue for shell/tmux/zellij once `arch` and `backend`
  define the adapter contract.
- Release smoke tests for top-level CLI commands and generated packet outputs.
- Release smoke coverage for profile-aware commands such as
  `fairway config validate` on example profile configs and
  `fairway adoption artifact` with configured route samples.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| Test failures' root cause | `backend` or `ui` (owner of the broken code) |
| Build flags that change behavior | `arch` |
| Release notes content | `governance` |
| Session-launch adapter contract or DB schema | `arch` / `backend` |

## Standards

- [Coding standards](../docs/governance/coding-standards.md)
- [Release process](../docs/governance/release.md)
- [Review guards](../docs/governance/review-guards.md)

## Cadence

- Every merge to `main` runs CI (test + lint).
- Tag → goreleaser build (no auto-publish until v0.1).
- Dependabot / Renovate opens PRs weekly for Go module updates.

## Typical handoffs out

- CI catches a flake → handoff to the role that owns the test.

## Typical handoffs in

- `backend` adds a new top-level command → `ops` adds the corresponding example to docs/quickstart and the release-binaries smoke test.
- `backend` lands `fairway session launch` → `ops` adds shell/tmux/zellij smoke
  coverage where practical.
- `backend` changes workstream profile config parsing → `ops` adds or updates
  smoke fixtures for example configs before release.
