# Plan: TUI v1.4 -- Async repo cache and responsiveness

## Goal

Make browse feel loaded and responsive without adding fancy UI, YAML config,
themes, disk cache, or new GitHub API surface. This is the final v1 stage:
solid interaction, bounded async loading, session cache, and clear pending
states.

**Prerequisite:** TUI v1.3 shipped with narrow-layout polish, pane-local
loading placement, left/right pane focus, and compact footer/help behavior.

## Current state

- `model.Init()` loads only star lists.
- `handleEnter()` loads repositories for one list on demand.
- `renderRepoPane()` shows `(press enter to view repos)` until the user drills
  into a list.
- `m.loading` is global, so list or repo loading replaces the whole screen with
  `Loading...`.
- The in-memory `githubapi.cacheService` caches service calls, but the TUI model
  does not know which list repos are loaded, loading, stale, or failed.
- Mutations close the modal immediately and show global loading while refreshes
  run after the mutation result.

## Recommended behavior

- Remove the press-enter-to-view flow. After lists load, focus the first list in
  the current sort order and immediately load its repos in the right pane.
- Start background repo loading for all lists in current sorted order. The
  current cursor list is always highest priority; later lists load behind it.
- Moving the list cursor updates the right pane immediately:
  - cached list: show cached repos;
  - in-flight list: show a right-pane spinner;
  - not-yet-started list: promote it, start loading, and show spinner;
  - failed list: show a compact pane-local error with refresh hint.
- Left/Right only switch active pane. Enter on list pane also switches to the
  repo pane; Enter on repo pane opens the repo. Left/Esc never clears cached
  repos.
- Use Bubbles `spinner.Model` for loading. Run `spinner.Tick` only while any
  list load, repo load, or modal mutation is pending.
- Keep all IO in Bubble Tea `tea.Cmd` functions. Use `tea.Batch` for bounded
  concurrent commands; do not mutate model state from external goroutines.

## Implementation changes

- Add model-owned session repo state keyed by `listID + withTopics`, tracking
  `loading`, `loaded`, `error`, `repos`, and request generation.
- Add a bounded preloader with concurrency limit `3`. Schedule repo loads in
  sorted list order and promote the focused uncached list ahead of background
  work.
- Default repo loads use `WithTopics:false`. Preview uses a separate
  `WithTopics:true` cache key and loads topics only for the focused preview
  list.
- `ctrl+r` invalidates the service cache and the TUI session cache, increments
  request generation, reloads lists, and ignores stale repo messages from older
  generations.
- Mutations keep their modal open in a disabled submitting state with an inline
  spinner until the command returns. On success, close the modal, show a toast,
  invalidate affected list cache entries, and schedule reloads. On error, keep
  the modal open with an inline error.
- Keep bulk add/remove/move sequential. Keep existing copy/merge bounded
  `errgroup` behavior. Do not add disk cache, config, themes, or keybinding
  customization.
- Re-bench `FilterRepositories` with a 500+ repo fixture. If per-keystroke
  latency is above 5ms on a mid-range machine, add a 50ms debounce with
  `tea.Tick`; otherwise leave search synchronous.

## Test additions

- Initial load: after `listsLoadedMsg`, first sorted list is focused, right pane
  shows spinner, and repo load commands are scheduled in sorted order.
- Cache rendering: moving the list cursor shows cached repos immediately;
  uncached or in-flight lists show a right-pane spinner.
- No hint regression: `(press enter to view repos)` is no longer rendered after
  lists are loaded.
- Concurrency: repo preloader never schedules more than 3 in-flight loads and
  promotes the focused uncached list.
- Refresh: stale repo responses from a prior generation are ignored after
  `ctrl+r`.
- Preview: `WithTopics:true` loads only for the focused preview list and does
  not force all-list topic preloading.
- Mutation pending: modal stays open with spinner while awaiting
  add/remove/delete/move; success closes and reloads affected caches; failure
  keeps modal open with error.
- Search benchmark: 500+ repo filtering records per-keystroke latency and
  documents whether debounce is needed.

## Additional deferred items from v1.3

These items were observed during v1.3 and are incorporated into this plan:

| Item | Rationale |
|---|---|
| Replace `spinnerFrame int` with `bubbles/spinner.Model` | v1.4 adds full async spinner system; the manual frame approach will conflict. Migrate as the first step of v1.4 spinner work. |
| Hover-aware mouse wheel | Current wheel scrolls the **active** pane regardless of where the cursor is. Wheel should scroll the pane under the mouse pointer. Requires tracking `msg.X` in `MouseWheelMsg` to determine which pane the event is over. |
| Mouse single-click on list row to auto-drill (load repos) | Today click just focuses the row. A click on an already-focused row could trigger the same load that Enter does. Fits naturally into the v1.4 eager-load model. |
| Mouse double-click on list row behaves as Enter | Double-click on a list row should load repos and focus the repo pane — same as pressing Enter. Requires detecting two rapid clicks within a threshold. If bubbletea v2 exposes double-click events natively, prefer that; otherwise track timestamps manually. |
| Search result count indicator | Show `"[N]"` or `"N / total"` at the right edge of the search bar when a query is active. One-line change to `renderListPane`/`renderRepoPane`; deferred because v1.4 changes the repo loading model and the total may be unknown. |
| Fix `%-8s` language column for long names | "TypeScript", "JavaScript" overflow the 8-char slot. Replace with dynamic column width based on the longest language in the current repo set, or just drop the fixed-width format and space with `lipgloss.Width`. |
| Align repo metadata columns | Right-pane metadata (Language, Stars, Age) is currently jagged due to variable-width fields. Align into strict columns to match `fzf-browse.sh` aesthetic. |
| Simplify list pane layout | Remove `|` separator and `Age` from list rows. Show only `Name` and `Count` (e.g. `Dotnet 1`) to reduce visual noise. |
| Clear `m.selected` on list navigation | Selection set should be scoped to the focused list. When `focusedList` changes (Enter into a different list, or after a refresh), `m.selected` should be cleared. |
| `repoCursor`/`repoOffset` reset on list change | When the user drills into a different list, cursor should reset to 0. Currently it is preserved from the previous list, which looks like a stale selection. |
| Suppress browser noise on Linux | Firefox logs to stderr when opening links (especially if closed). Redirect browser stderr to `/dev/null` in `internal/command/run.go` to prevent TUI corruption. |

## Verification

```
make check
make ascii-check
go test -bench=. ./internal/search/
```

Manual smoke:

```
go run . browse
# Initial load: lists appear, first list repos load automatically in right pane.
# Arrow down list pane: cached repos swap instantly; uncached list shows spinner.
# Rapid ctrl+r: stale responses never replace fresh list/repo state.
# Mutation: modal shows pending spinner, then success toast or inline error.
```
