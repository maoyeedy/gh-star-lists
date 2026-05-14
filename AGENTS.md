# Repository Guidelines

## Project Structure & Module Organization

`gh-star-lists` is a Go-based GitHub CLI extension. The entrypoint is `main.go`, which wires command execution to the production GitHub API service.

- `internal/command`: argument parsing, help text, and command orchestration.
- `internal/githubapi`: GraphQL client and GitHub API service logic.
- `internal/format`: human-readable, JSON, and TSV output formatting.
- `testdata`: sample GraphQL-style payloads used by tests and smoke coverage.
- `scripts/smoke-local.sh`: local pre-release smoke verification.
- `.github/workflows`: CI and tag-driven release workflows.

## Build, Test, and Development Commands

Run the standard checks before submitting changes:

```sh
go test ./...
go vet ./...
go build
```

`go test ./...` runs all unit and workflow tests. `go vet ./...` checks for common Go mistakes. `go build` verifies the extension binary builds from the repository root.

For local extension verification:

```sh
bash scripts/smoke-local.sh
```

The smoke script repeats the core checks, verifies help and usage-error paths, installs the checkout as a `gh` extension, and confirms `gh star-lists --help`.

## Coding Style & Naming Conventions

Use idiomatic Go formatting and keep edits small. Run `gofmt` on changed Go files before committing. Package names are short lowercase names such as `command`, `format`, and `githubapi`. Exported identifiers should describe public behavior; unexported helpers should stay close to their package tests.

Keep CLI output contracts stable. JSON fields use lowerCamelCase, and TSV field order is documented in `README.md`.

## Testing Guidelines

Tests use Go's standard `testing` package and follow the existing `*_test.go` convention. Prefer table-driven tests for argument parsing, formatting, and GraphQL pagination cases. Add or update `testdata` fixtures when behavior depends on realistic API payloads.

Run `go test ./...` for normal verification. Use `bash scripts/smoke-local.sh` before release-oriented changes or extension wiring changes.

## Commit & Pull Request Guidelines

Use Conventional Commits, matching the project history: `feat: ...`, `fix: ...`, `test: ...`, `docs: ...`, or similar. Keep each commit focused on one behavioral or documentation change.

Pull requests should include a concise summary, verification commands run, and any user-visible CLI output changes. Link related issues when available. Include screenshots only if terminal output formatting changes materially.

## Security & Configuration Tips

This extension delegates authentication to GitHub CLI and should not store tokens. Keep live API checks optional and documented separately from CI. Do not add write operations or GraphQL mutations without updating the README, tests, and release notes.
