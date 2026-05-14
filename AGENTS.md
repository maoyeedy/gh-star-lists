# Repository Guidelines

## Architecture Invariants

These rules must not be violated by any change:

1. **Service interface is the API boundary.** `githubapi.Service` is the single interface the `command` package consumes. All GitHub data flows through it. Do not call `githubapi.GraphQLService` or `go-gh` API directly from `command` or `format` packages.

2. **Parse does not touch GitHub.** `command.Parse` must never initialize a GitHub client, call an API, or import `githubapi`. It's pure argument parsing. This keeps help and usage paths auth-free.

3. **Output contracts are stable.** JSON field names and TSV column order are scriptable contracts. Machine output changes require coordinated consumer updates. Human output can be polished freely.

4. **Lazy init at the main boundary.** `NewProductionService` returns a lazy wrapper. The go-gh GraphQL client is constructed on first API call, not at startup. Any new service initialization must follow this pattern.

5. **No token storage.** The extension delegates auth entirely to `gh`. Never store, cache, or forward tokens.

## Package Responsibilities

| Package | Owns | Does Not Own |
|---------|------|-------------|
| `main` | Binary entrypoint, service wiring | Logic, formatting |
| `command` | CLI parsing, run orchestration, exit codes | GitHub API, output rendering details |
| `githubapi` | GraphQL queries, pagination, response mapping | CLI args, formatting |
| `format` | JSON/TSV/human/plain serialization | API calls, CLI state |

## Common Pitfalls

**Sort key literals.** Do not use raw strings `"added"`, `"name"`, `"stars"`, `"pushed"` in switch cases or comparisons. Use the named constants: `command.SortKeyAdded`, `command.SortKeyName`, `command.SortKeyStars`, `command.SortKeyPushed`. Defined in `internal/command/parse.go`.

**ANSI styling.** Do not inline raw escape sequences. Use `ansiStyle(enabled, code)` from `internal/format/human.go`. Pre-defined: `bold(bool)`, `faint(bool)`.

**JSON serialization.** Do not write nil-guard + `json.NewEncoder` patterns. Use generic `writeJSONSlice[T](w, data)` from `internal/format/human.go`.

**Closure allocation.** When using `bold()` or `faint()` in loops, pre-compute once before the loop. These closures are nil when color is disabled, so hoisting avoids N conditional allocations.

**Slice allocation.** Paginated GraphQL fetches should pre-allocate slices: `make([]T, 0, 100)`. Current page size is hardcoded to 100 in GraphQL query strings.

**Error wrapping.** All GraphQL executor errors must wrap with `"GitHub GraphQL request failed: %w"`. This string is asserted in tests.

**Context cancellation.** Both pagination loops check `ctx.Err()` at the top of each iteration. Any new pagination or API loop must do the same.

**Sort stability.** Use `sort.Slice` not `sort.SliceStable`. Comparators always return a total order via ID/URL fallback, so stability guarantees are unused.

**Non-ASCII characters in source.** Go string literals may contain non-ASCII (Unicode) characters, but punctuation should always be ASCII. Watch for em dashes (`—` U+2014), en dashes (`–` U+2013), smart quotes (`"" ''`), non-breaking spaces (`U+00A0`), zero-width spaces (`U+200B`), and similar copy-paste artifacts. These are invisible in diff review and break grep/search. Run `LC_ALL=C grep -Pn '[^\x00-\x7F]' --include='*.go' .` before committing to catch them.

## Code Review Checklist

When reviewing changes to this repo, check:

- [ ] New flags added to `Parse`? Update `validateSort`, help text, and usage text.
- [ ] New output mode? Must be handled in both `WriteStarListsWithOptions` and `WriteRepositoriesWithOptions` dispatch switches. Add to `SelectOutputMode` validation.
- [ ] New GraphQL query? Must paginate with cursor, accept `$endCursor`, check `HasNextPage`. Use `first: 100`.
- [ ] New test asserts on stdout? Must set `Now` in `Options` for deterministic timestamps. Use `testOutputOptions` helper in `run_test.go`.
- [ ] Test uses `errWriter`? Duplicate type defined in both `command_test` and `format_test` packages — this is normal Go isolation.
- [ ] Build passes? Run `go build && go test ./... && go vet ./...` before committing.

## Future Work (Agent Guidance)

When implementing these features, follow the patterns below:

- **`--limit N`**: Add flag in `Parse`. After service call, truncate slice to min(N, len). Respect sort order (truncate after sort, not before).
- **`--filter key:value`**: Add flag in `Parse`. Filter slice after fetch, before sort. Use `strings.Contains` for case-insensitive match on Name/NameWithOwner.
- **`--template`**: Use `go-gh/pkg/template`. Reserve for user-configurable output, not built-in formats. JSON output is the machine contract.
- **Secondary sort**: CLI plumbing for `--sort name,stars` or `--sort name --sort stars`. Compare function already chains fallbacks; just need the parser to pass multiple keys.

## Terminal / Platform

- Windows paths in tests use `go-gh/pkg/term` for terminal detection. CI runs on ubuntu-latest.
- WSL users: smoke tests skip on WSL bash. Use Git Bash or native Windows shell.
- Temp directory for smoke test fakes uses `t.TempDir()` — auto-cleaned by Go test runner.

## Format
For Go files:
- Output UTF-8 plain text only.
- Do not use smart quotes, non-breaking spaces, zero-width characters, or Markdown escapes.
- Go permits tabs; use gofmt, do not manually align with exotic whitespace
- For regexes and Windows paths, prefer raw string literals using backtick
- After editing, run: gofmt -w . && go test ./...
