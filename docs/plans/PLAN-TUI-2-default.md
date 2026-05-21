# Plan: TUI v2 -- Default experience and architecture

## Goal

Make the TUI the default experience for interactive use while keeping the CLI
scripting surface intact. Split the TUI code into focused components after the
v1 polish and async cache stages are stable.

**Prerequisite:** TUI v1.5 shipped with async repo cache, bounded preloader,
pane-local pending states, modal pending UX, render-width cache, and preview
scroll. v1.4 delivered narrow-layout polish, mouse, search, multi-select, and
viewport behavior.

## Scope

| In | Out |
|---|---|
| Bare `gh star-lists` on TTY opens TUI | New GitHub API surface |
| Architecture cleanup: component split and layout engine | Electron / web UI |
| Default help / README updates for bare TUI | Removing CLI scripting commands |

## Default bare-command switch

After v1.4 validates real-world use, flip the default:

```go
// run.go, in the ActionList case (or before the switch):
if parsed.Action == ActionList && len(parsed.RawArgs) == 0 && canPrompt() {
    // no flags that imply scripted use
    if !parsed.hasOutputFlags() {
        return runBrowse(ctx, parsed, service, stdout, stderr)
    }
}
```

`hasOutputFlags()` returns true if any of `--json`, `--tsv`, `--plain`,
`--fzf`, `--template`, `--jq`, `--output`, `--limit`, `--filter`, `--sort`,
`--search` were given. When true, fall through to CLI list output as today.

**Backward compatibility:** any script that calls `gh star-lists` without flags
in a TTY would now open the TUI. This is intentional. Document in README and
changelog. Provide `--no-tui` flag (maps to `ActionList` directly) as an
escape hatch for scripts that inadvertently run in a TTY.

**Help update:** bare `gh star-lists` help text gains a note:
`"Tip: in a terminal, running with no arguments opens the interactive browser."`

## Architecture: component split

By v2, `model.go` will be large. Split into focused files:

```
internal/tui/
  app.go          Options, Run
  model.go        root model struct, Init, Update, View
  layout.go       renderLayout, renderHeader, renderFooter, padRight
  pane_lists.go   renderListPane, list-pane key handlers
  pane_repos.go   renderRepoPane, repo-pane key handlers
  pane_preview.go renderPreviewPane
  modal.go        modal struct, all modal types and rendering
  messages.go     all Msg types and Cmd constructors
  sort.go         sort comparators
  keys.go         key bindings
  styles.go       shared styles
  search.go       (or import internal/search)
  model_test.go   root model tests
  layout_test.go  layout geometry tests
  modal_test.go   modal interaction tests
```

This split is mechanical -- no behavior change. Do it as the first PR of v2.

**Cleanup items to bundle with the split:**
- Inline or remove backward-compat style aliases (`styleFaint`, `styleSelected`
  in `styles.go`) — they were kept to let v1.4 phases compile independently.
- Move `truncateToWidth` helper (added in v1.4 for the preview pane) into a
  shared utility location rather than leaving it at the top of `model.go`.
- `repoPaneH()` is now `max(1, m.height-2)` — identical to the inline pane
  height used elsewhere. Either remove the helper or restore meaningful
  semantics when v1.5 adds per-pane heading rows.

## Non-goals for v2

- YAML config. Do not add `config.yaml`, `--no-config`, or config loading.
- Themes. Do not add light/dark theme selection or theme-aware rendering.
- Configurable keybindings. Keep keybindings in code and document them in help.
- Disk cache. Defer until the v1.5 session cache proves insufficient in practice.
- Repo prefetch/performance work. Owned by v1.5, not v2.
- Multiple GitHub accounts or `--host` switching within a running TUI session.
- Plugin system or scripting hooks.
- Removing or deprecating any CLI subcommand.

## Test additions

- Default-switch: bare `gh star-lists` on TTY calls `runTUI`; with `--no-tui`
  falls through to CLI list output; with `--json` falls through regardless.
- Component split: layout, pane, modal, and root model tests keep existing
  behavior unchanged.

## Additional deferred items from v1.3

These items were observed during v1.3 and are added to this plan:

