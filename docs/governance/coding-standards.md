# Coding standards

## Go

- **Version:** Go 1.22 or later.
- **Format:** `gofmt -s`. CI fails on diff.
- **Lint:** `golangci-lint` with config in `.golangci.yaml`. CI fails on any warning.
- **Vet:** `go vet ./...` runs as part of `go test`; must be clean.

## Dependencies

Allowed without justification:

- `github.com/spf13/cobra`
- `github.com/BurntSushi/toml`
- `modernc.org/sqlite`
- `gopkg.in/yaml.v3` (task import/export compatibility)
- Standard library.

Anything else requires a one-line justification in the PR description. Prefer the standard library when reasonable.

**No CGO.** This is load-bearing for cross-compilation and `brew install` simplicity.

## Package layout

- `cmd/<binary>/` — `main` package, thin wiring only.
- `internal/<concern>/` — implementation. Never imported from outside the module.
- `internal/<concern>/<thing>_test.go` — tests next to code.
- One exported type per file when reasonable; group small related types together.

Cross-package rules:

- `internal/store` may not import `internal/state` (layering violation — state validates, store persists).
- `internal/dashboard` may import `internal/store` (read views) but never writes.
- `internal/config` imports nothing else from `internal/*`.

## Error handling

- Return errors; do not log-and-continue.
- Wrap with `fmt.Errorf("doing X: %w", err)` to add context.
- Sentinel errors via `errors.New` at package top when callers need to `errors.Is`.
- No `panic` outside `main` and `init` for unrecoverable startup failures.

## Naming

- Package names: short, lowercase, no underscores.
- Exported identifiers use full words; abbreviations only for established terms (`ID`, `URL`, `SQL`).
- Test names: `TestFoo_Bar_WhenX` style for cases that need disambiguation; `TestFoo` when one case suffices.

## Comments

- Default: no comments. Code and identifiers should explain themselves.
- Write a comment only when the *why* is non-obvious: a hidden constraint, a workaround for a specific bug, an invariant the reader will not deduce.
- Doc comments on every exported identifier in a non-`main` package. One sentence is enough.
- Never write comments that narrate the code.

## Documentation

- Code and docs move together. If a change affects CLI verbs, flags, config
  shape, schema, migrations, dashboard behavior, report output, packet fields,
  gates, examples, or agent workflow, update the relevant docs in the same
  change.
- User-facing behavior belongs in `README.md`, `docs/quickstart.md`,
  `docs/config-reference.md`, or a focused guide under `docs/`.
- Design intent and future work belong in `docs/design/`.
- Agent operating rules belong in `AGENTS.md`, `docs/agent-guide.md`, or the
  relevant `agents/*.md` role file.
- Do not leave docs saying a feature is "planned" after code lands. If only a
  thin slice landed, document the implemented slice and the remaining limits.

## SQL

- Schema lives in `internal/store/migrations/NNN_*.sql`.
- Queries live in Go source as descriptive `const`s or in shared query files
  under `internal/store/queries/`.
- Use placeholders (`?`), never string concatenation for user input.
- Every multi-statement write is wrapped in `BEGIN ... COMMIT`.
- Indices declared in the same migration that creates the table, unless added later via a deliberate migration.
- SQL under `internal/store/queries/` must be reviewed for Postgres portability
  even while SQLite is the only v1 runtime.
- Do not use SQLite-only JSON1 functions, `rowid`, recursive CTE behavior, or
  autoincrement assumptions in shared query files.
- If a SQLite-specific construct is necessary, isolate it under
  `internal/store/sqlite/` behind the store interface and document the Postgres
  equivalent or incompatibility.
- State-changing store methods must make transaction boundaries explicit in the
  method or helper name.

## CSS (for the ui agent)

- Single stylesheet at `internal/dashboard/assets/app.css`.
- CSS custom properties (`--color-fg`, `--space-2`) at `:root`. No magic numbers in selectors.
- Mobile is not a target. Layout assumes ≥ 1280px viewport.
- Dark mode via `prefers-color-scheme: dark` from day 1.

## File hygiene

- LF line endings everywhere.
- No trailing whitespace.
- Files end with a newline.
- 4-space indent in YAML / TOML / Markdown; tabs in Go (per `gofmt`).
