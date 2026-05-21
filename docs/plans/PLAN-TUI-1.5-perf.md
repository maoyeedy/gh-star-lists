# Plan: TUI v1.5 — Async repo cache and responsiveness [SHIPPED]

## What shipped

v1.5 replaced the single `m.repos`/`m.loading` model with a per-`(listID,
withTopics)` session cache and a generation counter, shipped a bounded
background preloader, made the right pane respond instantly to cursor moves,
added a modal pending UX, cached render-layer widths, and added preview-pane
scroll. No new GitHub API surface, no disk cache, no config, no themes.

- **Session repo cache (P1).** Removed `m.repos []githubapi.Repository` and
  `m.loading bool`. Added `m.repoCache map[repoCacheKey]*repoCacheEntry` keyed
  by `{listID, withTopics}`, a `m.generation uint64` counter, and helpers
  `currentRepos()` / `anyPending()`. `loadReposCmd` now carries `gen`; the
  `reposLoadedMsg` handler drops stale responses whose generation does not
  match the current model generation.

- **Bounded preloader + live pane (P2).** After `listsLoadedMsg`, all lists
  are queued for background repo loads (concurrency cap 3) via `schedulePreload()`.
  The new `focusList(idx)` helper resolves the focused list and immediately
  reads from the cache — cached lists appear instantly, loading lists show a
  pane-local spinner, errored lists show a compact "ctrl+r to retry" message.
  The Enter-to-load gate is gone; Enter switches pane, Right arrow always
  switches pane. Single-click on an already-focused uncached list now triggers
  a load without switching pane (previously a no-op).

- **Refresh + generation (P3).** `ctrl+r` bumps `m.generation`, clears
  `m.repoCache`, resets the preload queue, calls `Invalidate()` on the
  service cache when available, and reissues `loadListsCmd`. All in-flight
  responses from the prior generation are dropped.

- **Modal pending UX (P3).** Modals no longer close on submit. Each mutation
  key handler sets `modal.submitting = true` and keeps the modal open while
  the command is in flight. On success the modal closes, a toast fires, and
  the affected list's cache entries are invalidated. On failure the modal stays
  open with `modal.submitErr` displayed below the form inputs.

- **Render-width cache (P3).** `renderRepoPane` no longer scans the visible
  row window every scroll frame to compute `starWidth`/`langWidth`. A
  sentinel-based `ensureRepoWidths()` helper computes them over the full
  `displayedRepos` slice once and caches the result; it recomputes only when
  the focused list ID, sort key, search query, or display count changes.

- **Preview pane scroll (P3).** `m.previewOffset int` tracks the scroll
  position; `slidePreviewOffset` clamps the delta. Mouse-wheel over the
  preview column now scrolls ±3 lines. `renderPreviewPane` was refactored
  into `previewContentLines` (full unscrolled line slice) plus a scroll-offset
  slice. Preview `withTopics=true` loads are dispatched only for the focused
  list when the preview toggle is on; the preloader stays `withTopics=false`
  for all background loads.

- **Search benchmark (P4).** Added `internal/search/search_bench_test.go`
  with `BenchmarkFilterRepositories_500/5000` and `BenchmarkFilterStarLists_500`.
  `FilterRepositories` at 500 repos measured 2.17 ms/op — well below the 5 ms
  threshold. Search remains synchronous; no debounce was added.

## Files changed

| File | Description |
|------|-------------|
| `internal/tui/model.go` | Session cache types + fields; `currentRepos`, `anyPending`, `schedulePreload`, `focusList`, `ensureRepoWidths`, `slidePreviewOffset`, `previewContentLines`, `countPreviewLines`; rewired all cursor/click/enter/right handlers; modal-pending dispatch; refresh handler; preview scroll; width cache in `renderRepoPane`; pane-local loading/error branches |
| `internal/tui/messages.go` | `reposLoadedMsg` gained `withTopics bool`, `err error`, `gen uint64`; `loadReposCmd` gained `gen uint64`; added `searchDebouncedMsg` type (unused — debounce not needed) |
| `internal/tui/modal.go` | Added `submitting bool`, `submitErr string` to `modal` struct; `update()` returns early while submitting; `view()` renders submitting indicator and prior `submitErr` |
| `internal/tui/model_test.go` | Migrated `m.repos`/`m.loading` reads to `currentRepos()`/`anyPending()`/`listsLoading`; updated `TestReposLoadedPopulatesRepos` for 3-slot preloader; added `executeBatch` helper; added 17 new tests across P1–P3 |
| `internal/search/search_bench_test.go` | New: three benchmarks with bench-output comment header |

## Design notes

- **Pointer receivers in value-receiver `Update`.** `schedulePreload` and
  `focusList` are pointer receivers because they mutate queue/inflight fields.
  Inside `Update` (value receiver) they are called as `(&m).method()`.

- **`preloadInFlight` only counts `withTopics=false` loads.** Topics loads
  dispatched by the preview toggle are excluded from the cap so they don't
  starve background preloading.

- **Modal pending intercept pattern.** `modal.update()` returns `(nil, cmd)`
  as the submit signal. The model's `Update` detects `cmd != nil` with `modal
  == nil` return and keeps the modal open by setting `submitting = true` before
  dispatching the cmd. No changes to the individual mutation cmd functions.

- **Sentinel-based width cache.** Rather than explicit invalidation calls at
  every cache-write site, `ensureRepoWidths` builds a string sentinel from
  `{focusedListID}|{sortKey}|{searchQuery}|{len(displayedRepos)}`. This covers
  all invalidation cases (list switch, sort change, search keystroke, new load)
  without coupling cache invalidation to individual state-change paths.

- **FilterRepositories at 2.17 ms/op (500 repos).** Below the 5 ms threshold;
  search stays synchronous. If corpus grows past 5 000 repos per list, re-run
  `go test -bench=BenchmarkFilterRepositories -benchmem ./internal/search/`
  and revisit the 50 ms `tea.Tick` debounce path sketched in `messages.go`.