| Item | Rationale |
|---|---|
| Per-pane independent search state | `m.searchQuery` is shared. Ideal UX: list pane has its own query, repo pane has its own. Requires splitting `searchQuery`/`searchActive`/`listOffset`/`repoCursor` resets into per-pane state; touches `handleSearchKey`, `rebuildDisplayed`, and all reset sites. Non-trivial model refactor best done alongside the v2 component split. |
| Toast duration proportional to message length | Bulk failure toasts listing 3+ NWOs need longer than 2 s to read. Scale expiry: 2 s for simple messages, 4 s when `len(failedNWOs) > 0`. |
| Help overlay scrolling | Two-column help table exceeds terminal height on short windows (<20 rows). Add viewport scrolling (Up/Down while help is open) or paginate. Fits the v2 architecture split where help could become a proper sub-model. |
| `Right` key when repos are loading | `Right` is a no-op if `len(m.repos) == 0`, even if a load is in-flight. After v1.4's session cache lands, `Right` can move focus to the repo pane showing the in-flight spinner rather than silently doing nothing. |
| Mouse double-click detection | ~~Resolved in v1.4.~~ bubbletea v2.0.6 does not expose native double-click events; v1.4 worked around this with time-based tracking (`lastClickPane`, `lastClickIndex`, `lastClickTime` on the model, 300ms threshold). If a future bubbletea version exposes native double-click, replace the time-tracking fields with the native event. |

## Deferred items from v1.5

These were observed or measured during the v1.5 implementation and are worth
addressing in v2. Items marked **critical** have a user-visible bug or a
confirmed performance cost.

| Priority | Item | Detail |
|----------|------|--------|
| **critical** | `FilterRepositories` allocation profile | Bench measured 4,536 allocs/op at 500 repos (2.17 ms/op). The edit-distance DP in `internal/search` allocates per-repo. The `command` package reuses `tokenCache`, `editPrev`, `editCurr` buffers via `growIntSlice` (`internal/command/search.go`) — the same pattern should be applied inside `search.FilterRepositories` itself. Debounce threshold was not hit but allocations will matter at larger corpora. |
| **critical** | Double-click discards `focusList` cmd | `handleMouseClick` double-click calls `focusList` then immediately sets `active = paneRepo`, but the returned `tea.Cmd` is discarded. If the list was idle, no load is scheduled until the background preloader eventually reaches it — the repo pane shows a spinner longer than necessary. Fix: capture and return the cmd from `focusList` in the double-click branch. |
| high | Bulk partial-failure keeps modal open | When a bulk add/remove/move partially fails, the modal closes and shows a count toast. A better UX would keep the modal open showing `failedNWOs` (already on `bulkDoneMsg`) with a retry affordance. Requires a multi-line error view in `modal.go`. |
| high | `previewOffset` missing reset sites | `previewOffset` resets on list focus change but not when: repo cursor moves (Down in repo pane), sort order changes (repos reorder), search query changes (different visible repos), or `showPreview` toggles off then back on. Add `m.previewOffset = 0` at each state-change site. |
| medium | Error recovery from list-load failure | If `loadListsCmd` fails (network error or auth), the TUI shows a global error and is stuck. Add a `ctrl+r to retry` line to the error view and wire the refresh key to clear the error and retry. Auto-retry with exponential backoff would be a further step. |
| medium | In-flight load cancellation / debounced focus | The preloader promotes the focused list but does not cancel in-flight loads for other lists. Under rate limiting or large lists, a promoted load may wait 2 slots for uninteresting loads to finish. Options: (a) reduce the cap from 3 to 1 during rapid cursor movement (debounced focus intent via `tea.Tick`), or (b) track a `context.CancelFunc` per in-flight cmd and cancel on de-focus. |
| low | `withTopics=true` background preload | When preview is on, topics data for non-focused lists is never preloaded — each list focus triggers a fresh topics fetch. After all `withTopics=false` preloads finish, optionally kick off `withTopics=true` loads for visible lists, still bounded and lowest priority. |
| low | Disk cache (cold-start UX) | Session cache evaporates on exit. A simple on-disk JSON cache keyed by `(listID, withTopics, last-modified timestamp)` would make the browser feel instant on cold start. Deferred because v1.5 session cache must first prove insufficient in practice. |

## Verification

```
make check
make ascii-check
# Default switch: gh star-lists (TTY) opens TUI; gh star-lists --json still
#   prints JSON.
```
