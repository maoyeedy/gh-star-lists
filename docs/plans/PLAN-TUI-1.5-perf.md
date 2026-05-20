# Plan: TUI v1.5 -- Async repo cache and responsiveness

## Goal

Make browse feel loaded and responsive without adding fancy UI, YAML config,
themes, disk cache, or new GitHub API surface. This is the final v1 stage:
solid interaction, bounded async loading, session cache, and clear pending
states.

**Prerequisite:** TUI v1.4 shipped visual polish, eager initial load for the
first sorted list, spinner migration to `bubbles/spinner.Model`, semantic styles,
shared geometry helper, and styled repo rows with progressive narrow-width hiding.

## Current state (post-v1.4)

- `Init()` loads lists and starts `m.spinner.Tick`; on `listsLoadedMsg` the
  first sorted list is auto-focused and its repos are fetched immediately.
- Moving the list cursor to any other list requires Enter to load repos. Only
  one list's repos are held in memory at a time (`m.repos`).
- `m.loading` is a single bool shared across list and repo loading. The spinner
  runs while any load is pending.
- The in-memory `githubapi.cacheService` caches service calls, but the TUI model
  has no per-list session state (loaded / in-flight / failed / generation).
- `starWidth` and `langWidth` in `renderRepoPane` are recomputed from visible
  rows on every frame during scroll, causing small repeated allocations.
- Preview pane wheel scroll is a no-op (comment at `model.go`: "no-op until
  preview scroll is implemented").
- Single-click on an already-focused list with repos not yet loaded does not
  trigger `loadReposCmd` (only double-click or Enter does).
- Mutations close modal immediately and show global loading during refresh.

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
- Cache `starWidth` and `langWidth` on the model (invalidated by
  `reposLoadedMsg` and sort changes) so `renderRepoPane` does not recompute
  them from visible rows on every scroll frame.
- Add preview pane scroll: `tea.MouseWheelMsg` over the preview column
  (currently no-op, comment at `model.go:266`) should scroll the preview block.
  Requires a `previewOffset int` field and `slidePreviewOffset` helper.
- Single-click on an already-focused list with unloaded repos: trigger
  `loadReposCmd` without switching to the repo pane (currently only Enter and
  double-click do this).

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
