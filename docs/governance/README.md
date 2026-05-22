# Governance

Process and quality rules for fairway development. All agents must read these before submitting changes.

| Document | Covers |
|---|---|
| [coding-standards.md](coding-standards.md) | Go style, package layout, error handling, naming, comments, SQL, CSS. |
| [testing.md](testing.md) | Unit / integration / golden tests. Test DB strategy. Coverage expectations. |
| [review-guards.md](review-guards.md) | Pre-merge checks, reviewer routing, verdict semantics, no-self-review rule. |
| [commits.md](commits.md) | Subject format, body conventions, trailers, merge style. |
| [release.md](release.md) | Versioning, tags, changelog, goreleaser, distribution. |

## Quick checklist before opening a PR

- [ ] `go test ./...` passes.
- [ ] `golangci-lint run` is clean.
- [ ] Tests cover the new behavior (see [testing.md](testing.md)).
- [ ] Commit subjects follow [commits.md](commits.md).
- [ ] If schema or state machine changed: a design doc in `docs/design/` is updated.
- [ ] If public CLI / config surface changed: `docs/config-reference.md` or `docs/design/cli.md` is updated.
- [ ] Review routing per [review-guards.md](review-guards.md) is correct.
