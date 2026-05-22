# gh-star-lists Guidelines

## Architecture Invariants

- `internal/domain` is the zero-dependency leaf. It owns StarList, Repository, PageInfo, input types, typed errors, and view-model rows (RepoRow, ListRow). Nothing in `internal/` may be imported by `domain`.
- `githubapi.Service` is the sole API boundary. `command`/`tui`/`format` never call GraphQL directly.
- `internal/app.StarListService` orchestrates use cases (fetch, filter, sort, limit). Commands parse args, call one app method, format output. No business logic in `command/`.
- `command.Parse` is pure arg parsing -- no `githubapi` imports, no API calls. Auth-free help paths.
- `internal/tui` owns its lipgloss rendering; never imports `internal/format`. View-model types (`RepoRow`, `ListRow`) live in `domain` so both `format` and `tui` can consume them.
- Typed errors at domain boundary: `domain.ErrAuth`, `domain.ErrNotFound`, `domain.ErrRateLimited`. `githubapi` normalizes via `normalizeError`; `command` maps to exit codes via `mapErrorToExitCode`. No string-based error detection.
- Decorator chain for cross-cutting concerns: `lazyService -> RetryService -> cacheService -> diskCacheService -> graphQLService`. Each decorator implements `Service`, wraps another `Service`, adds one concern.
- `NewProductionServiceWithOptions` lazily constructs `go-gh` GraphQL client. Cache decisions stay in `githubapi`; TUI invalidates via `svc.(interface{ Invalidate() })`.
- Extension delegates auth entirely to `gh`. Never store/forward tokens.

## Commands

| Command | Action |
|---------|--------|
| `make test` | `go test ./...` |
| `make vet` | `go vet ./...` |
| `make lint` | `golangci-lint run --fix` |
| `make build` | `go build -o ./gh-star-lists .` |
| `make check` | test + vet + build |
| `make ascii-check` | non-ASCII scanner |
| `make smoke` | `bash scripts/smoke-local.sh` |

After Go edits: `go tool goimports -w <file>`. Final gate: `make lint && make check`.

## Package Map

| Package | Owns | Does Not Own |
|---------|------|--------------|
| `main` | Entrypoint, wiring | Logic, formatting |
| `domain` | Core types, typed errors, RepoRow/ListRow | API calls, formatting, rendering |
| `app` | Use-case orchestration, filter/sort/limit | CLI args, rendering, API protocol |
| `command` | Parse, Run dispatch, exit codes | API calls, rendering, business logic |
| `githubapi` | GraphQL, pagination, caching, retry, mutations, error normalization | CLI args, formatting, business logic |
| `format` | JSON/TSV/human/template serialization, row constructors | API calls, CLI state |
| `tui` | Bubbletea two-pane browser, keys, sort, rendering | API calls, CLI args, format |
| `humanize` | `ShortAge`, `FormatStars` | API calls, styling |

## Split-File Discipline

Before adding to a monolithic file, check existing split files. If none covers the theme and the addition would exceed ~100 lines, create `<pkg>_<theme>.go`.

- `domain`: `domain.go` `page_info.go` `errors.go` `rows.go`
- `app`: `service.go` `options.go` `filter.go` `sort.go`
- `command`: `run.go` `run_action.go` `run_filter.go` `run_sort.go` `run_output.go` `run_prompt.go` `run_tui.go` `parse.go` `types.go` `validate.go` `help.go` `search.go`
- `githubapi`: `client.go` `graphql_queries.go` `graphql_types.go` `graphql_service.go` `graphql_helpers.go` `pagination.go` `errors.go` `retry_service.go` `retry.go` `cache.go` `diskcache.go` `diskcache_policy.go` `diskcache_store.go` `diskcache_coalesce.go` `diskcache_invalidate.go` `membership_index.go` `starred_at.go`
- `format`: `rows.go` `options.go` `mode.go` `human.go` `repositories.go` `star_lists.go`
- `search`: `search.go` `tokenize.go` `edit.go` `scoring.go`
- `tui`: `model.go` `update.go` `update_lists.go` `update_repos.go` `update_mutation.go` `update_refresh.go` `render.go` `render_repo.go` `render_list.go` `render_preview.go` `render_header.go` `render_footer.go` `render_help.go` `navigate.go` `selection.go` `preview.go` `keys.go` `input.go` `search.go` `sort.go` `cache.go` `modal.go` `modal_list.go` `modal_repo.go` `modal_bulk.go` `modal_help.go` `modal_update.go` `modal_view.go` `styles.go` `geometry.go` `viewport.go` `app.go` `messages.go`

## Code Review Checklist

