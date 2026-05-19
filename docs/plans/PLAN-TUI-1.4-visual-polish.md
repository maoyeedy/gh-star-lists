# Plan: TUI v1.4 — Visual polish and interaction refinement

## Goal

Polish the TUI with visual rendering fixes, mouse interaction improvements, and state management fixes deferred from v1.3. No async repo preloader, session cache, request generation, or bounded concurrency — those remain in v1.5.

**Prerequisite:** TUI v1.3 shipped with narrow-layout polish, pane-local loading, left/right focus, and compact footer/help.

## Scope boundary (NOT in v1.4 — deferred to v1.5)

- Background repo preloader or bounded concurrent loading
- Per-list session cache with loading/loaded/error states
- Request generation and stale-response filtering
- Topic-aware cache keys
- `ctrl+r` service cache invalidation
- Mutation modal pending spinner with affected-cache reload
- Filter debounce benchmarking

## Work items

### W1 — Migrate to `bubbles/spinner.Model`

Replace `spinnerFrame int` / `spinnerTickMsg` / manual `tea.Tick` with `bubbles/spinner.Model`.

- Add `spinner.Model` to the model, initialize in `Model.Init()`.
- Run `spinner.Tick` as a `tea.Cmd` from Init; stop by not returning it when idle.
- Use `m.spinner.View()` in `renderListPane` and `renderRepoPane` loading states.
- Delete `spinnerFrame`, `spinnerTickMsg`, manual frame rotation.
- Visual appearance identical — purely mechanical migration.

### W2 — Eager load repos for focused list on startup

After `listsLoadedMsg`, immediately load repos for the first sorted list. No press-enter-to-view delay for the initial list.

- In the `listsLoadedMsg` handler, after sorting, set `m.focusedList` and call the same `loadReposCmd` that `handleEnter` uses.
- Do NOT load repos for unfocused lists.
- Remove `(press enter to view repos)` hint rendering for the focused list.

### W3 — Mouse double-click on list row

Double-click on a list row acts like Enter: load repos and switch focus to repo pane.

- Check `tea.MouseClickMsg.Count` if bubbletea v2.0.x exposes it; otherwise store `lastClickTime` and detect <300ms interval.
- Hit-test list pane rows; on match, call `handleEnter` logic.

### W4 — Mouse single-click on focused row to drill

Single-click on an already-focused list row triggers repo load (same as Enter) without shifting focus.

- In mouse handler: if clicked row == `m.listCursor` and repos not yet loaded, call `loadReposCmd`.
- Already-loaded row: no-op.

### W5 — Hover-aware mouse wheel

Mouse wheel scrolls the pane under the pointer, not the active pane.

- In `tea.MouseWheelMsg` handler, compare `msg.X` against `leftW` boundary to determine target pane.
- Scroll the determined pane's cursor regardless of `m.focus`.
- Target pane with no content: no-op.

### W6 — Simplify list pane layout

Remove `|` separator and `Age` column. Show only `Name` and `Count` (e.g. `Rust 42`).

- Update `renderListPane` row format. Drop `padLeft(age,8)` and `|`.
- Update any layout-width calculations. Keep sort indicator compact.

### W7 — Search result count indicator

Show `N / total` at right edge of search bar when query is active.

- In `renderSearchBar`, compute filtered count vs total.
- Append `"N/total"` faint-styled. Narrow window: hide if insufficient width.
- Repo pane total unknown if repos not loaded.

### W8 — Dynamic language column width

Replace `%-8s` fixed width with max language name length across current repos.

- Compute `maxLangW = max(lipgloss.Width(lang))` over `m.repos`.
- Use dynamic width in meta format string. Min 4, max constrained by pane content width.

### W9 — Align repo metadata columns

Language/Stars/Age render in strict aligned columns.

- Per-column max widths from visible repos (Language dynamic, Stars right-6, Age right-8).
- 1-space gutter between columns.

### W10 — Clear `m.selected` on list change

When `handleEnter` drills into a different list, clear `m.selected`.

- Set `m.selected = nil` at top of `handleEnter` (and W2 eager-load path).

### W11 — Reset `repoCursor`/`repoOffset` on list change

When drilling into a new list, cursor and scroll offset reset to 0.

- `m.repoCursor = 0; m.repoOffset = 0` before loading repos for the new list.

### W12 — Suppress browser stderr noise on Linux

Firefox writes to stderr when opening URLs, corrupting TUI display.

- In `internal/command/run.go`, redirect browser stderr to `/dev/null` in the `OpenBrowser` path.

## Test additions

| Test | What it covers |
|------|----------------|
| Spinner migration | Existing pane-local loading tests pass identically |
| Eager initial load | After `listsLoadedMsg`, first list repos load, no hint shown |
| Double-click drill | Rapid clicks load repos and switch pane focus |
| Single-click drill | Click on cursor row triggers repo load |
| Hover wheel | Wheel over list pane scrolls list while repo pane active |
| Simplified list layout | Row format omits `\|` and Age |
| Search count indicator | Shows/hides correctly with/without query |
| Dynamic language width | Long language names don't overflow fixed slot |
| Metadata column alignment | Language/Stars/Age align in 3-column grid |
| Selection cleared | `m.selected == nil` after `handleEnter` |
| Cursor reset | `m.repoCursor == 0` after list change |

## Verification

```
make check
make ascii-check
```

Manual smoke:

```
go run . browse
# Initial load: first list repos load automatically (no press-enter-to-view).
# List rows: Name + Count only, no Age column.
# Mouse: click focused row loads repos; double-click loads and focuses.
# Wheel: scrolls pane under pointer regardless of active focus.
# Search: count indicator appears while typing.
# Language column: "TypeScript" etc. don't overflow.
# Metadata: Stars align in column regardless of language width.
# Focus switch: selection cleared when drilling into a new list.
# Browser: open link doesn't spew stderr noise.
```
