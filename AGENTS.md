# Repository Guidelines

## Architecture Invariants

These rules must not be violated by any change:

1. **Service interface is the API boundary.** `githubapi.Service` is the single interface the `command` package consumes. All GitHub data flows through it. Do not call `githubapi.GraphQLService` or `go-gh` API directly from `command` or `format` packages.

2. **Parse does not touch GitHub.** `command.Parse` must never initialize a GitHub client, call an API, or import `githubapi`. It's pure argument parsing. This keeps help and usage paths auth-free.

3. **Output contracts are stable.** JSON field names and TSV column order are scriptable contracts. Machine output changes require coordinated consumer updates. Human output can be polished freely.

4. **Lazy init at the main boundary.** `NewProductionService` returns a lazy wrapper. The go-gh GraphQL client is constructed on first API call, not at startup. Any new service initialization must follow this pattern.

5. **No token storage.** The extension delegates auth entirely to `gh`. Never store, cache, or forward tokens.

6. **Cache is a transparent wrapper.** `NewCacheService` wraps `Service`. Never add caching logic to `command` or `format` packages. Cache decisions stay in `githubapi`.

## Package Responsibilities

| Package | Owns | Does Not Own |
|---------|------|-------------|
| `main` | Binary entrypoint, service wiring | Logic, formatting |
| `command` | CLI parsing, run orchestration, exit codes | GitHub API, output rendering details |
| `githubapi` | GraphQL queries, pagination, response mapping, caching | CLI args, formatting |
| `format` | JSON/TSV/human/plain/template serialization | API calls, CLI state |

## Common Pitfalls

**Sort key literals.** Do not use raw strings `"added"`, `"name"`, `"stars"`, `"pushed"` in switch cases or comparisons. Use the named constants: `command.SortKeyAdded`, `command.SortKeyName`, `command.SortKeyStars`, `command.SortKeyPushed`. Defined in `internal/command/parse.go`.

**Filter key literals.** Do not use raw strings `"name"`, `"fork"` in filter switch cases or validation. Use `command.FilterKeyName`, `command.FilterKeyFork`. Defined in `internal/command/parse.go`.

**Filter value casing.** `Parse` lowers both filter key and value at parse time. Filter functions can compare `f.Value` directly against lowered field values — no `strings.ToLower(f.Value)` needed in run.go.

**ANSI styling.** Do not inline raw escape sequences. Use `ansiStyle(enabled, code)` from `internal/format/human.go`. Pre-defined: `bold(bool)`, `faint(bool)`.

**JSON serialization.** Do not write nil-guard + `json.NewEncoder` patterns. Use generic `writeJSONSlice[T](w, data)` from `internal/format/human.go`.

**Template serialization.** Use generic `writeTemplate[T](w, options, data)` from `internal/format/human.go`. Template mode implies JSON data model — the template engine receives JSON bytes.

**Closure allocation.** When using `bold()` or `faint()` in loops, pre-compute once before the loop. These closures are nil when color is disabled, so hoisting avoids N conditional allocations.

**Slice allocation.** Paginated GraphQL fetches pre-allocate slices using `s.pageSize`: `make([]T, 0, s.pageSize)`. Production page size is 100, set via `newGraphQLService(executor, 100)`.

**Error wrapping.** All GraphQL executor errors must wrap with `"GitHub GraphQL request failed: %w"`. This string is asserted in tests.

**Context cancellation.** Both pagination loops check `ctx.Err()` at the top of each iteration. Any new pagination or API loop must do the same.

**Sort stability.** Use `sort.Slice` not `sort.SliceStable`. Comparators always return a total order via ID/URL fallback, so stability guarantees are unused.

**Name resolution.** `resolveListID` in run.go maps human-readable list names to IDs by fetching all lists. On API error it propagates (no silent fallback). On name-not-found it returns the raw input unchanged — the subsequent `ListRepositories` call surfaces the real error.

**Filter action scoping.** `validateFilters` accepts an `Action` parameter. Filters valid only for `repos` (e.g., `fork`) must be rejected for `list` at parse time, not silently ignored at run time.

**Multi-key sort.** `--sort` accepts comma-separated keys and repeatable flags. `sortKeys` is a `[]string`. Comparators chain fallbacks per-key with a final tiebreaker (ID/URL).

**Non-ASCII characters in source.** Go string literals may contain non-ASCII (Unicode) characters, but punctuation should always be ASCII. Watch for em dashes (`—` U+2014), en dashes (`–` U+2013), smart quotes (`"" ''`), non-breaking spaces (`U+00A0`), zero-width spaces (`U+200B`), and similar copy-paste artifacts. These are invisible in diff review and break grep/search. Run `LC_ALL=C grep -Pn '[^\x00-\x7F]' --include='*.go' .` before committing to catch them.

## Code Review Checklist

When reviewing changes to this repo, check:

- [ ] New flags added to `Parse`? Update `validateSort`, `validateFilters`, help text, and usage text.
- [ ] New filter key? Add constant to `FilterKey*` block. Add to `validateFilters` switch. Handle in `filterStarLists` and/or `filterRepositories`.
- [ ] New output mode? Must be handled in both `WriteStarListsWithOptions` and `WriteRepositoriesWithOptions` dispatch switches. Add to `SelectOutputMode` validation.
- [ ] New GraphQL query? Must paginate with cursor, accept `$endCursor` and `$first`, check `HasNextPage`.
- [ ] New test asserts on stdout? Must set `Now` in `Options` for deterministic timestamps. Use `testOutputOptions` helper in `run_test.go`.
- [ ] Test uses `errWriter`? Duplicate type defined in both `command_test` and `format_test` packages - this is normal Go isolation.
- [ ] New service feature? Add to `Service` interface, update `cacheService` (both methods), update all `fakeService` implementations in tests.
- [ ] Build passes? Run `go build && go test ./... && go vet ./...` before committing.

## Future Work (Agent Guidance)

When implementing these features, follow the patterns below:

- **`--sort` per-key direction**: CLI plumbing for `--sort name:asc,stars:desc`. Comparator already chains fallbacks; need parser to split key:direction and a sort direction per key (not a single `desc` bool).
- **`--cache-ttl`**: Add duration flag in `Parse`. Expose on `NewCacheService` as a public option. Default 5 min. Do not add TTL to `Service` interface.

## Terminal / Platform

- Windows paths in tests use `go-gh/pkg/term` for terminal detection. CI runs on ubuntu-latest.
- WSL users: smoke tests skip on WSL bash. Use Git Bash or native Windows shell.
- Temp directory for smoke test fakes uses `t.TempDir()` - auto-cleaned by Go test runner.

## Go Files

- Use UTF-8 plain text only.
- Do not introduce smart quotes, non-breaking spaces, zero-width characters, Markdown escapes, or other exotic whitespace.
- Go permits tabs; do not manually align indentation. Let `gofmt` handle formatting.
- Prefer raw string literals with backticks for regexes and Windows paths when practical.
- Prefer syntax-aware or anchor-based edits over exact whitespace patches.
- Prefer `ast-grep` for structural Go edits such as switch cases, function signatures, methods, interfaces, and struct changes.
- Prefer `sd` for focused token/keyword-based replacements, insertions, and bulk renames. Match semantic anchors, not tab depth.
- Avoid multi-line edits that depend on counting tabs or exact indentation.
- After editing Go files, run `gofmt -w` on touched files. If imports changed, run `goimports -w` on touched files.
- Final validation: `gofmt -w . && go test ./...`