- [ ] New action? `Action*` constant -> `Parse` handler -> `run.go` case -> help text
- [ ] New filter/sort key? `FilterKey*`/`SortKey*` constant -> validate -> filter/compare in `app/filter.go` or `app/sort.go`
- [ ] New flag? `Parse` handler -> `Parsed` field -> thread into action -> help
- [ ] New output mode? `WriteStarListsWithOptions` + `WriteRepositoriesWithOptions` + `SelectOutputMode`
- [ ] New GraphQL query? paginate with `Pager[T]` (not raw `HasNextPage` loop); `ctx.Err()` guard in fetch closure
- [ ] New service method? add to `Service` + `lazyService` + `cacheService` + `RetryService` + all `fakeService` impls
- [ ] New app service method? add to `StarListService`; keep command action case thin (parse -> call app -> format)
- [ ] New domain type? place in `internal/domain`; ensure zero internal imports
- [ ] New error condition? add to `domain/errors.go`; normalize in `githubapi/errors.go`; exit-code-map in `command/run_output.go`
- [ ] New view-model field? add to `RepoRow`/`ListRow` in `domain/rows.go`; populate in constructor in `format/rows.go`
- [ ] Cross-cutting concern? Service decorator: wrap `Service`, implement `Service`, wire into `NewProductionServiceWithOptions`
- [ ] New TUI key binding? `keys.go` -> `model.handleKey` -> help overlay -> test
- [ ] Bulk operation? `errgroup.SetLimit(5)` + `atomic.Int64` + `sync.Mutex`
- [ ] New cache? test hit + miss; verify value varies; confirm receiver propagates
- [ ] New iota? `<Prefix>End` sentinel; cycle with `% int(<Prefix>End)`
- [ ] Uses `Repository.StarredAt` for list repos? Enrich via `githubapi.WithStarredAt` -- Star List items don't populate it
- [ ] Shared logic extracted to `githubapi`/`humanize`/neutral package before duplicating across `tui`/`command`?
- [ ] Right split file? (filters -> `app/filter.go`, sort -> `app/sort.go`, domain types -> `domain/domain.go`, view models -> `domain/rows.go`)
- [ ] Build passes? `make lint && make check`
- [ ] Non-ASCII? `make ascii-check`

## Common Pitfalls

- Import `domain` for types (StarList, Repository, RepoRow), `githubapi` only for `Service` interface and constructors.
- No business logic in `command/`. Filter/sort/limit belongs in `internal/app`. Commands parse + delegate.
- No string matching for errors. Use `errors.Is(err, domain.ErrAuth)` / `errors.As`.
- New paginated GraphQL queries use `Pager[T]` with a fetch closure returning `(nodes []T, pageInfo, error)`. No raw `HasNextPage` loops.
- Convert to `RepoRow`/`ListRow` in format/tui before rendering. Pre-compute fields in `domain/rows.go` rather than at render time.
- Cross-cutting logic uses Service decorator pattern, not inline in `graphql_service.go`.
- Use `command.SortKey*`/`FilterKey*` constants, not raw strings. Filter values pre-lowered by `Parse` -- compare `f.Value` directly.
- ANSI: `ansiStyle(enabled, code)` / `bold(bool)` / `faint(bool)` from `format/human.go`. No raw escapes.
- JSON: `writeJSONSlice[T](w, data)` or `writeJSONSliceWithOptions(w, opts, data)`. Template: `writeTemplate[T](w, opts, data)`.
- Sort: `sort.Slice` with ID/URL tiebreak. Comparators return `(int, bool)` respecting `SortTerm.Desc`.
- Destructive ops: `requireYes(parsed, action)`. Non-TTY needs `--yes` or `--dry-run`.
- Cache invalidation: write ops call `invalidateLists()`/`invalidateStarred()`/`invalidateAll()` in `githubapi` only.
- Starred timestamps: Star List `items` repos lack viewer star time. Use `githubapi.WithStarredAt`/`MergeStarredAt` from `ListStarredRepositories` when rendering `Starred:` or sorting by `SortKeyStarred`.
- Topics: `Repository.Topics` is `[]string` (`json:"-"`). `topicsNeeded()` guards fetch. TUI fetches only for preview/topic-dependent paths.
- TUI: `Model.View()` returns `tea.View` (`v.AltScreen = true`). `tea.WithColorProfile(colorprofile.NoTTY)` disables color. `lipgloss.Width(s)` not `len(s)`.
- Search buffer: hoist `tokenCache` map + `editPrev`/`editCurr` `[]int` outside repo loop; reuse via `growIntSlice`.
- Bulk ops: `errgroup.SetLimit(5)` + `atomic.Int64`. Check `ctx.Err()` at pagination/batch tops.
- Concurrency in tests: protect recorded call slices with `sync.Mutex` when fake service is called from `errgroup` goroutines.
- Sentinel iota: end every `iota` block with `<Prefix>End`. Cycle: `% int(<Prefix>End)`.

## Context7 Library IDs

`go-gh/v2` -> `/cli/go-gh`
`charm.land/bubbletea/v2` -> `/charmbracelet/bubbletea`
`charm.land/bubbles/v2` -> `/charmbracelet/bubbles`
`charm.land/lipgloss/v2` -> `/charmbracelet/lipgloss`
`golang.org/x/sync` -> `errgroup` for concurrent bulk ops
`github.com/charmbracelet/colorprofile` -> TUI color detection (`NoTTY`)
`golang.org/x/tools/cmd/goimports` -> import formatting (`make lint` gate)
