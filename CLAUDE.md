# gh-star-lists Guidelines

## Architecture Invariants

- `githubapi.Service` is the sole API boundary. `command`/`tui`/`format` never call GraphQL directly.
- `command.Parse` is pure arg parsing — no `githubapi` imports, no API calls. Auth-free help paths.
- `internal/tui` owns its lipgloss rendering; never imports `internal/format`.
- `Options.OpenBrowser` injected from `command/run.go`. TUI browser wrapper uses `io.Discard` for both stdout/stderr to protect alt screen.
- `NewProductionServiceWithOptions` lazily constructs `go-gh` GraphQL client on first call. Wraps failures as `fmt.Errorf("GitHub GraphQL request failed: %w", err)`.
- Cache decisions stay in `githubapi`. TUI invalidates via type-assert `svc.(interface{ Invalidate() })` (no-op on non-cache services).
- Extension delegates auth entirely to `gh`. Never store/forward tokens.

## Package Map

| Package | Owns | Doesn't Own |
|---------|------|-------------|
| `main` | Entrypoint, wiring | Logic, formatting |
| `command` | Parse, Run orchestration, exit codes | API calls, rendering |
| `githubapi` | GraphQL queries, pagination, caching, retry, mutations | CLI args, formatting |
| `format` | JSON/TSV/human/plain/template serialization | API calls, CLI state |
| `tui` | Bubbletea two-pane browser, key handling, sort, rendering | API calls, CLI args, format |
| `humanize` | `ShortAge`, `FormatStars` etc | API calls, styling |

## Quick Commands

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

## New Feature Planning

- **Split-file discipline.** Each package uses focused sub-files. Before adding to a monolithic file, check whether the code belongs in an existing split file; if no split file covers this theme and the addition would exceed ~100 lines, create `<pkg>_<theme>.go`. Key split files by package:
  - `command`: `run_action.go` · `run_filter.go` · `run_sort.go` · `run_index.go` · `run_prompt.go` · `run_output.go` · `run_tui.go` · `types.go` · `validate.go`
  - `githubapi`: `graphql_queries.go` · `graphql_types.go` · `graphql_service.go` · `graphql_helpers.go` · `diskcache_policy.go` · `diskcache_store.go` · `diskcache_coalesce.go` · `diskcache_invalidate.go`
  - `search`: `tokenize.go` · `edit.go` · `scoring.go`
  - `tui`: `navigate.go` · `selection.go` · `preview.go` · `update_lists.go` · `update_repos.go` · `update_mutation.go` · `update_refresh.go`
- **Shared before specialized.** Add logic to `githubapi` or neutral packages (e.g. `internal/humanize`) before `tui` or `command`. No data-transform or business-rule duplication across the boundary.
- **Package-level over interface method.** If new behavior can be built from existing `Service` methods, use a package-level function (no ripple to 3+ implementations).
- **Bulk ops.** Always `errgroup.SetLimit(5)` with `atomic.Int64` tracking. Check `ctx.Err()` at pagination/batch tops.
- **Caching.** Confirm cached value varies frame-to-frame, receiver propagates mutations, and test both hit + miss. Don't build speculative cache infra.
- **Sentinel iota.** End every `iota` block with `<Prefix>End`. Cycle: `% int(<Prefix>End)`.
- **Concurrency in tests.** If a fake service is called from `errgroup` goroutines, protect recorded call slices with `sync.Mutex`.
- **TUI wiring.** If launch sequence repeats across `run.go` cases, extract helper. Keep `Update` orchestration thin; split pane/preview/help/modal into focused helpers.

## Common Pitfalls

