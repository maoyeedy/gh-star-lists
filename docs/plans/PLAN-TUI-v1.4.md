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
