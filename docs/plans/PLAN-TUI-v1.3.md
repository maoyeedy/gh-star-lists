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
| Narrow layout polish | Right-pane metadata, list metadata, loading state, pane focus, and footer behavior on half-width terminals |
| Help overlay update | Full key reference including v1.2 additions (PgUp/PgDn, g/G, /, space) |

### Out of scope / deferred

- Parallel bulk mutations with `errgroup` (sequential is safer against rate limits)
- Multi-select for list pane
- Search operators (`lang:go`, `archived:false`, `topic:cli`)
- Persistent search history / saved queries
- Async repo preloading and session repo cache (v1.4)
- Search performance benchmarking and debounce decisions (v1.4)
- Cancel in-flight refresh / stale response handling (v1.4)
- YAML config, themes, or config-driven keybindings
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

### Narrow layout polish

Current screenshot behavior shows several width failures that come from the
current row renderers, not from GitHub data:

- `renderRepoPane` always appends a fixed metadata string
  (`language stars pushed`) and forces only a one-column gap when the pane is
  too narrow. In half-width terminals this lets metadata crowd the repo title.
- `renderRepoPane` clips language to 8 bytes, so `TypeScript` becomes
  `Typescri`. This looks like broken data rather than intentional truncation.
- Very long repo names can still run through the metadata area. Do not wrap
  repo rows to two lines; it hurts scanability. Keep one-line rows with
  truncation or ellipsis, and accept pathological long names as a known edge.
- `renderListPane` formats list metadata as one bracket blob:
  `(<n> repos, <age>)`. It is long and visually uneven because ages like
  `4d ago`, `2mo ago`, and `11d ago` vary in width.
- `renderContent` replaces the full screen with `Loading...`, which hides the
  surrounding navigation context during repo loads.
- Esc from the repo pane clears `focusedList` and `repos`, so returning to the
  list pane also clears the right pane back to `(press enter to view repos)`.
- `renderFooter` emits one long hint string per pane. It clips on narrow
  terminals and tries to show too many commands at once.

Recommended decisions:

- Hide right-side repo metadata below a narrow-width threshold. When metadata is
  shown, preserve a fixed minimum gap between repo title and metadata.
- Stop clipping language names to 8 bytes. Either show the full language only
  when metadata has room, or hide the metadata block entirely at narrow widths.
- Replace the list bracket blob with fixed right-aligned columns, for example
  `repos | age`, matching the right pane's table-like metadata style. This is
  the clearer terminal practice for scannable numeric/status metadata.
- Use an inline spinner in the pane that is loading. For repo loading, keep the
  list pane, header, and footer visible and show the spinner in the right pane
  only. For initial list loading, show the spinner in the list pane. Prefer a
  spinner over cycling dots because it is the common TUI loading affordance and
  reads clearly as activity. The all-list async repo loading system belongs to
  v1.4; v1.3 only fixes where loading is rendered.
- Add Left/Right pane focus semantics. Right/Enter moves focus to repos;
  Left/Esc moves focus back to lists while preserving the loaded repo list.
- Distinguish active and inactive cursors: the active pane gets the strong
  cursor/highlight; the inactive pane keeps a quieter contextual selection.
- Collapse the bottom bar to core commands only: search, open/select, help,
  quit, and the active pane's primary action. Move secondary mutation and
  navigation commands into `?`.
- Do not add YAML config, a theme system, or config-driven keybindings in v1.3
  or as a future follow-up.

### Help overlay

Add a compact, complete reference for commands moved out of the bottom bar:

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

## Test additions

- `TestDropLastRuneMultiByte`: verify `dropLastRune` on CJK strings.
- `TestSearchWhileFilterActiveActionKeys`: for each action key (a, x, m, u, o),
  assert the correct backing repo is operated on when a filter is active.
- `TestNarrowRepoPaneHidesMetadata`: narrow width does not crowd repo titles
  with right-side metadata.
- `TestFooterCoreHintsOnly`: footer stays within narrow width by showing only
  core commands and leaving full command reference to help.
- `TestBackPreservesRepoPane`: Left/Esc moves focus back to the list pane
  without clearing the loaded repo list.
- `TestLoadingRendersInsidePane`: repo loading keeps the surrounding layout
  visible and renders a pane-local loading indicator.
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
