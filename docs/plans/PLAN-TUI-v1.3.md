# Plan: TUI v1.3 -- Polish, edge cases, UX

## Goal

Harden v1 quality before v2 ships new features. This is a non-feature release:
no new commands, no new GitHub API calls. Focus exclusively on correctness,
robustness, and UX polish.

**Prerequisite:** TUI v1.2 shipped (search, viewport, multi-select).

## Scope

### In scope

| Area | Work |
|---|---|
| Mouse support | Click-to-focus, wheel scroll; opt-in `--mouse` flag to avoid conflict with terminal text selection |
| Search edge cases | Multi-byte rune handling in `dropLastRune` (already uses `utf8.DecodeLastRuneInString`, should be correct -- verify); very long query truncation in search bar; empty-result hint text |
| Bulk failure UX | Per-item error toast or summary modal when `bulkDoneMsg.failed > 0`; currently shows a combined count but no per-repo detail |
| Search + modal guard | Harden the `/` no-op guard when modal is open; add cursor-translation tests for every action key when a filter is active |
| Cancel in-flight refresh | Context cancellation when user triggers rapid mutations before a refresh completes |
| Footer hint truncation | Truncate long footer hint string on narrow terminal widths (< 60 cols) |
| Help overlay update | Full key reference including v1.2 additions (PgUp/PgDn, g/G, /, space) |
| Search performance | Re-bench `FilterRepositories` on lists with 500+ repos; check filter latency per keystroke |

### Out of scope (v2)

- Parallel bulk mutations with `errgroup` (sequential is safer against rate limits)
- Multi-select for list pane
- Search operators (`lang:go`, `archived:false`, `topic:cli`)
- Persistent search history / saved queries
- YAML config + themes
- `gh star-lists` bare command defaults to TUI
- Second TUI command variant

## Work areas

### Mouse

- Gate on `--mouse` CLI flag threaded through `Options.Mouse bool`.
- Use `tea.WithMouseCellMotion()` when enabled.
- Click: `tea.MouseClickMsg` on list row -> set cursor; on repo row -> set cursor.
- Wheel: `tea.MouseWheelMsg` -> call `moveCursor(+/-1)`.
- `CLAUDE.md` note: mouse capture prevents terminal text selection -- opt-in only.

### Search edge cases

- Verify `dropLastRune` handles multi-byte correctly (it uses `utf8.DecodeLastRuneInString` -- likely fine, write a test with CJK input to confirm).
- Search bar rendering: truncate `m.searchQuery` display if it exceeds pane width.
- When `displayedLists` or `displayedRepos` is empty after filtering, render a
  `"(no matches for <query>)"` hint instead of plain `"(no matches)"`.

### Bulk failure UX

- Enhance `bulkDoneMsg` handling: when `failed > 0`, show an expandable detail
  view or a second toast with failed repo names.
- Consider: `bulkDoneMsg` carries `[]string` of failed NWOs; toast says
  "3 added, 1 failed (owner/repo)".

### Help overlay

Add a second column or section for v1.2 additions:

```
  Navigation          Actions
  -------             -------
  up/k   move up      /      search
  down/j move down    space  select
  pgup   page up      a      add repo
  pgdn   page down    x      remove
  g      top          m      move
  G      bottom       u      unstar
  enter  open/select  p      preview
  esc    back/clear   s      sort
  o      open browser ctrl+r refresh
  ?      toggle help  q      quit
```

### Performance

- Run `go test -bench=. ./internal/search/` with a fixture of 500+ repos and
  log per-keystroke latency.
- If latency > 5ms on a mid-range machine, consider debouncing (`tea.Tick`
  50ms after last keypress before re-filtering).
- The `tokenCache` and `editPrev`/`editCurr` buffer reuse is already in place;
  check if the cache hit rate is high enough to matter.

## Test additions

- `TestDropLastRuneMultiByte`: verify `dropLastRune` on CJK strings.
- `TestSearchWhileFilterActiveActionKeys`: for each action key (a, x, m, u, o),
  assert the correct backing repo is operated on when a filter is active.
- `TestFooterTruncation`: narrow width does not panic or produce ANSI wrapping.
- `TestHelpOverlayContainsV12Keys`: rendered help string contains "space", "/",
  "pgup", "g/G".

## Verification

```
make check
make ascii-check
```

Manual smoke after mouse support:
```
go run . browse --mouse
# Click a list row: cursor moves to clicked row.
# Wheel up/down: list scrolls.
# Terminal text selection: click+drag still works (mouse capture is cell-motion only).
```
