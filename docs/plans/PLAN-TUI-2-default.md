# Plan: TUI v2 — Default experience and architecture

## Goal

Make the TUI the default experience for interactive use while keeping the CLI
scripting surface intact. Split the TUI code into focused components after the
v1 cleanup and stability release.

## Scope

| In | Out |
|---|---|
| Per-pane independent search state | YAML config, themes, keybindings config |
| In-flight load cancellation | Multiple accounts / host switching |
| `withTopics=true` background preload | Plugin system or scripting hooks |
| Disk cache (cold-start UX) | Deprecating any CLI subcommand |


## Nice-to-have features / future refactors

### Per-pane independent search state

`m.searchQuery` is shared across both panes. Ideal UX: list pane has its own
query, repo pane has its own. Requires splitting `searchQuery`, `searchActive`,
`listOffset`, and `repoCursor` resets into per-pane state. Touches
`handleSearchKey`, `rebuildDisplayed`, and all reset/handler sites. Non-trivial
model refactor best done alongside the v2 file split so each pane owns its
state file.

### In-flight load cancellation / debounced focus

The preloader promotes the focused list but does not cancel in-flight loads for
other lists. Under rate limiting or large lists, a promoted load may wait 2
slots for uninteresting loads to finish. Options:
(a) reduce the cap from 3 to 1 during rapid cursor movement (debounced focus
    intent via `tea.Tick`), or
(b) track a `context.CancelFunc` per in-flight cmd and cancel on de-focus.

Requires per-cmd cancel func tracking. Not a bug fix — the current cap-3
preloader works correctly, just suboptimal under contention.

### `withTopics=true` background preload

When preview is on, topics data for non-focused lists is never preloaded — each
list focus triggers a fresh topics fetch. After all `withTopics=false` preloads
finish, optionally kick off `withTopics=true` loads for visible lists, still
bounded and lowest priority.

### Disk cache (cold-start UX)

Session cache evaporates on exit. A simple on-disk JSON cache keyed by
`(listID, withTopics, last-modified timestamp)` would make the browser feel
instant on cold start. Deferred until v1 session cache proves insufficient in
practice.

## Non-goals

- YAML config. Do not add `config.yaml`, `--no-config`, or config loadin.
- Themes. Do not add light/dark theme selection or theme-aware rendering.
- Configurable keybindings. Keep keybindings in code and document them in help.
- Multiple GitHub accounts or `--host` switching within a running TUI session.
- Plugin system or scripting hooks.
- Removing or deprecating any CLI subcommand.

## Test additions

- Component split: layout, pane, modal, and root model tests keep existing
  behavior unchanged.
- Per-pane search: list and repo search queries apply independently.
- Load cancellation: focused list promotion preempts lower-priority in-flight
  loads under cap-3 contention.
