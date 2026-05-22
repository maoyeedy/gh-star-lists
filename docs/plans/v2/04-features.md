# Plan: TUI v2 - Nice-to-have UX

## Context

Collect focused, Star Lists-specific UX improvements inspired by `gh-dash` without turning this extension into a general GitHub dashboard. These items should make the TUI more intuitive, more discoverable, and faster to scan while staying within the current CLI capacity and existing `githubapi.Service` boundary.

**Prerequisite:** TUI v2 cleanup and performance plans provide stable component boundaries, per-pane state, contextual help, and improved preload/cache behavior.

## Scope

**In** | **Out**
---|---
More intuitive list/repo/preview scanning | PR, issue, notification, branch, or workflow dashboards
Better contextual affordances and status | YAML config, themes, configurable keybindings
Small TUI-only affordances backed by existing CLI actions | Plugin hooks, custom shell commands, scripting surface
Virtual TUI views for CLI-supported data like all starred/unlisted | New GitHub domain features outside Star Lists
Safer, clearer modal copy and confirmations | Changing machine output contracts

## Current state

- TUI has list/repo panes, optional repo preview, modals, search, sort, selection, bulk operations, mouse support, and browser open.
- Bulk operations (`bulkMutateReposCmd`) run concurrently via `errgroup.SetLimit(5)` with `atomic.Int64` result tracking.
- Single-repo mutations (`addRepoToListCmd`, `moveRepoCmd`, `removeRepoFromListCmd`) delegate to `githubapi.ModifyRepositoryMemberships`.
- `copyListCmd` uses `ModifyRepositoryMemberships` per-repo in an errgroup.
- `internal/humanize` package provides shared `ShortAge` formatting to both TUI and CLI.
- Column widths use a constant (`starWidth = 6`); per-frame width-caching infrastructure was removed as dead code.
- Sort enums use sentinel values (`sortListsEnd`, `sortReposEnd`); cycle bounds use `% int(sortXxxEnd)`.
- TUI launch wiring extracted into `launchTUI()` in `command/run.go`; the two call sites (explicit `browse` and list fallback) share it.
- Footer hints are useful but sparse and manually selected.
- Preview focuses on repositories only.
- Sort is cycle-only.
- Pick-list modals are not searchable.
- All starred and unlisted repositories exist in CLI but not as first-class TUI browsing entries.

## Design

- Favor immediate comprehension: visible counts, active pane cues, clear empty states, and action hints.
- Keep advanced features discoverable but not noisy.
- Avoid new config; choose conservative defaults in code.
- Prefer TUI affordances that reuse existing service methods and CLI behavior.
- Follow CLAUDE.md "When Planning New Features" section: shared before specialized, interface vs. package-level helper, concurrency-by-default for bulk, no speculative cache infrastructure, sentinel enum values, test concurrency safety.

## Phases

| Phase | Goal | Parallel-with | Depends-on | Files | Subagent |
|---|---|---|---|---|---|
| P1 - Scan polish | Improve visible structure, counts, empty states, and cursor/column readability | P2 | none | `internal/tui/render*.go`, `internal/tui/styles.go` | general-purpose |
| P2 - Preview affordances | Expand preview usefulness for list and repo focus without new API boundaries | P1 | none | `internal/tui/render_preview.go`, `internal/tui/input.go` | general-purpose |
| P3 - Modal ergonomics | Add search/filter and clearer copy to list pickers and destructive confirmations | none | P1 | `internal/tui/modal*.go` | general-purpose |
| P4 - Existing-capacity views | Add optional TUI entries for all starred and unlisted repos using existing service/CLI semantics | none | P2, P3 | `internal/tui/cache.go`, `internal/tui/render_list.go`, `internal/tui/messages.go` | general-purpose |

Phase order: (P1 || P2) -> P3 -> P4.

---

### P1 - Scan polish

