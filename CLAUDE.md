# Repository Guidelines

## Architecture Invariants

- `githubapi.Service` is the single API boundary consumed by `command` and `tui`. All GitHub data flows through it. Do not call `graphQLService` or `go-gh` API directly from `command`, `tui`, or `format`.
- `command.Parse` is pure argument parsing — never imports `githubapi`, never touches GitHub API. Keeps help and usage paths auth-free.
- `internal/tui` uses `githubapi.Service` only. Never calls GraphQL directly.
- `internal/tui` does its own row rendering. Never imports `internal/format`.
- Cache invalidation from TUI: type-assert `svc.(interface{ Invalidate() })`. Non-cache services are no-ops.
- `Options.OpenBrowser` passed from `command/run.go`; TUI never imports `cli/browser`.
- JSON field names and TSV column order are scriptable contracts. Machine output changes require coordinated consumer updates.
- `NewProductionServiceWithOptions` returns a lazy wrapper. `go-gh` GraphQL client constructed on first API call, not at startup.
- Extension delegates auth entirely to `gh`. Never store, cache, or forward tokens.
- `NewCacheServiceWithOptions` wraps `Service`. Cache decisions stay in `githubapi`. Never add caching logic to `command` or `format`.
- **Shared behavior extraction rule.** When asked "make CLI do what TUI does" or "extract shared logic": data transformations (sort, filter, resolve) go in `githubapi` or a neutral internal package; output rendering (styling, ANSI codes) stays split — `format` owns the CLI path, `tui` owns its lipgloss path, they share the convention not the code; business logic (validation, computation) goes in a new shared package both can import.

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
- Final gate: `make lint && make check`

## Package Responsibilities

| Package | Owns | Does Not Own |
|---------|------|-------------|
| `main` | Binary entrypoint, service wiring | Logic, formatting |
| `command` | CLI parsing (`Parse`), run orchestration (`Run`), exit codes | GitHub API calls, output rendering |
| `githubapi` | GraphQL queries, pagination, response mapping, caching, retry | CLI args, formatting |
| `format` | JSON/TSV/human/plain/template serialization (`--jq`, `--template`) | API calls, CLI state |
| `tui` | Charm v2 bubbletea two-pane browser, key handling, sort, rendering | GitHub API calls, CLI args, format package |

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

**Cache invalidation.** Write ops call `invalidateLists()`, `invalidateStarred()`, or `invalidateAll()`. Never in `command` or `format` packages. TUI: type-assert `svc.(interface{ Invalidate() })`.

**Search buffer reuse.** Hoist `tokenCache` map and `editPrev`/`editCurr` `[]int` buffers outside the repo loop in `searchRepositories()`. Reused via `growIntSlice` to avoid per-repo DP allocation.

**`Topics` field type.** `Repository.Topics` is `[]string` (`json:"-"`). Not in JSON/TSV. Used for `--filter topic:` and template matching.

**Non-ASCII characters.** Run `make ascii-check` before commit. Watch for em dashes, en dashes, smart quotes, non-breaking spaces.

**TUI Bubbletea v2 API quirks.** `Model.Init()` returns `tea.Cmd` only. `Model.View()` returns `tea.View` struct (set `v.AltScreen = true`). `tea.WithColorProfile(colorprofile.NoTTY)` is the no-color path.

**TUI key binding pattern.** Use `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` from `charm.land/bubbles/v2/key`. Match with `key.Matches(msg, keys.X)`.

**TUI contextual help.** Keep key help text close to `key.Binding` definitions. Prefer Bubbles `help.Model`/`ShortHelp`/`FullHelp` patterns over hand-maintained duplicate help tables when adding or reshaping help.

**TUI component boundaries.** For larger TUI work, keep root `Update` orchestration thin and split pane rendering/input, preview, footer/help, modal, and cache/preload logic into focused helpers. This borrows the useful part of `gh-dash` without copying its broad PR/issue/notification scope.

**TUI preview pattern.** A preview/detail pane should update from the current selection and stay inside Star Lists capacity: list metadata when the list pane is focused, repo metadata/topics when the repo pane is focused. Do not add unrelated GitHub dashboards.

**TUI load commands.** Keep API work in `tea.Cmd`, return typed messages, batch independent commands with `tea.Batch`, and drop stale async results with generation checks and/or canceled contexts.

**TUI cache invalidation.** Type-assert `m.svc.(invalidatable)` in model — check the `invalidatable` interface. Never assume service has `Invalidate()`.

**TUI sort enum cycle.** Add enum constant in `model.go`, case in `sort.go`, label in `currentSortLabel()`. Cycle count must match number of enum values.

