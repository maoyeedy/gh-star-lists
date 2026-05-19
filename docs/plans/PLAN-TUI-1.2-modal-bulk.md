# TUI v1.2 — Ship Record

## What shipped

Search package extraction, viewport scrolling, fuzzy search, multi-select with bulk mutations.

- `internal/search/` package extracted from `internal/command/search.go` to break a potential import cycle
- Viewport scrolling (`PgUp`/`PgDn`/`Home`/`End`) in both panes via offset tracking
- Fuzzy search (`/` key) with shared query across panes, `displayedLists`/`displayedRepos` filtered slices
- Multi-select (`space`) with `selected map[string]struct{}`, bulk add/remove/move commands
- Bulk cmds sequential with `ctx.Err()` guard; `bulkDoneMsg{verb, succeeded, failed}` for toast
- Esc clears selection on first press, navigates back on second

Mouse support deferred to v1.3 (keyboard-first is solid; mouse capture interferes with terminal text selection).

## Files changed

| File | Change |
|------|--------|
| `internal/search/search.go` | NEW — extracted fuzzy search algorithm |
| `internal/search/search_test.go` | NEW — primitive + filter tests |
| `internal/command/search.go` | Reduced to delegation wrapper |
| `internal/command/parse.go` | `search.Tokens` for query validation |
| `internal/tui/keys.go` | PgUp, PgDn, Home, End, Search, Select bindings |
| `internal/tui/model.go` | Offsets, displayed slices, search, selected |
| `internal/tui/modal.go` | Bulk modal constructors (`newBulkAdd`/`Remove`/`Move`) |
| `internal/tui/messages.go` | `bulkDoneMsg`, bulk commands |
| `internal/tui/styles.go` | `styleChecked` |
| `internal/tui/model_test.go` | Viewport, search, multi-select tests |

## Design notes

- **Offset sliding:** `slideListOffset()` / `slideRepoOffset()` keep cursor visible after every move. Offsets reset on pane switch, Back, Refresh, and cycleSort.
- **Search bar:** Consumes one row from pane height; viewport offset math accounts for it. Search bar truncation with left-ellipsis on overflow.
- **Enter during search:** Does ID-based lookup into backing `m.lists` (not `displayedLists`) for stable pointer across filtered views.
- **Selection:** `selected` keyed by `NameWithOwner` (stable across sort/refresh). Stale keys pruned in `reposLoadedMsg` for accurate toast counts.
- **Space key:** bubbletea v2 reports `String()="space"` not `" "`; binding uses `key.WithKeys("space")`.
