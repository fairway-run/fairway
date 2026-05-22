# Agent: ops

**Branch:** `agent/ops`
**Worktree:** `<root>/{repo}-ops`
**Provider (informational):** Gemini

## Scope

- `.github/workflows/` — CI, test, lint, release.
- `.goreleaser.yaml` — cross-compile, archive, checksum.
- Homebrew tap (when v0.1 stabilizes).
- Dependency hygiene: `go.mod` tidy, `go mod download` reproducibility.

## Out of scope — hand off to:

| If the task involves… | Hand off to |
|---|---|
| Test failures' root cause | `backend` or `ui` (owner of the broken code) |
| Build flags that change behavior | `arch` |
| Release notes content | `governance` |

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
