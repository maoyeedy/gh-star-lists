# TUI v1.2 - Ship Record

**Shipped:** 2026-05-19

## What shipped

| Feature | Commits |
|---|---|
| `internal/search/` package extracted | `af6774f` |
| Viewport scrolling (both panes) | `271b9a0` |
| Fuzzy search (`/` key, both panes) | `2530976` |
| Multi-select (`space`, bulk a/x/m) | `2f79896` |

Mouse support was deferred to v1.3 (keyboard-first is solid; mouse capture
interferes with terminal text selection).

## Architecture

**`internal/search/` extraction (Phase 0)**

- Moved the fuzzy search algorithm out of `internal/command/search.go` into
  a new `internal/search/` package to break a potential `tui -> command` import
  cycle.
- Exported: `FilterRepositories`, `FilterStarLists` (new), `Tokens`, `Score`,
  `Field`.
- `internal/command/search.go` reduced to a one-line delegation wrapper.
- `parse.go` uses `search.Tokens` for query validation.

**Viewport scrolling (Phase 1)**

- Added `listOffset int` / `repoOffset int` to model.
- `slideListOffset()` / `slideRepoOffset()` keep the cursor visible by sliding
  the window after every cursor move.
- Keys: `PgUp`/`PgDn` (page by `h-1`), `Home`/`g` (top), `End`/`G` (bottom).
- Offsets reset on pane switch, Back, Refresh, and cycleSort.

**Fuzzy search (Phase 2)**

- `searchActive bool` + `searchQuery string` + `displayedLists`/`displayedRepos`
  slices on model. No separate index map -- action handlers read `displayedX[cursor]`
  directly.
- `/` key activates; `handleSearchKey` routes printable chars to query, Esc
  clears, Enter deactivates while keeping filter.
- Navigation keys (Up/Down/PgUp/PgDn/Home/End) pass through `handleSearchKey`
  to `handleKey` so the list is scrollable during search.
- Search bar consumes one row from pane height via `totalH`/`h` pattern in
  renderers; viewport offset math remains correct.
- `handleEnter` does ID-based lookup into backing `m.lists` to get a stable
  pointer (required because `displayedLists` may be a filtered sub-slice
  returned by `search.FilterStarLists`).

**Multi-select (Phase 3)**

- `selected map[string]struct{}` keyed by `NameWithOwner` (stable across
  sort/refresh).
- `space` key (`key.WithKeys("space")` -- bubbletea v2 reports `String()="space"`
  not `" "` for the space key) toggles the focused repo.
- `[x]`/`[ ]` prefix rendered via `styleChecked` when selection is non-empty.
- `a`/`x`/`m` dispatch bulk variants (`bulkAddReposCmd`, `bulkRemoveReposCmd`,
  `bulkMoveReposCmd`) when `len(m.selected) > 0`; single-repo path unchanged.
- Bulk cmds are sequential with `ctx.Err()` guard between items.
- `bulkDoneMsg{verb, succeeded, failed}` drives the toast and triggers refresh.
- Esc clears selection first (single press), then navigates back on second press.
- Stale keys pruned in `reposLoadedMsg` so toast count is accurate.

## Files changed

| File | Change |
|---|---|
| `internal/search/search.go` | NEW -- extracted algorithm |
| `internal/search/search_test.go` | NEW -- primitive + filter tests |
| `internal/command/search.go` | Reduced to delegation |
| `internal/command/parse.go` | `search.Tokens` for validation |
| `internal/tui/keys.go` | PgUp, PgDn, Home, End, Search, Select bindings |
| `internal/tui/model.go` | Offsets, displayed slices, search, selected |
| `internal/tui/modal.go` | Bulk modal constructors (newBulkAdd/Remove/Move) |
| `internal/tui/messages.go` | `bulkDoneMsg`, bulk cmds |
| `internal/tui/styles.go` | `styleChecked` |
| `internal/tui/model_test.go` | Viewport, search, multi-select tests |

## Deferred to v1.3

- Mouse support (click-to-focus, scroll; `--mouse` opt-in flag)
- Search edge cases: multi-byte runes, very long queries, empty-result hint
- Bulk-mutation partial-failure UX (per-item error toast or summary modal)
- Search-while-modal-open guard hardening
- Footer hint truncation on narrow widths
- Help overlay updates for v1.2 additions
- Performance: re-bench `FilterRepositories` on lists with 500+ repos
