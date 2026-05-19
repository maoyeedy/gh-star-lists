# TUI v1.3 — Ship Record

**Status: shipped on feat/tui.**
**Prerequisite satisfied:** v1.3 is the stated prereq for v1.4 (async repo cache).

---

## What shipped

### A — Plumbing
- `--mouse` opt-in flag (`parse.go` → `run.go` → `tui.Options.Mouse`). Wired in `View()` as `v.MouseMode = tea.MouseModeCellMotion` (bubbletea v2 API; no `ProgramOption` equivalent exists).
- `bulkDoneMsg.failedNWOs []string` added. All three bulk command funcs (`bulkAddReposCmd`, `bulkRemoveReposCmd`, `bulkMoveReposCmd`) collect failed NWOs into the slice. Model stores `bulkFailedNWOs []string`.

### B — Render layer
- **B1** Active/inactive cursor: `styleCursorActive` (bold cyan) and `styleCursorInactive` (faint) in `styles.go`. Both panes show a ghost cursor when unfocused.
- **B2** List pane metadata: bracket blob `(N repos, age)` replaced with fixed right-aligned columns `padLeft(N,4) + " | " + padLeft(age,8)`. New `padLeft` helper alongside `padRight`.
- **B3** Repo pane narrow-aware metadata: hidden entirely below 60-col threshold. 8-byte language clip removed (was a UTF-8 bug). Full language name shown when metadata is visible. 2-space minimum gap enforced; title truncated with `...` if needed.
- **B4** Search bar truncation: left-ellipsis `... <tail>` when query overflows pane width. Empty-result hint changed to `(no matches for "...")` with query interpolated.
- **B5** Footer: collapsed to core hints only (`/ search  enter open  s sort  ? help  q quit` / `space select  o browser` for repo pane).
- **B6** Help overlay: rebuilt as two-column table (Navigation | Actions). Narrow fallback (<50 col) stays single-column. Includes `left`/`right`, `space`, `/`, `pgup`/`pgdn`, `g`/`G`.
- **B7** Bulk failure toast: lists up to 3 failed NWOs by name; `+N more` for larger batches. Format: `"3 added, 1 failed (owner/repo)"`.
- **B8** Pane-local loading spinner: full-screen `"Loading..."` removed from `renderContent`. Simple rotating-frame spinner (`| / - \`) renders inline inside the loading pane (`renderListPane` for initial list load; `renderRepoPane` for repo load). Surrounding layout (header, footer, other pane) stays visible. Spinner driven by `spinnerTickMsg` at 100 ms intervals; tick stops when `!m.loading`.

### C — Behavior
- **C1** Esc preserves repos: `Back` branch no longer clears `m.repos`, `m.focusedList`, `m.repoCursor`, `m.repoOffset`. Focus shifts to list pane; loaded repos remain. Updated `TestBackFromRepoPane` to assert preservation.
- **C1** `Left`/`Right` key bindings: explicit pane focus keys in `keys.go`. `Left` mirrors Esc (focus to list pane, no-op if already there). `Right` moves to repo pane only when `focusedList != nil && len(m.repos) > 0`.
- **C2** Mouse handlers: `tea.MouseClickMsg` hit-tests list/repo pane bounds using the same `leftW` formula as `renderLayout`; sets focus and cursor. `tea.MouseWheelMsg` calls `moveCursor(±1)` on the active pane. Both guarded against modal and search-active state.
- **C3** Dead guard removed: unreachable `m.modal != nil` check inside `Search` key handler deleted.

### D — Tests (8 new)
`TestDropLastRuneMultiByte`, `TestSearchWhileFilterActiveActionKeys`, `TestNarrowRepoPaneHidesMetadata`, `TestFooterCoreHintsOnly`, `TestLoadingRendersInsidePane`, `TestHelpOverlayContainsV12Keys`, `TestMouseClickFocusesPane`, `TestParseMouseFlag`.

---

## Implementation notes

**bubbletea v2 mouse API.** `WithMouseCellMotion()` as a `ProgramOption` does not exist in v2.0.6. Mouse mode is set per-frame in `Model.View()` via `v.MouseMode = tea.MouseModeCellMotion`. This means mouse is enabled/disabled every frame rather than once at program start — acceptable for now; v1.4 may want to confirm there is no flicker.

**Spinner approach.** Used a plain `spinnerFrame int` with manual `tea.Tick` rather than `bubbles/spinner.Model`. Simpler for v1.3 scope. v1.4 should migrate to `spinner.Model` when it owns the full async spinner system (the two implementations will conflict if both are present).

**Esc/repos preserved — selection is NOT cleared on pane switch.** `m.selected` survives Left/Right pane transitions. Only explicit Esc-with-selection clears it. If a user selects repos in list A, Esc back, drills into list B, the selection set from list A is still live. This is probably wrong long-term but is unchanged from pre-v1.3 behavior — v1.4 owns session repo state and is the right place to fix it.

**Shared `searchQuery` across panes.** A single `m.searchQuery` field filters both lists and repos. Switching panes carries the query across. Subtle: entering `/` in the repo pane sets `searchActive = true` and the existing query is cleared. There is no independent per-pane search history. Not a regression from v1.2; noted here because the two-pane model makes it more visible.

**`%-8s` language format with full language name.** B3 removed the 8-rune clip, but the meta format string is still `"%-8s %6s* %s"`. "TypeScript" (10 chars) overflows the 8-char column, shifting the star count right. Minor visual misalignment; the gap enforcement (`available := w - metaW - 2`) accounts for actual `metaW` via `lipgloss.Width`, so layout does not break, it just looks uneven on long language names.
