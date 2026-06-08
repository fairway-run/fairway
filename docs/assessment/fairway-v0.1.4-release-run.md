# Fairway v0.1.4 Release Run

Date: 2026-06-07
Task: FW-140
Owner: ops
Version: v0.1.4
Tag: v0.1.4

## Scope

Run the v0.1.4 release checklist after the public docs cleanup and remote push
intent guard. This is a release-run assessment, not new feature work.

## Candidate Content

- Public docs path refresh.
- Remote push intent guard.
- Review-debt assessment.
- Dashboard performance reconciliation.

## Source

Candidate source before the FW-140 release-run docs commit:

```text
d45e1b78d6a8e70ed96694ea99465ed35a3b48d4
```

Before tagging, rerun `git rev-parse HEAD` after FW-140 is reviewed, committed,
and pushed. The final tag must point at the reviewed, clean, pushed `main`.

## Local Checklist

| Check | Result | Notes |
|---|---|---|
| `go test ./...` | pass | Full Go test suite passed. |
| `go vet ./...` | pass | Full Go vet passed. |
| `git diff --check` | pass | Whitespace diff check passed. |
| `go run ./cmd/fairway config validate` | pass | Active Fairway config validates. |
| `cd website && npm run build` | pass | Docusaurus production build succeeded. |
| `goreleaser check` | pass | GoReleaser configuration validates. |
| `fairway release verify` | blocked | Expected before tag/release assets/Homebrew evidence exist. |

## Release Verify Status

`fairway release verify` should remain blocked until these external release
facts exist:

- v0.1.4 tag pushed from reviewed `main`,
- GitHub release exists and is public,
- release asset URLs return success,
- Homebrew tap commit exists,
- Homebrew cask version matches v0.1.4,
- `brew fetch --cask --force fairway-run/tap/fairway` passes,
- signing and notarization evidence are recorded from the release workflow.

## Decision

Current decision: blocked before tag.

Reason: local prerelease checks pass, but release verification cannot pass until
the release tag, GitHub assets, Homebrew tap update, public release state,
signing/notary logs, and brew fetch evidence exist.

Next action after FW-140 review/push: cut the v0.1.4 tag from clean pushed
`main`, watch the release workflow, then rerun `fairway release verify` with
real release URLs, asset statuses, tap commit, and brew fetch evidence.
