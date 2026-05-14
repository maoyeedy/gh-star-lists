# Repository Guidelines

## Project Structure

`gh-star-lists` is a Go GitHub CLI extension. `main.go` wires CLI arguments to `internal/command`, which calls `internal/githubapi` and writes output through `internal/format`.

- `internal/command`: argument parsing, help text, command execution, runtime diagnostics.
- `internal/githubapi`: GitHub GraphQL client and pagination.
- `internal/format`: human, JSON, and TSV output contracts.
- `scripts/smoke-local.sh`: local extension smoke check.
- `.github/workflows`: CI and tag-driven release workflows.
- `testdata`: small sample payloads or fixtures when needed.

## Commands

Run these before release-oriented changes:

```sh
go test ./...
go vet ./...
go build
bash scripts/smoke-local.sh
```

Use `go test ./internal/command ./internal/format ./internal/githubapi` for a quick focused pass while editing package code.

For local extension checks:

```sh
gh extension install . --force
gh star-lists --help
```

## Style

Use idiomatic Go and run `gofmt` on changed Go files. Keep packages small and named like the existing packages: `command`, `githubapi`, `format`.

Do not change JSON field names or TSV field order casually; those are the CLI's scriptable output contracts. Human output can be polished, but machine output should stay stable.

## Tests

Tests use Go's standard `testing` package and `*_test.go` files. Prefer table-driven tests for parsing, formatting, error messages, and GraphQL pagination. Add fixtures under `testdata` only when they make tests clearer than inline literals.

Smoke tests verify extension installation paths and help behavior. Live account checks stay manual because they depend on GitHub auth state and account data.

## Release

Releases are created by pushing a `v*` tag. The Release workflow uses `cli/gh-extension-precompile@v2` to upload platform assets. Wait for that workflow before testing `gh extension install maoyeedy/gh-star-lists`; installing too early can fail because assets are not uploaded yet.

Post-release check:

```sh
gh release view vX.Y.Z -R maoyeedy/gh-star-lists
gh extension remove star-lists
gh extension install maoyeedy/gh-star-lists
gh star-lists --help
```

## Git

Use Conventional Commits such as `feat:`, `fix:`, `test:`, and `docs:`. Keep commits focused. For this solo-maintained extension, prefer direct, minimal documentation over process-heavy contributor instructions.

## Security

Authentication is delegated to GitHub CLI. Do not store tokens. Keep the extension read-only unless README, tests, and release notes are updated for any future write behavior.
