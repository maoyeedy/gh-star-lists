# Plan: TUI v1.6 — Final cleanup before v1 stable — Ship Record

## What shipped

TUI v1.6 is the last v1-phase release, wrapping up bugs, perf regressions, and UX gaps deferred from earlier v1 releases. All five phases completed, with P5 alloc targets adjusted (33% reduction vs the projected ~48%, due to Go's existing inlining optimizations in the baseline).

- **P1 — Narrow data model queries:** Removed dead `name` field from `listRepositoriesQuery` (fetched but never mapped). Added `isPrivate` to list queries and structs. Added minimal `GetRepositoryID` query and replaced the expensive `GetRepository` (which fetched up to 20 topic nodes) as the fallback in `GetRepositoryMemberships`.
- **P2 — Reject `--sort starred` for repos:** `--sort starred` on `repos` action now returns an error instead of silently no-op sorting (repos fetched via nodes have no `starredAt`, so they all compare equal).
- **P3 — Inline style aliases:** Removed v1.4 backward-compat `styleFaint`/`styleSelected` aliases, replaced with direct `stylePaneSubtitle`/`styleCursorActive` references.
- **P4 — TUI UX fixes:** `repoPaneH()` now has meaningful semantics (`height - headingRows`). Double-click immediately dispatches the load cmd instead of discarding it. Bulk-failure toast duration scales to 4s (from 2s). Help overlay scrolls with Up/Down/PgUp/PgDn on short terminals.
- **P5 — Eliminate double-normalization in Score:** Refactored `Score` to accept pre-normalized fields, eliminating the second normalization pass. ~33% alloc reduction: `FilterRepositories_500` 4536→3036, `FilterStarLists_500` 3030→2030.

## Files changed

| File | Change |
|------|--------|
| `internal/githubapi/graphql.go` | Removed `name` from list query; added `isPrivate`; added `getRepositoryIDQuery`; replaced `GetRepository` fallback with `GetRepositoryID` |
| `internal/githubapi/client.go` | Added `GetRepositoryID` method on `graphQLService` |
| `internal/githubapi/graphql_test.go` | Tests for `IsPrivate` mapping and narrow ID query fallback |
| `internal/command/parse.go` | Added `SortKeyStarred` to unsupported sort keys for `ActionRepos` |
| `internal/command/parse_test.go` | Moved starred-sort test to error path |
| `internal/command/run_test.go` | Updated `TestRunUnlistedSortedByStarred`; added `isPrivate` to JSON want string |
| `internal/format/star_lists_test.go` | Added `isPrivate` to JSON want string |
| `internal/tui/styles.go` | Removed `styleFaint`/`styleSelected` aliases |
| `internal/tui/modal_view.go` | Replaced alias usages with direct style references |
| `internal/tui/modal_bulk.go` | Replaced alias usages with direct style references |
| `internal/tui/render_help.go` | Replaced alias usages; added help overlay offset rendering |
| `internal/tui/render_repo.go` | Replaced alias usages |
| `internal/tui/viewport.go` | `headingRows` const, `repoPaneH()` semantic cleanup |
| `internal/tui/input.go` | Double-click cmd capture, help overlay scroll handling, inline `repoPaneH()` replacements |
| `internal/tui/update.go` | `toastDuration` helper, bulk toast duration scaling |
| `internal/tui/model.go` | Added `helpViewportOffset` field |
| `internal/tui/mouse_test.go` | Double-click load cmd test |
| `internal/tui/update_status_test.go` | Toast duration scaling test |
| `internal/tui/navigation_keys_test.go` | Help overlay scroll tests |
| `internal/search/search.go` | Pre-normalized field scoring, `strings.Builder` for `allText` |
| `internal/search/search_test.go` | Updated for new scoring API |

## Design notes

- **P5 alloc targets were optimistic.** The plan projected ~48% reduction by counting eliminated `normalize`/`cachedSearchTerms` calls. Go's inliner already optimized many of those calls to stack allocations in the baseline, so the observable savings (33%) were proportionally less than the raw call-count math suggested. The structural improvement (single normalization pass) is confirmed correct.
- **Buffer hoisting of `preparedField` did not help.** Passing a `*[]preparedField` pointer for reuse interfered with escape analysis, forcing heap allocation. The per-repo `make` inside the scoring functions was already inlined and effectively free.
- **P3 ascii-check failure was pre-existing.** An em dash at `internal/tui/input.go:349` was already present before P3's style changes. P4's edits to `input.go` resolved it incidentally.
