# Repository Guidelines

## Architecture Invariants

- `githubapi.Service` is the single API boundary consumed by `command`. All GitHub data flows through it. Do not call `graphQLService` or `go-gh` API directly from `command` or `format`.
- `command.Parse` is pure argument parsing — never imports `githubapi`, never touches GitHub API. Keeps help and usage paths auth-free.
- JSON field names and TSV column order are scriptable contracts. Machine output changes require coordinated consumer updates.
- `NewProductionServiceWithOptions` returns a lazy wrapper. `go-gh` GraphQL client constructed on first API call, not at startup.
- Extension delegates auth entirely to `gh`. Never store, cache, or forward tokens.
- `NewCacheServiceWithOptions` wraps `Service`. Cache decisions stay in `githubapi`. Never add caching logic to `command` or `format`.

## Build / Commands

- `make test` — `go test ./...`
- `make vet` — `go vet ./...`
- `make build` — `go build -o ./gh-star-lists .`
- `make fmt` — `go tool goimports -w .`
- `make lint` — `golangci-lint run --fix`
- `make check` — `bash scripts/check.sh` (test + vet + build)
- `make ascii-check` — non-ASCII scanner for Go source
- `make smoke` — `bash scripts/smoke-local.sh`
- After editing Go files: `go tool goimports -w <file>`
- Final gate: `make check`

## Package Responsibilities

| Package | Owns | Does Not Own |
|---------|------|-------------|
| `main` | Binary entrypoint, service wiring | Logic, formatting |
| `command` | CLI parsing (`Parse`), run orchestration (`Run`), exit codes | GitHub API calls, output rendering |
| `githubapi` | GraphQL queries, pagination, response mapping, caching, retry | CLI args, formatting |
| `format` | JSON/TSV/human/plain/template serialization (`--jq`, `--template`) | API calls, CLI state |

## Common Pitfalls

**Sort key literals.** Use `command.SortKey*` constants, not raw strings `"added"`, `"stars"`, `"repos"`.

**Filter key literals.** Use `command.FilterKey*` constants, not raw strings.

**Filter values already lowered.** `Parse` lowers key+value. Compare `f.Value` directly — no `strings.ToLower` needed in filter functions.

**Adding a repos-only filter.** Add key to `reposOnlyFilterKeys` map, handle in `validateFilters` switch, implement in `filterRepositories`.

**ANSI styling.** Use `ansiStyle(enabled, code)` / `bold(bool)` / `faint(bool)` from `internal/format/human.go`. No raw escape sequences.

**JSON serialization.** Use `writeJSONSlice[T](w, data)` or `writeJSONSliceWithOptions(w, options, data)` (for `--jq`). No manual nil-guard + `json.NewEncoder`.

**Template serialization.** Use `writeTemplate[T](w, options, data)` from `internal/format/human.go`. Template engine receives JSON bytes.

**Closure allocation.** Pre-compute `bold()` / `faint()` before loops. Nil when color disabled, so hoisting avoids N conditional allocations.

**Slice pre-allocation.** Paginated fetches: `make([]T, 0, s.pageSize)`. Page size 100.

**Error wrapping.** All GraphQL errors: `fmt.Errorf("GitHub GraphQL request failed: %w", err)`. String asserted in tests.

**Context cancellation.** Check `ctx.Err()` at top of every pagination or batch iteration.

**Sort stability.** Use `sort.Slice`. Comparators provide total order via ID/URL fallback.

**Per-key sort direction.** Comparators return `(int, bool)` — `SortTerm.Desc`. Keys can mix directions: `--sort stars:desc,name:asc`.

**Name resolution.** `resolveList()` returns `resolvedList{ID, URL, Name}` by matching name or ID. `resolveListID` delegates to it. `--web` uses URL. Name-not-found returns raw input — subsequent call surfaces the real error.

**Server-side limit pushdown.** `directListOptions()` pushes `Limit` server-side only when no local post-processing exists (no filters, search, or sort). Otherwise fetches all pages, applies limit locally.

**Topics query guard.** `topicsNeeded()` returns true only for `--filter topic:` or template referencing `Topics`. Avoids fetching topics on every query.

**Destructive operations.** Use `requireYes(parsed, action)`. Non-TTY requires `--yes` or `--dry-run`.

**`membershipIndex` for bulk ops.** `loadMembershipIndex()` fetches all list memberships concurrently (errgroup, limit 5). Used by `unlisted` and `copy`/`merge`.

**Cache invalidation.** Write ops call `invalidateLists()`, `invalidateStarred()`, or `invalidateAll()`. Never in `command` or `format` packages.

**Search buffer reuse.** Hoist `tokenCache` map and `editPrev`/`editCurr` `[]int` buffers outside the repo loop in `searchRepositories()`. Reused via `growIntSlice` to avoid per-repo DP allocation.

**`Topics` field type.** `Repository.Topics` is `[]string` (`json:"-"`). Not in JSON/TSV. Used for `--filter topic:` and template matching.

**Non-ASCII characters.** Run `make ascii-check` before commit. Watch for em dashes, en dashes, smart quotes, non-breaking spaces.

## Code Review Checklist

- [ ] New action? Add `Action*` constant, `Parse` handler, `run.go` case, help/usage text
- [ ] New filter key? Add `FilterKey*` constant, `reposOnlyFilterKeys` if repos-only, `validateFilters`, `filterRepositories`
- [ ] New sort key? Add `SortKey*` constant, `validateSort`, comparator case
- [ ] New flag? Add `Parse` handler, `Parsed` field, thread into action, help/usage
- [ ] New output mode? Handle in both `WriteStarListsWithOptions` and `WriteRepositoriesWithOptions`, `SelectOutputMode` validation
- [ ] New GraphQL query? Paginate with `$endCursor`/`$first`, `HasNextPage`, `ctx.Err()` guard
- [ ] New service method? Add to `Service` interface, `lazyService`, `cacheService`, all `fakeService` impls
- [ ] Test on stdout? Set `Now` in `Options`, use `testOutputOptions` helper in `run_test.go`
- [ ] Test uses `errWriter`? Duplicate type in `command_test` and `format_test`
- [ ] Build passes? `make check`

## Style & Tooling

- UTF-8 LF (`.editorconfig` enforces). No smart quotes, non-breaking spaces, or exotic whitespace.
- Go tabs for indentation. Let `go tool goimports -w` handle formatting.
- Prefer raw string literals (backticks) for regexes.
- Prefer `ast-grep` for structural Go edits (switch cases, signatures, interfaces, structs).
- Prefer `sd` for focused token/replacement edits and renames.
- After Go edits: `go tool goimports -w <file>`
- Final validation: `make check`

## Context7 Library IDs

Pre-resolved. Use `query-docs` directly.

| Library | Context7 ID | Query when… |
|---------|-------------|-------------|
| `go-gh/v2` | `/cli/go-gh` | GraphQL executor, pagination, terminal, auth |
| `Masterminds/sprig` | `/masterminds/sprig` | `--template` function availability |
| `gopkg.in/yaml.v3` | `/yaml/go-yaml` | YAML marshal/unmarshal, struct tags |