- Use `command.SortKey*` / `command.FilterKey*` constants, not raw strings.
- Filter values pre-lowered by `Parse`. Compare `f.Value` directly.
- ANSI: `ansiStyle(enabled, code)` / `bold(bool)` / `faint(bool)` from `format/human.go`. No raw escapes.
- JSON: `writeJSONSlice[T](w, data)` or `writeJSONSliceWithOptions(w, opts, data)`.
- Template: `writeTemplate[T](w, opts, data)` — engine gets JSON bytes.
- Slice: paginated fetches `make([]T, 0, 100)`. Closure: pre-compute `bold()`/`faint()` nil outside loops.
- Sort: `sort.Slice` with ID/URL tiebreak. Comparators return `(int, bool)` — `SortTerm.Desc`.
- Name: `resolveList()` → `resolvedList{ID, URL, Name}`. `--web` uses URL. Not-found returns raw input.
- Destructive ops: `requireYes(parsed, action)`. Non-TTY needs `--yes` or `--dry-run`.
- Cache invalidation: write ops call `invalidateLists()`/`invalidateStarred()`/`invalidateAll()` in `githubapi` only.
- Topics: `Repository.Topics` is `[]string` (`json:"-"`). `topicsNeeded()` guards fetch. TUI fetches only for preview/topic-dependent paths.
- Search buffer: hoist `tokenCache` map + `editPrev`/`editCurr` `[]int` outside repo loop; reuse via `growIntSlice`.
- TUI: `Model.View()` returns `tea.View` (set `v.AltScreen = true`). `tea.WithColorProfile(colorprofile.NoTTY)` disables color. Use `key.NewBinding` + `key.Matches`. `lipgloss.Width(s)` not `len(s)`.

## Code Review Checklist

- [ ] New action? `Action*` constant → `Parse` handler → `run.go` case → help text
- [ ] New filter key? `FilterKey*` constant → add to `reposOnlyFilterKeys` → `validateFilters` → `filterRepositories`
- [ ] New sort key? `SortKey*` constant → `validateSort` → comparator
- [ ] New flag? `Parse` handler → `Parsed` field → thread into action → help
- [ ] New output mode? `WriteStarListsWithOptions` + `WriteRepositoriesWithOptions` + `SelectOutputMode`
- [ ] New GraphQL query? paginate with `$endCursor`/`$first` + `HasNextPage` + `ctx.Err()` guard
- [ ] New service method? add to `Service` + `lazyService` + `cacheService` + all `fakeService` impls
- [ ] New TUI action? wire `tui` in parse.go → dispatch in run.go → help → TTY guard
- [ ] New TUI key binding? keys.go → model.handleKey → help overlay → test
- [ ] Shared logic needed? extract to `githubapi`/`humanize`/neutral package before duplicating
- [ ] Bulk operation? `errgroup.SetLimit(5)` + `atomic.Int64` + `sync.Mutex`
- [ ] New cache? test hit + miss, verify value varies, receiver propagates
- [ ] New iota? `<Prefix>End` sentinel, cycle with `% int(<Prefix>End)`
- [ ] Code in the right split file? (e.g. filters → `run_filter.go`, sort → `run_sort.go`, TUI launch → `run_tui.go`; new theme with >100 lines → new `<pkg>_<theme>.go`)
- [ ] Build passes? `make lint && make check`
- [ ] Non-ASCII? `make ascii-check`

## README

Keep ~30 lines: install, basic usage, `--help` pointer. No duplicate `--help` output or advanced examples.

## Context7 Library IDs

| Library | Context7 ID | Query when… |
|---------|-------------|-------------|
| `go-gh/v2` | `/cli/go-gh` | GraphQL executor, pagination, terminal, auth |
| `Masterminds/sprig` | `/masterminds/sprig` | `--template` function availability |
| `gopkg.in/yaml.v3` | `/yaml/go-yaml` | YAML marshal/unmarshal, struct tags |
| `charm.land/bubbletea/v2` | `/charmbracelet/bubbletea` | TUI framework, models, updates, commands |
| `charm.land/bubbles/v2` | `/charmbracelet/bubbles` | TUI components |
| `charm.land/lipgloss/v2` | `/charmbracelet/lipgloss` | Terminal styling, colors, layouts |
