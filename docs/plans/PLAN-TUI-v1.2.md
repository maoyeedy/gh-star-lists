# Plan: TUI v1.2 — Power user features

## Goal

Add the three power-user features that make the TUI faster for people who
manage large star lists: incremental fuzzy search, multi-select bulk ops,
and optional mouse support.

**Prerequisite:** TUI v1.1 shipped (mutations, preview, sort parity).

## Scope

| In | Out |
|---|---|
| Incremental fuzzy search (in-pane, `/` to activate) | YAML config / themes |
| Multi-select with `space` → bulk add/remove/move | Default bare-command TUI switch |
| Mouse: click to focus, scroll to navigate | New GitHub API calls |
| Viewport scrolling for long repo lists | Second TUI command variant |

## Fuzzy search

**Goal:** `/` activates a search bar at the top of the active pane. Typing
filters rows in real time. Esc clears and deactivates search.

**Implementation:**
- Add `searchActive bool` and `searchInput textinput.Model` to the root model.
- When active, key events route to `searchInput.Update` first; text changes
  re-filter the display slice.
- The **display slice** is separate from the backing `lists`/`repos` slices.
  Filter produces `filteredLists []githubapi.StarList` and
  `filteredRepos []githubapi.Repository` by running the search token algorithm.
- Reuse the fuzzy-token algorithm from `internal/command/search.go`. That
  file's `searchRepositories` and `searchStarLists` functions are unexported.
  Options:
  1. **Export** `SearchRepositories` / `SearchStarLists` from `command` —
     clean but expands the public API.
  2. **Duplicate** the algorithm into `internal/tui/search.go` — isolates the
     TUI; the algorithm is ~80 lines.
  3. **Extract** into a new shared `internal/search/` package that both
     `command` and `tui` import.
  Recommendation: **option 3** (new `internal/search/` package). It's the
  only option that avoids duplication without creating a `tui → command`
  import cycle. Move `search.go` logic there; update `command` to import it.
- Search applies **after** sort, on the already-sorted display slice.
  Clearing search restores the sorted order without re-fetching.
- Cursor resets to 0 on each keystroke.
- `/` key is reserved; do not activate search if a modal is open.

**Files:**
- New `internal/search/` package: `search.go` with exported
  `FilterRepositories(repos []githubapi.Repository, query string) []githubapi.Repository`
  and `FilterStarLists(lists []githubapi.StarList, query string) []githubapi.StarList`.
- Move token-search algorithm from `internal/command/search.go` into the new
  package; update `command.searchRepositories` / `command.searchStarLists` to
  delegate to it.
- `internal/tui/model.go`: add `searchActive`, `searchInput`,
  `filteredLists`, `filteredRepos`; wire `/` key; re-filter on input change.
- `internal/tui/keys.go`: add `Search` binding (`/`).

## Multi-select

**Goal:** `space` marks/unmarks the focused item in the repo pane. Marked
items are highlighted. When one or more items are marked, mutation keys
(`a`, `x`, `m`) act on all marked items instead of just the focused one.

**Implementation:**
- Add `selected map[int]bool` (or `map[string]bool` keyed by `NameWithOwner`)
  to the root model.
- `space` toggles current cursor item in/out of `selected`.
- When `len(selected) > 0` and a mutation key is pressed, the mutation
  payload is a slice: e.g., `addReposCmd(svc, []string{...nwos}, targetListID)`.
- Visual: selected rows get a `*` or `[x]` prefix (ASCII only).
- Esc clears `selected` (in addition to existing back/quit behavior).
- Multi-select only in repo pane. List pane has no multi-select in v1.2.
- After bulk mutation: refresh pane, clear `selected`, set toast with count
  ("3 repos moved.").

**New service calls pattern:** `UpdateRepositoryLists` is per-repo. Bulk ops
are a loop of N calls. Wrap in a single command that fans out with context
cancellation, similar to `command.loadMembershipIndex`:
```go
func bulkMutationCmd(ctx context.Context, svc githubapi.Service, items []string, fn func(string) error) tea.Cmd {
    return func() tea.Msg {
        var errs []error
        for _, item := range items {
            if ctx.Err() != nil { break }
            if err := fn(item); err != nil { errs = append(errs, err) }
        }
        return bulkDoneMsg{succeeded: len(items) - len(errs), failed: len(errs)}
    }
}
```
No `errgroup` for v1.2 (sequential is safer against rate limits); parallelize
in v2 if it becomes a bottleneck.

**Files:**
- `internal/tui/model.go`: add `selected`, `space` key handler, bulk mutation path.
- `internal/tui/messages.go`: add `bulkDoneMsg`.
- `internal/tui/keys.go`: add `Select` binding (`space`).

## Mouse support

**Goal:** click to move cursor; scroll wheel to scroll.

**Implementation:**
- Add `tea.WithMouseCellMotion()` to `tea.NewProgram(...)` in `app.go`.
  Only enable when `!opts.NoMouse` (add `NoMouse bool` to `Options`).
- Handle `tea.MouseMsg` in `Update`:
  - `tea.MouseButton1` click: compute which pane and which row was clicked,
    set cursor. If already focused row is clicked in list pane, drill in.
  - `tea.MouseWheelUp` / `tea.MouseWheelDown`: equivalent to `up`/`down`.
- Mouse row hit-testing: `model.listPaneTop` and `model.repoPaneTop` track
  the Y offset where each pane starts rendering rows (set during `View`).
  This requires storing layout geometry computed in `renderLayout`.
- Mouse is additive — all keyboard shortcuts remain unchanged.
- `--no-mouse` flag in `parse.go` maps to `Options.NoMouse`. Default: enabled
  on TTY (mouse capture can interfere with terminal selection; document this).

**Files:**
- `internal/tui/app.go`: add `NoMouse` to `Options`; conditional mouse option.
- `internal/tui/model.go`: add `listPaneTop`, `repoPaneTop`; handle
  `tea.MouseMsg`.
- `internal/command/parse.go`: add `--no-mouse` flag for `browse`.
- `internal/command/run.go`: thread `parsed.NoMouse` into `tui.Options`.

## Viewport scrolling

Current v1 renders only the top N rows that fit. For lists with hundreds of
repos, rows below the visible window are unreachable.

Use `charm.land/bubbles/v2/viewport` to make each pane scrollable:
- Replace the raw `strings.Join(lines, "\n")` pane renderers with a
  `viewport.Model` per pane.
- Viewport handles all scroll commands (`pgup`/`pgdn`, `home`/`end`).
- Keep the cursor highlight by rendering into the viewport content, not as
  a separate overlay.
- `pgup`/`pgdn`/`home`/`end` already in `keys.go` from v1 — hook them
  into viewport's `Update`.

**Files:**
- `internal/tui/model.go`: replace manual pane rendering with viewport models
  `listVP`, `repoVP viewport.Model`; update layout code.

## Test additions

- Search: filter reduces display slice; clear restores; cursor resets.
- Multi-select: space marks; second space unmarks; mutation uses all marked.
- Bulk mutation toast shows count.
- Mouse: click moves cursor to correct row; scroll moves cursor.
- Viewport: rows beyond visible height are reachable via pgdn.

## Verification

```
make check
make ascii-check
# Search: open browse, press /, type partial name, confirm filter.
# Multi-select: mark 3 repos, press a, pick a list, confirm 3 moved.
# Mouse: click a list row, verify it focuses.
# Scroll: pgdn in a list with 30+ repos, verify bottom rows visible.
```