**TUI rendering.** `lipgloss.Width(s)` for visual width, not `len(s)`. Use `lipgloss.NewStyle().Width(w).Height(h)` for empty-state placeholders.

**TUI responsive layout.** Prefer stable pane dimensions from `geometry.go`, adaptive preview placement for narrow terminals, faint separators, full-row cursor cues, and fixed-width column calculations. Do not let labels, spinners, or dynamic counts shift pane boundaries.

**TUI topics guard.** Fetch `Repository.Topics` only for preview/detail or explicit topic-dependent behavior. Use `WithTopics: true` only on the paths that need those fields.

**TUI `fakeService` in tests.** Must implement all `githubapi.Service` methods. Add stubs for unused methods returning nil/nil.

**go-gh GraphQL usage.** Construct `api.NewGraphQLClient` lazily in `githubapi` only, pass `Host` through `api.ClientOptions`, use `DoWithContext`, and wrap failures as `fmt.Errorf("GitHub GraphQL request failed: %w", err)`.

**go-gh browser usage in TUI.** Browser opening is injected through `Options.OpenBrowser` from `command/run.go`. In TUI paths, construct browser writers with `io.Discard` so child-process output cannot corrupt the alt screen.

## Code Review Checklist

- [ ] New action? Add `Action*` constant, `Parse` handler, `run.go` case, help/usage text
- [ ] New filter key? Add `FilterKey*` constant, `reposOnlyFilterKeys` if repos-only, `validateFilters`, `filterRepositories`
- [ ] New sort key? Add `SortKey*` constant, `validateSort`, comparator case
- [ ] New flag? Add `Parse` handler, `Parsed` field, thread into action, help/usage
- [ ] New output mode? Handle in both `WriteStarListsWithOptions` and `WriteRepositoriesWithOptions`, `SelectOutputMode` validation
- [ ] New GraphQL query? Paginate with `$endCursor`/`$first`, `HasNextPage`, `ctx.Err()` guard
- [ ] New service method? Add to `Service` interface, `lazyService`, `cacheService`, all `fakeService` impls
- [ ] New TUI action? Wire `browse` in `parse.go`, dispatch in `run.go`, add help text, TTY guard
- [ ] New TUI key binding? Add to `keys.go`, handle in `model.handleKey`, add to help overlay, test
- [ ] New TUI sort mode? Add enum constant in `model.go`, case in `sort.go`, label in `currentSortLabel()`, test sort + cycle
- [ ] Test on stdout? Set `Now` in `Options`, use `testOutputOptions` helper in `run_test.go`
- [ ] Test uses `errWriter`? Duplicate type in `command_test` and `format_test`
- [ ] TUI model test? Use `update(m, msg)` pattern, check model fields, test all state transitions
- [ ] TUI test `fakeService`? Must implement all `githubapi.Service` methods, add `Invalidate()` on `fakeInvalidatableService`
- [ ] Build passes? `make lint && make check`

**`range` over strings is already rune-wise.** Write `for _, r := range s`, never `for _, r := range []rune(s)`. The conversion allocates. `staticcheck` SA6003 flags it.

## Style & Tooling

- UTF-8 LF (`.editorconfig` enforces). No smart quotes, non-breaking spaces, or exotic whitespace.
- Go tabs for indentation. Let `go tool goimports -w` handle formatting.
- Prefer raw string literals (backticks) for regexes.
- Prefer `ast-grep` for structural Go edits (switch cases, signatures, interfaces, structs).
- Prefer `sd` for focused token/replacement edits and renames.
- After Go edits: `go tool goimports -w <file>`
- Final validation: `make lint && make check`

## README

- Keep focused on quick-start essentials: install, basic usage, `--help` pointer.
- Do not duplicate `--help` output or document advanced flags/examples. Defer to `--help`.
- Limit to ~30 lines. If changes grow beyond that, push detail into `--help` instead.

## Context7 Library IDs

Pre-resolved. Use them for ctx7 mcp `query-docs` directly.

| Library | Context7 ID | Query when… |
|---------|-------------|-------------|
| `go-gh/v2` | `/cli/go-gh` | GraphQL executor, pagination, terminal, auth |
| `Masterminds/sprig` | `/masterminds/sprig` | `--template` function availability |
| `gopkg.in/yaml.v3` | `/yaml/go-yaml` | YAML marshal/unmarshal, struct tags |
| `charm.land/bubbletea/v2` | `/charmbracelet/bubbletea` | TUI framework, models, updates, commands |
| `charm.land/bubbles/v2` | `/charmbracelet/bubbles` | TUI components (table, input, viewport, etc.) |
| `charm.land/lipgloss/v2` | `/charmbracelet/lipgloss` | Terminal styling, colors, layouts |
