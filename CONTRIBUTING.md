# Contributing

Fairway is pre-alpha. The most useful contributions right now are issues:

- Bugs in the CLI or dashboard.
- Schema gaps that bite during dogfooding.
- Configuration patterns we haven't anticipated.

## Development

```bash
git clone git@github.com:subashram/fairway.git
cd fairway
go test ./...
go run ./cmd/fairway init
```

## Coding standards

- Go 1.22+. Standard `gofmt`, `go vet`, and `golangci-lint` run in CI.
- No CGO. SQLite via `modernc.org/sqlite`.
- Keep dependencies minimal. Anything beyond `cobra`, `BurntSushi/toml`, `modernc.org/sqlite`, and the standard library needs a one-line justification in the PR.
- Tests next to code (`foo.go` ↔ `foo_test.go`). Integration tests under `internal/store/*_integration_test.go` build-tagged `integration`.

## Commits

- Conventional-commits-ish but not enforced: `feat:`, `fix:`, `docs:`, `chore:`.
- Reference issue numbers when applicable.

## License

By contributing you agree your contributions are Apache-2.0 licensed.