Add lightweight visual cues that reduce guesswork: pane titles with counts, fixed repo owner alignment, repo header row, full-row active cursor highlight, faint separators, clearer active/inactive pane styling, and richer empty states for no lists, empty list, loading, error, and no search matches.

```text
Exit criteria:
- Rows remain aligned at narrow and wide widths.
- Footer and pane titles fit without wrapping.
- Existing render tests are updated for intentional visual changes.
```

### P2 - Preview affordances

Make preview useful in both panes. List focus should show list name, privacy, description, repo count, URL, and last-added date (using `humanize.ShortAge`). Repo focus should keep current repository details and add keyboard preview scrolling. Preview position can adapt to terminal width: right side when wide, bottom when narrow.

```text
Exit criteria:
- List pane preview does not require loading repository topics.
- Repo preview still uses `withTopics=true` only when preview data is needed.
- Narrow terminals keep list/repo panes usable.
```

### P3 - Modal ergonomics

Make list pickers searchable and make confirmations more explicit. Destination pickers should exclude impossible targets, show current selection context, and handle many lists. Destructive modals should state what will and will not happen, such as deleting a list without unstarring repos or merging then deleting the source list.

```text
Exit criteria:
- List picker search filters choices without losing cursor bounds.
- Delete, merge, unstar, and bulk modals include clear target names/counts.
- Modal tests cover search, empty picker results, and confirmation copy.
```

### P4 - Existing-capacity views

Expose TUI-only list entries for data the CLI already supports: all starred repositories and unlisted starred repositories. These should behave like read-only virtual lists where repo actions that make sense still work, but list edit/delete/copy/merge actions are disabled.

```text
Exit criteria:
- Virtual entries cannot be edited, deleted, copied, or merged as real lists.
- Repo open, search, sort, preview, selection, add-to-list, and unstar work where valid.
- Existing real Star List behavior is unchanged.
```

---

## Tests

| Test | What it covers |
|---|---|
| `TestPaneTitlesShowCounts` | Counts and titles fit across widths |
| `TestRepoRowsAlignOwnerAndHeader` | Owner/repo and column header alignment |
| `TestListPreviewContent` | List-focused preview renders metadata |
| `TestPreviewBottomLayoutNarrow` | Narrow terminal uses bottom preview without broken panes |
| `TestPickerSearch` | Modal list picker filters and preserves cursor bounds |
| `TestVirtualListActions` | Virtual all/unlisted entries disable invalid list actions |

## Verification

```text
go test ./internal/tui/...
make check
make ascii-check
```

Manual smoke:

```text
go run . browse --mouse
# - Resize from wide to narrow; preview remains usable.
# - Focus a list; preview shows list details.
# - Focus a repo; preview shows repo details and scrolls by keyboard/mouse.
# - Open add/move/copy pickers with many lists; search narrows choices.
# - Select virtual all/unlisted entries; invalid list actions are unavailable.
```

## Critical files

- `internal/tui/render.go` - adaptive layout and separators
- `internal/tui/render_preview.go` - list/repo preview content
- `internal/tui/modal_view.go` - picker and confirmation views
- `internal/tui/input.go` - action gating for real vs virtual lists

## Reused utilities

- `truncateToWidth`, `padRight`, `padLeft` - stable text fitting (TUI-side)
- `humanize.ShortAge(value, now)` - shared relative-age formatting used by both TUI and CLI
- `githubapi.ModifyRepositoryMemberships(ctx, svc, nwo, addIDs, removeIDs)` - shared membership mutation used by single-repo commands, copy, and bulk operations
- `lipgloss.Width` - visual width calculations
- Existing CLI concepts: `--all`, `--unlisted`, add/remove/move/unstar semantics
- `githubapi.Service` - all data and mutations stay behind the service boundary

## Out of scope

- New Star List API capabilities not already represented by the CLI
- User-defined layouts, themes, or keybindings
- Any broad `gh-dash` style PR/issue/notification workflow surface
